package api

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSourceSessionGenerationRejectsLogoutRace(t *testing.T) {
	a := New(Config{SessionSecret: []byte(strings.Repeat("s", 32)), IdentityKey: []byte(strings.Repeat("k", 32)), EncryptionKey: []byte(strings.Repeat("e", 32))})
	sess := session{User: user{ID: "member"}, Token: "session", Generation: 0}
	if err := a.private.checkSourceSession(context.Background(), sess); err != nil {
		t.Fatalf("active session unexpectedly rejected: %v", err)
	}
	a.private.generation[sess.User.ID]++
	if err := a.private.checkSourceSession(context.Background(), sess); err != errSourceSessionRevoked {
		t.Fatalf("stale session generation accepted: %v", err)
	}
}

// This test only mutates an explicitly configured loopback database. The
// parent project's production PostgreSQL URL must never be used here.
func TestLocalPostgresPrivateSourcePersistenceAndWrongKey(t *testing.T) {
	dsn := os.Getenv("WALACTV_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set WALACTV_TEST_DATABASE_URL to an isolated local test database")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Hostname() == "" {
		t.Fatal("test database URL is invalid")
	}
	host := strings.ToLower(parsed.Hostname())
	endpointIP := net.ParseIP(host)
	if host != "localhost" && (endpointIP == nil || !endpointIP.IsLoopback()) {
		t.Skip("database endpoint is not loopback; refusing to run mutating test")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("could not open local test database")
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		t.Fatal("local test database is not reachable")
	}
	userID, err := randomToken(12)
	if err != nil {
		t.Fatal(err)
	}
	username := "source-test-" + userID
	if _, err := db.Exec(ctx, "INSERT INTO users(id,username,password_hash,role) VALUES($1,$2,'unused','admin')", userID, username); err != nil {
		t.Fatal("test schema is not initialized")
	}
	defer db.Exec(ctx, "DELETE FROM users WHERE id=$1", userID)

	key := []byte(strings.Repeat("e", 32))
	a := New(Config{DB: db, Origin: "http://127.0.0.1:8081", SessionSecret: []byte(strings.Repeat("s", 32)), IdentityKey: []byte(strings.Repeat("k", 32)), EncryptionKey: key, AllowedHosts: []string{"provider.example"}})
	originalURL := "https://provider.example/get.php?username=private-user&password=private-password"
	base, usernameValue, passwordValue, _, err := parsePlaylistURL(originalURL)
	if err != nil {
		t.Fatal("test playlist URL was rejected")
	}
	sourceID, err := randomToken(18)
	if err != nil {
		t.Fatal(err)
	}
	src := &source{ID: sourceID, UserID: userID, Label: "Lista privada", AccountKey: a.private.identityKey(base, usernameValue), Fingerprint: a.private.fingerprint(base, usernameValue), PlaylistURL: originalURL, Base: base, Username: usernameValue, Password: passwordValue, MaxConnections: 1, ExpiresAt: time.Now().Add(time.Hour)}
	if err := a.private.saveSource(ctx, src, nil); err != nil {
		t.Fatal("could not save encrypted source")
	}
	var ciphertext []byte
	if err := db.QueryRow(ctx, "SELECT secret_ciphertext FROM sources WHERE id=$1 AND user_id=$2", sourceID, userID).Scan(&ciphertext); err != nil {
		t.Fatal("saved source row was not found")
	}
	if bytesContainsAny(ciphertext, []byte("private-user"), []byte("private-password"), []byte(originalURL)) {
		t.Fatal("database ciphertext contains source credentials in clear text")
	}
	cat := catalogData{expires: time.Now().Add(time.Minute), categories: map[string][]category{"movie": {{ID: "7", Name: "Films"}}}, items: map[string][]CatalogItem{"movie:7": {{ID: "25", Title: "Film", Kind: "movie", CategoryID: "7", ContainerExtension: "mp4", ContentKey: strings.Repeat("a", 64), URL: "https://provider.example/movie/u/private-password/25.mp4"}}}}
	if err := a.private.saveCatalog(ctx, src, "movie:7", cat); err != nil {
		t.Fatal("could not save sanitized catalog")
	}
	var payload string
	if err := db.QueryRow(ctx, "SELECT payload::text FROM source_catalogs WHERE source_id=$1 AND catalog_key='movie:7'", sourceID).Scan(&payload); err != nil {
		t.Fatal("saved catalog row was not found")
	}
	if strings.Contains(payload, "private-password") || strings.Contains(payload, "provider.example/movie") || strings.Contains(payload, "direct_source") {
		t.Fatal("persisted catalog contains a private provider URL")
	}

	token, err := randomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := db.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,idle_expires_at,absolute_expires_at) VALUES($1,$2,$3,$4)", hashToken(token), userID, now.Add(time.Hour), now.Add(12*time.Hour)); err != nil {
		t.Fatal("could not prepare local logout session")
	}
	a.private.sources[sourceID] = src
	logout := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	logout.AddCookie(&http.Cookie{Name: "walactv_session", Value: token})
	logout.Header.Set("Origin", "http://127.0.0.1:8081")
	logout.Header.Set("X-CSRF-Token", csrfToken(a.cfg.SessionSecret, token))
	logoutW := httptest.NewRecorder()
	a.Handler().ServeHTTP(logoutW, logout)
	if logoutW.Code != http.StatusNoContent {
		t.Fatalf("local logout failed: %d", logoutW.Code)
	}
	if a.private.sources[sourceID] != nil {
		t.Fatal("logout retained the in-memory source")
	}
	var retained int
	if err := db.QueryRow(ctx, "SELECT count(*) FROM sources WHERE id=$1 AND user_id=$2", sourceID, userID).Scan(&retained); err != nil || retained != 1 {
		t.Fatal("logout deleted the persisted source")
	}
	var preservedID string
	var catalogCount int

	// A provider check already in flight when logout completes must not replace
	// the saved source or repopulate the revoked session's RAM state.
	createToken, err := randomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,idle_expires_at,absolute_expires_at) VALUES($1,$2,$3,$4)", hashToken(createToken), userID, now.Add(time.Hour), now.Add(12*time.Hour)); err != nil {
		t.Fatal("could not prepare local source creation session")
	}
	providerStarted, providerRelease := make(chan struct{}), make(chan struct{})
	blockedClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		close(providerStarted)
		<-providerRelease
		return jsonResponse(http.StatusOK, `{"user_info":{"auth":1,"status":"Active","max_connections":"1"}}`), nil
	})}
	blockedApp := New(Config{DB: db, Origin: "http://127.0.0.1:8081", SessionSecret: []byte(strings.Repeat("s", 32)), IdentityKey: []byte(strings.Repeat("k", 32)), EncryptionKey: key, AllowedHosts: []string{"provider.example"}, HTTPClient: blockedClient})
	blockedApp.private.sources[sourceID] = src
	createSession := session{User: user{ID: userID, Username: username, Role: "admin"}, Token: createToken, Generation: blockedApp.private.generation[userID], Absolute: now.Add(12 * time.Hour)}
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(`{"playlistUrl":"https://provider.example/get.php?username=second-user&password=second-password"}`))
	createReq = createReq.WithContext(context.WithValue(createReq.Context(), sessionKey, createSession))
	createResult := httptest.NewRecorder()
	createDone := make(chan struct{})
	go func() {
		blockedApp.createSource(createResult, createReq)
		close(createDone)
	}()
	select {
	case <-providerStarted:
	case <-time.After(5 * time.Second):
		close(providerRelease)
		t.Fatal("provider request did not block during source creation")
	}
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutReq = logoutReq.WithContext(context.WithValue(logoutReq.Context(), sessionKey, createSession))
	logoutResult := httptest.NewRecorder()
	blockedApp.logout(logoutResult, logoutReq)
	if logoutResult.Code != http.StatusNoContent {
		close(providerRelease)
		t.Fatalf("logout during provider check failed: %d", logoutResult.Code)
	}
	close(providerRelease)
	select {
	case <-createDone:
	case <-time.After(5 * time.Second):
		t.Fatal("source request did not finish after provider release")
	}
	if createResult.Code != http.StatusUnauthorized || len(blockedApp.private.sources) != 0 {
		t.Fatalf("revoked request repopulated source state: status=%d", createResult.Code)
	}
	if err := db.QueryRow(ctx, "SELECT id FROM sources WHERE user_id=$1", userID).Scan(&preservedID); err != nil || preservedID != sourceID {
		t.Fatal("revoked source request replaced the existing saved source")
	}

	// If a catalog fetch finishes after source deletion, neither memory nor
	// PostgreSQL may regain catalog data for the deleted source.
	deleteToken, err := randomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	catalogUserID, err := randomToken(12)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO users(id,username,password_hash,role) VALUES($1,$2,'unused','admin')", catalogUserID, "catalog-test-"+catalogUserID); err != nil {
		t.Fatal("could not prepare local catalog-test user")
	}
	defer db.Exec(ctx, "DELETE FROM users WHERE id=$1", catalogUserID)
	catalogSourceID, err := randomToken(18)
	if err != nil {
		t.Fatal(err)
	}
	catalogSource := *src
	catalogSource.ID, catalogSource.UserID = catalogSourceID, catalogUserID
	catalogSource.AccountKey = a.private.identityKey(base, usernameValue)
	catalogSource.Fingerprint = a.private.fingerprint(base, usernameValue)
	if err := a.private.saveSource(ctx, &catalogSource, nil); err != nil {
		t.Fatal("could not prepare catalog-race source")
	}
	if _, err := db.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,idle_expires_at,absolute_expires_at) VALUES($1,$2,$3,$4)", hashToken(deleteToken), catalogUserID, now.Add(time.Hour), now.Add(12*time.Hour)); err != nil {
		t.Fatal("could not prepare local source deletion session")
	}
	catalogStarted, catalogRelease := make(chan struct{}), make(chan struct{})
	catalogClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		close(catalogStarted)
		<-catalogRelease
		return jsonResponse(http.StatusOK, `[{"category_id":"7","category_name":"Films"}]`), nil
	})}
	catalogApp := New(Config{DB: db, Origin: "http://127.0.0.1:8081", SessionSecret: []byte(strings.Repeat("s", 32)), IdentityKey: []byte(strings.Repeat("k", 32)), EncryptionKey: key, AllowedHosts: []string{"provider.example"}, HTTPClient: catalogClient})
	loadedSource, err := catalogApp.private.loadSource(ctx, catalogUserID, catalogSourceID, now.Add(12*time.Hour))
	if err != nil || loadedSource == nil {
		t.Fatal("could not load source for blocked catalog test")
	}
	catalogApp.private.sources[catalogSourceID] = loadedSource
	catalogDone := make(chan error, 1)
	go func() {
		_, err := catalogApp.private.catalog(ctx, loadedSource, "movie", "")
		catalogDone <- err
	}()
	select {
	case <-catalogStarted:
	case <-time.After(5 * time.Second):
		close(catalogRelease)
		t.Fatal("provider request did not block during catalog fetch")
	}
	deleteReq := sessionRequest(http.MethodDelete, "/api/v1/sources/"+catalogSourceID, catalogUserID, nil)
	deleteSession := requestSession(deleteReq)
	deleteSession.Token = deleteToken
	deleteSession.Absolute = now.Add(12 * time.Hour)
	deleteReq = deleteReq.WithContext(context.WithValue(deleteReq.Context(), sessionKey, deleteSession))
	deleteReq.SetPathValue("id", catalogSourceID)
	deleteResult := httptest.NewRecorder()
	catalogApp.deleteSource(deleteResult, deleteReq)
	if deleteResult.Code != http.StatusNoContent {
		close(catalogRelease)
		t.Fatalf("delete during catalog fetch failed: %d", deleteResult.Code)
	}
	close(catalogRelease)
	select {
	case <-catalogDone:
	case <-time.After(5 * time.Second):
		t.Fatal("catalog fetch did not finish after provider release")
	}
	catalogApp.private.mu.Lock()
	_, cached := catalogApp.private.catalogs[catalogSourceID+":movie:"]
	catalogStillInMemory := catalogApp.private.sources[catalogSourceID] != nil
	catalogApp.private.mu.Unlock()
	if catalogStillInMemory || cached {
		t.Fatal("deleted source or catalog was restored in RAM")
	}
	if err := db.QueryRow(ctx, "SELECT count(*) FROM source_catalogs WHERE source_id=$1", catalogSourceID).Scan(&catalogCount); err != nil || catalogCount != 0 {
		t.Fatal("catalog fetch restored PostgreSQL data after source deletion")
	}

	restarted := New(Config{DB: db, Origin: "http://127.0.0.1:8081", SessionSecret: []byte(strings.Repeat("s", 32)), IdentityKey: []byte(strings.Repeat("k", 32)), EncryptionKey: key, AllowedHosts: []string{"provider.example"}})
	loaded, err := restarted.private.loadSources(ctx, userID, time.Now().Add(time.Hour))
	if err != nil || len(loaded) != 1 || loaded[0].ID != sourceID || loaded[0].Password != passwordValue {
		t.Fatal("source did not rehydrate after API restart")
	}
	other, err := restarted.private.loadSource(ctx, "other-owner", sourceID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal("cross-owner source lookup returned a storage error")
	}
	if other != nil {
		t.Fatal("source was visible to another owner")
	}
	loadedCatalog, ok, err := restarted.private.loadCatalog(ctx, loaded[0], "movie:7")
	if err != nil || !ok || len(loadedCatalog.items["movie:7"]) != 1 || loadedCatalog.items["movie:7"][0].URL != "" {
		t.Fatal("sanitized catalog did not rehydrate with playback metadata")
	}

	wrongKey := New(Config{DB: db, Origin: "http://127.0.0.1:8081", SessionSecret: []byte(strings.Repeat("s", 32)), IdentityKey: []byte(strings.Repeat("k", 32)), EncryptionKey: []byte(strings.Repeat("x", 32)), AllowedHosts: []string{"provider.example"}})
	replacement := *src
	replacement.ID, err = randomToken(18)
	if err != nil {
		t.Fatal(err)
	}
	replacement.Password = "replacement-password"
	replacement.PlaylistURL = "https://provider.example/get.php?username=private-user&password=replacement-password"
	if err := wrongKey.private.saveSource(ctx, &replacement, nil); err == nil {
		t.Fatal("wrong encryption key overwrote the saved source")
	}
	if err := db.QueryRow(ctx, "SELECT id FROM sources WHERE user_id=$1", userID).Scan(&preservedID); err != nil || preservedID != sourceID {
		t.Fatal("wrong-key replacement did not preserve the existing source")
	}
	if _, err := db.Exec(ctx, "DELETE FROM sources WHERE id=$1 AND user_id=$2", sourceID, userID); err != nil {
		t.Fatal("could not delete test source")
	}
	if err := db.QueryRow(ctx, "SELECT count(*) FROM source_catalogs WHERE source_id=$1", sourceID).Scan(&catalogCount); err != nil || catalogCount != 0 {
		t.Fatal("deleting a source did not cascade to its catalog")
	}
}

func bytesContainsAny(haystack []byte, needles ...[]byte) bool {
	for _, needle := range needles {
		if len(needle) > 0 && strings.Contains(string(haystack), string(needle)) {
			return true
		}
	}
	return false
}
