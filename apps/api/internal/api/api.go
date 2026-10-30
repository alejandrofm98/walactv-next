package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Config struct {
	DB                    *pgxpool.Pool
	Origin                string
	SessionSecret         []byte
	IdentityKey           []byte
	EncryptionKey         []byte
	AllowedHosts          []string
	HTTPClient            *http.Client // supplied only by deterministic tests
	TMDBToken             string
	VODTranscodingEnabled bool
	VODTempDir            string // Internal override used by isolated tests; production defaults to the API-owned tmpfs.
}

type API struct {
	cfg      Config
	mux      *http.ServeMux
	private  *privateState
	metadata *metadataService
}

type user struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type session struct {
	User       user
	CSRF       string
	Token      string
	Absolute   time.Time
	Generation uint64
}

type contextKey string

const sessionKey contextKey = "session"

func New(cfg Config) *API {
	a := &API{cfg: cfg, mux: http.NewServeMux(), private: newPrivateState(cfg)}
	a.metadata = newMetadataService(cfg)
	a.routes()
	return a
}

func (a *API) Handler() http.Handler { return a.mux }

// Shutdown revokes in-memory media work before the HTTP server stops accepting
// requests, then waits for each playback's upstream and FFmpeg process to end.
func (a *API) Shutdown(ctx context.Context) error {
	a.private.mu.Lock()
	a.private.shuttingDown = true
	done := make([]<-chan struct{}, 0, len(a.private.playbacks))
	for id, p := range a.private.playbacks {
		a.private.releasePlaybackLocked(id)
		if p.Done != nil {
			done = append(done, p.Done)
		}
	}
	a.private.mu.Unlock()
	for _, ch := range done {
		select {
		case <-ch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (a *API) routes() {
	a.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := requestContext(r, 2*time.Second)
		defer cancel()
		if err := a.cfg.DB.Ping(ctx); err != nil {
			writeError(w, http.StatusServiceUnavailable, "UNHEALTHY", "Service unavailable")
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	a.mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := requestContext(r, 2*time.Second)
		defer cancel()
		if err := a.cfg.DB.Ping(ctx); err != nil {
			writeError(w, http.StatusServiceUnavailable, "UNHEALTHY", "Service unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	a.mux.HandleFunc("POST /api/v1/auth/login", a.login)
	a.mux.Handle("GET /api/v1/auth/me", a.auth(http.HandlerFunc(a.me)))
	a.mux.Handle("POST /api/v1/auth/logout", a.auth(a.csrf(http.HandlerFunc(a.logout))))
	a.mux.Handle("GET /api/v1/favorites", a.auth(http.HandlerFunc(a.listFavorites)))
	a.mux.Handle("POST /api/v1/favorites", a.auth(a.csrf(http.HandlerFunc(a.createFavorite))))
	a.mux.Handle("DELETE /api/v1/favorites/{id}", a.auth(a.csrf(http.HandlerFunc(a.deleteFavorite))))
	a.mux.Handle("GET /api/v1/progress", a.auth(http.HandlerFunc(a.listProgress)))
	a.mux.Handle("PUT /api/v1/progress", a.auth(a.csrf(http.HandlerFunc(a.putProgress))))
	a.mux.Handle("GET /api/v1/sources", a.auth(http.HandlerFunc(a.listSources)))
	a.mux.Handle("POST /api/v1/sources", a.auth(a.csrf(http.HandlerFunc(a.createSource))))
	a.mux.Handle("DELETE /api/v1/sources/{id}", a.auth(a.csrf(http.HandlerFunc(a.deleteSource))))
	a.mux.Handle("GET /api/v1/sources/{id}/categories", a.auth(http.HandlerFunc(a.getCategories)))
	a.mux.Handle("GET /api/v1/sources/{id}/languages", a.auth(http.HandlerFunc(a.getLanguages)))
	a.mux.Handle("GET /api/v1/sources/{id}/catalog", a.auth(http.HandlerFunc(a.getCatalogStatus)))
	a.mux.Handle("POST /api/v1/sources/{id}/catalog/refresh", a.auth(a.csrf(http.HandlerFunc(a.refreshCatalog))))
	a.mux.Handle("GET /api/v1/sources/{id}/images/{kind}/{itemId}", a.auth(http.HandlerFunc(a.getCatalogImage)))
	a.mux.Handle("GET /api/v1/sources/{id}/items", a.auth(http.HandlerFunc(a.getItems)))
	a.mux.Handle("GET /api/v1/sources/{id}/series/{seriesId}/episodes", a.auth(http.HandlerFunc(a.getEpisodes)))
	a.mux.Handle("GET /api/v1/sources/{id}/items/{kind}/{itemId}/metadata", a.auth(http.HandlerFunc(a.getOnDemandMetadata)))
	a.mux.Handle("GET /api/v1/sources/{id}/items/series/{itemId}/seasons/{season}/metadata", a.auth(http.HandlerFunc(a.getOnDemandSeasonMetadata)))
	a.mux.Handle("GET /api/v1/acestream/channels", a.auth(http.HandlerFunc(a.acestreamChannels)))
	a.mux.Handle("POST /api/v1/playbacks", a.auth(a.csrf(http.HandlerFunc(a.createPlayback))))
	a.mux.Handle("GET /api/v1/playbacks/{id}/diagnostic", a.auth(http.HandlerFunc(a.playbackDiagnostic)))
	a.mux.Handle("POST /api/v1/playbacks/{id}/heartbeat", a.auth(a.csrf(http.HandlerFunc(a.heartbeatPlayback))))
	a.mux.Handle("DELETE /api/v1/playbacks/{id}", a.auth(a.csrf(http.HandlerFunc(a.deletePlayback))))
	a.mux.Handle("POST /api/v1/playbacks/{id}/transcode", a.auth(a.csrf(http.HandlerFunc(a.startVODTranscode))))
	a.mux.Handle("DELETE /api/v1/playbacks/{id}/transcode", a.auth(a.csrf(http.HandlerFunc(a.cancelVODTranscode))))
	a.mux.Handle("GET /api/v1/playbacks/{id}/transcode/{generation}/{asset}", a.auth(http.HandlerFunc(a.serveVODAsset)))
	a.mux.Handle("GET /api/v1/media/{playback}/{asset}", a.auth(http.HandlerFunc(a.proxyMedia)))
	a.mux.Handle("HEAD /api/v1/media/{playback}/{asset}", a.auth(http.HandlerFunc(a.proxyMedia)))
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	if !a.validOrigin(r) {
		writeError(w, http.StatusForbidden, "ORIGIN", "Request origin is not allowed")
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &input); err != nil || strings.TrimSpace(input.Username) == "" || input.Password == "" {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid login request")
		return
	}
	ctx, cancel := requestContext(r, 5*time.Second)
	defer cancel()
	var u user
	var passwordHash string
	err := a.cfg.DB.QueryRow(ctx, "SELECT id,username,role,password_hash FROM users WHERE username=$1", input.Username).Scan(&u.ID, &u.Username, &u.Role, &passwordHash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE", "Service unavailable")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Username or password is incorrect")
		return
	}
	token, err := randomToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Could not create session")
		return
	}
	csrf := csrfToken(a.cfg.SessionSecret, token)
	now := time.Now().UTC()
	if _, err := a.cfg.DB.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,idle_expires_at,absolute_expires_at)
		VALUES($1,$2,$3,$4)`, hashToken(token), u.ID, now.Add(30*time.Minute), now.Add(12*time.Hour)); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Could not create session")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "walactv_session", Value: token, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(a.cfg.Origin, "https://"), SameSite: http.SameSiteStrictMode, Expires: now.Add(12 * time.Hour)})
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "csrfToken": csrf})
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	s := requestSession(r)
	writeJSON(w, http.StatusOK, map[string]any{"user": s.User, "csrfToken": s.CSRF})
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	s := requestSession(r)
	ctx, cancel := requestContext(r, 5*time.Second)
	defer cancel()
	if _, err := a.cfg.DB.Exec(ctx, "DELETE FROM sessions WHERE token_hash=$1", hashToken(s.Token)); err != nil {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE", "Could not end session")
		return
	}
	a.private.storeMu.Lock()
	defer a.private.storeMu.Unlock()
	a.private.mu.Lock()
	a.private.generation[s.User.ID]++
	for id, src := range a.private.sources {
		if src.UserID == s.User.ID {
			a.private.dropPlaybackForSourceLocked(id)
			delete(a.private.sources, id)
			a.private.evictSourceSessionLocked(id)
		}
	}
	a.private.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "walactv_session", Value: "", Path: "/", HttpOnly: true, Secure: strings.HasPrefix(a.cfg.Origin, "https://"), SameSite: http.SameSiteStrictMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("walactv_session")
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
			return
		}
		ctx, cancel := requestContext(r, 5*time.Second)
		defer cancel()
		var s session
		var expires, absolute time.Time
		err = a.cfg.DB.QueryRow(ctx, `SELECT u.id,u.username,u.role,s.idle_expires_at,s.absolute_expires_at
			FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1`, hashToken(cookie.Value)).Scan(&s.User.ID, &s.User.Username, &s.User.Role, &expires, &absolute)
		now := time.Now().UTC()
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE", "Service unavailable")
			return
		}
		if errors.Is(err, pgx.ErrNoRows) || now.After(expires) || now.After(absolute) {
			_, _ = a.cfg.DB.Exec(ctx, "DELETE FROM sessions WHERE token_hash=$1", hashToken(cookie.Value))
			http.SetCookie(w, &http.Cookie{Name: "walactv_session", Value: "", Path: "/", HttpOnly: true, Secure: strings.HasPrefix(a.cfg.Origin, "https://"), SameSite: http.SameSiteStrictMode, MaxAge: -1})
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
			return
		}
		a.private.mu.Lock()
		s.Generation = a.private.generation[s.User.ID]
		a.private.mu.Unlock()
		tag, err := a.cfg.DB.Exec(ctx, `UPDATE sessions SET idle_expires_at=$2 WHERE token_hash=$1 AND idle_expires_at>now() AND absolute_expires_at>now()`, hashToken(cookie.Value), minTime(now.Add(30*time.Minute), absolute))
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE", "Service unavailable")
			return
		}
		if tag.RowsAffected() != 1 {
			http.SetCookie(w, &http.Cookie{Name: "walactv_session", Value: "", Path: "/", HttpOnly: true, Secure: strings.HasPrefix(a.cfg.Origin, "https://"), SameSite: http.SameSiteStrictMode, MaxAge: -1})
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
			return
		}
		s.CSRF = csrfToken(a.cfg.SessionSecret, cookie.Value)
		s.Token = cookie.Value
		s.Absolute = absolute
		r = r.WithContext(context.WithValue(r.Context(), sessionKey, s))
		next.ServeHTTP(w, r)
	})
}

func (a *API) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.validOrigin(r) {
			writeError(w, http.StatusForbidden, "ORIGIN", "Request origin is not allowed")
			return
		}
		s := requestSession(r)
		got := r.Header.Get("X-CSRF-Token")
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(s.CSRF)) != 1 {
			writeError(w, http.StatusForbidden, "CSRF", "CSRF token is invalid")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) validOrigin(r *http.Request) bool {
	return r.Header.Get("Origin") != "" && r.Header.Get("Origin") == a.cfg.Origin
}

func requestSession(r *http.Request) session {
	v, _ := r.Context().Value(sessionKey).(session)
	return v
}

func requestContext(r *http.Request, limit time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), limit)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("multiple JSON values")
	}
	return nil
}

func hashToken(value string) []byte { sum := sha256.Sum256([]byte(value)); return sum[:] }

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func csrfToken(secret []byte, token string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte("walactv-csrf:" + token))
	return hex.EncodeToString(mac.Sum(nil))
}

func minTime(a, b time.Time) time.Time {
	if a.IsZero() {
		return b
	}
	if b.IsZero() || a.Before(b) {
		return a
	}
	return b
}
