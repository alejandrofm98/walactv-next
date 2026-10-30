package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type vodFixtureTransport struct {
	data         []byte
	active       atomic.Int32
	maxActive    atomic.Int32
	rangeSeen    atomic.Bool
	nonzeroRange atomic.Bool
	rangeMu      sync.Mutex
	ranges       []string
	closeEntered chan struct{}
	closeGate    <-chan struct{}
	statusCode   int
}

func (f *vodFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host != "provider.example" {
		return nil, fmt.Errorf("unexpected upstream host")
	}
	active := f.active.Add(1)
	for old := f.maxActive.Load(); active > old && !f.maxActive.CompareAndSwap(old, active); old = f.maxActive.Load() {
	}
	start, end := 0, len(f.data)-1
	status := f.statusCode
	if status == 0 {
		status = http.StatusOK
	}
	if status != http.StatusOK && status != http.StatusPartialContent {
		return &http.Response{StatusCode: status, Header: make(http.Header), ContentLength: 0, Body: &vodTrackedBody{Reader: strings.NewReader(""), onClose: func() { f.active.Add(-1) }}, Request: r}, nil
	}
	if value := r.Header.Get("Range"); value != "" {
		f.rangeSeen.Store(true)
		f.rangeMu.Lock()
		f.ranges = append(f.ranges, value)
		f.rangeMu.Unlock()
		var a, b int
		if strings.HasPrefix(value, "bytes=-") {
			var suffix int
			if _, err := fmt.Sscanf(value, "bytes=-%d", &suffix); err != nil {
				f.active.Add(-1)
				return nil, err
			}
			if suffix > len(f.data) {
				suffix = len(f.data)
			}
			a, b = len(f.data)-suffix, len(f.data)-1
		} else if _, err := fmt.Sscanf(value, "bytes=%d-%d", &a, &b); err != nil {
			if _, err = fmt.Sscanf(value, "bytes=%d-", &a); err != nil {
				return nil, err
			}
			b = len(f.data) - 1
		}
		if a > 0 {
			f.nonzeroRange.Store(true)
		}
		start, end, status = a, b, http.StatusPartialContent
		if start < 0 || start >= len(f.data) || end < start {
			f.active.Add(-1)
			return &http.Response{StatusCode: 416, Header: http.Header{"Content-Range": []string{fmt.Sprintf("bytes */%d", len(f.data))}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		}
		if end >= len(f.data) {
			end = len(f.data) - 1
		}
	}
	body := f.data[start : end+1]
	if r.Method == http.MethodHead {
		body = nil
	}
	header := http.Header{"Content-Type": []string{"video/x-matroska"}, "Content-Length": []string{strconv.Itoa(len(body))}, "Accept-Ranges": []string{"bytes"}}
	if status == http.StatusPartialContent {
		header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(f.data)))
	}
	return &http.Response{StatusCode: status, Header: header, ContentLength: int64(len(body)), Body: &vodTrackedBody{Reader: strings.NewReader(string(body)), onClose: func() {
		if f.closeEntered != nil {
			close(f.closeEntered)
		}
		if f.closeGate != nil {
			<-f.closeGate
		}
		f.active.Add(-1)
	}}, Request: r}, nil
}

type vodTrackedBody struct {
	io.Reader
	once    sync.Once
	onClose func()
}

func (b *vodTrackedBody) Close() error { b.once.Do(b.onClose); return nil }

type blockedMediaTransport struct {
	entered chan struct{}
	release <-chan struct{}
}

func (b *blockedMediaTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	close(b.entered)
	<-b.release // Deliberately ignore context cancellation to exercise the timeout path.
	body := io.NopCloser(strings.NewReader("data"))
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Length": []string{"4"}, "Content-Type": []string{"video/mp4"}}, ContentLength: 4, Body: body, Request: r}, nil
}

func TestVODFFmpegArgumentsStayOnFixedLocalInputAndBoundedProfile(t *testing.T) {
	args := buildVODFFmpegArgs("http://127.0.0.1:43210/input", "/var/lib/walactv-vod/job-private", "mkv", 123.5)
	joined := strings.Join(args, " ")
	for _, required := range []string{"-re", "-ss 123.500", "-f matroska", "http://127.0.0.1:43210/input", "libx264", "aac", "1280", "720", "-threads:v 2", "-hls_list_size 12"} {
		if !strings.Contains(joined, required) {
			t.Errorf("missing fixed FFmpeg option %q in %q", required, joined)
		}
	}
	for _, forbidden := range []string{"provider.example", "secret", "file,", "concat", "-y ", "shell"} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("unexpected value %q in FFmpeg arguments", forbidden)
		}
	}
	if got := buildVODFFmpegArgs("https://provider.example/private", "/var/lib/walactv-vod/job", "mkv", 0); got != nil {
		t.Fatal("accepted a non-loopback FFmpeg input")
	}
}

func TestVODManifestRewritesOnlyGeneratedSegmentNames(t *testing.T) {
	body := []byte("#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:4.0,\nsegment_000001.ts\n")
	got, err := rewriteVODManifest(body, "playback-id", "generation-id")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "/api/v1/playbacks/playback-id/transcode/generation-id/segment_000001.ts") || strings.Contains(string(got), "\nsegment_000001.ts\n") {
		t.Fatalf("segment was not privately rewritten: %s", got)
	}
	for _, invalid := range [][]byte{
		[]byte("#EXTM3U\n../../etc/passwd\n"),
		[]byte("#EXTM3U\nhttps://provider.example/secret.ts\n"),
		[]byte("#EXTM3U\nsegment_000001.ts.tmp\n"),
	} {
		if _, err := rewriteVODManifest(invalid, "p", "g"); err == nil {
			t.Errorf("accepted unsafe manifest reference %q", invalid)
		}
	}
	if validVODAsset("../index.m3u8") || validVODAsset("segment_000001.ts.tmp") || !validVODAsset("segment_000001.ts") || !validVODAsset("index.m3u8") {
		t.Fatal("VOD asset name validation is incorrect")
	}
}

func TestVODAssetsAreBoundToOwnerAndActiveGeneration(t *testing.T) {
	a := testAPI(nil)
	job := &vodJob{generation: "generation-a", dir: t.TempDir(), done: make(chan struct{})}
	job.lastUse.Store(time.Now().UnixNano())
	p := &playback{ID: "playback-a", UserID: "owner", SourceID: "source-a", ExpiresAt: time.Now().Add(time.Minute), VOD: job}
	a.private.playbacks[p.ID] = p
	for _, tc := range []struct {
		name, user, generation, asset string
		want                          int
	}{
		{"owner-current", "owner", "generation-a", "index.m3u8", http.StatusServiceUnavailable},
		{"other-user", "other", "generation-a", "index.m3u8", http.StatusNotFound},
		{"old-generation", "owner", "generation-old", "index.m3u8", http.StatusNotFound},
		{"traversal", "owner", "generation-a", "../index.m3u8", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := sessionRequest(http.MethodGet, "/api/v1/playbacks/playback-a/transcode/"+tc.generation+"/"+tc.asset, tc.user, nil)
			r.SetPathValue("id", "playback-a")
			r.SetPathValue("generation", tc.generation)
			r.SetPathValue("asset", tc.asset)
			w := httptest.NewRecorder()
			a.serveVODAsset(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d, want %d (%s)", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

func TestConcurrentVODCancelDuringPreparationIsSafe(t *testing.T) {
	a := testAPI(nil)
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	close(finished)
	p := &playback{ID: "playback-a", UserID: "owner", Ctx: context.Background(), VOD: &vodJob{generation: "pending", ctx: ctx, cancel: cancel, done: finished}}
	a.private.playbacks[p.ID] = p
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := sessionRequest(http.MethodDelete, "/api/v1/playbacks/playback-a/transcode", "owner", nil)
			r.SetPathValue("id", p.ID)
			w := httptest.NewRecorder()
			a.cancelVODTranscode(w, r)
			if w.Code != http.StatusNoContent {
				t.Errorf("cancel returned %d: %s", w.Code, w.Body.String())
			}
		}()
	}
	wg.Wait()
	a.private.mu.Lock()
	active := p.VOD != nil
	a.private.mu.Unlock()
	if active {
		t.Fatal("concurrent cancel left a preparation marker active")
	}
}

func TestDuplicateVODPostDuringPreparationReturnsPromptly(t *testing.T) {
	a := testAPI(nil)
	a.cfg.VODTranscodingEnabled = true
	p := &playback{ID: "playback-a", UserID: "owner", Format: "file", ExpiresAt: time.Now().Add(time.Minute), VOD: &vodJob{generation: "pending", offset: 5, pending: true}}
	a.private.playbacks[p.ID] = p
	r := sessionRequest(http.MethodPost, "/api/v1/playbacks/playback-a/transcode", "owner", strings.NewReader(`{"offsetSeconds":5}`))
	r.SetPathValue("id", p.ID)
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { a.startVODTranscode(w, r); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("duplicate POST waited indefinitely for a pending marker")
	}
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "TRANSCODER_PREPARING") {
		t.Fatalf("duplicate pending POST returned %d: %s", w.Code, w.Body.String())
	}
}

func TestShutdownRejectsNewTranscodes(t *testing.T) {
	a := testAPI(nil)
	a.cfg.VODTranscodingEnabled = true
	p := &playback{ID: "playback-a", UserID: "owner", Format: "file", ExpiresAt: time.Now().Add(time.Minute), Active: 1, Done: make(chan struct{})}
	a.private.playbacks[p.ID] = p
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- a.Shutdown(ctx) }()
	deadline := time.Now().Add(time.Second)
	for {
		a.private.mu.Lock()
		shuttingDown := a.private.shuttingDown
		a.private.mu.Unlock()
		if shuttingDown {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Shutdown did not enter the closing state")
		}
		time.Sleep(time.Millisecond)
	}
	r := sessionRequest(http.MethodPost, "/api/v1/playbacks/playback-a/transcode", "owner", strings.NewReader(`{"offsetSeconds":0}`))
	r.SetPathValue("id", p.ID)
	w := httptest.NewRecorder()
	a.startVODTranscode(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("transcode during shutdown returned %d: %s", w.Code, w.Body.String())
	}
	a.private.endMediaRequest(p.ID, p)
	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not complete after active playback drained")
	}
}

func TestDrainTimeoutKeepsDirectMediaBlockedUntilUpstreamFinishes(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	a := testAPI(&http.Client{Transport: &blockedMediaTransport{entered: entered, release: release}})
	a.cfg.VODTranscodingEnabled = true
	src := &source{ID: "source-a", UserID: "owner", ExpiresAt: time.Now().Add(time.Hour), AbsoluteExpiresAt: time.Now().Add(time.Hour), MaxConnections: 1}
	p := &playback{ID: "playback-a", UserID: "owner", SourceID: src.ID, Fingerprint: "account", Format: "file", RootAsset: "root", Assets: map[string]string{"root": "https://provider.example/movie.mp4"}, ExpiresAt: time.Now().Add(time.Hour), Ctx: context.Background(), Done: make(chan struct{})}
	p.MediaCtx, p.MediaCancel = context.WithCancel(p.Ctx)
	a.private.sources[src.ID] = src
	a.private.playbacks[p.ID] = p
	mediaRequest := sessionRequest(http.MethodGet, "/api/v1/media/playback-a/root", "owner", nil)
	mediaRequest.SetPathValue("playback", p.ID)
	mediaRequest.SetPathValue("asset", "root")
	mediaDone := make(chan struct{})
	go func() { w := httptest.NewRecorder(); a.proxyMedia(w, mediaRequest); close(mediaDone) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("direct upstream request did not start")
	}
	post := sessionRequest(http.MethodPost, "/api/v1/playbacks/playback-a/transcode", "owner", strings.NewReader(`{"offsetSeconds":0}`))
	post.SetPathValue("id", p.ID)
	postResponse := httptest.NewRecorder()
	started := time.Now()
	a.startVODTranscode(postResponse, post)
	if postResponse.Code != http.StatusConflict || time.Since(started) < 3*time.Second {
		close(release)
		t.Fatalf("drain timeout returned %d after %s: %s", postResponse.Code, time.Since(started), postResponse.Body.String())
	}
	a.private.mu.Lock()
	marker := p.VOD
	active := p.Active
	a.private.mu.Unlock()
	if marker == nil || !marker.pending || active != 1 {
		close(release)
		t.Fatalf("timeout reopened playback early: marker=%v active=%d", marker != nil, active)
	}
	blocked := sessionRequest(http.MethodGet, "/api/v1/media/playback-a/root", "owner", nil)
	blocked.SetPathValue("playback", p.ID)
	blocked.SetPathValue("asset", "root")
	blockedResponse := httptest.NewRecorder()
	a.proxyMedia(blockedResponse, blocked)
	if blockedResponse.Code != http.StatusConflict {
		close(release)
		t.Fatalf("direct media was not blocked during drain: %d", blockedResponse.Code)
	}
	close(release)
	select {
	case <-mediaDone:
	case <-time.After(time.Second):
		t.Fatal("upstream request did not finish after release")
	}
	select {
	case <-marker.done:
	case <-time.After(time.Second):
		t.Fatal("pending marker was not cleared after drain completed")
	}
	a.private.mu.Lock()
	active = p.Active
	vod := p.VOD
	mediaCtx := p.MediaCtx
	a.private.mu.Unlock()
	if active != 0 || vod != nil {
		t.Fatalf("playback did not recover after drain: active=%d vod=%v", active, vod != nil)
	}
	if err := mediaCtx.Err(); err != nil {
		t.Fatalf("direct media was not restored after drain: %v", err)
	}
}

func TestVODJobDoneWaitsForGatewayBodyClose(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg is supplied by the API image")
	}
	entered, release := make(chan struct{}), make(chan struct{})
	upstream := &vodFixtureTransport{data: []byte("not a video container"), closeEntered: entered, closeGate: release}
	a := testAPI(&http.Client{Transport: upstream})
	a.cfg.VODTempDir = t.TempDir()
	p := &playback{ID: "playback-close", UserID: "owner", SourceID: "source-close", Fingerprint: "account", Format: "file", RootAsset: "root", Assets: map[string]string{"root": "https://provider.example/vod/movie.mkv"}, ExpiresAt: time.Now().Add(time.Hour), Ctx: context.Background(), Done: make(chan struct{})}
	a.private.sources[p.SourceID] = &source{ID: p.SourceID, UserID: p.UserID, ExpiresAt: time.Now().Add(time.Hour)}
	a.private.playbacks[p.ID] = p
	a.private.vodSlots <- struct{}{}
	job, err := a.newVODJob(context.Background(), p, 0)
	if err != nil {
		t.Fatal(err)
	}
	a.private.mu.Lock()
	p.VOD = job
	p.Active++
	a.private.mu.Unlock()
	if err := a.startVODProcess(p, job); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		job.stop()
		close(release)
		t.Fatal("upstream body was not closed")
	}
	select {
	case <-job.done:
		t.Fatal("job completed before upstream Body.Close returned")
	default:
	}
	a.private.mu.Lock()
	active := p.Active
	a.private.mu.Unlock()
	if active != 1 {
		t.Fatalf("playback lease activity released while body close was blocked: %d", active)
	}
	if len(a.private.vodSlots) != 1 {
		t.Fatal("transcode slot released while body close was blocked")
	}
	deleteDone := make(chan struct{}, 2)
	for range 2 {
		go func() {
			r := sessionRequest(http.MethodDelete, "/api/v1/playbacks/playback-close/transcode", "owner", nil)
			r.SetPathValue("id", p.ID)
			w := httptest.NewRecorder()
			a.cancelVODTranscode(w, r)
			if w.Code != http.StatusNoContent {
				t.Errorf("concurrent DELETE returned %d: %s", w.Code, w.Body.String())
			}
			deleteDone <- struct{}{}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	a.private.mu.Lock()
	stillClosing := p.VOD == job
	a.private.mu.Unlock()
	if !stillClosing {
		t.Fatal("concurrent DELETE cleared the VOD marker before upstream body close")
	}
	mediaRequest := sessionRequest(http.MethodGet, "/api/v1/media/playback-close/root", "owner", nil)
	mediaRequest.SetPathValue("playback", p.ID)
	mediaRequest.SetPathValue("asset", "root")
	mediaResponse := httptest.NewRecorder()
	a.proxyMedia(mediaResponse, mediaRequest)
	if mediaResponse.Code != http.StatusConflict {
		t.Fatalf("direct source reopened while conversion was closing: %d", mediaResponse.Code)
	}
	close(release)
	select {
	case <-job.done:
	case <-time.After(5 * time.Second):
		t.Fatal("job did not finish after upstream Body.Close returned")
	}
	if upstream.active.Load() != 0 {
		t.Fatalf("upstream request still active after job completion: %d", upstream.active.Load())
	}
	for range 2 {
		select {
		case <-deleteDone:
		case <-time.After(4 * time.Second):
			t.Fatal("concurrent DELETE did not finish")
		}
	}
}

func TestPlaybackDiagnosticRemainsReadableDuringVOD(t *testing.T) {
	a := testAPI(nil)
	src := &source{ID: "source-a", UserID: "owner", ExpiresAt: time.Now().Add(time.Minute)}
	p := &playback{ID: "playback-a", UserID: "owner", SourceID: src.ID, ExpiresAt: time.Now().Add(time.Minute), VOD: &vodJob{generation: "g"}, Diagnostic: PlaybackDiagnostic{Cause: diagnosticNone}}
	a.private.sources[src.ID] = src
	a.private.playbacks[p.ID] = p
	r := sessionRequest(http.MethodGet, "/api/v1/playbacks/playback-a/diagnostic", "owner", nil)
	r.SetPathValue("id", p.ID)
	w := httptest.NewRecorder()
	a.playbackDiagnostic(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("diagnostic returned %d: %s", w.Code, w.Body.String())
	}
}

func TestVODProcessFailureRestoresDirectPlayback(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg is supplied by the API image")
	}
	upstream := &vodFixtureTransport{data: []byte("not a video container"), statusCode: http.StatusForbidden}
	a := testAPI(&http.Client{Transport: upstream})
	a.cfg.VODTempDir = t.TempDir()
	now := time.Now()
	src := &source{ID: "source-vod", UserID: "owner", Fingerprint: "account", ExpiresAt: now.Add(time.Hour)}
	p := &playback{ID: "playback-failure", UserID: "owner", SourceID: src.ID, Fingerprint: src.Fingerprint, Format: "file", RootAsset: "root", Assets: map[string]string{"root": "https://provider.example/vod/movie.mkv"}, ExpiresAt: now.Add(time.Hour), Ctx: context.Background(), Done: make(chan struct{})}
	p.MediaCtx, p.MediaCancel = context.WithCancel(p.Ctx)
	a.private.sources[src.ID] = src
	a.private.playbacks[p.ID] = p
	a.private.vodSlots <- struct{}{}
	job, err := a.newVODJob(context.Background(), p, 0)
	if err != nil {
		t.Fatal(err)
	}
	a.private.mu.Lock()
	p.VOD = job
	p.Active++
	a.private.mu.Unlock()
	if err := a.startVODProcess(p, job); err != nil {
		t.Fatal(err)
	}
	select {
	case <-job.done:
	case <-time.After(5 * time.Second):
		job.stop()
		t.Fatal("invalid input did not terminate FFmpeg")
	}
	a.private.mu.Lock()
	activeJob := p.VOD
	activeRequests := p.Active
	mediaCtx := p.MediaCtx
	a.private.mu.Unlock()
	if activeJob != nil || activeRequests != 0 {
		t.Fatalf("failed conversion left state active: job=%v requests=%d", activeJob != nil, activeRequests)
	}
	if p.Diagnostic.Cause != diagnosticUpstreamStatus || p.Diagnostic.UpstreamStatus != http.StatusForbidden {
		t.Fatalf("upstream failure was not exposed as a safe playback diagnostic: %+v", p.Diagnostic)
	}
	if err := mediaCtx.Err(); err != nil {
		t.Fatalf("direct media context was not restored: %v", err)
	}
	r := sessionRequest(http.MethodGet, "/api/v1/media/playback-failure/root", "owner", nil)
	upstream.statusCode = http.StatusOK
	r.SetPathValue("playback", p.ID)
	r.SetPathValue("asset", "root")
	w := httptest.NewRecorder()
	a.proxyMedia(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("direct retry remained blocked after transcoder failure: %d %s", w.Code, w.Body.String())
	}
}

func TestVODGatewayTranscodesMatroskaFixtureAndSeeksWithoutParallelUpstream(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is supplied by the API image")
	}
	fixtureRoot := t.TempDir()
	fixture := filepath.Join(fixtureRoot, "fixture.mkv")
	generate := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=160x90:rate=10", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "7", "-threads", "1", "-c:v", "libx265", "-x265-params", "pools=1:frame-threads=1:log-level=error", "-pix_fmt", "yuv420p", "-c:a", "eac3", "-b:a", "96k", "-f", "matroska", "-y", fixture)
	if output, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("could not create HEVC/EAC3 Matroska fixture: %v (%s)", err, output)
	}
	fixtureBytes, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	upstream := &vodFixtureTransport{data: fixtureBytes}
	a := testAPI(&http.Client{Transport: upstream})
	a.cfg.VODTempDir = t.TempDir()
	p := &playback{ID: "playback-vod", UserID: "owner", SourceID: "source-vod", Fingerprint: "account", Format: "file", RootAsset: "root", Assets: map[string]string{"root": "https://provider.example/vod/secret/movie.mkv"}, ExpiresAt: time.Now().Add(time.Hour), Ctx: context.Background(), Done: make(chan struct{})}
	a.private.playbacks[p.ID] = p
	startAndInspect := func(offset float64) *vodJob {
		a.private.vodSlots <- struct{}{}
		job, err := a.newVODJob(context.Background(), p, offset)
		if err != nil {
			<-a.private.vodSlots
			t.Fatal(err)
		}
		job.stopParent = context.AfterFunc(p.Ctx, job.cancel)
		a.private.mu.Lock()
		p.VOD = job
		p.Active++
		a.private.mu.Unlock()
		if err := a.startVODProcess(p, job); err != nil {
			t.Fatal("start FFmpeg fixture:", err)
		}
		select {
		case <-job.started:
		case <-job.done:
			t.Fatal("FFmpeg exited before its first HLS segment")
		case <-time.After(12 * time.Second):
			job.cancel()
			t.Fatal("timed out waiting for first HLS segment")
		}
		manifestRequest := sessionRequest(http.MethodGet, "/api/v1/playbacks/playback-vod/transcode/"+job.generation+"/index.m3u8", "owner", nil)
		manifestRequest.SetPathValue("id", p.ID)
		manifestRequest.SetPathValue("generation", job.generation)
		manifestRequest.SetPathValue("asset", "index.m3u8")
		manifestResponse := httptest.NewRecorder()
		a.serveVODAsset(manifestResponse, manifestRequest)
		if manifestResponse.Code != http.StatusOK || manifestResponse.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("private manifest failed: %d %s", manifestResponse.Code, manifestResponse.Body.String())
		}
		if strings.Contains(manifestResponse.Body.String(), "provider.example") || strings.Contains(manifestResponse.Body.String(), "secret") || !strings.Contains(manifestResponse.Body.String(), "/api/v1/playbacks/") {
			t.Fatalf("manifest leaked or omitted its private segment URL: %s", manifestResponse.Body.String())
		}
		segment := ""
		for _, line := range strings.Split(manifestResponse.Body.String(), "\n") {
			if strings.HasSuffix(line, ".ts") {
				segment = line[strings.LastIndex(line, "/")+1:]
			}
		}
		if segment == "" {
			t.Fatalf("manifest has no complete segment: %s", manifestResponse.Body.String())
		}
		segmentRequest := sessionRequest(http.MethodGet, "/api/v1/playbacks/playback-vod/transcode/"+job.generation+"/"+segment, "owner", nil)
		segmentRequest.SetPathValue("id", p.ID)
		segmentRequest.SetPathValue("generation", job.generation)
		segmentRequest.SetPathValue("asset", segment)
		segmentResponse := httptest.NewRecorder()
		a.serveVODAsset(segmentResponse, segmentRequest)
		if segmentResponse.Code != http.StatusOK || segmentResponse.Header().Get("Content-Type") != "video/mp2t" || segmentResponse.Body.Len() == 0 {
			t.Fatalf("private HLS segment failed: %d (%s)", segmentResponse.Code, segmentResponse.Body.String())
		}
		return job
	}
	first := startAndInspect(0)
	first.stop()
	select {
	case <-first.done:
	case <-time.After(4 * time.Second):
		t.Fatal("first FFmpeg process did not cancel")
	}
	second := startAndInspect(2)
	if !upstream.nonzeroRange.Load() {
		second.stop()
		<-second.done
		upstream.rangeMu.Lock()
		ranges := append([]string(nil), upstream.ranges...)
		upstream.rangeMu.Unlock()
		t.Fatalf("seek did not use a nonzero upstream byte range; observed %v", ranges)
	}
	second.stop()
	select {
	case <-second.done:
	case <-time.After(4 * time.Second):
		t.Fatal("seek FFmpeg process did not cancel")
	}
	mp4 := filepath.Join(fixtureRoot, "fixture-moov-at-end.mp4")
	generateMP4 := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=160x90:rate=10", "-f", "lavfi", "-i", "sine=frequency=660:sample_rate=48000", "-t", "7", "-threads", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", "96k", "-y", mp4)
	if output, err := generateMP4.CombinedOutput(); err != nil {
		t.Fatalf("could not create MP4 fixture with moov at end: %v (%s)", err, output)
	}
	upstream.data, err = os.ReadFile(mp4)
	if err != nil {
		t.Fatal(err)
	}
	upstream.nonzeroRange.Store(false)
	upstream.rangeMu.Lock()
	upstream.ranges = nil
	upstream.rangeMu.Unlock()
	p.Assets["root"] = "https://provider.example/vod/secret/movie.mp4"
	mp4Seek := startAndInspect(2)
	if !upstream.nonzeroRange.Load() {
		mp4Seek.stop()
		<-mp4Seek.done
		upstream.rangeMu.Lock()
		ranges := append([]string(nil), upstream.ranges...)
		upstream.rangeMu.Unlock()
		t.Fatalf("MP4 moov-at-end seek did not issue a byte range: %v", ranges)
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := a.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("API shutdown did not stop active FFmpeg: %v", err)
	}
	select {
	case <-mp4Seek.done:
	case <-time.After(time.Second):
		t.Fatal("API shutdown returned before FFmpeg cleanup")
	}
	if upstream.maxActive.Load() > 1 {
		t.Fatalf("gateway opened %d concurrent upstream requests", upstream.maxActive.Load())
	}
}
