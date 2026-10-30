package api

import (
	"container/list"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type source struct {
	ID, UserID, Label, AccountKey, Fingerprint string
	PlaylistURL, Base, Username, Password      string
	ExpiresAt                                  time.Time
	AbsoluteExpiresAt                          time.Time
	MaxConnections                             int
	Activity                                   time.Time
	VerifiedAt                                 time.Time
	Saved                                      bool
}

type SourceView struct {
	ID         string    `json:"id"`
	Label      string    `json:"label"`
	AccountKey string    `json:"accountKey"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Saved      bool      `json:"saved"`
}

type CatalogItem struct {
	ID                 string `json:"id"`
	Title              string `json:"title"`
	Kind               string `json:"kind"`
	ContainerExtension string `json:"containerExtension,omitempty"`
	CategoryID         string `json:"categoryId"`
	Language           string `json:"language"`
	ImageURL           string `json:"imageUrl,omitempty"`
	SeriesID           string `json:"-"`
	URL                string `json:"-"`
	ContentKey         string `json:"contentKey,omitempty"`
}

type Episode struct {
	ID                 string `json:"id"`
	Title              string `json:"title"`
	Season             int    `json:"season"`
	Episode            int    `json:"episode"`
	ContainerExtension string `json:"containerExtension"`
	URL                string `json:"-"`
	ContentKey         string `json:"contentKey,omitempty"`
}

type category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type playback struct {
	ID                   string
	UserID               string
	SourceID             string
	Fingerprint          string
	ExpiresAt            time.Time
	Title                string
	ContentKey           string
	Format               string
	RootAsset            string
	Assets               map[string]string
	AssetByURL           map[string]string
	AssetUsed            map[string]time.Time
	Ctx                  context.Context
	Cancel               context.CancelFunc
	Done                 chan struct{}
	Active               int
	Closing              bool
	Diagnostic           PlaybackDiagnostic
	DiagnosticGeneration uint64
	VOD                  *vodJob
	MediaCtx             context.Context
	MediaCancel          context.CancelFunc
}

// PlaybackDiagnostic contains only a coarse, safe-to-return transport cause.
// It is read and written while privateState.mu is held.
type PlaybackDiagnostic struct {
	Cause          string `json:"cause"`
	UpstreamStatus int    `json:"upstreamStatus,omitempty"`
}

type privateState struct {
	mu              sync.Mutex
	shuttingDown    bool
	storeMu         sync.Mutex
	cfg             Config
	generation      map[string]uint64
	sources         map[string]*source
	catalogs        map[string]catalogData
	flights         map[string]*catalogFlight
	playbacks       map[string]*playback
	leases          map[string]map[string]struct{}
	control         *http.Client
	providerCatalog *http.Client
	media           *http.Client
	images          *http.Client
	catalogSync     map[string]*catalogSyncJob
	catalogAccounts map[string]chan struct{}
	catalogSlots    chan struct{}
	imageMu         sync.Mutex
	imageCache      map[string]*cachedImage
	imageCacheLRU   *list.List
	imageCacheBytes int
	imageFlights    map[string]*imageFlight
	imageEpoch      map[string]uint64
	imageFetchSlots chan struct{}
	vodSlots        chan struct{}
	authorized      map[string]map[string]CatalogItem
}

type catalogData struct {
	expires    time.Time
	categories map[string][]category
	items      map[string][]CatalogItem
	episodes   map[string][]Episode
}
type catalogFlight struct {
	done chan struct{}
	data catalogData
	err  error
}

func newPrivateState(cfg Config) *privateState {
	s := &privateState{cfg: cfg, sources: map[string]*source{}, catalogs: map[string]catalogData{}, flights: map[string]*catalogFlight{}, playbacks: map[string]*playback{}, leases: map[string]map[string]struct{}{}, authorized: map[string]map[string]CatalogItem{}, generation: map[string]uint64{}, catalogSync: map[string]*catalogSyncJob{}, catalogAccounts: map[string]chan struct{}{}, catalogSlots: make(chan struct{}, 2), imageCache: map[string]*cachedImage{}, imageCacheLRU: list.New(), imageFlights: map[string]*imageFlight{}, imageEpoch: map[string]uint64{}, imageFetchSlots: make(chan struct{}, 4), vodSlots: make(chan struct{}, 1)}
	if cfg.HTTPClient != nil {
		s.control, s.providerCatalog, s.media, s.images = cfg.HTTPClient, cfg.HTTPClient, cfg.HTTPClient, cfg.HTTPClient
	} else {
		s.control = s.newControlClient()
		s.providerCatalog = s.newControlClient()
		s.providerCatalog.Timeout = catalogRequestTimeout
		s.media = s.newMediaClient()
		s.images = s.newImageClient()
	}
	go s.janitor()
	if cfg.DB != nil {
		s.recoverInterruptedCatalogSyncs()
	}
	return s
}

func (a *API) listSources(w http.ResponseWriter, r *http.Request) {
	sess := requestSession(r)
	ctx, cancel := requestContext(r, 5*time.Second)
	defer cancel()
	a.private.storeMu.Lock()
	defer a.private.storeMu.Unlock()
	if err := a.private.checkSourceSession(ctx, sess); err != nil {
		if errors.Is(err, errSourceSessionRevoked) {
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		} else {
			writeError(w, http.StatusServiceUnavailable, "SOURCE_STORE_UNAVAILABLE", "Could not load saved sources")
		}
		return
	}
	var loaded []*source
	var err error
	if a.cfg.DB == nil {
		a.private.mu.Lock()
		for _, src := range a.private.sources {
			if src.UserID == sess.User.ID {
				loaded = append(loaded, src)
			}
		}
		a.private.mu.Unlock()
	} else {
		loaded, err = a.private.loadSources(ctx, sess.User.ID, sess.Absolute)
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "SOURCE_STORE_UNAVAILABLE", "Could not load saved sources")
		return
	}
	a.private.mu.Lock()
	now := time.Now()
	seen := make(map[string]struct{}, len(loaded))
	out := make([]SourceView, 0, len(loaded))
	for _, restored := range loaded {
		seen[restored.ID] = struct{}{}
		src := restored
		if current := a.private.sources[src.ID]; current != nil && current.UserID == sess.User.ID {
			src = current
			src.AbsoluteExpiresAt = sess.Absolute
		}
		src.Activity = now
		src.ExpiresAt = minTime(now.Add(30*time.Minute), sess.Absolute)
		src.Saved = a.cfg.DB != nil
		a.private.sources[src.ID] = src
		out = append(out, src.view())
	}
	for id, current := range a.private.sources {
		if current.UserID == sess.User.ID {
			if _, ok := seen[id]; !ok {
				a.private.dropPlaybackForSourceLocked(id)
				delete(a.private.sources, id)
				a.private.forgetSourceLocked(id)
			}
		}
	}
	a.private.mu.Unlock()
	if a.cfg.DB != nil {
		for _, restored := range loaded {
			status, statusErr := a.private.catalogStatus(r.Context(), restored.ID)
			if statusErr == nil && status.Status == "empty" {
				a.private.startCatalogSync(restored)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": out})
}

func (a *API) createSource(w http.ResponseWriter, r *http.Request) {
	var input struct {
		PlaylistURL string `json:"playlistUrl"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, 400, "INVALID_INPUT", "Invalid source request")
		return
	}
	base, username, password, host, err := parsePlaylistURL(input.PlaylistURL)
	if err != nil || !a.private.allowedHost(host) {
		writeError(w, 400, "INVALID_SOURCE", "Playlist URL is not allowed")
		return
	}
	ctx, cancel := requestContext(r, 15*time.Second)
	defer cancel()
	info, err := a.private.providerCall(ctx, base, username, password, "", nil)
	if err != nil || !accountActive(info) {
		writeError(w, 502, "SOURCE_REJECTED", "The provider did not accept this account")
		return
	}
	userInfo, _ := info["user_info"].(map[string]any)
	maxConn := asInt(userInfo["max_connections"])
	if maxConn < 1 || maxConn > 64 {
		maxConn = 1
	}
	user := requestSession(r)
	accountKey := a.private.identityKey(base, username)
	fingerprint := a.private.fingerprint(base, username)
	id, err := randomToken(18)
	if err != nil {
		writeError(w, 500, "INTERNAL", "Could not create source")
		return
	}
	now := time.Now().UTC()
	abs := user.Absolute
	if abs.IsZero() {
		abs = now.Add(12 * time.Hour)
	}
	created := &source{ID: id, UserID: user.User.ID, Label: "Lista privada", AccountKey: accountKey, Fingerprint: fingerprint, PlaylistURL: strings.TrimSpace(input.PlaylistURL), Base: base, Username: username, Password: password, ExpiresAt: minTime(now.Add(30*time.Minute), abs), AbsoluteExpiresAt: abs, Activity: now, VerifiedAt: now, MaxConnections: maxConn, Saved: a.cfg.DB != nil}
	a.private.storeMu.Lock()
	storeCtx, storeCancel := requestContext(r, 5*time.Second)
	defer storeCancel()
	if err := a.private.checkSourceSession(storeCtx, user); err != nil {
		a.private.storeMu.Unlock()
		if errors.Is(err, errSourceSessionRevoked) {
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		} else {
			writeError(w, http.StatusServiceUnavailable, "SOURCE_STORE_UNAVAILABLE", "Could not save source")
		}
		return
	}
	if err := a.private.saveSource(storeCtx, created, hashToken(user.Token)); err != nil {
		a.private.storeMu.Unlock()
		if errors.Is(err, errSourceSessionRevoked) {
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "SOURCE_STORE_UNAVAILABLE", "Could not save source")
		return
	}
	a.private.mu.Lock()
	for oldID, old := range a.private.sources {
		if old.UserID == user.User.ID {
			a.private.dropPlaybackForSourceLocked(oldID)
			delete(a.private.sources, oldID)
			a.private.forgetSourceLocked(oldID)
		}
	}
	a.private.sources[id] = created
	a.private.mu.Unlock()
	a.private.storeMu.Unlock()
	if a.cfg.DB != nil {
		a.private.startCatalogSync(created)
	}
	writeJSON(w, http.StatusCreated, created.view())
}

func (a *API) deleteSource(w http.ResponseWriter, r *http.Request) {
	s := requestSession(r)
	id := r.PathValue("id")
	a.private.storeMu.Lock()
	defer a.private.storeMu.Unlock()
	ctx, cancel := requestContext(r, 5*time.Second)
	defer cancel()
	if err := a.private.checkSourceSession(ctx, s); err != nil {
		if errors.Is(err, errSourceSessionRevoked) {
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		} else {
			writeError(w, http.StatusServiceUnavailable, "SOURCE_STORE_UNAVAILABLE", "Could not delete source")
		}
		return
	}
	deleted, err := a.private.deleteSavedSource(ctx, s.User.ID, id)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "SOURCE_STORE_UNAVAILABLE", "Could not delete source")
		return
	}
	if !deleted {
		writeError(w, 404, "NOT_FOUND", "Source not found")
		return
	}
	a.private.mu.Lock()
	defer a.private.mu.Unlock()
	src := a.private.sources[id]
	if (src == nil || src.UserID != s.User.ID) && a.cfg.DB == nil {
		writeError(w, 404, "NOT_FOUND", "Source not found")
		return
	}
	if src != nil && src.UserID == s.User.ID {
		a.private.dropPlaybackForSourceLocked(id)
		delete(a.private.sources, id)
		a.private.forgetSourceLocked(id)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) getCategories(w http.ResponseWriter, r *http.Request) {
	if a.cfg.DB != nil {
		src, ok := a.catalogSource(w, r)
		if !ok {
			return
		}
		typ := r.URL.Query().Get("type")
		if !validCatalogType(typ) {
			writeError(w, 400, "INVALID_INPUT", "Invalid catalog type")
			return
		}
		rows, err := a.private.dbCategories(r.Context(), src.ID, typ)
		if err != nil {
			writeError(w, 503, "SOURCE_STORE_UNAVAILABLE", "Could not load catalog categories")
			return
		}
		writeJSON(w, 200, map[string]any{"categories": rows})
		return
	}
	sess := requestSession(r)
	src, ok, err := a.private.sourceFor(r.Context(), sess, r.PathValue("id"))
	if !ok {
		if err != nil {
			if errors.Is(err, errSourceSessionRevoked) {
				writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
				return
			}
			writeError(w, http.StatusServiceUnavailable, "SOURCE_STORE_UNAVAILABLE", "Could not load source")
			return
		}
		writeError(w, 404, "NOT_FOUND", "Source not found")
		return
	}
	typ := r.URL.Query().Get("type")
	if !validCatalogType(typ) {
		writeError(w, 400, "INVALID_INPUT", "Invalid catalog type")
		return
	}
	data, err := a.private.catalog(r.Context(), src, typ, "")
	if err != nil {
		providerError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"categories": data.categories[typ]})
}

func (a *API) getItems(w http.ResponseWriter, r *http.Request) {
	if a.cfg.DB != nil {
		src, ok := a.catalogSource(w, r)
		if !ok {
			return
		}
		kind, cat, lang, page, pageSize, err := parseCatalogPage(r.URL.Query())
		if err != nil {
			writeError(w, 400, "INVALID_INPUT", "Invalid catalog query")
			return
		}
		items, total, err := a.queryCatalogItems(r.Context(), src.ID, kind, cat, lang, strings.TrimSpace(r.URL.Query().Get("search")), page, pageSize)
		if err != nil {
			writeError(w, 503, "SOURCE_STORE_UNAVAILABLE", "Could not load catalog items")
			return
		}
		a.private.mu.Lock()
		if a.private.authorized[src.ID] == nil {
			a.private.authorized[src.ID] = map[string]CatalogItem{}
		}
		for _, item := range items {
			a.private.authorized[src.ID][item.ContentKey] = item
		}
		a.private.mu.Unlock()
		writeJSON(w, 200, map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
		return
	}
	sess := requestSession(r)
	src, ok, err := a.private.sourceFor(r.Context(), sess, r.PathValue("id"))
	if !ok {
		if err != nil {
			if errors.Is(err, errSourceSessionRevoked) {
				writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
				return
			}
			writeError(w, http.StatusServiceUnavailable, "SOURCE_STORE_UNAVAILABLE", "Could not load source")
			return
		}
		writeError(w, 404, "NOT_FOUND", "Source not found")
		return
	}
	q := r.URL.Query()
	typ, cat := q.Get("type"), q.Get("categoryId")
	if !validCatalogType(typ) || !numericID(cat) {
		writeError(w, 400, "INVALID_INPUT", "Invalid catalog query")
		return
	}
	page := positiveInt(q.Get("page"), 1)
	pageSize := positiveInt(q.Get("pageSize"), 40)
	if pageSize > 100 {
		pageSize = 100
	}
	data, err := a.private.catalog(r.Context(), src, typ, cat)
	if err != nil {
		providerError(w, err)
		return
	}
	all := data.items[typ+":"+cat]
	search := strings.ToLower(strings.TrimSpace(q.Get("search")))
	filtered := make([]CatalogItem, 0, len(all))
	for _, item := range all {
		if search == "" || strings.Contains(strings.ToLower(item.Title), search) {
			filtered = append(filtered, item)
		}
	}
	start := (page - 1) * pageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	writeJSON(w, 200, map[string]any{"items": filtered[start:end], "total": len(filtered), "page": page, "pageSize": pageSize})
}

func (a *API) getEpisodes(w http.ResponseWriter, r *http.Request) {
	sess := requestSession(r)
	src, ok, err := a.private.sourceFor(r.Context(), sess, r.PathValue("id"))
	if !ok {
		if err != nil {
			if errors.Is(err, errSourceSessionRevoked) {
				writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
				return
			}
			writeError(w, http.StatusServiceUnavailable, "SOURCE_STORE_UNAVAILABLE", "Could not load source")
			return
		}
		writeError(w, 404, "NOT_FOUND", "Source not found")
		return
	}
	seriesID := r.PathValue("seriesId")
	if !numericID(seriesID) {
		writeError(w, 400, "INVALID_INPUT", "Invalid series id")
		return
	}
	data, err := a.private.episodes(r.Context(), src, seriesID)
	if err != nil {
		providerError(w, err)
		return
	}
	eps := append([]Episode(nil), data.episodes[seriesID]...)
	sort.Slice(eps, func(i, j int) bool {
		if eps[i].Season == eps[j].Season {
			return eps[i].Episode < eps[j].Episode
		}
		return eps[i].Season < eps[j].Season
	})
	writeJSON(w, 200, map[string]any{"episodes": eps})
}

func (a *API) acestreamChannels(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"channels": []map[string]any{{"id": "sample-sport", "name": "Canal de ejemplo", "contentId": "574e7432eb6b80e541fd42759850dd8bc7b7fa28", "qualities": []map[string]string{{"label": "HD", "id": "574e7432eb6b80e541fd42759850dd8bc7b7fa28"}}}}})
}

func (s *source) view() SourceView {
	return SourceView{s.ID, s.Label, s.AccountKey, s.ExpiresAt, s.Saved}
}
func (s *privateState) sourceFor(ctx context.Context, sess session, id string) (*source, bool, error) {
	userID, absolute := sess.User.ID, sess.Absolute
	generation := sess.Generation
	s.storeMu.Lock()
	defer s.storeMu.Unlock()
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.checkSourceSession(checkCtx, sess); err != nil {
		return nil, false, err
	}
	s.mu.Lock()
	src := s.sources[id]
	if src != nil && src.UserID != userID {
		s.mu.Unlock()
		return nil, false, nil
	}
	now := time.Now()
	if src != nil && now.After(src.ExpiresAt) {
		s.dropPlaybackForSourceLocked(id)
		delete(s.sources, id)
		s.forgetSourceLocked(id)
		src = nil
	}
	if !absolute.IsZero() && now.After(absolute) {
		s.mu.Unlock()
		return nil, false, nil
	}
	if src != nil {
		if !absolute.IsZero() {
			src.AbsoluteExpiresAt = absolute
		}
		src.ExpiresAt = minTime(now.Add(30*time.Minute), src.AbsoluteExpiresAt)
		src.Activity = now
		s.mu.Unlock()
		return src, true, nil
	}
	s.mu.Unlock()
	if s.cfg.DB == nil {
		return nil, false, nil
	}
	restored, err := s.loadSource(checkCtx, userID, id, absolute)
	if err != nil {
		return nil, false, err
	}
	if restored == nil {
		return nil, false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.generation[userID] != generation {
		return nil, false, nil
	}
	if current := s.sources[id]; current != nil {
		if current.UserID != userID {
			return nil, false, nil
		}
		return current, true, nil
	}
	s.sources[id] = restored
	return restored, true, nil
}

func (s *privateState) ensureSourceActive(ctx context.Context, src *source) error {
	now := time.Now()
	s.mu.Lock()
	if s.sources[src.ID] != src || now.After(src.ExpiresAt) {
		s.mu.Unlock()
		return errSourceRemoved
	}
	if !src.VerifiedAt.IsZero() && now.Sub(src.VerifiedAt) < 60*time.Second {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()
	info, err := s.providerCall(ctx, src.Base, src.Username, src.Password, "", nil)
	if err != nil {
		return err
	}
	if !accountActive(info) {
		return errProvider
	}
	userInfo, _ := info["user_info"].(map[string]any)
	maxConn := asInt(userInfo["max_connections"])
	if maxConn < 1 || maxConn > 64 {
		maxConn = 1
	}
	s.mu.Lock()
	if s.sources[src.ID] != src || time.Now().After(src.ExpiresAt) {
		s.mu.Unlock()
		return errSourceRemoved
	}
	src.MaxConnections = maxConn
	src.VerifiedAt = time.Now()
	s.mu.Unlock()
	if s.cfg.DB != nil {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		tag, err := s.cfg.DB.Exec(ctx, "UPDATE sources SET max_connections=$2,updated_at=now() WHERE id=$1 AND user_id=$3", src.ID, maxConn, src.UserID)
		if err != nil || tag.RowsAffected() != 1 {
			return errSourceStore
		}
	}
	return nil
}

func (s *privateState) knownContent(userID, key, kind string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for sourceID, src := range s.sources {
		if src.UserID != userID || now.After(src.ExpiresAt) {
			continue
		}
		if item, ok := s.authorized[sourceID][key]; ok && item.Kind == kind {
			return item.Title, true
		}
	}
	return "", false
}

func (s *privateState) authorizedItem(sourceID, kind, id string) (CatalogItem, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.authorized[sourceID] {
		if item.ID == id && item.Kind == kind {
			return item, true
		}
	}
	return CatalogItem{}, false
}

func (s *privateState) forgetSourceLocked(sourceID string) {
	if job := s.catalogSync[sourceID]; job != nil {
		job.cancel()
	}
	delete(s.authorized, sourceID)
	for key := range s.catalogs {
		if strings.HasPrefix(key, sourceID+":") {
			delete(s.catalogs, key)
		}
	}
	s.purgeSourceImages(sourceID)
}

// evictSourceSessionLocked drops session-scoped state while leaving a persistent
// source and its background catalog synchronization intact (logout path).
func (s *privateState) evictSourceSessionLocked(sourceID string) {
	delete(s.authorized, sourceID)
	for key := range s.catalogs {
		if strings.HasPrefix(key, sourceID+":") {
			delete(s.catalogs, key)
		}
	}
	s.purgeSourceImages(sourceID)
}

func (s *privateState) dropPlaybackForSourceLocked(sourceID string) {
	for id, p := range s.playbacks {
		if p.SourceID == sourceID {
			s.releasePlaybackLocked(id)
		}
	}
}
func (s *privateState) releasePlaybackLocked(id string) {
	p := s.playbacks[id]
	if p == nil {
		return
	}
	p.Closing = true
	if p.VOD != nil {
		p.VOD.stop()
	}
	if p.Cancel != nil {
		p.Cancel()
	}
	if p.Active == 0 {
		s.finalizePlaybackLocked(id, p)
	}
}

func (s *privateState) finalizePlaybackLocked(id string, p *playback) {
	if s.playbacks[id] != p {
		return
	}
	delete(s.playbacks, id)
	if set := s.leases[p.Fingerprint]; set != nil {
		delete(set, id)
		if len(set) == 0 {
			delete(s.leases, p.Fingerprint)
		}
	}
	if p.Done != nil {
		close(p.Done)
	}
}

func (s *privateState) endMediaRequest(id string, p *playback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.playbacks[id] != p {
		return
	}
	if p.Active > 0 {
		p.Active--
	}
	if p.Closing && p.Active == 0 {
		s.finalizePlaybackLocked(id, p)
	}
}

func (s *privateState) catalog(ctx context.Context, src *source, typ, id string) (catalogData, error) {
	key := src.ID + ":" + typ + ":" + id
	s.mu.Lock()
	if data, ok := s.catalogs[key]; ok && time.Now().Before(data.expires) {
		s.mu.Unlock()
		return data, nil
	}
	if flight, ok := s.flights[key]; ok {
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return catalogData{}, ctx.Err()
		case <-flight.done:
			return flight.data, flight.err
		}
	}
	flight := &catalogFlight{done: make(chan struct{})}
	s.flights[key] = flight
	s.mu.Unlock()
	data, loaded, err := s.loadCatalog(ctx, src, catalogStoreKey(typ, id))
	if err == nil && !loaded {
		data, err = s.fetchCatalog(ctx, src, typ, id)
		if err == nil {
			data.expires = time.Now().Add(60 * time.Second)
			err = s.saveCatalog(ctx, src, catalogStoreKey(typ, id), data)
		}
	}
	s.mu.Lock()
	flight.data, flight.err = data, err
	if err == nil && s.sources[src.ID] != src {
		flight.err = errSourceRemoved
	} else if err == nil {
		s.catalogs[key] = data
		s.rememberCatalogLocked(src.ID, data)
		flight.data = data
	}
	delete(s.flights, key)
	close(flight.done)
	s.mu.Unlock()
	return data, err
}

func (s *privateState) fetchCatalog(ctx context.Context, src *source, typ, id string) (catalogData, error) {
	data := catalogData{categories: map[string][]category{}, items: map[string][]CatalogItem{}, episodes: map[string][]Episode{}}
	catAction, streamAction := "get_live_categories", "get_live_streams"
	idField := "stream_id"
	switch typ {
	case "movie":
		catAction, streamAction, idField = "get_vod_categories", "get_vod_streams", "stream_id"
	case "series":
		catAction, streamAction, idField = "get_series_categories", "get_series", "series_id"
	}
	raw, err := s.providerCall(ctx, src.Base, src.Username, src.Password, catAction, nil)
	if err != nil {
		return data, err
	}
	data.categories[typ] = parseCategories(raw)
	if id != "" {
		values := url.Values{"category_id": {id}}
		streams, err := s.providerCall(ctx, src.Base, src.Username, src.Password, streamAction, values)
		if err != nil {
			return data, err
		}
		data.items[typ+":"+id] = parseItems(streams, typ, idField, src.AccountKey, s.cfg.IdentityKey)
		for i := range data.items[typ+":"+id] {
			if data.items[typ+":"+id][i].CategoryID == "" {
				data.items[typ+":"+id][i].CategoryID = id
			}
		}
	}
	return data, nil
}

func (s *privateState) episodes(ctx context.Context, src *source, seriesID string) (catalogData, error) {
	key := src.ID + ":episodes:" + seriesID
	s.mu.Lock()
	if data, ok := s.catalogs[key]; ok && time.Now().Before(data.expires) {
		s.mu.Unlock()
		return data, nil
	}
	if flight, ok := s.flights[key]; ok {
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return catalogData{}, ctx.Err()
		case <-flight.done:
			return flight.data, flight.err
		}
	}
	flight := &catalogFlight{done: make(chan struct{})}
	s.flights[key] = flight
	s.mu.Unlock()
	storeKey := episodesStoreKey(seriesID)
	data, loaded, err := s.loadCatalog(ctx, src, storeKey)
	if err == nil && !loaded {
		data = catalogData{categories: map[string][]category{}, items: map[string][]CatalogItem{}, episodes: map[string][]Episode{}}
		raw, providerErr := s.providerCall(ctx, src.Base, src.Username, src.Password, "get_series_info", url.Values{"series_id": {seriesID}})
		err = providerErr
		if err == nil {
			episodes := parseEpisodes(raw, src.AccountKey, s.cfg.IdentityKey)
			if len(episodes) == 0 {
				err = errCatalog
			} else {
				data.episodes[seriesID] = episodes
			}
		}
		if err == nil {
			data.expires = time.Now().Add(60 * time.Second)
			err = s.saveCatalog(ctx, src, storeKey, data)
		}
	}
	s.mu.Lock()
	flight.data, flight.err = data, err
	if err == nil && s.sources[src.ID] != src {
		flight.err = errSourceRemoved
	} else if err == nil {
		s.catalogs[key] = data
		s.rememberCatalogLocked(src.ID, data)
		flight.data = data
	}
	delete(s.flights, key)
	close(flight.done)
	s.mu.Unlock()
	return data, flight.err
}

func (s *privateState) rememberCatalogLocked(sourceID string, data catalogData) {
	if s.authorized[sourceID] == nil {
		s.authorized[sourceID] = map[string]CatalogItem{}
	}
	for _, items := range data.items {
		for _, item := range items {
			s.authorized[sourceID][item.ContentKey] = item
		}
	}
	for _, episodes := range data.episodes {
		for _, episode := range episodes {
			s.authorized[sourceID][episode.ContentKey] = CatalogItem{ID: episode.ID, Title: episode.Title, Kind: "episode", ContainerExtension: episode.ContainerExtension, ContentKey: episode.ContentKey}
		}
	}
}

func (s *privateState) providerCall(ctx context.Context, base, username, password, action string, extra url.Values) (map[string]any, error) {
	u, err := url.Parse(base + "/player_api.php")
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("username", username)
	q.Set("password", password)
	if action != "" {
		q.Set("action", action)
	}
	for k, values := range extra {
		for _, v := range values {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()
	client := s.control
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errProvider
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20+1))
	if err != nil {
		return nil, err
	}
	if len(body) > 5<<20 {
		return nil, errTooLarge
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err == nil && out != nil {
		return out, nil
	}
	var list []map[string]any
	if err := json.Unmarshal(body, &list); err == nil {
		return map[string]any{"_list": list}, nil
	}
	return nil, errProvider
}

func (s *privateState) newControlClient() *http.Client {
	transport := &http.Transport{Proxy: nil, ForceAttemptHTTP2: true, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 15 * time.Second, IdleConnTimeout: 30 * time.Second, DialContext: s.dialPublic}
	return &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: s.controlCheckRedirect}
}

func (s *privateState) controlCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > 0 && (req.URL.Host != via[0].URL.Host || req.URL.Scheme != via[0].URL.Scheme) {
		return http.ErrUseLastResponse
	}
	if len(via) > 3 {
		return errors.New("too many redirects")
	}
	if (req.URL.Scheme != "http" && req.URL.Scheme != "https") || req.URL.User != nil || !s.allowedHost(req.URL.Hostname()) {
		return errors.New("redirect host is not allowed")
	}
	return nil
}

func (s *privateState) dialPublic(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if !s.allowedHost(host) {
		return nil, errors.New("host is not allowed")
	}
	return s.dialPublicHost(ctx, network, host, port)
}

func (s *privateState) dialMediaPublic(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if strings.Contains(host, "%") {
		return nil, errors.New("IPv6 zones are not allowed")
	}
	return s.dialMediaPublicHost(ctx, network, host, port)
}

func (s *privateState) dialPublicHost(ctx context.Context, network, host, port string) (net.Conn, error) {
	if port != "80" && port != "443" {
		return nil, errors.New("provider port is not allowed")
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	return dialSelectedIP(ctx, network, host, port, ips, publicIP, dialer.DialContext)
}

func (s *privateState) dialMediaPublicHost(ctx context.Context, network, host, port string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	return dialMediaHostWith(ctx, network, host, port, resolveMediaHost, dialer.DialContext)
}

func dialMediaHostWith(ctx context.Context, network, host, port string, lookup func(context.Context, string) ([]netip.Addr, error), dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	if !mediaDialPortAllowed(port) {
		return nil, mediaPolicyError{cause: diagnosticDestinationRejected}
	}
	ips, err := lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if !publicMediaIP(ip) {
			return nil, mediaPolicyError{cause: diagnosticDestinationRejected}
		}
	}
	if len(ips) == 0 {
		return nil, mediaPolicyError{cause: diagnosticDestinationRejected}
	}
	return dialSelectedPublicIP(ctx, network, host, port, ips, dial)
}

func mediaDialPortAllowed(port string) bool {
	switch port {
	case "80", "443", "8000", "8080", "8081", "8443", "8880", "8888":
		return true
	default:
		return false
	}
}

func resolveMediaHost(ctx context.Context, host string) ([]netip.Addr, error) {
	if ip, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{ip}, nil
	}
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

func dialSelectedPublicIP(ctx context.Context, network, host, port string, ips []netip.Addr, dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	return dialSelectedIP(ctx, network, host, port, ips, publicMediaIP, dial)
}

func dialSelectedIP(ctx context.Context, network, host, port string, ips []netip.Addr, isPublic func(netip.Addr) bool, dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	var chosen netip.Addr
	for _, ip := range ips {
		if !isPublic(ip) {
			return nil, errors.New("host resolves to a non-public address")
		}
		if !chosen.IsValid() {
			chosen = ip
		}
	}
	if !chosen.IsValid() {
		return nil, errors.New("host has no public address")
	}
	return dial(ctx, network, net.JoinHostPort(chosen.String(), port))
}

func (s *privateState) allowedHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	for _, allowed := range s.cfg.AllowedHosts {
		if strings.EqualFold(host, strings.TrimSpace(allowed)) {
			return true
		}
	}
	return false
}
func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
		return false
	}
	if ip.Is6() {
		if !netip.MustParsePrefix("2000::/3").Contains(ip) {
			return false
		}
		for _, raw := range []string{"2001:db8::/32", "2001::/23", "2002::/16"} {
			if netip.MustParsePrefix(raw).Contains(ip) {
				return false
			}
		}
		return true
	}
	reserved := []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4"}
	for _, raw := range reserved {
		if netip.MustParsePrefix(raw).Contains(ip) {
			return false
		}
	}
	return true
}

func publicMediaIP(ip netip.Addr) bool {
	return ip.IsValid() && !ip.Is4In6() && publicIP(ip)
}

func (s *privateState) identityKey(base, username string) string {
	mac := hmac.New(sha256.New, s.cfg.IdentityKey)
	fmt.Fprintf(mac, "%s\x00%s", base, strings.ToLower(strings.TrimSpace(username)))
	return hex.EncodeToString(mac.Sum(nil))
}
func (s *privateState) fingerprint(base, username string) string {
	u, _ := url.Parse(base)
	host := ""
	if u != nil {
		host = strings.ToLower(u.Hostname())
	}
	mac := hmac.New(sha256.New, s.cfg.IdentityKey)
	fmt.Fprintf(mac, "%s\x00%s", host, strings.ToLower(strings.TrimSpace(username)))
	return hex.EncodeToString(mac.Sum(nil))
}

func parsePlaylistURL(raw string) (base, username, password, host string, err error) {
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || u == nil {
		return "", "", "", "", errInvalidSource
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" || u.Path != "/get.php" {
		return "", "", "", "", errInvalidSource
	}
	q := u.Query()
	username, password = q.Get("username"), q.Get("password")
	if username == "" || password == "" || len(username) > 256 || len(password) > 512 {
		return "", "", "", "", errInvalidSource
	}
	if u.RawQuery == "" || len(q["username"]) != 1 || len(q["password"]) != 1 || len(q) < 2 || len(q) > 4 {
		return "", "", "", "", errInvalidSource
	}
	for key, values := range q {
		if (key != "username" && key != "password" && key != "type" && key != "output") || len(values) != 1 {
			return "", "", "", "", errInvalidSource
		}
	}
	host = u.Hostname()
	if host == "" || u.Port() != "" && (u.Port() != "80" && u.Port() != "443") {
		return "", "", "", "", errInvalidSource
	}
	if u.Scheme == "https" && u.Port() == "80" || u.Scheme == "http" && u.Port() == "443" {
		return "", "", "", "", errInvalidSource
	}
	if strings.ContainsAny(host, "%") {
		return "", "", "", "", errInvalidSource
	}
	canonicalHost := strings.ToLower(u.Host)
	if u.Port() == "80" && u.Scheme == "http" || u.Port() == "443" && u.Scheme == "https" {
		canonicalHost = strings.ToLower(u.Hostname())
		if strings.Contains(canonicalHost, ":") {
			canonicalHost = "[" + canonicalHost + "]"
		}
	}
	return strings.ToLower(u.Scheme) + "://" + canonicalHost, username, password, strings.ToLower(host), nil
}

var (
	errInvalidSource = errors.New("invalid source")
	errProvider      = errors.New("provider error")
	errTooLarge      = errors.New("provider response too large")
	errCatalog       = errors.New("catalog unavailable")
	errSourceRemoved = errors.New("source no longer available")
)

func accountActive(v map[string]any) bool {
	user, ok := v["user_info"].(map[string]any)
	if !ok {
		return false
	}
	auth := asInt(user["auth"])
	status, _ := user["status"].(string)
	return auth == 1 && strings.EqualFold(status, "Active")
}
func validCatalogType(t string) bool { return t == "live" || t == "movie" || t == "series" }
func numericID(v string) bool {
	if v == "" || len(v) > 32 {
		return false
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func positiveInt(raw string, fallback int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 1_000_000 {
		return fallback
	}
	return n
}
func asInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}
func parseCategories(raw map[string]any) []category {
	list, _ := raw["_list"].([]map[string]any)
	out := make([]category, 0, len(list))
	for _, row := range list {
		id := toString(row["category_id"])
		name := toString(row["category_name"])
		if numericID(id) && name != "" {
			out = append(out, category{id, name})
		}
	}
	return out
}
func parseItems(raw map[string]any, kind, idField, accountKey string, identity []byte) []CatalogItem {
	list, _ := raw["_list"].([]map[string]any)
	out := make([]CatalogItem, 0, len(list))
	for _, row := range list {
		id := toString(row[idField])
		title := toString(row["name"])
		if !numericID(id) || title == "" {
			continue
		}
		ext := validatedExtension(toString(row["container_extension"]))
		item := CatalogItem{ID: id, Title: title, Kind: kind, ContainerExtension: ext, CategoryID: toString(row["category_id"]), URL: toString(row["direct_source"])}
		if kind == "live" {
			item.URL = ""
		}
		item.ContentKey = stableContentKey(identity, accountKey, kind, id)
		out = append(out, item)
	}
	return out
}
func parseEpisodes(raw map[string]any, accountKey string, identity []byte) []Episode {
	series, _ := raw["episodes"].(map[string]any)
	type pair struct {
		season int
		rows   []any
	}
	groups := []pair{}
	for season, anyRows := range series {
		num, err := strconv.Atoi(season)
		if err != nil {
			continue
		}
		rows, _ := anyRows.([]any)
		groups = append(groups, pair{num, rows})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].season < groups[j].season })
	out := []Episode{}
	for _, g := range groups {
		for i, rawRow := range g.rows {
			row, ok := rawRow.(map[string]any)
			if !ok {
				continue
			}
			id := toString(row["id"])
			title := toString(row["title"])
			if !numericID(id) || title == "" {
				continue
			}
			episode := asInt(row["episode_num"])
			if episode < 1 {
				episode = i + 1
			}
			ext := validatedExtension(toString(row["container_extension"]))
			out = append(out, Episode{ID: id, Title: title, Season: g.season, Episode: episode, ContainerExtension: ext, URL: toString(row["direct_source"]), ContentKey: stableContentKey(identity, accountKey, "episode", id)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Season == out[j].Season {
			return out[i].Episode < out[j].Episode
		}
		return out[i].Season < out[j].Season
	})
	return out
}
func validatedExtension(v string) string {
	v = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(v)), ".")
	switch v {
	case "mp4", "mkv", "avi", "ts", "m3u8":
		return v
	}
	return ""
}
func toString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case json.Number:
		return string(x)
	}
	return ""
}
func stableContentKey(key []byte, account, kind, id string) string {
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%s\x00%s\x00%s", account, kind, id)
	return hex.EncodeToString(mac.Sum(nil))
}
func providerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errSourceStore):
		writeError(w, http.StatusServiceUnavailable, "SOURCE_STORE_UNAVAILABLE", "Could not load saved catalog")
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, 504, "UPSTREAM_TIMEOUT", "Provider request timed out")
	case errors.Is(err, errTooLarge):
		writeError(w, 413, "CATALOG_TOO_LARGE", "Provider catalog is too large")
	default:
		writeError(w, 502, "UPSTREAM_ERROR", "Provider request failed")
	}
}
func (s *privateState) janitor() {
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for id, src := range s.sources {
			if now.After(src.ExpiresAt) || !src.AbsoluteExpiresAt.IsZero() && now.After(src.AbsoluteExpiresAt) {
				s.dropPlaybackForSourceLocked(id)
				delete(s.sources, id)
				s.forgetSourceLocked(id)
			}
		}
		for id, p := range s.playbacks {
			if now.After(p.ExpiresAt) {
				s.releasePlaybackLocked(id)
			}
		}
		s.mu.Unlock()
	}
}

var _ = io.EOF
