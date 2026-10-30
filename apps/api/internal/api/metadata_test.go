package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type metadataRoundTripper func(*http.Request) (*http.Response, error)

func (f metadataRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMetadataIdentityNormalization(t *testing.T) {
	year, tmdb, imdb := providerVODIdentity(map[string]any{
		"info": map[string]any{"tmdb_id": float64(603), "imdb_id": "tt0133093"},
	}, "The Matrix (1999) 1080p WEB-DL")
	if year == nil || *year != 1999 || tmdb == nil || *tmdb != 603 || imdb == nil || *imdb != "tt0133093" {
		t.Fatalf("unexpected structured identity: year=%v tmdb=%v imdb=%v", year, tmdb, imdb)
	}
	if got := normalizeMetadataTitle("The Matrix (1999) 1080p WEB-DL"); got != "the matrix" {
		t.Fatalf("normalized title = %q", got)
	}
	if got := normalizeMetadataTitle("The Matrix Reloaded"); got != "the matrix reloaded" {
		t.Fatalf("normalization removed significant words: %q", got)
	}
}

func TestMetadataFingerprintUsesStableValues(t *testing.T) {
	yearA, yearB := 1999, 1999
	tmdbA, tmdbB := int64(603), int64(603)
	imdbA, imdbB := "tt0133093", "tt0133093"
	first := metadataFingerprint("The Matrix (1999)", &yearA, &tmdbA, &imdbA)
	second := metadataFingerprint("The Matrix (1999)", &yearB, &tmdbB, &imdbB)
	if first != second {
		t.Fatalf("same metadata identity produced unstable fingerprints: %q != %q", first, second)
	}
	yearB++
	if changed := metadataFingerprint("The Matrix (1999)", &yearB, &tmdbB, &imdbB); changed == first {
		t.Fatal("changed metadata identity kept the same fingerprint")
	}
}

func TestMetadataRecordMatchesLocalizedTitleByOriginalAndYear(t *testing.T) {
	year := 1999
	identity := metadataIdentity{title: "The Matrix", year: &year}
	record := metadataRecord{Title: "Matrix", Original: "The Matrix", Year: 1999}
	if !metadataRecordMatches(record, identity) {
		t.Fatal("localized title should match the exact original title and year")
	}
	record.Year = 2003
	if metadataRecordMatches(record, identity) {
		t.Fatal("metadata from another year should not match")
	}
}

func TestMetadataAdapterUsesFixedHostAndNormalizesSpanishFields(t *testing.T) {
	client := &http.Client{Transport: metadataRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" || req.URL.Host != "api.themoviedb.org" || req.URL.Path != "/3/movie/603" {
			t.Fatalf("unexpected TMDB request URL: %s", req.URL)
		}
		if req.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatal("missing backend bearer token")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":603,"title":"Matrix","original_title":"The Matrix","release_date":"1999-03-30","overview":"Sinopsis","poster_path":"/matrix.jpg","backdrop_path":"https://evil.example/image.jpg","vote_average":8.2,"runtime":136,"genres":[{"id":1,"name":"Ciencia ficción"}],"credits":{"cast":[{"name":"Keanu Reeves","character":"Neo"}]}}`))}, nil
	})}
	m := newMetadataService(Config{TMDBToken: "test-token", HTTPClient: client})
	record, err := m.fetchDetails(context.Background(), "movie", 603)
	if err != nil {
		t.Fatal(err)
	}
	if record.Title != "Matrix" || record.Year != 1999 || record.Overview != "Sinopsis" || record.Runtime != 136 || record.RatingSource != "TMDB" {
		t.Fatalf("unexpected normalized record: %+v", record)
	}
	if record.PosterURL != "https://image.tmdb.org/t/p/w500/matrix.jpg" || record.BackdropURL != "" {
		t.Fatalf("unsafe or missing artwork URL: poster=%q backdrop=%q", record.PosterURL, record.BackdropURL)
	}
	if got := tmdbImage("/../private.jpg"); got != "" {
		t.Fatalf("accepted unsafe image path: %q", got)
	}
}

func TestRetryAfterAndEmptySpanishOverview(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	if got := retryAfter("120", now); !got.Equal(now.Add(120 * time.Second)) {
		t.Fatalf("seconds Retry-After = %v", got)
	}
	if got := retryAfter("invalid", now); !got.Equal(now.Add(time.Minute)) {
		t.Fatalf("invalid Retry-After = %v", got)
	}
	record := normalizeTMDBRecord("movie", map[string]any{"id": float64(1), "title": "Título", "original_title": "Title", "overview": "  "})
	if record.Overview != "" {
		t.Fatalf("empty localized overview was changed: %q", record.Overview)
	}
}

func TestRateLimitedResponseIsHandledBeforeReadingBody(t *testing.T) {
	client := &http.Client{Transport: metadataRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Retry-After": []string{"120"}},
			Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", (2<<20)+8))),
		}, nil
	})}
	m := newMetadataService(Config{TMDBToken: "mock-token", HTTPClient: client})
	_, err := m.tmdbGET(context.Background(), "/search/movie", nil)
	if err == nil || err.Error() != "rate limited" {
		t.Fatalf("oversized 429 response error = %v, want rate limited", err)
	}
}

func TestFetchSeasonRejectsMalformedPayloadAndInitializesEpisodes(t *testing.T) {
	response := `{"season_number":2,"episodes":[]}`
	client := &http.Client{Transport: metadataRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(response))}, nil
	})}
	m := newMetadataService(Config{TMDBToken: "mock-token", HTTPClient: client})
	season, err := m.fetchSeason(context.Background(), 10, 2)
	if err != nil || season.Episodes == nil || len(season.Episodes) != 0 {
		t.Fatalf("valid empty season = (%+v, %v), want non-nil empty episodes", season, err)
	}
	response = `{"season_number":1,"episodes":[]}`
	if _, err = m.fetchSeason(context.Background(), 10, 2); err == nil {
		t.Fatal("mismatched season number was accepted")
	}
	response = `{"season_number":2}`
	if _, err = m.fetchSeason(context.Background(), 10, 2); err == nil {
		t.Fatal("missing episodes array was accepted")
	}
	for _, malformed := range []string{
		`{"season_number":2.5,"episodes":[]}`,
		`{"season_number":2,"episodes":[{"name":"missing number"}]}`,
		`{"season_number":2,"episodes":[{"episode_number":1.5}]}`,
		`{"season_number":2,"episodes":[{"episode_number":0}]}`,
	} {
		response = malformed
		if _, err = m.fetchSeason(context.Background(), 10, 2); err == nil {
			t.Fatalf("malformed numeric season/episode payload was accepted: %s", malformed)
		}
	}
}

func TestMetadataResolutionRequiresUniqueExactYear(t *testing.T) {
	client := &http.Client{Transport: metadataRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "api.themoviedb.org" || req.URL.Path != "/3/search/movie" || req.URL.Query().Get("language") != "es-ES" {
			t.Fatalf("unexpected search request: %s", req.URL)
		}
		body := `{"total_results":2,"results":[{"id":1,"title":"Matrix","release_date":"1999-03-30"},{"id":2,"title":"Matrix","release_date":"2003-05-15"}]}`
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	m := newMetadataService(Config{TMDBToken: "mock-token", HTTPClient: client})
	year := 1999
	id, status, err := m.resolve(context.Background(), metadataIdentity{kind: "movie", title: "Matrix 1080p", year: &year})
	if err != nil || status != "resolved" || id != 1 {
		t.Fatalf("resolution=(%d,%q,%v), want exact 1999 candidate", id, status, err)
	}
	if _, status, err = m.resolve(context.Background(), metadataIdentity{kind: "movie", title: "Matrix"}); err != nil || status != "unresolved" {
		t.Fatalf("yearless resolution=(%q,%v), want unresolved", status, err)
	}
}

func TestMetadataSearchRejectsMalformedAndIncompletePayloads(t *testing.T) {
	validCandidate := `{"id":1,"title":"Matrix","release_date":"1999-03-30"}`
	tests := []struct {
		name, body string
		wantStatus string
		wantError  bool
	}{
		{name: "missing results", body: `{"total_results":0}`, wantError: true},
		{name: "missing count", body: `{"results":[]}`, wantError: true},
		{name: "string count", body: `{"total_results":"0","results":[]}`, wantError: true},
		{name: "count less than results", body: `{"total_results":0,"results":[` + validCandidate + `]}`, wantError: true},
		{name: "invalid result entry", body: `{"total_results":1,"results":[null]}`, wantError: true},
		{name: "valid truncated page", body: `{"total_results":21,"results":[` + validCandidate + `]}`, wantStatus: "ambiguous"},
		{name: "valid empty result", body: `{"total_results":0,"results":[]}`, wantStatus: "unresolved"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{Transport: metadataRoundTripper(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tt.body))}, nil
			})}
			m := newMetadataService(Config{TMDBToken: "mock-token", HTTPClient: client})
			year := 1999
			_, status, err := m.resolve(context.Background(), metadataIdentity{kind: "movie", title: "Matrix", year: &year})
			if tt.wantError && err == nil {
				t.Fatalf("malformed HTTP 200 payload returned cacheable status %q", status)
			}
			if !tt.wantError && (err != nil || status != tt.wantStatus) {
				t.Fatalf("resolve=(%q,%v), want status %q", status, err, tt.wantStatus)
			}
		})
	}
}

func TestMetadataFindRejectsMalformedCandidates(t *testing.T) {
	client := &http.Client{Transport: metadataRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"movie_results":[{}]}`))}, nil
	})}
	m := newMetadataService(Config{TMDBToken: "mock-token", HTTPClient: client})
	imdb := "tt0133093"
	_, status, err := m.resolve(context.Background(), metadataIdentity{kind: "movie", title: "Matrix", imdbID: &imdb})
	if err == nil {
		t.Fatalf("malformed find candidate returned cacheable status %q", status)
	}
}

func TestMetadataCanonicalFlightDeduplicatesColdLoads(t *testing.T) {
	m := newMetadataService(Config{})
	var calls atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan metadataPublicData, 2)
	load := func(ctx context.Context) metadataPublicData {
		calls.Add(1)
		close(entered)
		if ctx.Err() != nil {
			t.Errorf("canonical loader context unexpectedly cancelled: %v", ctx.Err())
		}
		<-release
		return metadataPublicData{record: &metadataRecord{ID: 42, Kind: "movie", Title: "Matrix", Original: "Matrix", Year: 1999}}
	}
	ctxA, cancelA := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		results <- m.canonicalFlight(ctxA, "details/movie/42/es-ES", load)
	}()
	<-entered
	cancelA()
	wg.Add(1)
	go func() {
		defer wg.Done()
		results <- m.canonicalFlight(context.Background(), "details/movie/42/es-ES", load)
	}()
	deadline := time.Now().Add(time.Second)
	for {
		m.mu.Lock()
		joined := m.canonicalCalls["details/movie/42/es-ES"].waiters > 0
		m.mu.Unlock()
		if joined {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second caller did not join canonical flight")
		}
		time.Sleep(time.Millisecond)
	}
	close(release)
	wg.Wait()
	close(results)
	if calls.Load() != 1 {
		t.Fatalf("canonical loads = %d, want 1", calls.Load())
	}
	var canceled, active metadataPublicData
	for result := range results {
		if result.err != nil {
			canceled = result
		} else {
			active = result
		}
	}
	if canceled.err == nil {
		t.Fatal("cancelled caller did not stop waiting independently")
	}
	if active.record == nil || active.record.ID != 42 {
		t.Fatalf("active caller did not receive public canonical record: %+v", active)
	}
	year := 1999
	callerA := metadataIdentity{title: "Different title", year: &year}
	callerB := metadataIdentity{title: "Matrix", year: &year}
	if got := metadataRecordResponse(callerA, *active.record, false, true); got.Status != "unresolved" || got.Metadata != nil {
		t.Fatalf("contradictory cancelled caller inherited private flight state: %+v", got)
	}
	if got := metadataRecordResponse(callerB, *active.record, false, true); got.Status != "ready" || got.Metadata == nil || got.Metadata.ID != 42 {
		t.Fatalf("active caller did not independently revalidate public data: %+v", got)
	}
}

func TestCanonicalRecordIsRevalidatedForEachIdentity(t *testing.T) {
	year := 1999
	contradictory := metadataIdentity{title: "Different title", year: &year}
	record := metadataRecord{ID: 42, Kind: "movie", Title: "Matrix", Original: "Matrix", Year: 1999}
	if got := metadataRecordResponse(contradictory, record, false, true); got.Status != "unresolved" || got.Metadata != nil {
		t.Fatalf("contradictory caller inherited shared canonical response: %+v", got)
	}
	authorized := metadataIdentity{title: "Matrix", year: &year}
	if got := metadataRecordResponse(authorized, record, false, false); got.Status != "unavailable" || got.Metadata != nil {
		t.Fatalf("caller whose current item changed received shared response: %+v", got)
	}
}

func TestColdSeasonFlightAdmitsEveryCallerBeforeJoining(t *testing.T) {
	m := newMetadataService(Config{})
	userID := "season-user"
	now := time.Now()
	m.perUser[userID] = make([]time.Time, 9)
	for i := range m.perUser[userID] {
		m.perUser[userID][i] = now.Add(-time.Second)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	var loads atomic.Int32
	firstResult := make(chan metadataPublicData, 1)
	go func() {
		firstResult <- m.coldSeasonFlight(context.Background(), userID, "season/42/es-ES/1", func(context.Context) metadataPublicData {
			loads.Add(1)
			close(entered)
			<-release
			return metadataPublicData{season: &metadataSeason{Number: 1, Episodes: []metadataEpisode{}}}
		})
	}()
	<-entered // Series metadata is already hot; the season is still cold.
	second := m.coldSeasonFlight(context.Background(), userID, "season/42/es-ES/1", func(context.Context) metadataPublicData {
		loads.Add(1)
		return metadataPublicData{season: &metadataSeason{Number: 1, Episodes: []metadataEpisode{}}}
	})
	if second.err == nil {
		t.Fatal("caller over the per-user limit was allowed to join a cold season flight")
	}
	m.mu.Lock()
	waiters := m.canonicalCalls["season/42/es-ES/1"].waiters
	m.mu.Unlock()
	if waiters != 0 {
		t.Fatalf("rate-limited caller joined canonical flight; waiters=%d", waiters)
	}
	close(release)
	first := <-firstResult
	if first.err != nil || first.season == nil || loads.Load() != 1 {
		t.Fatalf("first admitted load = %+v, loads=%d", first, loads.Load())
	}
}
