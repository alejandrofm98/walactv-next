package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	vodMaxOutputBytes = int64(256 << 20)
	vodStartupLimit   = 60 * time.Second
	vodStallLimit     = 30 * time.Second
	vodMaxLifetime    = 6 * time.Hour
	vodIdleLimit      = 60 * time.Second
)

type vodJob struct {
	generation      string
	offset          float64
	trackRevision   string
	audioTrackID    string
	subtitleTrackID string
	audioStream     int
	subtitleStream  int
	dir             string
	ctx             context.Context
	cancel          context.CancelFunc
	done            chan struct{}
	started         chan struct{}
	startOnce       sync.Once
	doneOnce        sync.Once
	inputURL        string
	inputSlots      chan struct{}
	lastUse         atomic.Int64
	failed          atomic.Bool
	bytes           atomic.Int64
	stopParent      func() bool
	pending         bool
	gatewayWG       sync.WaitGroup
	diagnosticMu    sync.Mutex
	failureCause    string
	upstreamStatus  atomic.Int32
	cueMu           sync.Mutex
	cues            []vodCue
	cueBytes        int
	subtitleWG      sync.WaitGroup
}

func (j *vodJob) stop() {
	if j != nil && j.cancel != nil {
		j.cancel()
	}
}

func (j *vodJob) finish() {
	if j != nil && j.done != nil {
		j.doneOnce.Do(func() { close(j.done) })
	}
}

func (j *vodJob) setFailureCause(cause string, status int) {
	if j == nil {
		return
	}
	j.diagnosticMu.Lock()
	if j.failureCause == "" || j.failureCause == diagnosticTranscoderFailed {
		j.failureCause = cause
		j.upstreamStatus.Store(int32(status))
	}
	j.diagnosticMu.Unlock()
}

func (j *vodJob) getFailureDiagnostic() PlaybackDiagnostic {
	j.diagnosticMu.Lock()
	defer j.diagnosticMu.Unlock()
	cause := j.failureCause
	if cause == "" {
		cause = diagnosticTranscoderFailed
	}
	return PlaybackDiagnostic{Cause: cause, UpstreamStatus: int(j.upstreamStatus.Load())}
}

type vodTranscodeView struct {
	PlaybackID      string  `json:"playbackId"`
	Generation      string  `json:"generation"`
	ManifestURL     string  `json:"manifestUrl"`
	OffsetSeconds   float64 `json:"offsetSeconds"`
	DurationSeconds float64 `json:"durationSeconds"`
	AudioTrackID    string  `json:"audioTrackId,omitempty"`
	SubtitleTrackID string  `json:"subtitleTrackId,omitempty"`
}

func (a *API) startVODTranscode(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.VODTranscodingEnabled {
		writeError(w, http.StatusServiceUnavailable, "TRANSCODING_DISABLED", "La conversión de vídeo no está activada")
		return
	}
	var input struct {
		OffsetSeconds   float64 `json:"offsetSeconds"`
		TrackRevision   string  `json:"trackRevision,omitempty"`
		AudioTrackID    string  `json:"audioTrackId,omitempty"`
		SubtitleTrackID string  `json:"subtitleTrackId,omitempty"`
	}
	if decodeJSON(w, r, &input) != nil || input.OffsetSeconds < 0 || input.OffsetSeconds > 24*60*60 {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "La posición solicitada no es válida")
		return
	}
	sess, id := requestSession(r), r.PathValue("id")
	a.private.mu.Lock()
	if a.private.shuttingDown {
		a.private.mu.Unlock()
		writeError(w, http.StatusServiceUnavailable, "SHUTTING_DOWN", "El servicio se está cerrando")
		return
	}
	p := a.private.playbacks[id]
	if p == nil || p.UserID != sess.User.ID || p.Closing || time.Now().After(p.ExpiresAt) || (p.Format != "file") {
		a.private.mu.Unlock()
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Playback no encontrado")
		return
	}
	audioStream, subtitleStream := -1, -1
	if input.AudioTrackID != "" || input.SubtitleTrackID != "" || input.TrackRevision != "" {
		if p.Tracks == nil || input.TrackRevision == "" || input.TrackRevision != p.Tracks.revision {
			a.private.mu.Unlock()
			writeError(w, http.StatusConflict, "TRACKS_STALE", "El inventario de pistas ha cambiado; vuelve a buscar las pistas")
			return
		}
		for _, track := range p.Tracks.tracks {
			if track.ID == input.AudioTrackID && track.Type == "audio" && track.Selectable {
				audioStream = track.streamIndex
			}
			if track.ID == input.SubtitleTrackID && track.Type == "subtitle" && track.Selectable {
				subtitleStream = track.streamIndex
			}
		}
		if input.AudioTrackID != "" && audioStream < 0 || input.SubtitleTrackID != "" && subtitleStream < 0 {
			a.private.mu.Unlock()
			writeError(w, http.StatusBadRequest, "INVALID_TRACK", "La pista solicitada no está disponible")
			return
		}
	}
	if p.VOD != nil && p.VOD.offset == input.OffsetSeconds && p.VOD.audioTrackID == input.AudioTrackID && p.VOD.subtitleTrackID == input.SubtitleTrackID && !p.VOD.failed.Load() {
		job := p.VOD
		if job.pending {
			a.private.mu.Unlock()
			writeError(w, http.StatusConflict, "TRANSCODER_PREPARING", "La conversión solicitada se está preparando")
			return
		}
		a.private.mu.Unlock()
		select {
		case <-job.started:
			writeJSON(w, http.StatusOK, a.vodView(p, job))
		case <-r.Context().Done():
		case <-time.After(vodStartupLimit):
			writeError(w, http.StatusGatewayTimeout, "TRANSCODER_TIMEOUT", "La versión compatible no se preparó a tiempo")
		}
		return
	}
	previous := p.VOD
	if previous != nil {
		previous.stop()
	}
	// A conversion is a mode transition. Prevent new direct reads while the
	// current playback requests drain; the same playback lease remains active.
	markerCtx, markerCancel := context.WithCancel(context.Background())
	marker := &vodJob{generation: "pending", offset: input.OffsetSeconds, ctx: markerCtx, cancel: markerCancel, started: make(chan struct{}), done: make(chan struct{}), pending: true}
	p.VOD = marker
	if p.MediaCancel != nil {
		p.MediaCancel()
	}
	playDone := p.Done
	a.private.mu.Unlock()
	finishMarker := true
	defer func() {
		if finishMarker {
			marker.finish()
		}
	}()
	if previous != nil {
		select {
		case <-previous.done:
		case <-time.After(3 * time.Second):
			finishMarker = false
			go a.finishVODTransitionAfterDrain(p, marker, previous)
			writeError(w, http.StatusConflict, "TRANSCODER_CLOSING", "La reproducción anterior todavía se está cerrando")
			return
		}
	}
	// Existing direct media requests are bound to the playback context. Revoke
	// them only for the mode switch and never create another provider lease.
	if playDone != nil {
		select {
		case <-playDone: // playback was concurrently removed
			a.clearVODMarker(p, marker)
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Playback no encontrado")
			return
		default:
		}
	}
	if !a.waitPlaybackMediaDrain(p, 3*time.Second) {
		finishMarker = false
		go a.finishVODTransitionAfterDrain(p, marker, nil)
		writeError(w, http.StatusConflict, "MEDIA_CLOSING", "La lectura anterior todavía se está cerrando")
		return
	}
	if marker.ctx.Err() != nil {
		a.clearVODMarker(p, marker)
		writeError(w, http.StatusConflict, "TRANSCODER_CLOSING", "La conversión se está cancelando")
		return
	}
	select {
	case a.private.vodSlots <- struct{}{}:
	default:
		a.clearVODMarker(p, marker)
		writeError(w, http.StatusConflict, "TRANSCODER_BUSY", "Ya hay otra conversión en curso")
		return
	}
	job, err := a.newVODJob(r.Context(), p, input.OffsetSeconds)
	if err != nil {
		<-a.private.vodSlots
		a.clearVODMarker(p, marker)
		writeError(w, http.StatusBadGateway, "TRANSCODER_UNAVAILABLE", "No se pudo preparar la versión compatible")
		return
	}
	job.trackRevision, job.audioTrackID, job.subtitleTrackID = input.TrackRevision, input.AudioTrackID, input.SubtitleTrackID
	job.audioStream, job.subtitleStream = audioStream, subtitleStream
	job.stopParent = context.AfterFunc(p.Ctx, job.cancel)
	a.private.mu.Lock()
	if a.private.shuttingDown || a.private.playbacks[id] != p || p.Closing || p.VOD != marker || marker.ctx.Err() != nil {
		a.private.mu.Unlock()
		job.stop()
		_ = os.RemoveAll(job.dir)
		if job.stopParent != nil {
			job.stopParent()
		}
		<-a.private.vodSlots
		writeError(w, http.StatusConflict, "PLAYBACK_CLOSING", "La reproducción se está cerrando")
		return
	}
	p.VOD = job
	p.Active++
	a.private.mu.Unlock()
	if err := a.startVODProcess(p, job); err != nil {
		job.cancel()
		_ = os.RemoveAll(job.dir)
		close(job.done)
		<-a.private.vodSlots
		a.private.endMediaRequest(id, p)
		a.clearVODMarker(p, job)
		writeError(w, http.StatusBadGateway, "TRANSCODER_FAILED", "No se pudo iniciar la conversión")
		return
	}
	select {
	case <-job.started:
		writeJSON(w, http.StatusOK, a.vodView(p, job))
	case <-job.done:
		writeError(w, http.StatusBadGateway, "TRANSCODER_FAILED", "El archivo no pudo convertirse a un formato compatible")
	case <-time.After(vodStartupLimit):
		job.failed.Store(true)
		job.cancel()
		writeError(w, http.StatusGatewayTimeout, "TRANSCODER_TIMEOUT", "La conversión no produjo vídeo a tiempo")
	}
}

// Keep the playback in VOD-pending mode after a drain timeout. A detached read
// may ignore context cancellation, so wait for its lease activity to finish
// before reopening direct media or permitting another conversion.
func (a *API) finishVODTransitionAfterDrain(p *playback, marker, previous *vodJob) {
	if previous != nil && previous.done != nil {
		<-previous.done
	}
	for {
		a.private.mu.Lock()
		current := a.private.playbacks[p.ID] == p && p.VOD == marker
		closing := p.Closing
		active := p.Active
		done := p.Done
		a.private.mu.Unlock()
		if !current || closing {
			a.clearVODMarker(p, marker)
			marker.finish()
			return
		}
		if active == 0 {
			a.clearVODMarker(p, marker)
			marker.finish()
			return
		}
		if done != nil {
			select {
			case <-done:
				a.clearVODMarker(p, marker)
				marker.finish()
				return
			default:
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (a *API) cancelVODTranscode(w http.ResponseWriter, r *http.Request) {
	sess, id := requestSession(r), r.PathValue("id")
	a.private.mu.Lock()
	p := a.private.playbacks[id]
	if p == nil || p.UserID != sess.User.ID {
		a.private.mu.Unlock()
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Playback no encontrado")
		return
	}
	job := p.VOD
	if job != nil {
		job.stop()
	}
	a.private.mu.Unlock()
	if job != nil {
		select {
		case <-job.done:
		case <-time.After(3 * time.Second):
			writeError(w, http.StatusGatewayTimeout, "TRANSCODER_CLOSING", "La conversión todavía se está cerrando")
			return
		}
		a.private.mu.Lock()
		if a.private.playbacks[id] == p && p.VOD == job {
			p.VOD = nil
		}
		a.private.mu.Unlock()
	}
	a.restoreDirectMedia(p)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) serveVODAsset(w http.ResponseWriter, r *http.Request) {
	sess := requestSession(r)
	id, generation, asset := r.PathValue("id"), r.PathValue("generation"), r.PathValue("asset")
	if !validVODAsset(asset) {
		writeError(w, 404, "NOT_FOUND", "Media not found")
		return
	}
	a.private.mu.Lock()
	p := a.private.playbacks[id]
	if p == nil || p.UserID != sess.User.ID || p.Closing || time.Now().After(p.ExpiresAt) || p.VOD == nil || p.VOD.generation != generation {
		a.private.mu.Unlock()
		writeError(w, 404, "NOT_FOUND", "Media not found")
		return
	}
	job := p.VOD
	if job.failed.Load() {
		a.private.mu.Unlock()
		writeError(w, 410, "TRANSCODER_FAILED", "La conversión ha terminado")
		return
	}
	job.lastUse.Store(time.Now().UnixNano())
	dir := job.dir
	a.private.mu.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if asset == "index.m3u8" {
		data, err := os.ReadFile(filepath.Join(dir, asset))
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "TRANSCODER_STARTING", "La conversión todavía se está preparando")
			return
		}
		if len(data) > 64*1024 {
			writeError(w, 502, "TRANSCODER_FAILED", "Manifiesto no válido")
			return
		}
		manifest, err := rewriteVODManifest(data, id, generation)
		if err != nil {
			writeError(w, 502, "TRANSCODER_FAILED", "Manifiesto no válido")
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write(manifest)
		return
	}
	file := filepath.Join(dir, asset)
	st, err := os.Stat(file)
	if err != nil || !st.Mode().IsRegular() || st.Size() > vodMaxOutputBytes {
		writeError(w, 404, "NOT_FOUND", "Media not found")
		return
	}
	w.Header().Set("Content-Type", "video/mp2t")
	http.ServeFile(w, r, file)
}

func (a *API) vodView(p *playback, job *vodJob) vodTranscodeView {
	return vodTranscodeView{PlaybackID: p.ID, Generation: job.generation, ManifestURL: fmt.Sprintf("/api/v1/playbacks/%s/transcode/%s/index.m3u8", p.ID, job.generation), OffsetSeconds: job.offset, DurationSeconds: 0, AudioTrackID: job.audioTrackID, SubtitleTrackID: job.subtitleTrackID}
}

func (a *API) clearVODMarker(p *playback, job *vodJob) {
	a.private.mu.Lock()
	if a.private.playbacks[p.ID] == p && p.VOD == job {
		p.VOD = nil
		if !p.Closing {
			p.MediaCtx, p.MediaCancel = context.WithCancel(p.Ctx)
		}
	}
	a.private.mu.Unlock()
}

func (a *API) newVODJob(parent context.Context, p *playback, offset float64) (*vodJob, error) {
	generation, err := randomToken(18)
	if err != nil {
		return nil, err
	}
	tempRoot := a.cfg.VODTempDir
	if tempRoot == "" {
		tempRoot = "/var/lib/walactv-vod"
	}
	dir, err := os.MkdirTemp(tempRoot, "job-")
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(dir, 0700)
	_ = parent // job lifetime follows playback/session, not the short POST request.
	ctx, cancel := context.WithTimeout(context.Background(), vodMaxLifetime)
	job := &vodJob{generation: generation, offset: offset, dir: dir, ctx: ctx, cancel: cancel, done: make(chan struct{}), started: make(chan struct{}), inputSlots: make(chan struct{}, 1)}
	job.lastUse.Store(time.Now().UnixNano())
	return job, nil
}

func (a *API) startVODProcess(p *playback, job *vodJob) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	var cueReader, cueWriter *os.File
	if job.subtitleStream >= 0 {
		cueReader, cueWriter, err = os.Pipe()
		if err != nil {
			_ = listener.Close()
			return err
		}
	}
	secretPath := "/input"
	job.inputURL = "http://" + listener.Addr().String() + secretPath
	inputServer := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		job.gatewayWG.Add(1)
		defer job.gatewayWG.Done()
		a.serveVODInput(w, r, p, job, secretPath)
	}), ReadHeaderTimeout: 3 * time.Second, IdleTimeout: 10 * time.Second}
	go func() { _ = inputServer.Serve(listener) }()
	args := buildVODFFmpegArgsWithTracks(job.inputURL, job.dir, p.RootExtension(), job.offset, job.audioStream, job.subtitleStream)
	cmd := exec.CommandContext(job.ctx, "ffmpeg", args...)
	if cueWriter != nil {
		cmd.ExtraFiles = []*os.File{cueWriter}
	}
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=/tmp", "LANG=C"}
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.WaitDelay = 3 * time.Second
	if err := cmd.Start(); err != nil {
		if cueReader != nil {
			_ = cueReader.Close()
		}
		if cueWriter != nil {
			_ = cueWriter.Close()
		}
		_ = inputServer.Close()
		_ = os.RemoveAll(job.dir)
		if job.stopParent != nil {
			job.stopParent()
		}
		return err
	}
	if cueWriter != nil {
		_ = cueWriter.Close()
		job.subtitleWG.Add(1)
		go func() { defer job.subtitleWG.Done(); a.readVODSubtitles(job, cueReader) }()
	}
	go func() {
		defer close(job.done)
		defer func() {
			job.cancel()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = inputServer.Shutdown(shutdownCtx)
			shutdownCancel()
			_ = inputServer.Close()
			_ = inputServer.Shutdown(context.Background())
			job.gatewayWG.Wait()
			job.subtitleWG.Wait()
			_ = os.RemoveAll(job.dir)
			if job.stopParent != nil {
				job.stopParent()
			}
			if job.failed.Load() {
				a.private.mu.Lock()
				if a.private.playbacks[p.ID] == p && p.VOD == job && !p.Closing {
					p.VOD = nil
					p.MediaCtx, p.MediaCancel = context.WithCancel(p.Ctx)
					p.Diagnostic = job.getFailureDiagnostic()
				}
				a.private.mu.Unlock()
			}
			a.private.endMediaRequest(p.ID, p)
			<-a.private.vodSlots
		}()
		wait := make(chan error, 1)
		go func() { wait <- cmd.Wait() }()
		started := time.Now()
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		lastProgress := time.Now()
		lastSize, lastModified := int64(0), time.Time{}
		processDone := false
		for {
			select {
			case err := <-wait:
				processDone = true
				if err != nil || !vodHasFirstSegment(job.dir) {
					job.setFailureCause(diagnosticTranscoderFailed, 0)
					job.failed.Store(true)
					return
				}
				job.startOnce.Do(func() { close(job.started) })
			case <-job.ctx.Done():
				job.cancel()
				if !processDone {
					<-wait
				}
				return
			case now := <-ticker.C:
				size, modified, _ := directoryState(job.dir)
				job.bytes.Store(size)
				if size > vodMaxOutputBytes {
					job.setFailureCause(diagnosticTranscoderFailed, 0)
					job.failed.Store(true)
					job.cancel()
					if !processDone {
						<-wait
					}
					return
				}
				if size != lastSize || !modified.Equal(lastModified) {
					lastProgress = now
					lastSize, lastModified = size, modified
				}
				if vodHasFirstSegment(job.dir) && !processDone {
					job.startOnce.Do(func() { close(job.started) })
				}
				if now.Sub(started) > vodStartupLimit && !vodHasFirstSegment(job.dir) || !processDone && now.Sub(lastProgress) > vodStallLimit {
					job.setFailureCause(diagnosticTranscoderFailed, 0)
					job.failed.Store(true)
					job.cancel()
					if !processDone {
						<-wait
					}
					return
				}
				if now.Sub(time.Unix(0, job.lastUse.Load())) > vodIdleLimit {
					job.failed.Store(true)
					job.cancel()
					if !processDone {
						<-wait
					}
					return
				}
			}
		}
	}()
	return nil
}

func (p *playback) RootExtension() string {
	if p == nil {
		return "mp4"
	}
	for _, ext := range []string{"mkv", "mp4", "avi", "ts"} {
		if strings.HasSuffix(strings.ToLower(p.Assets[p.RootAsset]), "."+ext) {
			return ext
		}
	}
	return "mp4"
}

func buildVODFFmpegArgs(inputURL, outputDir, ext string, offset float64) []string {
	return buildVODFFmpegArgsWithTracks(inputURL, outputDir, ext, offset, -1, -1)
}

func buildVODFFmpegArgsWithTracks(inputURL, outputDir, ext string, offset float64, audioStream, subtitleStream int) []string {
	demuxer := map[string]string{"mkv": "matroska", "mp4": "mov", "avi": "avi", "ts": "mpegts"}[ext]
	if demuxer == "" {
		demuxer = "mov"
	}
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-protocol_whitelist", "http,tcp", "-threads", "2", "-filter_threads", "1"}
	if strings.Contains(inputURL, "\x00") || !strings.HasPrefix(inputURL, "http://127.0.0.1:") {
		return nil
	}
	args = append(args, "-re")
	if !strings.HasSuffix(inputURL, "/input") || filepath.Base(outputDir) == "." || filepath.Base(outputDir) == string(filepath.Separator) {
		return nil
	}
	if offset > 0 {
		args = append(args, "-ss", strconv.FormatFloat(offset, 'f', 3, 64))
	}
	args = append(args, "-f", demuxer, "-seekable", "1", "-i", inputURL, "-map", "0:v:0?")
	if audioStream >= 0 {
		args = append(args, "-map", fmt.Sprintf("0:%d?", audioStream))
	} else {
		args = append(args, "-map", "0:a:0?")
	}
	args = append(args, "-dn", "-vf", "scale='min(1280,iw)':'min(720,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-crf", "23", "-maxrate", "2500k", "-bufsize", "5000k", "-threads:v", "2", "-c:a", "aac", "-ac", "2", "-b:a", "128k", "-threads:a", "1", "-force_key_frames", "expr:gte(t,n_forced*4)", "-f", "hls", "-hls_time", "4", "-hls_list_size", "12", "-hls_flags", "delete_segments+independent_segments+temp_file", "-hls_segment_filename", filepath.Join(outputDir, "segment_%06d.ts"), "-progress", "pipe:1", filepath.Join(outputDir, "index.m3u8"))
	if subtitleStream >= 0 {
		args = append(args, "-map", fmt.Sprintf("0:%d?", subtitleStream), "-c:s", "webvtt", "-flush_packets", "1", "-f", "webvtt", "pipe:3")
	}
	return args
}

func (a *API) serveVODInput(w http.ResponseWriter, r *http.Request, p *playback, job *vodJob, fixedPath string) {
	if r.URL.Path != fixedPath || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		http.NotFound(w, r)
		return
	}
	select {
	case job.inputSlots <- struct{}{}:
		defer func() { <-job.inputSlots }()
	case <-r.Context().Done():
		return
	case <-job.ctx.Done():
		http.Error(w, "gone", 410)
		return
	case <-time.After(2 * time.Second):
		http.Error(w, "busy", 429)
		return
	}
	a.private.mu.Lock()
	current := a.private.playbacks[p.ID] == p && p.VOD == job && !p.Closing
	upstream := p.Assets[p.RootAsset]
	a.private.mu.Unlock()
	if !current {
		http.Error(w, "gone", 410)
		return
	}
	u, err := url.Parse(upstream)
	if err != nil || !validMediaURL(u) {
		http.Error(w, "upstream unavailable", 502)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	stop := context.AfterFunc(job.ctx, cancel)
	defer func() { stop(); cancel() }()
	req, err := http.NewRequestWithContext(ctx, r.Method, u.String(), nil)
	if err != nil {
		http.Error(w, "upstream unavailable", 502)
		return
	}
	for _, h := range []string{"Range", "If-Range"} {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}
	resp, err := a.private.media.Do(req)
	if err != nil {
		job.setFailureCause(classifyMediaError(err), 0)
		http.Error(w, "upstream unavailable", 502)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 206 && resp.StatusCode != 416 {
		job.setFailureCause(diagnosticUpstreamStatus, resp.StatusCode)
		http.Error(w, "upstream unavailable", 502)
		return
	}
	for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	if r.Method != http.MethodHead && resp.StatusCode != 416 {
		if _, err := io.Copy(w, resp.Body); err != nil && job.ctx.Err() == nil {
			job.setFailureCause(classifyMediaError(err), 0)
		}
	}
}

func (a *API) stopVOD(job *vodJob) {
	if job != nil {
		job.cancel()
		select {
		case <-job.done:
		case <-time.After(3 * time.Second):
		}
	}
}

func validVODAsset(s string) bool {
	if s == "index.m3u8" {
		return true
	}
	if len(s) != 17 || !strings.HasPrefix(s, "segment_") || !strings.HasSuffix(s, ".ts") {
		return false
	}
	for _, r := range s[8:14] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func rewriteVODManifest(body []byte, id, generation string) ([]byte, error) {
	if len(body) > 64*1024 {
		return nil, errors.New("manifest too large")
	}
	text := strings.TrimSpace(string(body))
	if !strings.HasPrefix(text, "#EXTM3U") {
		return nil, errors.New("invalid manifest")
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if !validVODAsset(trim) {
			return nil, errors.New("invalid asset")
		}
		lines[i] = fmt.Sprintf("/api/v1/playbacks/%s/transcode/%s/%s", id, generation, trim)
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

func vodHasFirstSegment(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if validVODAsset(e.Name()) && e.Name() != "index.m3u8" {
			if st, err := e.Info(); err == nil && st.Size() > 0 && st.Mode().IsRegular() {
				return true
			}
		}
	}
	return false
}

func directoryState(dir string) (int64, time.Time, error) {
	var size int64
	var modified time.Time
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		size += info.Size()
		if info.ModTime().After(modified) {
			modified = info.ModTime()
		}
		return nil
	})
	return size, modified, err
}

func (a *API) waitPlaybackMediaDrain(p *playback, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		a.private.mu.Lock()
		active := p.Active
		a.private.mu.Unlock()
		if active == 0 {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func (a *API) restoreDirectMedia(p *playback) {
	a.private.mu.Lock()
	defer a.private.mu.Unlock()
	if a.private.playbacks[p.ID] != p || p.Closing || p.VOD != nil {
		return
	}
	if p.MediaCancel != nil {
		p.MediaCancel()
	}
	p.MediaCtx, p.MediaCancel = context.WithCancel(p.Ctx)
}
