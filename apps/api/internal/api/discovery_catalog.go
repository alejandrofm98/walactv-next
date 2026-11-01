package api

import (
	"bytes"
	"container/list"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const (
	maxCatalogResponse    = 256 << 20
	maxCatalogItems       = 500_000
	maxCatalogCategories  = 20_000
	catalogRequestTimeout = 120 * time.Second
	maxCatalogRuntime     = 10 * time.Minute
	imageMaxBytes         = 5 << 20
	imageCacheMaxBytes    = 20 << 20
	imageCacheMaxItems    = 512
)

var (
	errCatalogNetwork   = errors.New("catalog request failed")
	errCatalogHTTP      = errors.New("catalog provider rejected request")
	errCatalogMalformed = errors.New("catalog response is invalid")
)

type CatalogStatus struct {
	Status    string         `json:"status"`
	UpdatedAt *time.Time     `json:"updatedAt,omitempty"`
	ErrorCode string         `json:"errorCode,omitempty"`
	Counts    map[string]int `json:"counts"`
}

type catalogEntry struct {
	ID, Title, CategoryID, Language, Extension, ContentKey string
	ImageCiphertext                                        []byte
	ReleaseYear                                            *int
	TMDBID                                                 *int64
	IMDBID                                                 *string
}

type catalogSyncJob struct{ cancel context.CancelFunc }

type cachedImage struct {
	key         string
	body        []byte
	contentType string
	element     *list.Element
}
type imageFlight struct {
	epoch       uint64
	done        chan struct{}
	body        []byte
	contentType string
	err         error
}

func (s *privateState) cachedImage(ctx context.Context, sourceID, key string, fetch func() ([]byte, string, error)) ([]byte, string, error) {
	s.imageMu.Lock()
	if entry := s.imageCache[key]; entry != nil {
		s.imageCacheLRU.MoveToFront(entry.element)
		body, typ := entry.body, entry.contentType
		s.imageMu.Unlock()
		return body, typ, nil
	}
	epoch := s.imageEpoch[sourceID]
	flightKey := fmt.Sprintf("%s\x00%d", key, epoch)
	if flight := s.imageFlights[flightKey]; flight != nil {
		s.imageMu.Unlock()
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-flight.done:
			return flight.body, flight.contentType, flight.err
		}
	}
	flight := &imageFlight{epoch: epoch, done: make(chan struct{})}
	s.imageFlights[flightKey] = flight
	s.imageMu.Unlock()
	var body []byte
	var typ string
	var err error
	select {
	case s.imageFetchSlots <- struct{}{}:
		body, typ, err = fetch()
		<-s.imageFetchSlots
	case <-ctx.Done():
		err = ctx.Err()
	}
	s.imageMu.Lock()
	if s.imageEpoch[sourceID] != epoch && err == nil {
		body = nil
		typ = ""
		err = errSourceRemoved
	}
	flight.body, flight.contentType, flight.err = body, typ, err
	if s.imageFlights[flightKey] == flight {
		delete(s.imageFlights, flightKey)
	}
	if err == nil && s.imageEpoch[sourceID] == epoch && len(body) <= imageCacheMaxBytes {
		for len(s.imageCache) >= imageCacheMaxItems || s.imageCacheBytes+len(body) > imageCacheMaxBytes {
			last := s.imageCacheLRU.Back()
			if last == nil {
				break
			}
			oldKey, _ := last.Value.(string)
			old := s.imageCache[oldKey]
			delete(s.imageCache, oldKey)
			s.imageCacheBytes -= len(old.body)
			s.imageCacheLRU.Remove(last)
		}
		el := s.imageCacheLRU.PushFront(key)
		s.imageCache[key] = &cachedImage{key: key, body: body, contentType: typ, element: el}
		s.imageCacheBytes += len(body)
	}
	close(flight.done)
	s.imageMu.Unlock()
	return body, typ, err
}

func (s *privateState) purgeSourceImages(sourceID string) {
	s.imageMu.Lock()
	defer s.imageMu.Unlock()
	// Invalidate any current flight without permanently revoking this source ID.
	// A user can log out and later restore the same saved source.
	s.imageEpoch[sourceID]++
	for key, entry := range s.imageCache {
		if strings.Contains(key, "\x00"+sourceID+"\x00") {
			delete(s.imageCache, key)
			s.imageCacheBytes -= len(entry.body)
			s.imageCacheLRU.Remove(entry.element)
		}
	}
}

func emptyCounts() map[string]int { return map[string]int{"live": 0, "movie": 0, "series": 0} }

func (s *privateState) recoverInterruptedCatalogSyncs() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.cfg.DB.Exec(ctx, `UPDATE source_catalog_state SET status='error',error_code='interrupted',sync_started_at=NULL WHERE status='syncing'`)
}

func (a *API) getCatalogStatus(w http.ResponseWriter, r *http.Request) {
	src, ok := a.catalogSource(w, r)
	if !ok {
		return
	}
	status, err := a.private.catalogStatus(r.Context(), src.ID)
	if err != nil {
		writeError(w, 503, "SOURCE_STORE_UNAVAILABLE", "Could not load catalog status")
		return
	}
	writeJSON(w, 200, status)
}

func (a *API) refreshCatalog(w http.ResponseWriter, r *http.Request) {
	src, ok := a.catalogSource(w, r)
	if !ok {
		return
	}
	status, err := a.private.catalogStatus(r.Context(), src.ID)
	if err != nil {
		writeError(w, 503, "SOURCE_STORE_UNAVAILABLE", "Could not load catalog status")
		return
	}
	if status.Status != "syncing" {
		a.private.startCatalogSync(src)
	}
	status.Status = "syncing"
	status.ErrorCode = ""
	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, http.StatusAccepted, status)
}

func (a *API) catalogSource(w http.ResponseWriter, r *http.Request) (*source, bool) {
	src, ok, err := a.private.sourceFor(r.Context(), requestSession(r), r.PathValue("id"))
	if !ok {
		if errors.Is(err, errSourceSessionRevoked) {
			writeError(w, 401, "UNAUTHENTICATED", "Authentication required")
		} else if err != nil {
			writeError(w, 503, "SOURCE_STORE_UNAVAILABLE", "Could not load source")
		} else {
			writeError(w, 404, "NOT_FOUND", "Source not found")
		}
		return nil, false
	}
	return src, true
}

func (s *privateState) catalogStatus(ctx context.Context, sourceID string) (CatalogStatus, error) {
	status := CatalogStatus{Status: "empty", Counts: emptyCounts()}
	if s.cfg.DB == nil {
		return status, nil
	}
	var updated *time.Time
	var countsJSON []byte
	var state string
	err := s.cfg.DB.QueryRow(ctx, `SELECT status,updated_at,COALESCE(error_code,''),counts FROM source_catalog_state WHERE source_id=$1`, sourceID).Scan(&state, &updated, &status.ErrorCode, &countsJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return status, nil
	}
	if err != nil {
		return status, errSourceStore
	}
	status.Status = state
	status.UpdatedAt = updated
	if len(countsJSON) > 0 {
		_ = json.Unmarshal(countsJSON, &status.Counts)
	}
	if status.Counts == nil {
		status.Counts = emptyCounts()
	}
	if status.Status == "syncing" {
		var stale bool
		if err := s.cfg.DB.QueryRow(ctx, `SELECT now()-COALESCE(sync_started_at,now()) > interval '11 minutes' FROM source_catalog_state WHERE source_id=$1`, sourceID).Scan(&stale); err == nil && stale {
			_, _ = s.cfg.DB.Exec(ctx, `UPDATE source_catalog_state SET status='error',error_code='interrupted',sync_started_at=NULL WHERE source_id=$1 AND status='syncing'`, sourceID)
			status.Status, status.ErrorCode = "error", "interrupted"
		}
	}
	return status, nil
}

func (s *privateState) startCatalogSync(src *source) {
	ctx, cancel := context.WithTimeout(context.Background(), maxCatalogRuntime)
	job := &catalogSyncJob{cancel: cancel}
	s.mu.Lock()
	if _, exists := s.catalogSync[src.ID]; exists || s.sources[src.ID] != src {
		s.mu.Unlock()
		cancel()
		return
	}
	s.catalogSync[src.ID] = job
	s.mu.Unlock()
	go func() {
		defer func() {
			cancel()
			s.mu.Lock()
			if s.catalogSync[src.ID] == job {
				delete(s.catalogSync, src.ID)
			}
			s.mu.Unlock()
		}()
		if s.cfg.DB != nil {
			if err := s.markCatalogSyncing(ctx, src.ID); err != nil {
				return
			}
		}
		release, acquired := s.acquireCatalogAccount(ctx, src.Fingerprint)
		if !acquired {
			if ctx.Err() != nil {
				s.markCatalogError(src.ID, "account_queue_timeout")
			}
			return
		}
		defer release()
		_ = s.syncCatalog(ctx, src)
	}()
}

func (s *privateState) acquireCatalogAccount(ctx context.Context, fingerprint string) (func(), bool) {
	s.mu.Lock()
	slot := s.catalogAccounts[fingerprint]
	if slot == nil {
		slot = make(chan struct{}, 1)
		s.catalogAccounts[fingerprint] = slot
	}
	s.mu.Unlock()
	select {
	case slot <- struct{}{}:
		select {
		case s.catalogSlots <- struct{}{}:
		case <-ctx.Done():
			<-slot
			return nil, false
		}
		return func() {
			<-s.catalogSlots
			<-slot
		}, true
	case <-ctx.Done():
		return nil, false
	}
}

func (s *privateState) markCatalogSyncing(ctx context.Context, sourceID string) error {
	_, err := s.cfg.DB.Exec(ctx, `INSERT INTO source_catalog_state(source_id,status,sync_started_at) SELECT id,'syncing',now() FROM sources WHERE id=$1 ON CONFLICT(source_id) DO UPDATE SET status='syncing',error_code=NULL,sync_started_at=now()`, sourceID)
	if err != nil {
		return errSourceStore
	}
	return nil
}

func (s *privateState) markCatalogError(sourceID, code string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.cfg.DB.Exec(ctx, `UPDATE source_catalog_state SET status='error',error_code=$2,sync_started_at=NULL WHERE source_id=$1`, sourceID, code)
}

func (s *privateState) syncCatalog(ctx context.Context, src *source) error {
	if s.cfg.DB == nil {
		return errSourceStore
	}
	// The deployment is one API replica; the lock also protects two concurrent requests in-process.
	var generation int64
	if err := s.cfg.DB.QueryRow(ctx, `INSERT INTO source_catalog_state(source_id,status,counts,sync_started_at) SELECT id,'syncing','{"live":0,"movie":0,"series":0}'::jsonb,now() FROM sources WHERE id=$1 ON CONFLICT(source_id) DO UPDATE SET status='syncing',error_code=NULL,sync_started_at=now() RETURNING active_generation+1`, src.ID).Scan(&generation); err != nil {
		return errSourceStore
	}
	_, _ = s.cfg.DB.Exec(ctx, `DELETE FROM source_catalog_entries WHERE source_id=$1 AND generation=$2`, src.ID, generation)
	_, _ = s.cfg.DB.Exec(ctx, `DELETE FROM source_catalog_categories WHERE source_id=$1 AND generation=$2`, src.ID, generation)
	_, _ = s.cfg.DB.Exec(ctx, `DELETE FROM source_catalog_languages WHERE source_id=$1 AND generation=$2`, src.ID, generation)
	counts := emptyCounts()
	stage := "catalog"
	fail := func(err error) error {
		code := "provider_error"
		if errors.Is(err, errTooLarge) {
			code = "too_large"
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			code = "timeout"
		}
		if errors.Is(err, errSourceStore) {
			code = "database"
		}
		if errors.Is(err, errCatalogNetwork) {
			code = "network"
		}
		if errors.Is(err, errCatalogHTTP) {
			code = "upstream_http"
		}
		if errors.Is(err, errCatalogMalformed) {
			code = "malformed"
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = s.cfg.DB.Exec(cleanupCtx, `DELETE FROM source_catalog_entries WHERE source_id=$1 AND generation=$2`, src.ID, generation)
		_, _ = s.cfg.DB.Exec(cleanupCtx, `DELETE FROM source_catalog_categories WHERE source_id=$1 AND generation=$2`, src.ID, generation)
		_, _ = s.cfg.DB.Exec(cleanupCtx, `DELETE FROM source_catalog_languages WHERE source_id=$1 AND generation=$2`, src.ID, generation)
		_, _ = s.cfg.DB.Exec(cleanupCtx, `UPDATE source_catalog_state SET status='error',error_code=$2,sync_started_at=NULL WHERE source_id=$1`, src.ID, stage+"_"+code)
		return err
	}
	for _, kind := range []string{"live", "movie", "series"} {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		catAction, streamAction, idField := "get_live_categories", "get_live_streams", "stream_id"
		if kind == "movie" {
			catAction, streamAction = "get_vod_categories", "get_vod_streams"
		}
		if kind == "series" {
			catAction, streamAction, idField = "get_series_categories", "get_series", "series_id"
		}
		stage = kind + "_categories"
		categories, err := s.providerList(ctx, src, catAction)
		if err != nil {
			return fail(err)
		}
		categoryNames := make(map[string]string, len(categories))
		for _, row := range categories {
			id, name := toString(row["category_id"]), toString(row["category_name"])
			if !numericID(id) || name == "" {
				continue
			}
			categoryNames[id] = name
			if _, err = s.cfg.DB.Exec(ctx, `INSERT INTO source_catalog_categories(source_id,generation,kind,id,name) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, src.ID, generation, kind, id, name); err != nil {
				return fail(errSourceStore)
			}
		}
		batch := make([]catalogEntry, 0, 500)
		seenIDs := make(map[string]struct{})
		langs := map[string]struct{}{}
		flush := func() error {
			if len(batch) == 0 {
				return nil
			}
			tx, e := s.cfg.DB.Begin(ctx)
			if e != nil {
				return e
			}
			defer tx.Rollback(context.Background())
			for _, entry := range batch {
				_, e = tx.Exec(ctx, `INSERT INTO source_catalog_entries(source_id,generation,kind,id,title,title_search,category_id,language,extension,content_key,image_ciphertext,release_year,tmdb_id,imdb_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) ON CONFLICT(source_id,generation,kind,id) DO NOTHING`, src.ID, generation, kind, entry.ID, entry.Title, strings.ToLower(entry.Title), entry.CategoryID, entry.Language, entry.Extension, entry.ContentKey, nilIfEmpty(entry.ImageCiphertext), entry.ReleaseYear, entry.TMDBID, entry.IMDBID)
				if e != nil {
					return e
				}
			}
			if e = tx.Commit(ctx); e != nil {
				return e
			}
			batch = batch[:0]
			return nil
		}
		stage = kind + "_items"
		rowCount := 0
		err = s.providerEach(ctx, src, streamAction, func(row map[string]any) error {
			rowCount++
			if rowCount > maxCatalogItems {
				return errTooLarge
			}
			id, title := toString(row[idField]), toString(row["name"])
			if !numericID(id) || title == "" {
				return nil
			}
			if _, exists := seenIDs[id]; exists {
				return nil
			}
			seenIDs[id] = struct{}{}
			catID := toString(row["category_id"])
			if !numericID(catID) {
				catID = ""
			}
			language := inferCatalogLanguage(title, categoryNames[catID])
			langs[language] = struct{}{}
			entry := catalogEntry{ID: id, Title: title, CategoryID: catID, Language: language, Extension: validatedExtension(toString(row["container_extension"])), ContentKey: stableContentKey(s.cfg.IdentityKey, src.AccountKey, kind, id)}
			entry.ReleaseYear, entry.TMDBID, entry.IMDBID = providerVODIdentity(row, title)
			imageURL := toString(row["stream_icon"])
			if imageURL == "" {
				imageURL = toString(row["cover"])
			}
			if imageURL != "" {
				entry.ImageCiphertext, _ = encryptCatalogImage(s.cfg.EncryptionKey, src.UserID, src.ID, kind, id, imageURL)
			}
			batch = append(batch, entry)
			counts[kind]++
			if counts[kind] > maxCatalogItems {
				return errTooLarge
			}
			if len(batch) == cap(batch) {
				if err := flush(); err != nil {
					return errSourceStore
				}
			}
			return nil
		})
		if err != nil {
			return fail(err)
		}
		if err := flush(); err != nil {
			return fail(errSourceStore)
		}
		langIDs := make([]string, 0, len(langs))
		for value := range langs {
			langIDs = append(langIDs, value)
		}
		sort.Strings(langIDs)
		for _, value := range langIDs {
			if _, err := s.cfg.DB.Exec(ctx, `INSERT INTO source_catalog_languages(source_id,generation,kind,id,name) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, src.ID, generation, kind, value, languageName(value)); err != nil {
				return fail(errSourceStore)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	tx, err := s.cfg.DB.Begin(ctx)
	if err != nil {
		return fail(errSourceStore)
	}
	var owner string
	if err = tx.QueryRow(ctx, `SELECT user_id FROM sources WHERE id=$1 FOR UPDATE`, src.ID).Scan(&owner); err != nil || owner != src.UserID {
		_ = tx.Rollback(context.Background())
		return fail(errSourceRemoved)
	}
	var previous int64
	if err = tx.QueryRow(ctx, `SELECT active_generation FROM source_catalog_state WHERE source_id=$1 FOR UPDATE`, src.ID).Scan(&previous); err != nil {
		_ = tx.Rollback(context.Background())
		return fail(errSourceStore)
	}
	countsJSON, _ := json.Marshal(counts)
	if _, err = tx.Exec(ctx, `UPDATE source_catalog_state SET status='ready',active_generation=$2,updated_at=now(),error_code=NULL,counts=$3::jsonb,sync_started_at=NULL WHERE source_id=$1`, src.ID, generation, string(countsJSON)); err != nil {
		_ = tx.Rollback(context.Background())
		return fail(errSourceStore)
	}
	if previous > 0 {
		if _, err = tx.Exec(ctx, `DELETE FROM source_catalog_entries WHERE source_id=$1 AND generation=$2`, src.ID, previous); err != nil {
			_ = tx.Rollback(context.Background())
			return fail(errSourceStore)
		}
		if _, err = tx.Exec(ctx, `DELETE FROM source_catalog_categories WHERE source_id=$1 AND generation=$2`, src.ID, previous); err != nil {
			_ = tx.Rollback(context.Background())
			return fail(errSourceStore)
		}
		if _, err = tx.Exec(ctx, `DELETE FROM source_catalog_languages WHERE source_id=$1 AND generation=$2`, src.ID, previous); err != nil {
			_ = tx.Rollback(context.Background())
			return fail(errSourceStore)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		_ = tx.Rollback(context.Background())
		return fail(errSourceStore)
	}
	return nil
}

func nilIfEmpty(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func (s *privateState) providerList(ctx context.Context, src *source, action string) ([]map[string]any, error) {
	rows := make([]map[string]any, 0, 32)
	err := s.providerEach(ctx, src, action, func(row map[string]any) error {
		rows = append(rows, row)
		if len(rows) > maxCatalogCategories {
			return errTooLarge
		}
		return nil
	})
	return rows, err
}

func (s *privateState) providerEach(ctx context.Context, src *source, action string, each func(map[string]any) error) error {
	u, err := url.Parse(src.Base + "/player_api.php")
	if err != nil {
		return errProvider
	}
	q := url.Values{"username": {src.Username}, "password": {src.Password}, "action": {action}}
	u.RawQuery = q.Encode()
	requestCtx, cancel := context.WithTimeout(ctx, catalogRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return errProvider
	}
	client := s.providerCatalog
	if client == nil {
		client = s.control
	}
	resp, err := client.Do(req)
	if err != nil {
		if requestCtx.Err() != nil {
			return requestCtx.Err()
		}
		return errCatalogNetwork
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errCatalogHTTP
	}
	limited := &io.LimitedReader{R: resp.Body, N: maxCatalogResponse + 1}
	dec := json.NewDecoder(limited)
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		if limited.N == 0 {
			return errTooLarge
		}
		return errCatalogMalformed
	}
	if tok == json.Delim('{') {
		var obj map[string]any
		if err := dec.Decode(&obj); err != nil {
			if limited.N == 0 {
				return errTooLarge
			}
			return errCatalogMalformed
		}
		return errCatalogMalformed
	}
	if tok != json.Delim('[') {
		return errCatalogMalformed
	}
	count := 0
	for dec.More() {
		var row map[string]any
		if err := dec.Decode(&row); err != nil {
			if limited.N == 0 {
				return errTooLarge
			}
			return errCatalogMalformed
		}
		if row != nil {
			count++
			if count > maxCatalogItems {
				return errTooLarge
			}
			if err := each(row); err != nil {
				return err
			}
		}
	}
	if _, err := dec.Token(); err != nil {
		if limited.N == 0 {
			return errTooLarge
		}
		return errCatalogMalformed
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if limited.N == 0 {
			return errTooLarge
		}
		return errCatalogMalformed
	}
	if limited.N == 0 {
		return errTooLarge
	}
	return nil
}

func (a *API) getLanguages(w http.ResponseWriter, r *http.Request) {
	src, ok := a.catalogSource(w, r)
	if !ok {
		return
	}
	kind := r.URL.Query().Get("type")
	if !validCatalogType(kind) {
		writeError(w, 400, "INVALID_INPUT", "Invalid catalog type")
		return
	}
	rows, err := a.private.dbLanguages(r.Context(), src.ID, kind)
	if err != nil {
		writeError(w, 503, "SOURCE_STORE_UNAVAILABLE", "Could not load catalog languages")
		return
	}
	writeJSON(w, 200, map[string]any{"languages": rows})
}

func (s *privateState) activeGeneration(ctx context.Context, sourceID string) (int64, error) {
	var generation int64
	err := s.cfg.DB.QueryRow(ctx, `SELECT active_generation FROM source_catalog_state WHERE source_id=$1`, sourceID).Scan(&generation)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return generation, err
}

func (s *privateState) dbCatalogItem(ctx context.Context, src *source, kind, itemID string) (CatalogItem, bool, error) {
	var item CatalogItem
	var hasImage bool
	err := s.cfg.DB.QueryRow(ctx, `SELECT e.id,e.title,e.category_id,e.language,e.extension,e.content_key,(e.image_ciphertext IS NOT NULL) FROM source_catalog_state s JOIN source_catalog_entries e ON e.source_id=s.source_id AND e.generation=s.active_generation WHERE s.source_id=$1 AND e.kind=$2 AND e.id=$3`, src.ID, kind, itemID).Scan(&item.ID, &item.Title, &item.CategoryID, &item.Language, &item.ContainerExtension, &item.ContentKey, &hasImage)
	if errors.Is(err, pgx.ErrNoRows) {
		return CatalogItem{}, false, nil
	}
	if err != nil {
		return CatalogItem{}, false, errSourceStore
	}
	item.Kind = kind
	if hasImage {
		item.ImageURL = fmt.Sprintf("/api/v1/sources/%s/images/%s/%s", url.PathEscape(src.ID), kind, url.PathEscape(item.ID))
	}
	return item, true, nil
}

func (s *privateState) dbCategories(ctx context.Context, sourceID, kind, language string) ([]category, error) {
	rows, err := s.cfg.DB.Query(ctx, `SELECT c.id,c.name FROM source_catalog_state s JOIN source_catalog_categories c ON c.source_id=s.source_id AND c.generation=s.active_generation WHERE s.source_id=$1 AND c.kind=$2 AND ($3='' OR EXISTS (SELECT 1 FROM source_catalog_entries e WHERE e.source_id=c.source_id AND e.generation=c.generation AND e.kind=c.kind AND e.category_id=c.id AND e.language=$3)) ORDER BY c.name,c.id`, sourceID, kind, language)
	if err != nil {
		return nil, errSourceStore
	}
	defer rows.Close()
	out := []category{}
	for rows.Next() {
		var x category
		if rows.Scan(&x.ID, &x.Name) != nil {
			return nil, errSourceStore
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *privateState) dbLanguages(ctx context.Context, sourceID, kind string) ([]map[string]string, error) {
	rows, err := s.cfg.DB.Query(ctx, `SELECT l.id,l.name FROM source_catalog_state s JOIN source_catalog_languages l ON l.source_id=s.source_id AND l.generation=s.active_generation WHERE s.source_id=$1 AND l.kind=$2 ORDER BY l.name,l.id`, sourceID, kind)
	if err != nil {
		return nil, errSourceStore
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var id, name string
		if rows.Scan(&id, &name) != nil {
			return nil, errSourceStore
		}
		out = append(out, map[string]string{"id": id, "name": name})
	}
	return out, rows.Err()
}

func (a *API) getCatalogImage(w http.ResponseWriter, r *http.Request) {
	src, ok := a.catalogSource(w, r)
	if !ok {
		return
	}
	kind, id := r.PathValue("kind"), r.PathValue("itemId")
	if !validCatalogType(kind) || !numericID(id) {
		writeError(w, 404, "NOT_FOUND", "Image not found")
		return
	}
	var gen int64
	var sealed []byte
	err := a.cfg.DB.QueryRow(r.Context(), `SELECT s.active_generation,e.image_ciphertext FROM source_catalog_state s JOIN source_catalog_entries e ON e.source_id=s.source_id AND e.generation=s.active_generation WHERE s.source_id=$1 AND e.kind=$2 AND e.id=$3`, src.ID, kind, id).Scan(&gen, &sealed)
	if err != nil || len(sealed) == 0 {
		writeError(w, 404, "NOT_FOUND", "Image not found")
		return
	}
	cacheKey := src.UserID + "\x00" + src.ID + "\x00" + strconv.FormatInt(gen, 10) + "\x00" + kind + "\x00" + id
	body, contentType, err := a.private.cachedImage(r.Context(), src.ID, cacheKey, func() ([]byte, string, error) { return a.fetchCatalogImage(r, src, kind, id, sealed) })
	if err != nil {
		writeError(w, 502, "IMAGE_UNAVAILABLE", "Image unavailable")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.WriteHeader(200)
	_, _ = w.Write(body)
}

func (a *API) fetchCatalogImage(r *http.Request, src *source, kind, id string, sealed []byte) ([]byte, string, error) {
	raw, err := decryptCatalogImage(a.cfg.EncryptionKey, src.UserID, src.ID, kind, id, sealed)
	if err != nil {
		return nil, "", errSourceStore
	}
	u, err := url.Parse(raw)
	if err != nil || !validImageURL(u) {
		return nil, "", errSourceStore
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", errCatalogNetwork
	}
	resp, err := a.private.images.Do(req)
	if err != nil {
		return nil, "", errCatalogNetwork
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", errCatalogHTTP
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, imageMaxBytes+1))
	if err != nil || len(body) == 0 || len(body) > imageMaxBytes {
		return nil, "", errTooLarge
	}
	typ := verifiedImageType(resp.Header.Get("Content-Type"), body)
	if typ == "" {
		return nil, "", errCatalogMalformed
	}
	return body, typ, nil
}

func validImageURL(u *url.URL) bool {
	return u != nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil && u.Fragment == "" && (u.Port() == "" || u.Port() == "80" || u.Port() == "443")
}

func (s *privateState) newImageClient() *http.Client {
	transport := &http.Transport{Proxy: nil, ForceAttemptHTTP2: true, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 8 * time.Second, IdleConnTimeout: 20 * time.Second, DialContext: s.dialImagePublic}
	return &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !validImageURL(req.URL) {
			return http.ErrUseLastResponse
		}
		return nil
	}}
}
func (s *privateState) dialImagePublic(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || (port != "80" && port != "443") {
		return nil, errors.New("image host rejected")
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	var chosen netip.Addr
	for _, ip := range ips {
		if !publicIP(ip) {
			return nil, errors.New("image host rejected")
		}
		if !chosen.IsValid() {
			chosen = ip
		}
	}
	if !chosen.IsValid() {
		return nil, errors.New("image host rejected")
	}
	return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(chosen.String(), port))
}

func verifiedImageType(header string, body []byte) string {
	declared, _, err := mime.ParseMediaType(header)
	if err != nil {
		return ""
	}
	if len(body) >= 8 && bytes.Equal(body[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		if declared == "image/png" {
			return declared
		}
	}
	if len(body) >= 3 && body[0] == 0xff && body[1] == 0xd8 && body[2] == 0xff {
		if declared == "image/jpeg" {
			return declared
		}
	}
	if len(body) >= 12 && string(body[:4]) == "RIFF" && string(body[8:12]) == "WEBP" {
		if declared == "image/webp" {
			return declared
		}
	}
	if len(body) >= 6 && (string(body[:6]) == "GIF87a" || string(body[:6]) == "GIF89a") {
		if declared == "image/gif" {
			return declared
		}
	}
	return ""
}

func encryptCatalogImage(key []byte, userID, sourceID, kind, itemID, raw string) ([]byte, error) {
	if len(key) != 32 || len(raw) > 4096 {
		return nil, errSourceStore
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errSourceStore
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errSourceStore
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, errSourceStore
	}
	aad := []byte("walactv:image:v1\x00" + userID + "\x00" + sourceID + "\x00" + kind + "\x00" + itemID)
	return append(nonce, aead.Seal(nil, nonce, []byte(raw), aad)...), nil
}
func decryptCatalogImage(key []byte, userID, sourceID, kind, itemID string, sealed []byte) (string, error) {
	if len(key) != 32 {
		return "", errSourceStore
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", errSourceStore
	}
	aead, err := cipher.NewGCM(block)
	if err != nil || len(sealed) < aead.NonceSize()+aead.Overhead() {
		return "", errSourceStore
	}
	aad := []byte("walactv:image:v1\x00" + userID + "\x00" + sourceID + "\x00" + kind + "\x00" + itemID)
	plain, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], aad)
	if err != nil || len(plain) > 4096 {
		return "", errSourceStore
	}
	return string(plain), nil
}

func inferCatalogLanguage(title, categoryName string) string {
	labels := []string{strings.ToLower(strings.TrimSpace(title)), strings.ToLower(strings.TrimSpace(categoryName))}
	for _, label := range labels {
		if lang := explicitLanguageTag(label); lang != "" {
			return lang
		}
	}
	// Full language names in category labels are useful evidence; ordinary title
	// words are deliberately ignored (for example, “French Connection”).
	if categoryName != "" {
		tokens := languageTokens(strings.ToLower(categoryName))
		for _, language := range []struct {
			id    string
			names []string
		}{
			{"es", []string{"español", "spanish", "castellano", "latino", "latina", "latam"}},
			{"en", []string{"english", "inglés", "ingles"}}, {"fr", []string{"français", "francais", "french", "francés", "frances"}},
			{"de", []string{"deutsch", "german", "alemán", "aleman"}}, {"it", []string{"italiano", "italian"}},
			{"pt", []string{"português", "portugues", "portuguese"}}, {"ru", []string{"русский", "russian"}},
			{"ar", []string{"arabic", "árabe", "arabe"}}, {"tr", []string{"turkish", "turco"}},
		} {
			for _, name := range language.names {
				for i, token := range tokens {
					if token != name {
						continue
					}
					// Require a standalone label or a familiar media/audio qualifier.
					if len(tokens) == 1 || i == 0 && isMediaQualifier(tokens[1]) || i > 0 && isMediaQualifier(tokens[i-1]) || i == len(tokens)-1 && i > 0 && (tokens[i-1] == "in" || tokens[i-1] == "de" || tokens[i-1] == "audio") {
						return language.id
					}
				}
			}
		}
	}
	return "und"
}

func explicitLanguageTag(label string) string {
	for _, lang := range []string{"es", "en", "fr", "de", "it", "pt", "ru", "ar", "tr"} {
		for _, open := range []string{"[", "("} {
			for _, close := range []string{"]", ")"} {
				if strings.Contains(label, open+lang+close) {
					return lang
				}
			}
		}
		for _, suffix := range []string{"|", "-", ":", "_"} {
			if strings.HasPrefix(label, lang+suffix) {
				return lang
			}
		}
	}
	// Bracketed long names are explicit labels too.
	for _, name := range []struct{ id, label string }{{"es", "spanish"}, {"es", "español"}, {"en", "english"}, {"fr", "french"}, {"de", "german"}, {"it", "italian"}, {"pt", "portuguese"}} {
		for _, open := range []string{"[", "("} {
			for _, close := range []string{"]", ")"} {
				if strings.Contains(label, open+name.label+close) {
					return name.id
				}
			}
		}
	}
	return ""
}

func languageTokens(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}
func isMediaQualifier(v string) bool {
	switch v {
	case "movies", "movie", "films", "film", "cinema", "cinéma", "películas", "peliculas", "series", "channels", "channel", "tv", "live", "audio", "dubbed", "subbed", "vo", "v.o.":
		return true
	}
	return false
}
func languageName(id string) string {
	names := map[string]string{"es": "Español", "en": "Inglés", "fr": "Francés", "de": "Alemán", "it": "Italiano", "pt": "Portugués", "ru": "Ruso", "ar": "Árabe", "tr": "Turco", "und": "Desconocido"}
	if v := names[id]; v != "" {
		return v
	}
	return id
}

func parseCatalogPage(q url.Values) (string, string, string, int, int, error) {
	kind, cat, lang := q.Get("type"), q.Get("categoryId"), q.Get("language")
	if !validCatalogType(kind) || cat != "" && !numericID(cat) || lang != "" && (lang != "und" && !validLanguageID(lang)) {
		return "", "", "", 0, 0, errInvalidInput
	}
	search := strings.TrimSpace(q.Get("search"))
	if utf8.RuneCountInString(search) > 150 {
		return "", "", "", 0, 0, errInvalidInput
	}
	page, pageErr := parseStrictPositive(q.Get("page"), 1)
	size, sizeErr := parseStrictPositive(q.Get("pageSize"), 40)
	if pageErr != nil || sizeErr != nil || size > 100 || int64(page-1) > 1_000_000 {
		return "", "", "", 0, 0, errInvalidInput
	}
	return kind, cat, lang, page, size, nil
}
func parseStrictPositive(raw string, fallback int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return 0, errInvalidInput
	}
	return v, nil
}
func validLanguageID(v string) bool {
	if len(v) != 2 {
		return false
	}
	for _, r := range v {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}
func validCatalogLanguage(v string) bool {
	return v == "" || v == "und" || validLanguageID(v)
}
func escapeLike(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `%`, `\%`)
	return strings.ReplaceAll(v, `_`, `\_`)
}

func (a *API) queryCatalogItems(ctx context.Context, sourceID, kind, cat, lang, search string, page, pageSize int) ([]CatalogItem, int, error) {
	tx, err := a.cfg.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, errSourceStore
	}
	defer tx.Rollback(context.Background())
	var g int64
	err = tx.QueryRow(ctx, `SELECT active_generation FROM source_catalog_state WHERE source_id=$1`, sourceID).Scan(&g)
	if errors.Is(err, pgx.ErrNoRows) || g == 0 {
		return []CatalogItem{}, 0, nil
	}
	if err != nil {
		return nil, 0, errSourceStore
	}
	pattern := "%" + escapeLike(strings.ToLower(search)) + "%"
	filters := `source_id=$1 AND generation=$2 AND kind=$3 AND ($4='' OR category_id=$4) AND ($5='' OR language=$5) AND ($6='' OR title_search LIKE $7 ESCAPE E'\\')`
	args := []any{sourceID, g, kind, cat, lang, search, pattern}
	var total int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM source_catalog_entries WHERE `+filters, args...).Scan(&total); err != nil {
		return nil, 0, errSourceStore
	}
	offset := (page - 1) * pageSize
	rows, err := tx.Query(ctx, `SELECT id,title,category_id,language,extension,content_key,(image_ciphertext IS NOT NULL) FROM source_catalog_entries WHERE `+filters+` ORDER BY title_search,id LIMIT $8 OFFSET $9`, append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, errSourceStore
	}
	defer rows.Close()
	out := []CatalogItem{}
	for rows.Next() {
		var item CatalogItem
		var hasImage bool
		if err := rows.Scan(&item.ID, &item.Title, &item.CategoryID, &item.Language, &item.ContainerExtension, &item.ContentKey, &hasImage); err != nil {
			return nil, 0, errSourceStore
		}
		item.Kind = kind
		if hasImage {
			item.ImageURL = fmt.Sprintf("/api/v1/sources/%s/images/%s/%s", url.PathEscape(sourceID), kind, url.PathEscape(item.ID))
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, errSourceStore
	}
	return out, total, nil
}
