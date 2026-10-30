package api

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

const (
	diagnosticNone                = "none"
	diagnosticUpstreamStatus      = "upstream_status"
	diagnosticTimeout             = "timeout"
	diagnosticDNSConnection       = "dns_connection"
	diagnosticTLS                 = "tls"
	diagnosticDestinationRejected = "destination_rejected"
	diagnosticRedirectLimit       = "redirect_limit"
	diagnosticInvalidResponse     = "invalid_response"
	diagnosticUnknown             = "unknown"
	diagnosticTranscoderFailed    = "transcoder_failed"
)

type mediaAttemptKey struct{}

type mediaAttempt struct {
	mu    sync.Mutex
	cause string
}

func (a *mediaAttempt) setCause(cause string) {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.cause = cause
	a.mu.Unlock()
}

func (a *mediaAttempt) getCause() string {
	if a == nil {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cause
}

type mediaPolicyError struct {
	cause string
}

func (e mediaPolicyError) Error() string { return "media request rejected by policy" }

type PlaybackView struct {
	ID         string    `json:"id"`
	URL        string    `json:"url"`
	Format     string    `json:"format"`
	Title      string    `json:"title"`
	ContentKey string    `json:"contentKey"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

func (a *API) createPlayback(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SourceID string `json:"sourceId"`
		Kind     string `json:"kind"`
		ItemID   string `json:"itemId"`
		SeriesID string `json:"seriesId"`
	}
	if decodeJSON(w, r, &input) != nil || !numericID(input.ItemID) || !(input.Kind == "live" || input.Kind == "movie" || input.Kind == "episode") {
		writeError(w, 400, "INVALID_INPUT", "Invalid playback request")
		return
	}
	sess := requestSession(r)
	src, ok, err := a.private.sourceFor(r.Context(), sess, input.SourceID)
	if !ok {
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "SOURCE_STORE_UNAVAILABLE", "Could not load source")
			return
		}
		writeError(w, 404, "NOT_FOUND", "Source not found")
		return
	}
	if err := a.private.ensureSourceActive(r.Context(), src); err != nil {
		if errors.Is(err, errSourceStore) {
			writeError(w, http.StatusServiceUnavailable, "SOURCE_STORE_UNAVAILABLE", "Could not validate saved source")
		} else {
			providerError(w, err)
		}
		return
	}
	var title, key, ext string
	if input.Kind == "episode" {
		if !numericID(input.SeriesID) {
			writeError(w, 400, "INVALID_INPUT", "Invalid episode request")
			return
		}
		data, err := a.private.episodes(r.Context(), src, input.SeriesID)
		if err != nil {
			providerError(w, err)
			return
		}
		for _, ep := range data.episodes[input.SeriesID] {
			if ep.ID == input.ItemID {
				title, key, ext = ep.Title, ep.ContentKey, ep.ContainerExtension
				break
			}
		}
	} else {
		var candidate CatalogItem
		var exists bool
		if a.cfg.DB != nil {
			candidate, exists, err = a.private.dbCatalogItem(r.Context(), src, input.Kind, input.ItemID)
			if err != nil {
				writeError(w, 503, "SOURCE_STORE_UNAVAILABLE", "Could not load catalog item")
				return
			}
		} else {
			candidate, exists = a.private.authorizedItem(src.ID, input.Kind, input.ItemID)
		}
		if !exists || a.cfg.DB == nil && !numericID(candidate.CategoryID) {
			writeError(w, 404, "NOT_FOUND", "Catalog item not found")
			return
		}
		if a.cfg.DB != nil {
			title, key, ext = candidate.Title, candidate.ContentKey, candidate.ContainerExtension
		} else {
			data, err := a.private.catalog(r.Context(), src, input.Kind, candidate.CategoryID)
			if err != nil {
				providerError(w, err)
				return
			}
			for _, item := range data.items[input.Kind+":"+candidate.CategoryID] {
				if item.ID == input.ItemID && item.Kind == input.Kind {
					title, key, ext = item.Title, item.ContentKey, item.ContainerExtension
					break
				}
			}
		}
	}
	if title == "" {
		writeError(w, 404, "NOT_FOUND", "Catalog item not found")
		return
	}
	if input.Kind == "live" {
		ext = "m3u8"
	}
	if !validPlaybackExtension(ext) {
		writeError(w, 415, "UNSUPPORTED_FORMAT", "Provider media format is not supported")
		return
	}
	upstream, err := providerMediaURL(src, input.Kind, input.ItemID, ext)
	if err != nil {
		writeError(w, 502, "UPSTREAM_ERROR", "Provider request failed")
		return
	}
	rootURL, err := url.Parse(upstream)
	if err != nil || !a.private.allowedHost(rootURL.Hostname()) || !validMediaURL(rootURL) {
		writeError(w, 502, "UPSTREAM_ERROR", "Provider request failed")
		return
	}
	format := "file"
	if ext == "m3u8" {
		format = "hls"
	}
	id, err := randomToken(18)
	if err != nil {
		writeError(w, 500, "INTERNAL", "Could not create playback")
		return
	}
	asset, err := randomToken(18)
	if err != nil {
		writeError(w, 500, "INTERNAL", "Could not create playback")
		return
	}
	playCtx, cancel := context.WithCancel(context.Background())
	mediaCtx, mediaCancel := context.WithCancel(playCtx)
	now := time.Now()
	p := &playback{ID: id, UserID: sess.User.ID, SourceID: src.ID, Fingerprint: src.Fingerprint, ExpiresAt: minTime(now.Add(30*time.Second), minTime(sess.Absolute, src.AbsoluteExpiresAt)), Title: title, ContentKey: key, Format: format, RootAsset: asset, Assets: map[string]string{asset: upstream}, AssetByURL: map[string]string{upstream: asset}, AssetUsed: map[string]time.Time{asset: now}, Ctx: playCtx, Cancel: cancel, Done: make(chan struct{}), MediaCtx: mediaCtx, MediaCancel: mediaCancel}
	a.private.storeMu.Lock()
	checkCtx, checkCancel := context.WithTimeout(r.Context(), 5*time.Second)
	if err := a.private.checkSourceSession(checkCtx, sess); err != nil {
		checkCancel()
		a.private.storeMu.Unlock()
		cancel()
		if errors.Is(err, errSourceSessionRevoked) {
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		} else {
			writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE", "Could not verify session")
		}
		return
	}
	checkCancel()
	a.private.mu.Lock()
	if a.private.shuttingDown {
		a.private.mu.Unlock()
		a.private.storeMu.Unlock()
		cancel()
		writeError(w, http.StatusServiceUnavailable, "SHUTTING_DOWN", "Service is shutting down")
		return
	}
	for existingID, existing := range a.private.playbacks {
		if time.Now().After(existing.ExpiresAt) {
			a.private.releasePlaybackLocked(existingID)
		}
	}
	if a.private.sources[src.ID] != src || time.Now().After(src.ExpiresAt) {
		a.private.mu.Unlock()
		a.private.storeMu.Unlock()
		cancel()
		writeError(w, 404, "NOT_FOUND", "Source not found")
		return
	}
	active := a.private.leases[src.Fingerprint]
	if len(active) >= src.MaxConnections {
		a.private.mu.Unlock()
		a.private.storeMu.Unlock()
		cancel()
		writeError(w, 409, "PLAYBACK_LIMIT", "This provider account is already in use")
		return
	}
	if active == nil {
		active = map[string]struct{}{}
		a.private.leases[src.Fingerprint] = active
	}
	active[id] = struct{}{}
	a.private.playbacks[id] = p
	a.private.mu.Unlock()
	a.private.storeMu.Unlock()
	writeJSON(w, 201, PlaybackView{id, "/api/v1/media/" + id + "/" + asset, format, title, key, p.ExpiresAt})
}

func (a *API) heartbeatPlayback(w http.ResponseWriter, r *http.Request) {
	sess := requestSession(r)
	id := r.PathValue("id")
	a.private.mu.Lock()
	defer a.private.mu.Unlock()
	p := a.private.playbacks[id]
	if p == nil || p.UserID != sess.User.ID || p.Closing || time.Now().After(p.ExpiresAt) {
		writeError(w, 404, "NOT_FOUND", "Playback not found")
		return
	}
	src := a.private.sources[p.SourceID]
	if src == nil || src.UserID != sess.User.ID {
		a.private.releasePlaybackLocked(id)
		writeError(w, 404, "NOT_FOUND", "Playback not found")
		return
	}
	now := time.Now()
	p.ExpiresAt = minTime(now.Add(30*time.Second), minTime(sess.Absolute, src.AbsoluteExpiresAt))
	src.Activity = now
	src.ExpiresAt = minTime(now.Add(30*time.Minute), minTime(sess.Absolute, src.AbsoluteExpiresAt))
	w.WriteHeader(204)
}

func (a *API) playbackDiagnostic(w http.ResponseWriter, r *http.Request) {
	sess := requestSession(r)
	id := r.PathValue("id")
	w.Header().Set("Cache-Control", "no-store")
	a.private.mu.Lock()
	defer a.private.mu.Unlock()
	p := a.private.playbacks[id]
	if p == nil || p.UserID != sess.User.ID || p.Closing || time.Now().After(p.ExpiresAt) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Playback not found")
		return
	}
	src := a.private.sources[p.SourceID]
	if src == nil || src.UserID != sess.User.ID || time.Now().After(src.ExpiresAt) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Playback not found")
		return
	}
	diagnostic := p.Diagnostic
	if diagnostic.Cause == "" {
		diagnostic.Cause = diagnosticNone
	}
	writeJSON(w, http.StatusOK, diagnostic)
}

func (a *API) deletePlayback(w http.ResponseWriter, r *http.Request) {
	sess := requestSession(r)
	id := r.PathValue("id")
	a.private.mu.Lock()
	p := a.private.playbacks[id]
	if p != nil && p.UserID != sess.User.ID {
		a.private.mu.Unlock()
		writeError(w, 404, "NOT_FOUND", "Playback not found")
		return
	}
	if p != nil {
		a.private.releasePlaybackLocked(id)
	}
	a.private.mu.Unlock()
	if p != nil && p.Done != nil {
		select {
		case <-p.Done:
		case <-time.After(5 * time.Second):
			writeError(w, 504, "PLAYBACK_CLOSING", "Playback requests are still closing")
			return
		}
	}
	w.WriteHeader(204)
}

func (a *API) proxyMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Media method is not supported")
		return
	}
	sess := requestSession(r)
	id, assetID := r.PathValue("playback"), r.PathValue("asset")
	a.private.mu.Lock()
	p := a.private.playbacks[id]
	if p == nil || p.UserID != sess.User.ID || p.Closing || time.Now().After(p.ExpiresAt) {
		a.private.mu.Unlock()
		writeError(w, 404, "NOT_FOUND", "Media not found")
		return
	}
	if p.VOD != nil {
		a.private.mu.Unlock()
		writeError(w, http.StatusConflict, "TRANSCODING_ACTIVE", "La reproducción está usando la versión compatible")
		return
	}
	src := a.private.sources[p.SourceID]
	upstream := p.Assets[assetID]
	if src == nil || src.UserID != sess.User.ID || time.Now().After(src.ExpiresAt) || upstream == "" {
		a.private.mu.Unlock()
		writeError(w, 404, "NOT_FOUND", "Media not found")
		return
	}
	p.Active++
	if p.AssetUsed != nil {
		p.AssetUsed[assetID] = time.Now()
	}
	if assetID == p.RootAsset {
		p.DiagnosticGeneration++
		p.Diagnostic = PlaybackDiagnostic{Cause: diagnosticNone}
	}
	diagnosticGeneration := p.DiagnosticGeneration
	src.Activity = time.Now()
	src.ExpiresAt = minTime(time.Now().Add(30*time.Minute), src.AbsoluteExpiresAt)
	playCtx := p.MediaCtx
	if playCtx == nil {
		playCtx = p.Ctx
	}
	a.private.mu.Unlock()
	defer a.private.endMediaRequest(id, p)
	u, err := url.Parse(upstream)
	if err != nil || !validMediaURL(u) {
		a.setPlaybackDiagnostic(p, diagnosticGeneration, diagnosticDestinationRejected, 0)
		writeError(w, 502, "UPSTREAM_ERROR", "Provider request failed")
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	attempt := &mediaAttempt{}
	ctx = context.WithValue(ctx, mediaAttemptKey{}, attempt)
	stop := context.AfterFunc(playCtx, cancel)
	defer func() { stop(); cancel() }()
	req, err := http.NewRequestWithContext(ctx, r.Method, u.String(), nil)
	if err != nil {
		a.setPlaybackDiagnostic(p, diagnosticGeneration, diagnosticInvalidResponse, 0)
		writeError(w, 502, "UPSTREAM_ERROR", "Provider request failed")
		return
	}
	for _, header := range []string{"Range", "If-Range"} {
		if value := r.Header.Get(header); value != "" {
			req.Header.Set(header, value)
		}
	}
	client := a.private.media
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return
		}
		cause := attempt.getCause()
		if cause == "" {
			cause = classifyMediaError(err)
		}
		a.setPlaybackDiagnostic(p, diagnosticGeneration, cause, 0)
		writeError(w, 502, "UPSTREAM_ERROR", "Provider request failed")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		cause := attempt.getCause()
		if cause == "" {
			cause = diagnosticInvalidResponse
		}
		a.setPlaybackDiagnostic(p, diagnosticGeneration, cause, 0)
		writeError(w, 502, "UPSTREAM_ERROR", "Provider request failed")
		return
	}
	if resp.StatusCode != 200 && resp.StatusCode != 206 && resp.StatusCode != 416 {
		a.setPlaybackDiagnostic(p, diagnosticGeneration, diagnosticUpstreamStatus, resp.StatusCode)
		writeError(w, 502, "UPSTREAM_ERROR", "Provider request failed")
		return
	}
	copyMediaHeaders(w.Header(), resp.Header)
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", a.mediaContentType(upstream, ""))
	}
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		w.Header().Set("Content-Length", "0")
	}
	var stream io.Reader = resp.Body
	if p.Format == "hls" && r.Method == http.MethodGet && resp.StatusCode == http.StatusOK {
		manifestReader := bufio.NewReaderSize(resp.Body, 1024)
		prefix, _ := manifestReader.Peek(1024)
		stream = manifestReader
		isManifest := strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "mpegurl") || strings.HasSuffix(strings.ToLower(u.Path), ".m3u8") || isHLSManifestPrefix(prefix)
		if isManifest {
			body, err := io.ReadAll(io.LimitReader(manifestReader, (5<<20)+1))
			if err != nil {
				a.setPlaybackDiagnostic(p, diagnosticGeneration, classifyMediaError(err), 0)
				writeError(w, 502, "UPSTREAM_ERROR", "Provider request failed")
				return
			}
			if len(body) > 5<<20 {
				a.setPlaybackDiagnostic(p, diagnosticGeneration, diagnosticInvalidResponse, 0)
				writeError(w, 502, "UPSTREAM_ERROR", "Provider request failed")
				return
			}
			manifestText := strings.TrimSpace(strings.TrimPrefix(string(body), "\ufeff"))
			if !strings.HasPrefix(manifestText, "#EXTM3U") {
				a.setPlaybackDiagnostic(p, diagnosticGeneration, diagnosticInvalidResponse, 0)
				writeError(w, 502, "UPSTREAM_ERROR", "Provider request failed")
				return
			}
			manifestURL := u
			if resp.Request != nil && resp.Request.URL != nil {
				manifestURL = resp.Request.URL
			}
			rewritten, err := a.rewriteManifest(p, manifestURL, body)
			if err != nil {
				cause := diagnosticInvalidResponse
				if policyCause := classifyMediaError(err); policyCause == diagnosticDestinationRejected || policyCause == diagnosticRedirectLimit {
					cause = policyCause
				}
				a.setPlaybackDiagnostic(p, diagnosticGeneration, cause, 0)
				writeError(w, 502, "UPSTREAM_ERROR", "Provider request failed")
				return
			}
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Header().Del("Content-Length")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(200)
			_, _ = w.Write(rewritten)
			return
		}
	}
	w.WriteHeader(resp.StatusCode)
	if r.Method == http.MethodHead || resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		return
	}
	if _, err := io.Copy(w, stream); err != nil && r.Context().Err() == nil {
		a.setPlaybackDiagnostic(p, diagnosticGeneration, classifyMediaError(err), 0)
	}
}

func (a *API) setPlaybackDiagnostic(p *playback, generation uint64, cause string, upstreamStatus int) {
	if cause == "" {
		cause = diagnosticUnknown
	}
	a.private.mu.Lock()
	defer a.private.mu.Unlock()
	if current := a.private.playbacks[p.ID]; current == p && p.DiagnosticGeneration == generation && !p.Closing && time.Now().Before(p.ExpiresAt) {
		p.Diagnostic = PlaybackDiagnostic{Cause: cause, UpstreamStatus: upstreamStatus}
	}
}

func classifyMediaError(err error) string {
	if err == nil {
		return diagnosticUnknown
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return diagnosticTimeout
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return diagnosticInvalidResponse
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return diagnosticTimeout
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return diagnosticDNSConnection
	}
	var verifyErr *tls.CertificateVerificationError
	var unknownAuthority x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var certInvalid x509.CertificateInvalidError
	var recordErr tls.RecordHeaderError
	if errors.As(err, &verifyErr) || errors.As(err, &unknownAuthority) || errors.As(err, &hostnameErr) || errors.As(err, &certInvalid) || errors.As(err, &recordErr) {
		return diagnosticTLS
	}
	var policyErr mediaPolicyError
	if errors.As(err, &policyErr) {
		return policyErr.cause
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if strings.Contains(strings.ToLower(opErr.Op), "tls") || opErr.Err != nil && strings.Contains(strings.ToLower(opErr.Err.Error()), "tls") {
			return diagnosticTLS
		}
		return diagnosticDNSConnection
	}
	return diagnosticUnknown
}

func isHLSManifestPrefix(prefix []byte) bool {
	text := strings.TrimSpace(string(prefix))
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "#EXTM3U") {
		return true
	}
	for _, tag := range []string{"#EXT-X-STREAM-INF", "#EXT-X-I-FRAME-STREAM-INF", "#EXT-X-TARGETDURATION", "#EXT-X-MEDIA-SEQUENCE", "#EXT-X-KEY", "#EXT-X-MAP", "#EXTINF:"} {
		if strings.Contains(text, tag) {
			return true
		}
	}
	return false
}

func (s *privateState) newMediaClient() *http.Client {
	return &http.Client{Transport: &http.Transport{Proxy: nil, ForceAttemptHTTP2: true, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 30 * time.Second, IdleConnTimeout: 30 * time.Second, DialContext: s.dialMediaPublic}, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		attempt, _ := req.Context().Value(mediaAttemptKey{}).(*mediaAttempt)
		if len(via) > 8 {
			attempt.setCause(diagnosticRedirectLimit)
			return http.ErrUseLastResponse
		}
		if !validMediaURL(req.URL) || (req.Method != http.MethodGet && req.Method != http.MethodHead) || req.Response != nil && strings.Contains(req.Response.Header.Get("Location"), "#") {
			attempt.setCause(diagnosticDestinationRejected)
			return http.ErrUseLastResponse
		}
		// net/http synthesizes Referer on redirects. Playback URLs may include
		// provider credentials, so never let it cross a redirect boundary.
		allowed := make(http.Header)
		for _, header := range []string{"Range", "If-Range"} {
			if values := req.Header.Values(header); len(values) != 0 {
				allowed[http.CanonicalHeaderKey(header)] = append([]string(nil), values...)
			}
		}
		req.Header = allowed
		return nil
	}}
}

func (a *API) rewriteManifest(p *playback, base *url.URL, body []byte) ([]byte, error) {
	lines := strings.Split(string(body), "\n")
	uriCount := 0
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if strings.HasPrefix(trim, "#") {
			uriCount += strings.Count(line, "URI=")
			if uriCount > 4096 {
				return nil, errors.New("too many manifest assets")
			}
			rewritten, err := rewriteURIAttribute(line, func(value string) (string, error) { return a.opaqueAsset(p, base, value) })
			if err != nil {
				return nil, err
			}
			lines[i] = rewritten
			continue
		}
		uriCount++
		if uriCount > 4096 {
			return nil, errors.New("too many manifest assets")
		}
		asset, err := a.opaqueAsset(p, base, trim)
		if err != nil {
			return nil, err
		}
		prefix := line[:strings.Index(line, trim)]
		suffix := line[strings.Index(line, trim)+len(trim):]
		lines[i] = prefix + asset + suffix
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func (a *API) opaqueAsset(p *playback, base *url.URL, raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(u)
	if strings.Contains(raw, "#") || !validMediaURL(resolved) {
		return "", mediaPolicyError{cause: diagnosticDestinationRejected}
	}
	if resolved.Path == "" {
		resolved.Path = path.Clean(base.Path)
	}
	a.private.mu.Lock()
	if current := a.private.playbacks[p.ID]; current == nil || current != p {
		a.private.mu.Unlock()
		return "", errors.New("playback was released")
	}
	now := time.Now()
	if asset := p.AssetByURL[resolved.String()]; asset != "" {
		p.AssetUsed[asset] = now
		a.private.mu.Unlock()
		return "/api/v1/media/" + p.ID + "/" + asset, nil
	}
	if len(p.Assets) >= 8192 {
		oldest, oldestAt := "", time.Time{}
		for asset, lastUsed := range p.AssetUsed {
			if asset == p.RootAsset {
				continue
			}
			if oldest == "" || lastUsed.Before(oldestAt) {
				oldest, oldestAt = asset, lastUsed
			}
		}
		if oldest == "" {
			a.private.mu.Unlock()
			return "", errors.New("asset map is full")
		}
		delete(p.AssetByURL, p.Assets[oldest])
		delete(p.Assets, oldest)
		delete(p.AssetUsed, oldest)
	}
	asset, err := randomToken(18)
	if err != nil {
		a.private.mu.Unlock()
		return "", err
	}
	p.Assets[asset] = resolved.String()
	p.AssetByURL[resolved.String()] = asset
	p.AssetUsed[asset] = now
	a.private.mu.Unlock()
	return "/api/v1/media/" + p.ID + "/" + asset, nil
}

func rewriteURIAttribute(line string, rewrite func(string) (string, error)) (string, error) {
	for search := 0; ; {
		idx := strings.Index(line[search:], "URI=")
		if idx < 0 {
			return line, nil
		}
		start := search + idx + 4
		if start >= len(line) || line[start] != '"' {
			search = start
			continue
		}
		end := strings.IndexByte(line[start+1:], '"')
		if end < 0 {
			return "", errors.New("invalid manifest attribute")
		}
		end += start + 1
		value, err := rewrite(line[start+1 : end])
		if err != nil {
			return "", err
		}
		line = line[:start+1] + value + line[end:]
		search = start + 1 + len(value) + 1
	}
}

func copyMediaHeaders(dst, src http.Header) {
	for _, key := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Last-Modified", "ETag"} {
		if value := src.Get(key); value != "" {
			dst.Set(key, value)
		}
	}
	dst.Set("Cache-Control", "no-store")
}

func providerMediaURL(src *source, kind, id, ext string) (string, error) {
	base, err := url.Parse(src.Base)
	if err != nil {
		return "", err
	}
	route := kind
	if kind == "episode" {
		route = "series"
	}
	return strings.TrimRight(base.String(), "/") + "/" + route + "/" + url.PathEscape(src.Username) + "/" + url.PathEscape(src.Password) + "/" + id + "." + ext, nil
}

func validPlaybackExtension(ext string) bool {
	return ext == "m3u8" || ext == "mp4" || ext == "mkv" || ext == "avi" || ext == "ts"
}

func providerPortAllowed(u *url.URL) bool {
	port := u.Port()
	if port == "" {
		return true
	}
	return u.Scheme == "http" && (port == "80" || port == "8000" || port == "8080" || port == "8081" || port == "8880" || port == "8888") || u.Scheme == "https" && (port == "443" || port == "8443")
}

func validMediaURL(u *url.URL) bool {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawFragment != "" || strings.Contains(u.Hostname(), "%") {
		return false
	}
	return providerPortAllowed(u)
}

func (a *API) mediaContentType(upstream, header string) string {
	if header != "" {
		if typ, _, err := mime.ParseMediaType(header); err == nil {
			return typ
		}
	}
	u, err := url.Parse(upstream)
	if err == nil {
		if typ := mime.TypeByExtension(path.Ext(u.Path)); typ != "" {
			return typ
		}
	}
	return "application/octet-stream"
}
