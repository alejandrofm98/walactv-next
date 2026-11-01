package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	vodProbeTimeout     = 10 * time.Second
	vodProbeMaxBytes    = int64(16 << 20)
	vodProbeMaxJSON     = 256 << 10
	vodProbeMaxStreams  = 64
	vodProbeMaxRequests = 32
)

type vodTrack struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Language    string `json:"language,omitempty"`
	Label       string `json:"label"`
	Codec       string `json:"codec,omitempty"`
	Selectable  bool   `json:"selectable"`
	Unavailable string `json:"unavailable,omitempty"`
	streamIndex int
}

type vodTrackInventory struct {
	revision     string
	tracks       []vodTrack
	asset        string
	assetVersion string
	partial      bool
}

type vodTracksResponse struct {
	Status   string     `json:"status"`
	Revision string     `json:"revision,omitempty"`
	Tracks   []vodTrack `json:"tracks"`
	Reason   string     `json:"reason,omitempty"`
}

func (a *API) inspectVODTracks(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.VODTranscodingEnabled {
		writeError(w, http.StatusServiceUnavailable, "TRANSCODING_DISABLED", "La inspección de pistas de archivo no está activada")
		return
	}
	sess, id := requestSession(r), r.PathValue("id")
	a.private.mu.Lock()
	p := a.private.playbacks[id]
	if p == nil || p.UserID != sess.User.ID || p.Closing || time.Now().After(p.ExpiresAt) || p.Format != "file" {
		a.private.mu.Unlock()
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Playback no encontrado")
		return
	}
	if p.Tracks != nil && p.Tracks.asset == p.RootAsset && p.Tracks.assetVersion != "" && p.Tracks.assetVersion == p.RootValidator {
		response := tracksView(p.Tracks)
		a.private.mu.Unlock()
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, response)
		return
	}
	if p.VOD != nil {
		a.private.mu.Unlock()
		writeError(w, http.StatusConflict, "TRACKS_BUSY", "La conversión actual debe terminar antes de inspeccionar las pistas")
		return
	}
	markerCtx, markerCancel := context.WithCancel(context.Background())
	marker := &vodJob{generation: "probe", ctx: markerCtx, cancel: markerCancel, done: make(chan struct{}), started: make(chan struct{}), pending: true}
	p.VOD = marker
	if p.MediaCancel != nil {
		p.MediaCancel()
	}
	playDone := p.Done
	a.private.mu.Unlock()
	defer marker.finish()
	defer marker.cancel()
	if playDone != nil {
		select {
		case <-playDone:
			a.clearVODMarker(p, marker)
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Playback no encontrado")
			return
		default:
		}
	}
	if !a.waitPlaybackMediaDrain(p, 3*time.Second) {
		go a.finishVODTransitionAfterDrain(p, marker, nil)
		writeError(w, http.StatusConflict, "MEDIA_CLOSING", "La lectura anterior todavía se está cerrando")
		return
	}
	select {
	case a.private.vodSlots <- struct{}{}:
	default:
		a.clearVODMarker(p, marker)
		writeError(w, http.StatusConflict, "TRACKS_BUSY", "El recurso de vídeo está ocupado")
		return
	}
	defer func() { <-a.private.vodSlots }()

	requestCtx, cancelRequest := context.WithTimeout(r.Context(), vodProbeTimeout)
	defer cancelRequest()
	ctx, cancel := context.WithCancel(requestCtx)
	stopMarker := context.AfterFunc(marker.ctx, cancel)
	defer func() { stopMarker(); cancel() }()
	output, probeErr := a.runVODProbe(ctx, p, marker)
	a.private.mu.Lock()
	stillCurrent := a.private.playbacks[id] == p && !p.Closing && p.VOD == marker
	if stillCurrent && probeErr == nil {
		p.Tracks = &vodTrackInventory{revision: output.revision, tracks: output.tracks, asset: p.RootAsset, assetVersion: output.assetVersion, partial: output.partial}
		p.RootValidator = output.assetVersion
	}
	a.private.mu.Unlock()
	a.clearVODMarker(p, marker)
	if !stillCurrent {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Playback no encontrado")
		return
	}
	if probeErr != nil {
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, vodTracksResponse{Status: "partial", Tracks: []vodTrack{}, Reason: "probe_limited"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, tracksView(p.Tracks))
}

func tracksView(inventory *vodTrackInventory) vodTracksResponse {
	if inventory == nil {
		return vodTracksResponse{Status: "unavailable", Tracks: []vodTrack{}, Reason: "not_inspected"}
	}
	tracks := make([]vodTrack, len(inventory.tracks))
	copy(tracks, inventory.tracks)
	for i := range tracks {
		tracks[i].streamIndex = 0
	}
	status := "complete"
	if inventory.partial {
		status = "partial"
	}
	return vodTracksResponse{Status: status, Revision: inventory.revision, Tracks: tracks}
}

type ffprobeResult struct {
	Streams []struct {
		Index     int               `json:"index"`
		CodecType string            `json:"codec_type"`
		CodecName string            `json:"codec_name"`
		Tags      map[string]string `json:"tags"`
	} `json:"streams"`
}

type probeOutput struct {
	revision     string
	tracks       []vodTrack
	assetVersion string
	partial      bool
}

type vodCue struct {
	Start float64
	End   float64
	Text  string
}

func (a *API) readVODSubtitles(job *vodJob, pipe io.ReadCloser) {
	defer pipe.Close()
	reader := bufio.NewReaderSize(pipe, 32*1024)
	var block strings.Builder
	for {
		line, oversized, err := readBoundedCueLine(reader, 16*1024)
		if oversized || block.Len()+len(line) > 20*1024 {
			block.Reset()
		} else {
			block.WriteString(line)
		}
		if strings.TrimSpace(line) == "" {
			a.storeVODCueBlock(job, block.String())
			block.Reset()
		}
		if err != nil {
			if block.Len() > 0 {
				a.storeVODCueBlock(job, block.String())
			}
			return
		}
	}
}

func readBoundedCueLine(reader *bufio.Reader, limit int) (string, bool, error) {
	var line strings.Builder
	oversized := false
	for {
		fragment, err := reader.ReadSlice('\n')
		if !oversized {
			if line.Len()+len(fragment) > limit {
				oversized = true
				line.Reset()
			} else {
				line.Write(fragment)
			}
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		return line.String(), oversized, err
	}
}

func (a *API) storeVODCueBlock(job *vodJob, text string) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	timing := -1
	for i, line := range lines {
		if strings.Contains(line, "-->") {
			timing = i
			break
		}
	}
	if timing < 0 || timing+1 >= len(lines) {
		return
	}
	parts := strings.SplitN(strings.TrimSpace(lines[timing]), "-->", 2)
	if len(parts) != 2 {
		return
	}
	start, ok1 := parseVODTimestamp(strings.TrimSpace(parts[0]))
	endFields := strings.Fields(strings.TrimSpace(parts[1]))
	if len(endFields) == 0 {
		return
	}
	end, ok2 := parseVODTimestamp(endFields[0])
	if !ok1 || !ok2 || end <= start {
		return
	}
	var body strings.Builder
	for _, line := range lines[timing+1:] {
		line = stripVODMarkup(line)
		if line == "" {
			continue
		}
		if body.Len() > 0 {
			body.WriteByte('\n')
		}
		body.WriteString(line)
		if body.Len() > 16*1024 {
			return
		}
	}
	cueText := strings.TrimSpace(body.String())
	if cueText == "" {
		return
	}
	cue := vodCue{Start: job.offset + start, End: job.offset + end, Text: cueText}
	job.cueMu.Lock()
	defer job.cueMu.Unlock()
	job.cues = append(job.cues, cue)
	job.cueBytes += len(cue.Text)
	cutoff := cue.Start - 48
	for len(job.cues) > 0 && (len(job.cues) > 512 || job.cueBytes > 2<<20 || job.cues[0].End < cutoff) {
		job.cueBytes -= len(job.cues[0].Text)
		job.cues = job.cues[1:]
	}
}

func parseVODTimestamp(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	seconds, err := strconv.ParseFloat(parts[len(parts)-1], 64)
	if err != nil || seconds < 0 || seconds >= 60 {
		return 0, false
	}
	minutes, err := strconv.Atoi(parts[len(parts)-2])
	if err != nil || minutes < 0 || minutes >= 60 {
		return 0, false
	}
	total := float64(minutes)*60 + seconds
	if len(parts) == 3 {
		hours, err := strconv.Atoi(parts[0])
		if err != nil || hours < 0 || hours > 999 {
			return 0, false
		}
		total += float64(hours) * 3600
	}
	return total, true
}

func stripVODMarkup(value string) string {
	var output strings.Builder
	inside := false
	for _, r := range value {
		if r == '<' {
			inside = true
			continue
		}
		if r == '>' && inside {
			inside = false
			continue
		}
		if inside || r < 0x20 && r != '\t' || r == 0x7f {
			continue
		}
		output.WriteRune(r)
		if output.Len() > 16*1024 {
			break
		}
	}
	return strings.TrimSpace(output.String())
}

func (a *API) serveVODSubtitles(w http.ResponseWriter, r *http.Request) {
	sess, id, generation := requestSession(r), r.PathValue("id"), r.PathValue("generation")
	a.private.mu.Lock()
	p := a.private.playbacks[id]
	job := (*vodJob)(nil)
	if p != nil && p.UserID == sess.User.ID && !p.Closing && p.Format == "file" && p.VOD != nil && p.VOD.generation == generation && p.VOD.subtitleStream >= 0 {
		job = p.VOD
	}
	a.private.mu.Unlock()
	if job == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Subtítulos no encontrados")
		return
	}
	at := job.offset
	if value := r.URL.Query().Get("at"); value != "" {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil || parsed < 0 || parsed > 24*60*60 {
			writeError(w, 400, "INVALID_INPUT", "Posición de subtítulos no válida")
			return
		}
		at = parsed
	}
	job.lastUse.Store(time.Now().UnixNano())
	job.cueMu.Lock()
	cues := make([]vodCue, 0, len(job.cues))
	for _, cue := range job.cues {
		if cue.End >= at && cue.Start <= at+48 {
			cues = append(cues, cue)
		}
	}
	job.cueMu.Unlock()
	a.private.mu.Lock()
	stillCurrent := a.private.playbacks[id] == p && p.UserID == sess.User.ID && !p.Closing && p.VOD == job && job.generation == generation
	a.private.mu.Unlock()
	if !stillCurrent {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Subtítulos no encontrados")
		return
	}
	var output strings.Builder
	output.WriteString("WEBVTT\n\n")
	for i, cue := range cues {
		fmt.Fprintf(&output, "%d\n%s --> %s\n%s\n\n", i+1, formatVODTimestamp(cue.Start), formatVODTimestamp(cue.End), escapeVODVTT(cue.Text))
	}
	if output.Len() > 2<<20 {
		writeError(w, http.StatusRequestEntityTooLarge, "CUES_LIMIT", "La ventana de subtítulos supera el límite permitido")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.WriteString(w, output.String())
}

func formatVODTimestamp(seconds float64) string {
	ms := int64(seconds*1000 + 0.5)
	hours, remainder := ms/3_600_000, ms%3_600_000
	minutes, remainder := remainder/60_000, remainder%60_000
	wholeSeconds, millis := remainder/1000, remainder%1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, wholeSeconds, millis)
}

func escapeVODVTT(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	return strings.ReplaceAll(value, ">", "&gt;")
}

func (a *API) runVODProbe(parent context.Context, p *playback, marker *vodJob) (probeOutput, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return probeOutput{}, err
	}
	defer listener.Close()
	var total atomic.Int64
	var requests atomic.Int32
	var active atomic.Bool
	var partial atomic.Bool
	var validatorMu sync.Mutex
	assetVersion := ""
	validatorConflict := false
	secretPath := "/probe"
	inputURL := "http://" + listener.Addr().String() + secretPath
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != secretPath || r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		if requests.Add(1) > vodProbeMaxRequests {
			partial.Store(true)
			http.Error(w, "request limit", http.StatusTooManyRequests)
			return
		}
		if !active.CompareAndSwap(false, true) {
			http.Error(w, "busy", http.StatusTooManyRequests)
			return
		}
		defer active.Store(false)
		a.private.mu.Lock()
		current := a.private.playbacks[p.ID] == p && p.VOD == marker && !p.Closing
		upstream := p.Assets[p.RootAsset]
		a.private.mu.Unlock()
		if !current {
			http.Error(w, "gone", http.StatusGone)
			return
		}
		u, parseErr := url.Parse(upstream)
		if parseErr != nil || !validMediaURL(u) {
			http.Error(w, "unavailable", http.StatusBadGateway)
			return
		}
		ctx, cancel := context.WithCancel(r.Context())
		stop := context.AfterFunc(parent, cancel)
		defer func() { stop(); cancel() }()
		req, reqErr := http.NewRequestWithContext(ctx, r.Method, u.String(), nil)
		if reqErr != nil {
			http.Error(w, "unavailable", http.StatusBadGateway)
			return
		}
		if value := r.Header.Get("Range"); value != "" {
			req.Header.Set("Range", value)
		}
		resp, doErr := a.private.media.Do(req)
		if doErr != nil {
			http.Error(w, "unavailable", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			http.Error(w, "unavailable", http.StatusBadGateway)
			return
		}
		observedVersion := mediaResourceVersion(resp.Header)
		if observedVersion != "" {
			validatorMu.Lock()
			if assetVersion == "" {
				assetVersion = observedVersion
			} else if assetVersion != observedVersion {
				validatorConflict = true
			}
			validatorMu.Unlock()
		}
		for _, name := range []string{"Content-Type", "Content-Range", "Accept-Ranges"} {
			if value := resp.Header.Get(name); value != "" {
				w.Header().Set(name, value)
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(resp.StatusCode)
		if r.Method == http.MethodHead {
			return
		}
		remaining := vodProbeMaxBytes - total.Load()
		if remaining <= 0 {
			partial.Store(true)
			return
		}
		limited := &atomicLimitReader{r: resp.Body, remaining: remaining, total: &total, partial: &partial}
		_, _ = io.Copy(w, limited)
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second, IdleTimeout: 2 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	ctx, cancel := context.WithTimeout(parent, vodProbeTimeout)
	defer cancel()
	args := []string{"-v", "error", "-protocol_whitelist", "http,tcp", "-probesize", strconv.FormatInt(vodProbeMaxBytes, 10), "-analyzeduration", "10000000", "-f", probeDemuxer(p.RootExtension()), "-show_entries", "stream=index,codec_type,codec_name:stream_tags=language,title", "-of", "json", inputURL}
	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	cmd.WaitDelay = 3 * time.Second
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=/tmp", "LANG=C"}
	cmd.Stderr = io.Discard
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return probeOutput{}, err
	}
	if err := cmd.Start(); err != nil {
		return probeOutput{}, err
	}
	data, readErr := io.ReadAll(io.LimitReader(stdout, vodProbeMaxJSON+1))
	waitErr := cmd.Wait()
	if len(data) > vodProbeMaxJSON || readErr != nil {
		return probeOutput{}, errors.New("probe failed or output limit exceeded")
	}
	var result ffprobeResult
	if err := json.Unmarshal(data, &result); err != nil || len(result.Streams) > vodProbeMaxStreams {
		return probeOutput{}, errors.New("probe metadata invalid or limited")
	}
	if waitErr != nil {
		partial.Store(true)
	}
	revision, err := randomToken(18)
	if err != nil {
		return probeOutput{}, err
	}
	tracks := make([]vodTrack, 0, len(result.Streams))
	for _, stream := range result.Streams {
		if stream.CodecType != "audio" && stream.CodecType != "subtitle" {
			continue
		}
		id, err := randomToken(18)
		if err != nil {
			return probeOutput{}, err
		}
		language := sanitizeTrackLabel(stream.Tags["language"])
		title := sanitizeTrackLabel(stream.Tags["title"])
		kind, selectable, reason := stream.CodecType, true, ""
		if kind == "audio" && !supportedAudioCodec(stream.CodecName) {
			selectable, reason = false, "audio_codec_unavailable"
		}
		if kind == "subtitle" && !textSubtitleCodec(stream.CodecName) {
			selectable, reason = false, "subtitle_not_text"
		}
		label := title
		if label == "" {
			label = language
		}
		if label == "" {
			label = fmt.Sprintf("Pista %d", len(tracks)+1)
		}
		tracks = append(tracks, vodTrack{ID: id, Type: kind, Language: language, Label: label, Codec: safeCodec(stream.CodecName), Selectable: selectable, Unavailable: reason, streamIndex: stream.Index})
	}
	validatorMu.Lock()
	version, conflict := assetVersion, validatorConflict
	validatorMu.Unlock()
	if conflict {
		tracks = nil
		partial.Store(true)
		version = ""
	}
	return probeOutput{revision: revision, tracks: tracks, assetVersion: version, partial: partial.Load()}, nil
}

func mediaResourceVersion(headers http.Header) string {
	etag := strings.TrimSpace(headers.Get("ETag"))
	modified := strings.TrimSpace(headers.Get("Last-Modified"))
	if len(etag) > 256 || len(modified) > 256 || etag == "" && modified == "" {
		return ""
	}
	return etag + "\x00" + modified
}

type atomicLimitReader struct {
	r         io.Reader
	remaining int64
	total     *atomic.Int64
	partial   *atomic.Bool
}

func (r *atomicLimitReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		r.partial.Store(true)
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.r.Read(p)
	r.remaining -= int64(n)
	r.total.Add(int64(n))
	if r.remaining == 0 {
		r.partial.Store(true)
	}
	return n, err
}

func probeDemuxer(ext string) string {
	switch strings.ToLower(ext) {
	case "mkv":
		return "matroska"
	case "mp4", "m4v", "mov":
		return "mov"
	case "avi":
		return "avi"
	case "ts":
		return "mpegts"
	default:
		return "mov"
	}
}
func textSubtitleCodec(codec string) bool {
	switch strings.ToLower(codec) {
	case "subrip", "ass", "ssa", "mov_text", "webvtt":
		return true
	default:
		return false
	}
}
func supportedAudioCodec(codec string) bool {
	codec = strings.ToLower(codec)
	switch codec {
	case "aac", "mp3", "ac3", "eac3", "opus", "vorbis", "flac", "alac", "dts", "truehd", "wmav1", "wmav2", "pcm_s16le", "pcm_s16be", "pcm_s24le", "pcm_s24be", "pcm_s32le", "pcm_s32be", "pcm_u8", "pcm_mulaw", "pcm_alaw":
		return true
	default:
		return false
	}
}
func sanitizeTrackLabel(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= 0x20 && r != 0x7f {
			b.WriteRune(r)
		}
		if b.Len() >= 128 {
			break
		}
	}
	return strings.TrimSpace(b.String())
}
func safeCodec(value string) string {
	if value == "" || len(value) > 32 {
		return ""
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return ""
		}
	}
	return value
}
