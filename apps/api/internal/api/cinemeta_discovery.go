package api

import (
	"container/list"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	cinemetaBaseURL     = "https://v3-cinemeta.strem.io"
	cinemetaTimeout     = 5 * time.Second
	cinemetaMaxBody     = 2 << 20
	cinemetaPageSize    = 100
	cinemetaPageLimit   = 3
	cinemetaResultLimit = 40
	cinemetaContextTTL  = 10 * time.Minute
)

var imdbIDPattern = regexp.MustCompile(`^tt[0-9]{7,10}$`)

var cinemetaGenres = map[string]string{
	"Action": "Acción", "Adventure": "Aventura", "Animation": "Animación", "Biography": "Biografía",
	"Comedy": "Comedia", "Crime": "Crimen", "Documentary": "Documental", "Drama": "Drama",
	"Family": "Familiar", "Fantasy": "Fantasía", "History": "Historia",
	"Horror": "Terror", "Mystery": "Misterio",
	"Romance": "Romance", "Sci-Fi": "Ciencia ficción", "Sport": "Deportes", "Thriller": "Suspense",
	"War": "Bélica", "Western": "Western", "Reality-TV": "Reality", "Talk-Show": "Debate", "Game-Show": "Concursos",
}

var cinemetaSeriesOnlyGenres = map[string]bool{"Reality-TV": true, "Talk-Show": true, "Game-Show": true}

type cinemetaMeta struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
	Year any    `json:"year"`
}

type cinemetaBatch struct {
	items   []cinemetaMeta
	expires time.Time
	size    int
}
type discoveryOutput struct {
	Items      []discoveryItem `json:"items"`
	Cursor     string          `json:"cursor,omitempty"`
	NextCursor string          `json:"nextCursor,omitempty"`
	HasMore    bool            `json:"hasMore"`
	Partial    bool            `json:"partial"`
	Status     string          `json:"status"`
	Coverage   string          `json:"coverage"`
}
type discoveryItem struct {
	CatalogItem
	ExternalID string `json:"externalId"`
}
type discoveryPage struct {
	output  discoveryOutput
	expires time.Time
}
type discoveryContext struct {
	userID, sessionHash, sourceID, kind, genre, search, categoryID, language string
	generation                                                               int64
	skip                                                                     int
	externalHasMore                                                          bool
	seen                                                                     map[string]bool
	buffer                                                                   []discoveryItem
	emitted                                                                  int
	created                                                                  time.Time
	mu                                                                       sync.Mutex
	bytes                                                                    int
	limited                                                                  bool
	activeCancels                                                            map[uint64]context.CancelFunc
	nextCancelID                                                             uint64
	cancelMu                                                                 sync.Mutex
	pages                                                                    map[string]discoveryPage
	usedCursors                                                              map[string]bool
	element                                                                  *list.Element
}
type cinemetaService struct {
	client       *http.Client
	mu           sync.Mutex
	cache        map[string]cinemetaBatch
	cacheLRU     *list.List
	cacheBytes   int
	contexts     map[string]*discoveryContext
	contextLRU   *list.List
	userRequests map[string][]time.Time
	userActive   map[string]int
	flights      map[string]chan struct{}
	active       chan struct{}
	queue        chan struct{}
	lastRequest  time.Time
	rateGate     chan struct{}
	cooldown     time.Time
}

func newCinemetaService(cfg Config) *cinemetaService {
	client := &http.Client{Timeout: cinemetaTimeout}
	if cfg.HTTPClient != nil {
		copyClient := *cfg.HTTPClient
		client = &copyClient
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		host := strings.ToLower(req.URL.Hostname())
		port := req.URL.Port()
		if req.URL.Scheme != "https" || req.URL.User != nil || (port != "" && port != "443") || (host != "v3-cinemeta.strem.io" && host != "cinemeta-catalogs.strem.io") || len(via) > 2 {
			return http.ErrUseLastResponse
		}
		return nil
	}
	return &cinemetaService{client: client, cache: map[string]cinemetaBatch{}, cacheLRU: list.New(), contexts: map[string]*discoveryContext{}, contextLRU: list.New(), userRequests: map[string][]time.Time{}, userActive: map[string]int{}, flights: map[string]chan struct{}{}, active: make(chan struct{}, 2), queue: make(chan struct{}, 16), rateGate: make(chan struct{}, 1)}
}

func parseCinemetaYear(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		y := int(v)
		return y, y >= 1888 && y <= 2200
	case int:
		return v, v >= 1888 && v <= 2200
	case string:
		if len(v) >= 4 {
			y, err := strconv.Atoi(v[:4])
			return y, err == nil && y >= 1888 && y <= 2200
		}
	}
	return 0, false
}

func (s *cinemetaService) catalog(ctx context.Context, kind, genre, search string, skip int) ([]cinemetaMeta, error) {
	key := strings.Join([]string{kind, genre, strings.ToLower(strings.TrimSpace(search)), strconv.Itoa(skip)}, "\x00")
	now := time.Now()
	s.mu.Lock()
	if batch, ok := s.cache[key]; ok && now.Before(batch.expires) {
		s.cacheLRU.MoveToFront(s.cacheLRUElement(key))
		out := append([]cinemetaMeta(nil), batch.items...)
		s.mu.Unlock()
		return out, nil
	}
	if f := s.flights[key]; f != nil {
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-f:
		}
		s.mu.Lock()
		batch, ok := s.cache[key]
		s.mu.Unlock()
		if ok && time.Now().Before(batch.expires) {
			return append([]cinemetaMeta(nil), batch.items...), nil
		}
		return nil, errCatalogNetwork
	}
	f := make(chan struct{})
	s.flights[key] = f
	if time.Now().Before(s.cooldown) {
		delete(s.flights, key)
		close(f)
		s.mu.Unlock()
		return nil, errCatalogHTTP
	}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if s.flights[key] == f {
			delete(s.flights, key)
		}
		close(f)
		s.mu.Unlock()
	}()
	select {
	case s.queue <- struct{}{}:
		defer func() { <-s.queue }()
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return nil, errCatalogHTTP
	}
	select {
	case s.active <- struct{}{}:
		defer func() { <-s.active }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case s.rateGate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	rateHeld := true
	defer func() {
		if rateHeld {
			<-s.rateGate
		}
	}()
	s.mu.Lock()
	delay := time.Second/4 - time.Since(s.lastRequest)
	s.mu.Unlock()
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	s.mu.Lock()
	s.lastRequest = time.Now()
	s.mu.Unlock()
	<-s.rateGate
	rateHeld = false
	extraArgs := []string{}
	if genre != "" {
		extraArgs = append(extraArgs, "genre="+escapeCinemetaExtraValue(genre))
	}
	if search != "" {
		extraArgs = append(extraArgs, "search="+escapeCinemetaExtraValue(search))
	}
	if skip > 0 {
		extraArgs = append(extraArgs, "skip="+strconv.Itoa(skip))
	}
	path := "/catalog/" + kind + "/top"
	if len(extraArgs) > 0 {
		path += "/" + strings.Join(extraArgs, "&")
	}
	u := cinemetaBaseURL + path + ".json"
	requestCtx, requestCancel := context.WithTimeout(ctx, cinemetaTimeout)
	defer requestCancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, u, nil)
	if err != nil {
		return nil, errCatalogNetwork
	}
	s.mu.Lock()
	stillCooling := time.Now().Before(s.cooldown)
	s.mu.Unlock()
	if stillCooling {
		return nil, errCatalogHTTP
	}
	resp, err := s.client.Do(req)
	if err != nil {
		s.extendCooldown(time.Now().Add(30 * time.Second))
		return nil, errCatalogNetwork
	}
	defer resp.Body.Close()
	if resp.Request != nil && resp.Request.URL.Hostname() != "v3-cinemeta.strem.io" && resp.Request.URL.Hostname() != "cinemeta-catalogs.strem.io" {
		return nil, errCatalogHTTP
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		cool := time.Minute
		if resp.StatusCode == 429 {
			if v := resp.Header.Get("Retry-After"); v != "" {
				if seconds, e := strconv.ParseInt(v, 10, 64); e == nil && seconds > 0 {
					if seconds < 9_000_000_000 {
						cool = time.Duration(seconds) * time.Second
					} else {
						cool = time.Duration(1<<63 - 1)
					}
				} else if t, e := http.ParseTime(v); e == nil && t.After(time.Now()) {
					cool = t.Sub(time.Now())
				}
			}
		}
		s.extendCooldown(time.Now().Add(cool))
		return nil, errCatalogHTTP
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errCatalogHTTP
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, cinemetaMaxBody+1))
	if err != nil || len(body) > cinemetaMaxBody {
		return nil, errCatalogMalformed
	}
	var payload struct {
		Metas []cinemetaMeta `json:"metas"`
	}
	if json.Unmarshal(body, &payload) != nil || payload.Metas == nil {
		return nil, errCatalogMalformed
	}
	out := make([]cinemetaMeta, 0, len(payload.Metas))
	for _, m := range payload.Metas {
		y, _ := parseCinemetaYear(m.Year)
		if m.Type == kind && imdbIDPattern.MatchString(m.ID) && strings.TrimSpace(m.Name) != "" {
			if y > 0 {
				m.Year = y
			}
			out = append(out, m)
		}
	}
	s.mu.Lock()
	if old, ok := s.cache[key]; ok {
		s.cacheBytes -= old.size
		if oldEl := s.cacheLRUElement(key); oldEl != nil {
			s.cacheLRU.Remove(oldEl)
		}
	}
	entryBytes := len(body)
	for len(s.cache) >= 128 || s.cacheBytes+entryBytes > 16<<20 {
		el := s.cacheLRU.Back()
		if el == nil {
			break
		}
		k := el.Value.(string)
		s.cacheBytes -= s.cache[k].size
		delete(s.cache, k)
		s.cacheLRU.Remove(el)
	}
	if entryBytes <= 16<<20 {
		s.cache[key] = cinemetaBatch{items: append([]cinemetaMeta(nil), out...), expires: time.Now().Add(15 * time.Minute), size: entryBytes}
		s.cacheBytes += entryBytes
		s.cacheLRU.PushFront(key)
	}
	s.mu.Unlock()
	return out, nil
}

func escapeCinemetaExtraValue(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

func (s *cinemetaService) extendCooldown(until time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if until.After(s.cooldown) {
		s.cooldown = until
	}
}

func (s *cinemetaService) cacheLRUElement(key string) *list.Element {
	for el := s.cacheLRU.Front(); el != nil; el = el.Next() {
		if el.Value.(string) == key {
			return el
		}
	}
	return nil
}

func (s *cinemetaService) allowUser(userID string) bool {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	recent := s.userRequests[userID][:0]
	for _, t := range s.userRequests[userID] {
		if now.Sub(t) < time.Minute {
			recent = append(recent, t)
		}
	}
	if len(recent) >= 10 {
		s.userRequests[userID] = recent
		return false
	}
	s.userRequests[userID] = append(recent, now)
	return true
}

func (s *cinemetaService) beginUser(userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.userActive[userID] >= 2 {
		return false
	}
	s.userActive[userID]++
	return true
}
func (s *cinemetaService) endUser(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.userActive[userID] <= 1 {
		delete(s.userActive, userID)
	} else {
		s.userActive[userID]--
	}
}

func (s *cinemetaService) removeContextLocked(target *discoveryContext) {
	target.cancelMu.Lock()
	for _, cancel := range target.activeCancels {
		cancel()
	}
	target.cancelMu.Unlock()
	for key, value := range s.contexts {
		if value == target {
			delete(s.contexts, key)
		}
	}
	if target.element != nil {
		s.contextLRU.Remove(target.element)
		target.element = nil
	}
}

func (s *cinemetaService) rememberContextLocked(token string, target *discoveryContext) {
	if _, exists := s.contexts[token]; exists {
		s.contexts[token] = target
		if target.element != nil {
			s.contextLRU.MoveToFront(target.element)
		}
		return
	}
	for _, value := range s.contexts {
		if value == target {
			s.contexts[token] = target
			if target.element != nil {
				s.contextLRU.MoveToFront(target.element)
			}
			return
		}
	}
	now := time.Now()
	unique := map[*discoveryContext]bool{}
	for _, value := range s.contexts {
		unique[value] = true
		if now.Sub(value.created) > cinemetaContextTTL {
			s.removeContextLocked(value)
		}
	}
	users := 0
	var oldest *discoveryContext
	unique = map[*discoveryContext]bool{}
	for _, value := range s.contexts {
		if unique[value] {
			continue
		}
		unique[value] = true
		if value.userID == target.userID {
			users++
			if value != target && (oldest == nil || value.created.Before(oldest.created)) {
				oldest = value
			}
		}
	}
	if users >= 8 && oldest != nil {
		s.removeContextLocked(oldest)
	}
	unique = map[*discoveryContext]bool{}
	count := 0
	var globalOldest *discoveryContext
	for _, value := range s.contexts {
		if unique[value] {
			continue
		}
		unique[value] = true
		count++
		if value != target && (globalOldest == nil || value.created.Before(globalOldest.created)) {
			globalOldest = value
		}
	}
	if count >= 128 && globalOldest != nil {
		s.removeContextLocked(globalOldest)
	}
	s.contexts[token] = target
	if target.element == nil {
		target.element = s.contextLRU.PushFront(target)
	} else {
		s.contextLRU.MoveToFront(target.element)
	}
}

func (s *cinemetaService) invalidateUser(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[*discoveryContext]bool{}
	for _, state := range s.contexts {
		if state.userID == userID && !seen[state] {
			seen[state] = true
			s.removeContextLocked(state)
		}
	}
	delete(s.userRequests, userID)
}

func (s *cinemetaService) invalidateSource(sourceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[*discoveryContext]bool{}
	for _, state := range s.contexts {
		if state.sourceID == sourceID && !seen[state] {
			seen[state] = true
			s.removeContextLocked(state)
		}
	}
}

func (a *API) getCinemetaDiscovery(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	q := r.URL.Query()
	kind, genre, search := q.Get("type"), q.Get("genre"), strings.TrimSpace(q.Get("search"))
	categoryID, language, cursor := q.Get("categoryId"), q.Get("language"), q.Get("cursor")
	allowed := map[string]bool{"type": true, "genre": true, "search": true, "categoryId": true, "language": true, "cursor": true}
	for key, values := range q {
		if !allowed[key] || len(values) != 1 {
			writeError(w, 400, "INVALID_INPUT", "Invalid discovery query")
			return
		}
	}
	if (kind != "movie" && kind != "series") || (genre != "" && cinemetaGenres[genre] == "") || (cinemetaSeriesOnlyGenres[genre] && kind != "series") || len([]rune(search)) > 150 || !numericID(categoryID) || !validCatalogLanguage(language) || len(cursor) > 128 {
		writeError(w, 400, "INVALID_INPUT", "Invalid discovery query")
		return
	}
	src, ok := a.catalogSource(w, r)
	if !ok {
		return
	}
	sess := requestSession(r)
	if !a.discovery.allowUser(sess.User.ID) {
		writeError(w, 429, "DISCOVERY_LIMITED", "Espera un momento antes de volver a buscar.")
		return
	}
	if !a.discovery.beginUser(sess.User.ID) {
		writeError(w, 429, "DISCOVERY_LIMITED", "Ya hay búsquedas activas. Espera a que terminen.")
		return
	}
	defer a.discovery.endUser(sess.User.ID)
	ctx, cancel := context.WithTimeout(r.Context(), 16*time.Second)
	defer cancel()
	generation, err := a.private.activeGeneration(ctx, src.ID)
	if err != nil {
		writeError(w, 503, "SOURCE_STORE_UNAVAILABLE", "No se pudo consultar el catálogo IPTV.")
		return
	}
	var state *discoveryContext
	if cursor == "" {
		token, e := randomToken(24)
		if e != nil {
			writeError(w, 500, "INTERNAL", "No se pudo iniciar la búsqueda.")
			return
		}
		state = &discoveryContext{userID: sess.User.ID, sessionHash: hex.EncodeToString(hashToken(sess.Token)), sourceID: src.ID, kind: kind, genre: genre, search: search, categoryID: categoryID, language: language, generation: generation, seen: map[string]bool{}, pages: map[string]discoveryPage{}, usedCursors: map[string]bool{}, activeCancels: map[uint64]context.CancelFunc{}, created: time.Now()}
		state.pages[token] = discoveryPage{}
		cursor = token
		a.discovery.mu.Lock()
		a.discovery.rememberContextLocked(token, state)
		a.discovery.mu.Unlock()
	}
	a.discovery.mu.Lock()
	state = a.discovery.contexts[cursor]
	if state != nil && time.Since(state.created) > cinemetaContextTTL {
		a.discovery.removeContextLocked(state)
		state = nil
	}
	if state != nil {
		a.discovery.contextLRU.MoveToFront(state.element)
	}
	a.discovery.mu.Unlock()
	if state == nil || state.userID != sess.User.ID || state.sessionHash != hex.EncodeToString(hashToken(sess.Token)) || state.sourceID != src.ID || state.kind != kind || state.genre != genre || state.search != search || state.categoryID != categoryID || state.language != language || state.generation != generation || time.Since(state.created) > cinemetaContextTTL {
		writeJSON(w, 200, discoveryOutput{Items: []discoveryItem{}, Status: "restart_required", Coverage: "imdb-only"})
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	a.discovery.mu.Lock()
	if a.discovery.contexts[cursor] != state {
		a.discovery.mu.Unlock()
		writeJSON(w, 200, discoveryOutput{Items: []discoveryItem{}, Status: "restart_required", Coverage: "imdb-only"})
		return
	}
	state.nextCancelID++
	cancelID := state.nextCancelID
	requestCtx, cancelRequest := context.WithTimeout(ctx, 16*time.Second)
	state.cancelMu.Lock()
	state.activeCancels[cancelID] = cancelRequest
	state.cancelMu.Unlock()
	a.discovery.mu.Unlock()
	defer func() {
		cancelRequest()
		state.cancelMu.Lock()
		delete(state.activeCancels, cancelID)
		state.cancelMu.Unlock()
	}()
	ctx = requestCtx
	if prior, ok := state.pages[cursor]; ok && prior.output.Status != "" {
		currentGeneration, generationErr := a.private.activeGeneration(ctx, src.ID)
		_, sourceValid, sourceErr := a.private.sourceFor(ctx, sess, src.ID)
		if !sourceValid || sourceErr != nil || generationErr != nil || currentGeneration != generation {
			writeJSON(w, 200, discoveryOutput{Items: []discoveryItem{}, Status: "restart_required", Coverage: "imdb-only"})
			return
		}
		writeJSON(w, 200, prior.output)
		return
	}
	if state.usedCursors[cursor] {
		writeJSON(w, 200, discoveryOutput{Items: []discoveryItem{}, Status: "restart_required", Coverage: "imdb-only"})
		return
	}
	state.usedCursors[cursor] = true
	originalSkip := state.skip
	originalExternalHasMore := state.externalHasMore
	originalBuffer := append([]discoveryItem(nil), state.buffer...)
	originalSeen := make(map[string]bool, len(state.seen))
	for k, v := range state.seen {
		originalSeen[k] = v
	}
	originalEmitted, originalBytes, originalLimited := state.emitted, state.bytes, state.limited
	restore := func() {
		state.skip = originalSkip
		state.externalHasMore = originalExternalHasMore
		state.buffer = originalBuffer
		state.seen = originalSeen
		state.emitted = originalEmitted
		state.bytes = originalBytes
		state.limited = originalLimited
		state.usedCursors[cursor] = false
	}
	out := discoveryOutput{Items: []discoveryItem{}, Status: "ready", Coverage: "imdb-only"}
	externalPages := 0
	for len(out.Items) < cinemetaResultLimit && externalPages < cinemetaPageLimit {
		if len(state.buffer) == 0 {
			metas, e := a.discovery.catalog(ctx, kind, genre, search, state.skip)
			if e != nil {
				restore()
				writeJSON(w, 503, discoveryOutput{Items: []discoveryItem{}, Cursor: cursor, Status: "unavailable", Coverage: "imdb-only"})
				return
			}
			state.skip += cinemetaPageSize
			externalPages++
			if len(metas) == 0 {
				state.skip = -1
				state.externalHasMore = false
				break
			}
			state.externalHasMore = len(metas) == cinemetaPageSize
			ids := make([]string, 0, len(metas))
			years := map[string]int{}
			for _, m := range metas {
				ids = append(ids, m.ID)
				if y, ok := parseCinemetaYear(m.Year); ok {
					years[m.ID] = y
				}
			}
			matches, truncated, e := a.queryDiscoveryBatch(ctx, src.ID, generation, kind, ids, years, categoryID, language)
			if e != nil {
				restore()
				writeError(w, 503, "SOURCE_STORE_UNAVAILABLE", "No se pudo consultar el catálogo IPTV.")
				return
			}
			for _, match := range matches {
				size := len(match.ID) + len(match.Title) + len(match.CategoryID) + len(match.Language) + len(match.ExternalID) + 320
				if state.bytes+size > 1<<20 {
					state.limited = true
					break
				}
				state.buffer = append(state.buffer, match)
				state.bytes += size
			}
			if truncated {
				state.limited = true
			}
			if len(metas) < cinemetaPageSize {
				state.skip = -1
				state.externalHasMore = false
			}
		}
		for len(state.buffer) > 0 && len(out.Items) < cinemetaResultLimit {
			item := state.buffer[0]
			state.buffer = state.buffer[1:]
			key := item.Kind + ":" + item.ID
			if state.seen[key] {
				continue
			}
			state.seen[key] = true
			state.emitted++
			out.Items = append(out.Items, item)
		}
		if state.skip < 0 {
			break
		}
	}
	out.Cursor = cursor
	out.Partial = len(out.Items) == 0 && state.skip >= 0
	out.HasMore = len(state.buffer) > 0 || (state.skip >= 0 && (externalPages == cinemetaPageLimit || state.externalHasMore))
	if out.HasMore {
		next, e := randomToken(24)
		if e != nil {
			restore()
			writeError(w, 500, "INTERNAL", "No se pudo continuar la búsqueda.")
			return
		}
		out.NextCursor = next
		state.pages[next] = discoveryPage{}
		a.discovery.mu.Lock()
		a.discovery.rememberContextLocked(next, state)
		a.discovery.mu.Unlock()
	}
	if state.emitted >= 2000 {
		out.Partial = true
		out.HasMore = false
		out.NextCursor = ""
		out.Status = "restart_required"
	}
	if state.limited {
		out.Partial = true
		out.HasMore = false
		out.NextCursor = ""
		out.Status = "restart_required"
	}
	pageBytes := 0
	for _, item := range out.Items {
		pageBytes += len(item.ID) + len(item.Title) + len(item.ExternalID) + 320
	}
	if state.bytes+pageBytes > 1<<20 {
		out.Partial = true
		out.HasMore = false
		out.NextCursor = ""
		out.Status = "restart_required"
	} else {
		state.bytes += pageBytes
	}
	state.pages[cursor] = discoveryPage{output: out, expires: time.Now().Add(cinemetaContextTTL)}
	currentGeneration, generationErr := a.private.activeGeneration(ctx, src.ID)
	if _, valid, checkErr := a.private.sourceFor(ctx, sess, src.ID); !valid || checkErr != nil || generationErr != nil || currentGeneration != generation {
		writeJSON(w, 200, discoveryOutput{Items: []discoveryItem{}, Status: "restart_required", Coverage: "imdb-only"})
		return
	}
	writeJSON(w, 200, out)
}

func (a *API) queryDiscoveryBatch(ctx context.Context, sourceID string, generation int64, kind string, ids []string, years map[string]int, categoryID, language string) ([]discoveryItem, bool, error) {
	if len(ids) == 0 {
		return []discoveryItem{}, false, nil
	}
	rows, err := a.cfg.DB.Query(ctx, `SELECT e.id,e.title,e.category_id,e.language,e.extension,e.content_key,(e.image_ciphertext IS NOT NULL),e.imdb_id,e.release_year FROM source_catalog_state s JOIN source_catalog_entries e ON e.source_id=s.source_id AND e.generation=s.active_generation WHERE s.source_id=$1 AND s.active_generation=$2 AND e.kind=$3 AND e.imdb_id=ANY($4::text[]) AND ($5='' OR e.category_id=$5) AND ($6='' OR e.language=$6) ORDER BY array_position($4::text[],e.imdb_id),e.id LIMIT $7`, sourceID, generation, kind, ids, categoryID, language, 2501)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := []discoveryItem{}
	truncated := false
	for rows.Next() {
		var item discoveryItem
		var hasImage bool
		var imdb *string
		var year *int
		if rows.Scan(&item.ID, &item.Title, &item.CategoryID, &item.Language, &item.ContainerExtension, &item.ContentKey, &hasImage, &imdb, &year) != nil {
			return nil, false, errSourceStore
		}
		if len(out) >= 2500 {
			truncated = true
			break
		}
		if imdb == nil || !imdbIDPattern.MatchString(*imdb) {
			continue
		}
		externalYear := years[*imdb]
		if externalYear > 0 && year != nil && externalYear != *year {
			continue
		}
		item.Kind = kind
		item.ExternalID = *imdb
		if hasImage {
			item.ImageURL = fmt.Sprintf("/api/v1/sources/%s/images/%s/%s", url.PathEscape(sourceID), kind, url.PathEscape(item.ID))
		}
		out = append(out, item)
	}
	if rows.Err() != nil {
		return nil, false, rows.Err()
	}
	return out, truncated, nil
}
