package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

const metadataLanguage = "es-ES"

type metadataPerson struct {
	Name      string `json:"name"`
	Character string `json:"character,omitempty"`
	Job       string `json:"job,omitempty"`
}

type metadataGenre struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type metadataRecord struct {
	ID           int64            `json:"id"`
	Kind         string           `json:"kind"`
	Title        string           `json:"title,omitempty"`
	Original     string           `json:"originalTitle,omitempty"`
	Overview     string           `json:"overview,omitempty"`
	ReleaseDate  string           `json:"releaseDate,omitempty"`
	Year         int              `json:"year,omitempty"`
	Genres       []metadataGenre  `json:"genres,omitempty"`
	Runtime      int              `json:"runtime,omitempty"`
	Rating       *float64         `json:"rating,omitempty"`
	RatingSource string           `json:"ratingSource,omitempty"`
	PosterURL    string           `json:"posterUrl,omitempty"`
	BackdropURL  string           `json:"backdropUrl,omitempty"`
	Credits      []metadataPerson `json:"credits,omitempty"`
}

type metadataResponse struct {
	Status   string          `json:"status"`
	Language string          `json:"language,omitempty"`
	Source   string          `json:"source,omitempty"`
	Metadata *metadataRecord `json:"metadata,omitempty"`
	Season   *metadataSeason `json:"season,omitempty"`
}

type metadataSeason struct {
	Number   int               `json:"number"`
	Name     string            `json:"name,omitempty"`
	Overview string            `json:"overview,omitempty"`
	Episodes []metadataEpisode `json:"episodes"`
}
type metadataEpisode struct {
	Number   int    `json:"number"`
	Name     string `json:"name,omitempty"`
	Overview string `json:"overview,omitempty"`
	StillURL string `json:"stillUrl,omitempty"`
	AirDate  string `json:"airDate,omitempty"`
}

type metadataService struct {
	cfg            Config
	client         *http.Client
	mu             sync.Mutex
	flights        map[string]chan struct{}
	coldCalls      map[string]*metadataCall
	canonicalCalls map[string]*metadataCall
	active         chan struct{}
	queued         chan struct{}
	perUser        map[string][]time.Time
	rate           <-chan time.Time
}

type metadataCall struct {
	done    chan struct{}
	result  metadataResponse
	public  metadataPublicData
	waiters int
}

// metadataPublicData contains only provider/cache values. It must never carry
// an IPTV identity, request status, user ID, or per-caller authorization result.
type metadataPublicData struct {
	record *metadataRecord
	season *metadataSeason
	stale  bool
	err    error
}

func newMetadataService(cfg Config) *metadataService {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	return &metadataService{cfg: cfg, client: client, flights: map[string]chan struct{}{}, coldCalls: map[string]*metadataCall{}, canonicalCalls: map[string]*metadataCall{}, active: make(chan struct{}, 2), queued: make(chan struct{}, 34), perUser: map[string][]time.Time{}, rate: ticker.C}
}

var yearTail = regexp.MustCompile(`\s*\(?((?:18|19|20|21)\d{2})\)?\s*$`)
var qualityTag = regexp.MustCompile(`(?i)\b(4k|2160p|1080p|720p|480p|hdtv|web[- ]?dl|blu[- ]?ray|brrip|dvdrip|hdrip|x264|x265|hevc|aac|latino|castellano|español|espanol|dual audio|subtitulado)\b`)
var imdbPattern = regexp.MustCompile(`^tt[0-9]{7,10}$`)

func normalizeMetadataTitle(v string) string {
	v = qualityTag.ReplaceAllString(v, " ")
	v = yearTail.ReplaceAllString(v, "")
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(v))), " ")
}

func providerVODIdentity(row map[string]any, title string) (*int, *int64, *string) {
	info, _ := row["info"].(map[string]any)
	field := func(names ...string) string {
		for _, name := range names {
			if v := strings.TrimSpace(toString(row[name])); v != "" {
				return v
			}
			if info != nil {
				if v := strings.TrimSpace(toString(info[name])); v != "" {
					return v
				}
			}
		}
		return ""
	}
	yearText := field("year", "release_year")
	if yearText == "" {
		if m := yearTail.FindStringSubmatch(qualityTag.ReplaceAllString(title, " ")); len(m) == 2 {
			yearText = m[1]
		}
	}
	var year *int
	if n, err := strconv.Atoi(yearText); err == nil && n >= 1888 && n <= time.Now().Year()+2 {
		year = &n
	}
	var tmdbID *int64
	if n, err := strconv.ParseInt(field("tmdb_id", "tmdb"), 10, 64); err == nil && n > 0 {
		tmdbID = &n
	}
	var imdbID *string
	if v := field("imdb_id", "imdb"); imdbPattern.MatchString(v) {
		imdbID = &v
	}
	return year, tmdbID, imdbID
}

type metadataIdentity struct {
	userID, sourceID, kind, itemID, title, fingerprint string
	year                                               *int
	tmdbID                                             *int64
	imdbID                                             *string
}

func (a *API) getOnDemandMetadata(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	identity, ok, err := a.metadataIdentityFromRequest(w, r, kind)
	if err != nil || !ok {
		return
	}
	result := a.metadata.get(r.Context(), identity)
	writeJSON(w, http.StatusOK, result)
}

func (a *API) metadataIdentityFromRequest(w http.ResponseWriter, r *http.Request, kind string) (metadataIdentity, bool, error) {
	src, ok := a.catalogSource(w, r)
	if !ok {
		return metadataIdentity{}, false, errors.New("not_found")
	}
	id := r.PathValue("itemId")
	if !numericID(id) || (kind != "movie" && kind != "series") {
		writeError(w, 404, "NOT_FOUND", "Item not found")
		return metadataIdentity{}, false, errors.New("not_found")
	}
	var v metadataIdentity
	var year *int
	var tmdbID *int64
	var imdbID *string
	err := a.cfg.DB.QueryRow(r.Context(), `SELECT e.title,e.release_year,e.tmdb_id,e.imdb_id FROM source_catalog_state s JOIN source_catalog_entries e ON e.source_id=s.source_id AND e.generation=s.active_generation WHERE s.source_id=$1 AND e.kind=$2 AND e.id=$3`, src.ID, kind, id).Scan(&v.title, &year, &tmdbID, &imdbID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "NOT_FOUND", "Item not found")
		return metadataIdentity{}, false, errors.New("not_found")
	}
	if err != nil {
		writeError(w, 503, "SOURCE_STORE_UNAVAILABLE", "Could not load catalog item")
		return metadataIdentity{}, false, err
	}
	v.userID = requestSession(r).User.ID
	v.sourceID = src.ID
	v.kind = kind
	v.itemID = id
	v.year = year
	v.tmdbID = tmdbID
	v.imdbID = imdbID
	v.fingerprint = metadataFingerprint(v.title, year, tmdbID, imdbID)
	return v, true, nil
}

func metadataFingerprint(title string, year *int, tmdbID *int64, imdbID *string) string {
	// Hash values, never pointer formatting: pointer addresses vary across requests
	// and would prevent persisted decisions from matching the same catalog item.
	yearValue, tmdbValue, imdbValue := "", "", ""
	if year != nil {
		yearValue = strconv.Itoa(*year)
	}
	if tmdbID != nil {
		tmdbValue = strconv.FormatInt(*tmdbID, 10)
	}
	if imdbID != nil {
		imdbValue = *imdbID
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{normalizeMetadataTitle(title), yearValue, tmdbValue, imdbValue}, "\x00")))
	return hex.EncodeToString(sum[:])
}

func metadataRecordMatches(record metadataRecord, identity metadataIdentity) bool {
	want := normalizeMetadataTitle(identity.title)
	if want == "" || (normalizeMetadataTitle(record.Title) != want && normalizeMetadataTitle(record.Original) != want) {
		return false
	}
	return identity.year == nil || record.Year == *identity.year
}

func (a *API) getOnDemandSeasonMetadata(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("itemId")
	season, err := strconv.Atoi(r.PathValue("season"))
	if !numericID(id) || err != nil || season < 0 || season > 99 {
		writeError(w, 404, "NOT_FOUND", "Season not found")
		return
	}
	src, ok := a.catalogSource(w, r)
	if !ok {
		return
	}
	r.SetPathValue("kind", "series")
	identity, ok, e := a.metadataIdentityFromRequest(w, r, "series")
	if e != nil || !ok {
		return
	}
	// The season must exist in the active IPTV episode set before an external request is permitted.
	data, e := a.private.episodes(r.Context(), src, id)
	if e != nil {
		writeError(w, 503, "SOURCE_STORE_UNAVAILABLE", "Could not load episodes")
		return
	}
	found := false
	for _, ep := range data.episodes[id] {
		if ep.Season == season {
			found = true
			break
		}
	}
	if !found {
		writeError(w, 404, "NOT_FOUND", "Season not found")
		return
	}
	writeJSON(w, 200, a.metadata.getSeason(r.Context(), identity, season))
}

func (m *metadataService) get(ctx context.Context, v metadataIdentity) metadataResponse {
	key := fmt.Sprintf("%s/%s/%s/%s/%s", v.userID, v.sourceID, v.kind, v.itemID, v.fingerprint)
	m.mu.Lock()
	if call := m.coldCalls[key]; call != nil {
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return metadataResponse{Status: "unavailable"}
		case <-call.done:
			return call.result
		}
	}
	call := &metadataCall{done: make(chan struct{})}
	m.coldCalls[key] = call
	m.mu.Unlock()
	call.result = m.loadMetadata(ctx, v)
	m.mu.Lock()
	delete(m.coldCalls, key)
	close(call.done)
	m.mu.Unlock()
	return call.result
}

// canonicalFlight joins provider/cache loads by public canonical key. The loader
// runs with its own bounded background context; callers can cancel independently.
func (m *metadataService) canonicalFlight(ctx context.Context, key string, load func(context.Context) metadataPublicData) metadataPublicData {
	m.mu.Lock()
	if call := m.canonicalCalls[key]; call != nil {
		call.waiters++
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return metadataPublicData{err: ctx.Err()}
		case <-call.done:
			return call.public
		}
	}
	call := &metadataCall{done: make(chan struct{})}
	m.canonicalCalls[key] = call
	m.mu.Unlock()

	go func() {
		loadCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		call.public = load(loadCtx)
		m.mu.Lock()
		delete(m.canonicalCalls, key)
		close(call.done)
		m.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return metadataPublicData{err: ctx.Err()}
	case <-call.done:
		return call.public
	}
}

func (m *metadataService) loadMetadata(ctx context.Context, v metadataIdentity) metadataResponse {
	if m.cfg.DB == nil {
		return metadataResponse{Status: "disabled"}
	}
	if !m.currentItem(ctx, v) {
		return metadataResponse{Status: "unavailable"}
	}
	state, canonical, known := m.readResolution(ctx, v)
	if known && state != "resolved" {
		return metadataResponse{Status: state}
	}
	if known && state == "resolved" {
		if rec, stale, found := m.readRecord(ctx, v.kind, canonical); found {
			if metadataRecordMatches(rec, v) && m.currentItem(ctx, v) {
				if stale {
					m.refreshAsync(v.kind, canonical)
				}
				status := "ready"
				if stale {
					status = "stale"
				}
				return metadataResponse{Status: status, Language: metadataLanguage, Source: "TMDB", Metadata: &rec}
			}
		}
	}
	if strings.TrimSpace(m.cfg.TMDBToken) == "" {
		return metadataResponse{Status: "disabled"}
	}
	if !m.allowUser(v.userID) {
		return metadataResponse{Status: "unavailable"}
	}
	var status string
	var err error
	if !known {
		err = m.doJob(ctx, func(c context.Context) error { var e error; canonical, status, e = m.resolve(c, v); return e })
		if err != nil {
			return metadataResponse{Status: "unavailable"}
		}
		if status != "resolved" {
			m.saveResolution(ctx, v, status, nil)
			return metadataResponse{Status: status}
		}
	}
	public := m.canonicalFlight(ctx, fmt.Sprintf("details/%s/%d/%s", v.kind, canonical, metadataLanguage), func(loadCtx context.Context) metadataPublicData {
		return m.loadCanonicalRecord(loadCtx, v.kind, canonical)
	})
	if public.err != nil || public.record == nil {
		return metadataResponse{Status: "unavailable"}
	}
	result := metadataRecordResponse(v, *public.record, public.stale, m.currentItem(ctx, v))
	if result.Status == "ready" || result.Status == "stale" {
		m.saveResolution(ctx, v, "resolved", &canonical)
		if result.Status == "stale" {
			m.refreshAsync(v.kind, canonical)
		}
	} else if result.Status == "unresolved" {
		m.saveResolution(ctx, v, "unresolved", nil)
	}
	return result
}

func (m *metadataService) loadCanonicalRecord(ctx context.Context, kind string, id int64) metadataPublicData {
	if rec, stale, found := m.readRecord(ctx, kind, id); found {
		return metadataPublicData{record: &rec, stale: stale}
	}
	return m.fetchCanonicalRecord(ctx, kind, id)
}

func (m *metadataService) fetchCanonicalRecord(ctx context.Context, kind string, id int64) metadataPublicData {
	var rec metadataRecord
	if err := m.doJob(ctx, func(c context.Context) error {
		var fetchErr error
		rec, fetchErr = m.fetchDetails(c, kind, id)
		return fetchErr
	}); err != nil {
		return metadataPublicData{err: err}
	}
	if rec.ID != id || rec.Kind != kind {
		return metadataPublicData{err: errors.New("invalid provider data")}
	}
	m.saveRecord(ctx, "tmdb_metadata_cache", kind, id, 0, rec)
	return metadataPublicData{record: &rec}
}

func metadataRecordResponse(v metadataIdentity, record metadataRecord, stale, current bool) metadataResponse {
	if !current {
		return metadataResponse{Status: "unavailable"}
	}
	if !metadataRecordMatches(record, v) {
		return metadataResponse{Status: "unresolved"}
	}
	status := "ready"
	if stale {
		status = "stale"
	}
	return metadataResponse{Status: status, Language: metadataLanguage, Source: "TMDB", Metadata: &record}
}

func (m *metadataService) getSeason(ctx context.Context, v metadataIdentity, season int) metadataResponse {
	return m.loadSeason(ctx, v, season)
}

func (m *metadataService) loadSeason(ctx context.Context, v metadataIdentity, season int) metadataResponse {
	base := m.get(ctx, v)
	if base.Metadata == nil {
		return base
	}
	if cached, found := m.readCanonicalSeason(ctx, base.Metadata.ID, season); found {
		if !metadataRecordMatches(*base.Metadata, v) || !m.currentItem(ctx, v) {
			return metadataResponse{Status: "unavailable"}
		}
		status := "ready"
		if cached.stale {
			status = "stale"
			m.refreshSeasonAsync(base.Metadata.ID, season)
		}
		return metadataResponse{Status: status, Language: metadataLanguage, Source: "TMDB", Metadata: base.Metadata, Season: cached.season}
	}
	public := m.coldSeasonFlight(ctx, v.userID, fmt.Sprintf("season/%d/%s/%d", base.Metadata.ID, metadataLanguage, season), func(loadCtx context.Context) metadataPublicData {
		return m.loadCanonicalSeason(loadCtx, base.Metadata.ID, season)
	})
	if public.err != nil || public.season == nil {
		return metadataResponse{Status: "unavailable", Metadata: base.Metadata}
	}
	if !metadataRecordMatches(*base.Metadata, v) || !m.currentItem(ctx, v) {
		return metadataResponse{Status: "unavailable"}
	}
	status := "ready"
	if public.stale {
		status = "stale"
		m.refreshSeasonAsync(base.Metadata.ID, season)
	}
	return metadataResponse{Status: status, Language: metadataLanguage, Source: "TMDB", Metadata: base.Metadata, Season: public.season}
}

// Each request is admitted independently after its cache miss and before it can
// create or join the public canonical season flight.
func (m *metadataService) coldSeasonFlight(ctx context.Context, userID, key string, load func(context.Context) metadataPublicData) metadataPublicData {
	if !m.allowUser(userID) {
		return metadataPublicData{err: errors.New("user metadata rate limit")}
	}
	return m.canonicalFlight(ctx, key, load)
}

func (m *metadataService) loadCanonicalSeason(ctx context.Context, id int64, season int) metadataPublicData {
	if cached, found := m.readCanonicalSeason(ctx, id, season); found {
		return cached
	}
	if strings.TrimSpace(m.cfg.TMDBToken) == "" {
		return metadataPublicData{err: errors.New("provider disabled")}
	}
	return m.fetchCanonicalSeason(ctx, id, season)
}

func (m *metadataService) readCanonicalSeason(ctx context.Context, id int64, season int) (metadataPublicData, bool) {
	var cached metadataSeason
	var fetched time.Time
	var expires time.Time
	err := m.cfg.DB.QueryRow(ctx, `SELECT payload,fetched_at,expires_at FROM tmdb_season_cache WHERE kind='series' AND canonical_id=$1 AND language=$2 AND season_number=$3`, id, metadataLanguage, season).Scan(newJSON(&cached), &fetched, &expires)
	if err == nil && time.Now().Before(expires) {
		return metadataPublicData{season: &cached}, true
	}
	if err == nil && time.Since(fetched) <= 30*24*time.Hour {
		return metadataPublicData{season: &cached, stale: true}, true
	}
	return metadataPublicData{}, false
}

func (m *metadataService) fetchCanonicalSeason(ctx context.Context, id int64, season int) metadataPublicData {
	if strings.TrimSpace(m.cfg.TMDBToken) == "" {
		return metadataPublicData{err: errors.New("provider disabled")}
	}
	var value metadataSeason
	if err := m.doJob(ctx, func(c context.Context) error {
		var e error
		value, e = m.fetchSeason(c, id, season)
		return e
	}); err != nil {
		return metadataPublicData{err: err}
	}
	encoded, _ := json.Marshal(value)
	now := time.Now().UTC()
	_, _ = m.cfg.DB.Exec(ctx, `INSERT INTO tmdb_season_cache(kind,canonical_id,language,season_number,payload,fetched_at,expires_at) VALUES('series',$1,$2,$3,$4::jsonb,$5,$6) ON CONFLICT(kind,canonical_id,language,season_number) DO UPDATE SET payload=EXCLUDED.payload,fetched_at=EXCLUDED.fetched_at,expires_at=EXCLUDED.expires_at`, id, metadataLanguage, season, string(encoded), now, now.Add(7*24*time.Hour))
	return metadataPublicData{season: &value}
}

func newJSON(v any) interface{ Scan(any) error } { return jsonScanner{v: v} }

type jsonScanner struct{ v any }

func (s jsonScanner) Scan(src any) error {
	var b []byte
	switch x := src.(type) {
	case []byte:
		b = x
	case string:
		b = []byte(x)
	default:
		return fmt.Errorf("invalid JSON cache")
	}
	return json.Unmarshal(b, s.v)
}

func (m *metadataService) readRecord(ctx context.Context, kind string, id int64) (metadataRecord, bool, bool) {
	var rec metadataRecord
	var fetched, expires time.Time
	err := m.cfg.DB.QueryRow(ctx, `SELECT payload,fetched_at,expires_at FROM tmdb_metadata_cache WHERE kind=$1 AND canonical_id=$2 AND language=$3`, kind, id, metadataLanguage).Scan(newJSON(&rec), &fetched, &expires)
	if err != nil {
		return rec, false, false
	}
	if time.Now().Before(expires) {
		return rec, false, true
	}
	if time.Since(fetched) <= 30*24*time.Hour {
		return rec, true, true
	}
	return rec, false, false
}

func (m *metadataService) readResolution(ctx context.Context, v metadataIdentity) (string, int64, bool) {
	var fingerprint, status string
	var expires time.Time
	var canonical *int64
	err := m.cfg.DB.QueryRow(ctx, `SELECT fingerprint,status,canonical_id,expires_at FROM tmdb_item_resolutions WHERE user_id=$1 AND source_id=$2 AND kind=$3 AND item_id=$4`, v.userID, v.sourceID, v.kind, v.itemID).Scan(&fingerprint, &status, &canonical, &expires)
	if err != nil || fingerprint != v.fingerprint || !time.Now().Before(expires) {
		return "", 0, false
	}
	if canonical != nil {
		return status, *canonical, true
	}
	return status, 0, true
}

func (m *metadataService) saveResolution(ctx context.Context, v metadataIdentity, status string, id *int64) {
	ttl := 24 * time.Hour
	if status == "resolved" {
		ttl = 365 * 24 * time.Hour
	}
	_, _ = m.cfg.DB.Exec(ctx, `INSERT INTO tmdb_item_resolutions(user_id,source_id,kind,item_id,fingerprint,status,canonical_id,checked_at,expires_at) SELECT $1,$2,$3,$4,$5,$6,$7,now(),now()+$8::interval WHERE EXISTS (SELECT 1 FROM source_catalog_state s JOIN source_catalog_entries e ON e.source_id=s.source_id AND e.generation=s.active_generation WHERE s.source_id=$2 AND s.active_generation>0 AND e.kind=$3 AND e.id=$4 AND e.title=$9 AND e.release_year IS NOT DISTINCT FROM $10 AND e.tmdb_id IS NOT DISTINCT FROM $11 AND e.imdb_id IS NOT DISTINCT FROM $12) ON CONFLICT(user_id,source_id,kind,item_id) DO UPDATE SET fingerprint=EXCLUDED.fingerprint,status=EXCLUDED.status,canonical_id=EXCLUDED.canonical_id,checked_at=EXCLUDED.checked_at,expires_at=EXCLUDED.expires_at`, v.userID, v.sourceID, v.kind, v.itemID, v.fingerprint, status, id, fmt.Sprintf("%f seconds", ttl.Seconds()), v.title, v.year, v.tmdbID, v.imdbID)
}

func (m *metadataService) currentItem(ctx context.Context, v metadataIdentity) bool {
	var exists bool
	err := m.cfg.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM source_catalog_state s JOIN source_catalog_entries e ON e.source_id=s.source_id AND e.generation=s.active_generation WHERE s.source_id=$1 AND e.kind=$2 AND e.id=$3 AND e.title=$4 AND e.release_year IS NOT DISTINCT FROM $5 AND e.tmdb_id IS NOT DISTINCT FROM $6 AND e.imdb_id IS NOT DISTINCT FROM $7)`, v.sourceID, v.kind, v.itemID, v.title, v.year, v.tmdbID, v.imdbID).Scan(&exists)
	return err == nil && exists
}

func (m *metadataService) saveRecord(ctx context.Context, table, kind string, id int64, season int, rec any) {
	payload, _ := json.Marshal(rec)
	now := time.Now().UTC()
	_, _ = m.cfg.DB.Exec(ctx, `INSERT INTO tmdb_metadata_cache(kind,canonical_id,language,payload,fetched_at,expires_at) VALUES($1,$2,$3,$4::jsonb,$5,$6) ON CONFLICT(kind,canonical_id,language) DO UPDATE SET payload=EXCLUDED.payload,fetched_at=EXCLUDED.fetched_at,expires_at=EXCLUDED.expires_at`, kind, id, metadataLanguage, string(payload), now, now.Add(7*24*time.Hour))
}

func (m *metadataService) doJob(ctx context.Context, job func(context.Context) error) error {
	select {
	case m.queued <- struct{}{}:
	default:
		return errors.New("busy")
	}
	defer func() { <-m.queued }()
	select {
	case m.active <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-m.active }()
	jobCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	select {
	case <-jobCtx.Done():
		return jobCtx.Err()
	case <-m.rate:
	}
	return job(jobCtx)
}

func (m *metadataService) allowUser(userID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	times := m.perUser[userID][:0]
	for _, at := range m.perUser[userID] {
		if now.Sub(at) < time.Minute {
			times = append(times, at)
		}
	}
	if len(times) >= 10 {
		m.perUser[userID] = times
		return false
	}
	m.perUser[userID] = append(times, now)
	return true
}

func (m *metadataService) refreshAsync(kind string, id int64) {
	if strings.TrimSpace(m.cfg.TMDBToken) == "" {
		return
	}
	key := fmt.Sprintf("%s/%d/refresh", kind, id)
	m.mu.Lock()
	if m.flights[key] != nil {
		m.mu.Unlock()
		return
	}
	m.flights[key] = make(chan struct{})
	m.mu.Unlock()
	select {
	case m.queued <- struct{}{}:
	default:
		m.mu.Lock()
		delete(m.flights, key)
		m.mu.Unlock()
		return
	}
	go func() {
		defer func() { <-m.queued; m.mu.Lock(); delete(m.flights, key); m.mu.Unlock() }()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if m.checkCooldown(ctx) {
			return
		}
		public := m.canonicalFlight(ctx, fmt.Sprintf("details/%s/%d/%s", kind, id, metadataLanguage), func(loadCtx context.Context) metadataPublicData {
			return m.fetchCanonicalRecord(loadCtx, kind, id)
		})
		if public.stale {
			_ = m.fetchCanonicalRecord(ctx, kind, id)
		}
	}()
}

func (m *metadataService) refreshSeasonAsync(id int64, season int) {
	if strings.TrimSpace(m.cfg.TMDBToken) == "" {
		return
	}
	if m.checkCooldown(context.Background()) {
		return
	}
	key := fmt.Sprintf("season/%d/%d", id, season)
	m.mu.Lock()
	if m.flights[key] != nil {
		m.mu.Unlock()
		return
	}
	m.flights[key] = make(chan struct{})
	m.mu.Unlock()
	select {
	case m.queued <- struct{}{}:
	default:
		m.mu.Lock()
		delete(m.flights, key)
		m.mu.Unlock()
		return
	}
	go func() {
		defer func() { <-m.queued; m.mu.Lock(); delete(m.flights, key); m.mu.Unlock() }()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if m.checkCooldown(ctx) {
			return
		}
		public := m.canonicalFlight(ctx, fmt.Sprintf("season/%d/%s/%d", id, metadataLanguage, season), func(loadCtx context.Context) metadataPublicData {
			return m.fetchCanonicalSeason(loadCtx, id, season)
		})
		if public.stale {
			_ = m.fetchCanonicalSeason(ctx, id, season)
		}
	}()
}

func (m *metadataService) resolve(ctx context.Context, v metadataIdentity) (int64, string, error) {
	if m.checkCooldown(ctx) {
		return 0, "", errors.New("cooldown")
	}
	if v.year == nil && v.tmdbID == nil && v.imdbID == nil {
		return 0, "unresolved", nil
	}
	var candidates []map[string]any
	if v.tmdbID != nil {
		return *v.tmdbID, "resolved", nil
	} else if v.imdbID != nil {
		payload, err := m.tmdbGET(ctx, "/find/"+url.PathEscape(*v.imdbID), url.Values{"external_source": {"imdb_id"}, "language": {metadataLanguage}})
		if err != nil {
			return 0, "", err
		}
		key := "movie_results"
		if v.kind == "series" {
			key = "tv_results"
		}
		results, ok := payload[key].([]any)
		if !ok {
			return 0, "", errors.New("invalid provider search response")
		}
		for _, item := range results {
			obj, ok := item.(map[string]any)
			if !ok || !validSearchCandidate(obj, v.kind) {
				return 0, "", errors.New("invalid provider search response")
			}
			candidates = append(candidates, obj)
		}
	} else {
		path := "/search/movie"
		if v.kind == "series" {
			path = "/search/tv"
		}
		params := url.Values{"query": {normalizeMetadataTitle(v.title)}, "language": {metadataLanguage}, "page": {"1"}}
		if v.year != nil {
			key := "year"
			if v.kind == "series" {
				key = "first_air_date_year"
			}
			params.Set(key, strconv.Itoa(*v.year))
		}
		payload, err := m.tmdbGET(ctx, path, params)
		if err != nil {
			return 0, "", err
		}
		results, total, exhaustive, err := validateSearchPayload(payload, v.kind)
		if err != nil {
			return 0, "", err
		}
		// A valid but truncated first page cannot prove that there is no exact
		// title/year match on a later page. Treat it as ambiguous, never negative.
		if !exhaustive {
			return 0, "ambiguous", nil
		}
		if len(results) == 0 {
			return 0, "unresolved", nil
		}
		_ = total // validated here so malformed count fields cannot become negative cache entries
		candidates = results
	}
	matches := []int64{}
	want := normalizeMetadataTitle(v.title)
	for _, c := range candidates {
		var gotTitle, originalTitle, date string
		if v.kind == "movie" {
			gotTitle, _ = c["title"].(string)
			originalTitle, _ = c["original_title"].(string)
			date, _ = c["release_date"].(string)
		} else {
			gotTitle, _ = c["name"].(string)
			originalTitle, _ = c["original_name"].(string)
			date, _ = c["first_air_date"].(string)
		}
		year := yearFromDate(date)
		if want == "" || (normalizeMetadataTitle(gotTitle) != want && normalizeMetadataTitle(originalTitle) != want) || (v.year != nil && year != *v.year) || (v.year == nil && v.tmdbID == nil && v.imdbID == nil) {
			continue
		}
		id, ok := numberID(c["id"])
		if ok {
			matches = append(matches, id)
		}
	}
	if len(matches) == 0 {
		return 0, "unresolved", nil
	}
	if len(matches) != 1 {
		return 0, "ambiguous", nil
	}
	return matches[0], "resolved", nil
}

func validateSearchPayload(payload map[string]any, kind string) ([]map[string]any, int, bool, error) {
	items, ok := payload["results"].([]any)
	if !ok || len(items) > 20 {
		return nil, 0, false, errors.New("invalid provider search response")
	}
	totalValue, ok := payload["total_results"].(float64)
	if !ok || totalValue < 0 || totalValue != float64(int64(totalValue)) || totalValue > float64(int(^uint(0)>>1)) {
		return nil, 0, false, errors.New("invalid provider search response")
	}
	total := int(totalValue)
	if total < len(items) {
		return nil, 0, false, errors.New("invalid provider search response")
	}
	results := make([]map[string]any, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, 0, false, errors.New("invalid provider search response")
		}
		if !validSearchCandidate(obj, kind) {
			return nil, 0, false, errors.New("invalid provider search response")
		}
		results = append(results, obj)
	}
	return results, total, total == len(results), nil
}

func validSearchCandidate(obj map[string]any, kind string) bool {
	if _, ok := numberID(obj["id"]); !ok {
		return false
	}
	titleField, dateField := "title", "release_date"
	if kind == "series" {
		titleField, dateField = "name", "first_air_date"
	}
	if _, ok := obj[titleField].(string); !ok {
		return false
	}
	if _, ok := obj[dateField].(string); !ok {
		return false
	}
	return true
}
func numberID(v any) (int64, bool) {
	switch x := v.(type) {
	case float64:
		if x > 0 && x == float64(int64(x)) {
			return int64(x), true
		}
	case int64:
		if x > 0 {
			return x, true
		}
	}
	return 0, false
}
func yearFromDate(v string) int {
	if len(v) < 4 {
		return 0
	}
	n, _ := strconv.Atoi(v[:4])
	return n
}

func (m *metadataService) tmdbGET(ctx context.Context, path string, params url.Values) (map[string]any, error) {
	requestScope := requestCooldownScope(path, params)
	if m.checkCooldown(ctx) || m.checkCooldownScope(ctx, requestScope) {
		return nil, errors.New("provider cooling down")
	}
	u := url.URL{Scheme: "https", Host: "api.themoviedb.org", Path: "/3" + path, RawQuery: params.Encode()}
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(m.cfg.TMDBToken))
	req.Header.Set("Accept", "application/json")
	res, err := m.client.Do(req)
	if err != nil {
		_ = m.persistCooldown(requestScope, time.Now().Add(30*time.Second))
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		_ = m.persistCooldown("global", retryAfter(res.Header.Get("Retry-After"), time.Now()))
		return nil, errors.New("rate limited")
	}
	if res.StatusCode == 401 || res.StatusCode == 403 {
		_ = m.persistCooldown("global", time.Now().Add(5*time.Minute))
		return nil, errors.New("provider unavailable")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_ = m.persistCooldown(requestScope, time.Now().Add(30*time.Second))
		return nil, errors.New("provider unavailable")
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
	if err != nil || len(body) > 2<<20 {
		_ = m.persistCooldown(requestScope, time.Now().Add(30*time.Second))
		return nil, errors.New("provider response invalid")
	}
	var out map[string]any
	if json.Unmarshal(body, &out) != nil {
		_ = m.persistCooldown(requestScope, time.Now().Add(30*time.Second))
		return nil, errors.New("provider response invalid")
	}
	return out, nil
}

func (m *metadataService) fetchDetails(ctx context.Context, kind string, id int64) (metadataRecord, error) {
	path := fmt.Sprintf("/movie/%d", id)
	if kind == "series" {
		path = fmt.Sprintf("/tv/%d", id)
	}
	payload, err := m.tmdbGET(ctx, path, url.Values{"language": {metadataLanguage}, "append_to_response": {"credits"}})
	if err != nil {
		return metadataRecord{}, err
	}
	rec := normalizeTMDBRecord(kind, payload)
	if rec.ID == 0 || rec.ID != id {
		return metadataRecord{}, errors.New("invalid provider data")
	}
	return rec, nil
}
func (m *metadataService) fetchSeason(ctx context.Context, id int64, season int) (metadataSeason, error) {
	payload, err := m.tmdbGET(ctx, fmt.Sprintf("/tv/%d/season/%d", id, season), url.Values{"language": {metadataLanguage}})
	if err != nil {
		return metadataSeason{}, err
	}
	out := metadataSeason{Number: season, Episodes: []metadataEpisode{}}
	seasonNumber, ok := payload["season_number"].(float64)
	if !ok || seasonNumber < 0 || seasonNumber > 99 || seasonNumber != float64(int(seasonNumber)) || int(seasonNumber) != season {
		return metadataSeason{}, errors.New("invalid season response")
	}
	items, ok := payload["episodes"].([]any)
	if !ok {
		return metadataSeason{}, errors.New("invalid season response")
	}
	out.Name, _ = payload["name"].(string)
	out.Overview, _ = payload["overview"].(string)
	for _, v := range items {
		e, ok := v.(map[string]any)
		if !ok {
			return metadataSeason{}, errors.New("invalid episode response")
		}
		num, ok := e["episode_number"].(float64)
		if !ok || num < 1 || num > 999 || num != float64(int(num)) {
			return metadataSeason{}, errors.New("invalid episode response")
		}
		title, _ := e["name"].(string)
		overview, _ := e["overview"].(string)
		air, _ := e["air_date"].(string)
		still, _ := e["still_path"].(string)
		out.Episodes = append(out.Episodes, metadataEpisode{Number: int(num), Name: title, Overview: overview, AirDate: air, StillURL: tmdbImage(still)})
	}
	return out, nil
}

func normalizeTMDBRecord(kind string, p map[string]any) metadataRecord {
	r := metadataRecord{Kind: kind, RatingSource: "TMDB"}
	r.ID, _ = numberID(p["id"])
	if kind == "movie" {
		r.Title, _ = p["title"].(string)
		r.Original, _ = p["original_title"].(string)
		r.ReleaseDate, _ = p["release_date"].(string)
		if n, ok := p["runtime"].(float64); ok {
			r.Runtime = int(n)
		}
	} else {
		r.Title, _ = p["name"].(string)
		r.Original, _ = p["original_name"].(string)
		r.ReleaseDate, _ = p["first_air_date"].(string)
	}
	r.Year = yearFromDate(r.ReleaseDate)
	r.Overview, _ = p["overview"].(string)
	r.Overview = strings.TrimSpace(r.Overview)
	poster, _ := p["poster_path"].(string)
	backdrop, _ := p["backdrop_path"].(string)
	r.PosterURL = tmdbImage(poster)
	r.BackdropURL = tmdbImage(backdrop)
	if n, ok := p["vote_average"].(float64); ok && n >= 0 && n <= 10 {
		r.Rating = &n
	}
	if genres, ok := p["genres"].([]any); ok {
		for _, item := range genres {
			if g, ok := item.(map[string]any); ok {
				id, _ := numberID(g["id"])
				name, _ := g["name"].(string)
				if name != "" {
					r.Genres = append(r.Genres, metadataGenre{ID: id, Name: name})
				}
			}
		}
	}
	if credits, ok := p["credits"].(map[string]any); ok {
		if cast, ok := credits["cast"].([]any); ok {
			for _, item := range cast {
				if len(r.Credits) >= 20 {
					break
				}
				if c, ok := item.(map[string]any); ok {
					name, _ := c["name"].(string)
					char, _ := c["character"].(string)
					if name != "" {
						r.Credits = append(r.Credits, metadataPerson{Name: name, Character: char})
					}
				}
			}
		}
		if crew, ok := credits["crew"].([]any); ok {
			for _, item := range crew {
				if len(r.Credits) >= 20 {
					break
				}
				if c, ok := item.(map[string]any); ok {
					name, _ := c["name"].(string)
					job, _ := c["job"].(string)
					if name != "" && (job == "Director" || job == "Writer") {
						r.Credits = append(r.Credits, metadataPerson{Name: name, Job: job})
					}
				}
			}
		}
	}
	return r
}
func tmdbImage(path string) string {
	if path == "" || !strings.HasPrefix(path, "/") || strings.Contains(path, "..") || strings.ContainsAny(path, "?#") {
		return ""
	}
	return "https://image.tmdb.org/t/p/w500" + path
}

func retryAfter(value string, now time.Time) time.Time {
	if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && n >= 0 {
		return now.Add(time.Duration(n) * time.Second)
	}
	if parsed, err := http.ParseTime(value); err == nil && parsed.After(now) {
		return parsed
	}
	return now.Add(time.Minute)
}
func requestCooldownScope(path string, params url.Values) string {
	// Search terms can identify private IPTV titles. Store only a one-way digest
	// as the per-request cooldown key, never the raw query.
	sum := sha256.Sum256([]byte(path + "\x00" + params.Encode()))
	return "request:" + hex.EncodeToString(sum[:])
}

func (m *metadataService) persistCooldown(scope string, until time.Time) error {
	if m.cfg.DB == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := m.cfg.DB.Exec(ctx, `INSERT INTO tmdb_provider_cooldowns(scope,retry_after) VALUES($1,$2) ON CONFLICT(scope) DO UPDATE SET retry_after=GREATEST(tmdb_provider_cooldowns.retry_after,EXCLUDED.retry_after)`, scope, until)
	return err
}

func (m *metadataService) checkCooldownScope(ctx context.Context, scope string) bool {
	if m.cfg.DB == nil {
		return false
	}
	var until time.Time
	err := m.cfg.DB.QueryRow(ctx, `SELECT retry_after FROM tmdb_provider_cooldowns WHERE scope=$1`, scope).Scan(&until)
	return err == nil && time.Now().Before(until)
}
func (m *metadataService) checkCooldown(ctx context.Context) bool {
	if m.cfg.DB == nil {
		return false
	}
	var until time.Time
	err := m.cfg.DB.QueryRow(ctx, `SELECT retry_after FROM tmdb_provider_cooldowns WHERE scope='global'`).Scan(&until)
	return err == nil && time.Now().Before(until)
}
