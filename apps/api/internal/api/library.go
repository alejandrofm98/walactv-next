package api

import (
	"math"
	"net/http"
	"strings"
	"time"
)

func (a *API) listFavorites(w http.ResponseWriter, r *http.Request) {
	s := requestSession(r)
	ctx, cancel := requestContext(r, 5*time.Second)
	defer cancel()
	rows, err := a.cfg.DB.Query(ctx, "SELECT id,content_key,title,kind FROM favorites WHERE user_id=$1 ORDER BY created_at DESC", s.User.ID)
	if err != nil {
		writeError(w, 500, "INTERNAL", "Could not load favorites")
		return
	}
	defer rows.Close()
	items := []map[string]string{}
	for rows.Next() {
		var id, key, title, kind string
		if rows.Scan(&id, &key, &title, &kind) != nil {
			writeError(w, 500, "INTERNAL", "Could not load favorites")
			return
		}
		items = append(items, map[string]string{"id": id, "contentKey": key, "title": title, "kind": kind})
	}
	if rows.Err() != nil {
		writeError(w, 500, "INTERNAL", "Could not load favorites")
		return
	}
	writeJSON(w, 200, map[string]any{"favorites": items})
}

func (a *API) createFavorite(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ContentKey string `json:"contentKey"`
		Title      string `json:"title"`
		Kind       string `json:"kind"`
	}
	if decodeJSON(w, r, &input) != nil || len(input.ContentKey) != 64 || strings.TrimSpace(input.Title) == "" || len(input.Title) > 500 || !validLibraryKind(input.Kind) {
		writeError(w, 400, "INVALID_INPUT", "Invalid favorite")
		return
	}
	s := requestSession(r)
	title, ok := a.private.knownContent(s.User.ID, input.ContentKey, input.Kind)
	if !ok {
		writeError(w, 404, "NOT_FOUND", "Catalog item not found")
		return
	}
	if title != "" {
		input.Title = title
	}
	id, err := randomToken(18)
	if err != nil {
		writeError(w, 500, "INTERNAL", "Could not save favorite")
		return
	}
	ctx, cancel := requestContext(r, 5*time.Second)
	defer cancel()
	var favoriteID string
	err = a.cfg.DB.QueryRow(ctx, `INSERT INTO favorites(id,user_id,content_key,title,kind) VALUES($1,$2,$3,$4,$5)
		ON CONFLICT(user_id,content_key) DO UPDATE SET title=EXCLUDED.title RETURNING id`, id, s.User.ID, input.ContentKey, input.Title, input.Kind).Scan(&favoriteID)
	if err != nil {
		writeError(w, 500, "INTERNAL", "Could not save favorite")
		return
	}
	writeJSON(w, 201, map[string]string{"id": favoriteID, "contentKey": input.ContentKey, "title": input.Title, "kind": input.Kind})
}

func (a *API) deleteFavorite(w http.ResponseWriter, r *http.Request) {
	s := requestSession(r)
	ctx, cancel := requestContext(r, 5*time.Second)
	defer cancel()
	_, err := a.cfg.DB.Exec(ctx, "DELETE FROM favorites WHERE id=$1 AND user_id=$2", r.PathValue("id"), s.User.ID)
	if err != nil {
		writeError(w, 500, "INTERNAL", "Could not delete favorite")
		return
	}
	w.WriteHeader(204)
}

func (a *API) listProgress(w http.ResponseWriter, r *http.Request) {
	s := requestSession(r)
	ctx, cancel := requestContext(r, 5*time.Second)
	defer cancel()
	rows, err := a.cfg.DB.Query(ctx, `SELECT content_key,title,kind,position_seconds,duration_seconds,updated_at FROM progress WHERE user_id=$1 ORDER BY updated_at DESC`, s.User.ID)
	if err != nil {
		writeError(w, 500, "INTERNAL", "Could not load progress")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var key, title, kind string
		var position, duration float64
		var updated time.Time
		if rows.Scan(&key, &title, &kind, &position, &duration, &updated) != nil {
			writeError(w, 500, "INTERNAL", "Could not load progress")
			return
		}
		items = append(items, map[string]any{"contentKey": key, "title": title, "kind": kind, "positionSeconds": position, "durationSeconds": duration, "updatedAt": updated})
	}
	if rows.Err() != nil {
		writeError(w, 500, "INTERNAL", "Could not load progress")
		return
	}
	writeJSON(w, 200, map[string]any{"progress": items})
}

func (a *API) putProgress(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ContentKey string  `json:"contentKey"`
		Title      string  `json:"title"`
		Kind       string  `json:"kind"`
		Position   float64 `json:"positionSeconds"`
		Duration   float64 `json:"durationSeconds"`
	}
	if decodeJSON(w, r, &input) != nil || len(input.ContentKey) != 64 || strings.TrimSpace(input.Title) == "" || len(input.Title) > 500 || !validLibraryKind(input.Kind) || math.IsNaN(input.Position) || math.IsInf(input.Position, 0) || math.IsNaN(input.Duration) || math.IsInf(input.Duration, 0) || input.Position < 0 || input.Duration < 0 || input.Duration > 0 && input.Position > input.Duration {
		writeError(w, 400, "INVALID_INPUT", "Invalid progress")
		return
	}
	s := requestSession(r)
	title, ok := a.private.knownContent(s.User.ID, input.ContentKey, input.Kind)
	if !ok {
		writeError(w, 404, "NOT_FOUND", "Catalog item not found")
		return
	}
	if title != "" {
		input.Title = title
	}
	ctx, cancel := requestContext(r, 5*time.Second)
	defer cancel()
	var updated time.Time
	err := a.cfg.DB.QueryRow(ctx, `INSERT INTO progress(user_id,content_key,title,kind,position_seconds,duration_seconds) VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT(user_id,content_key) DO UPDATE SET title=EXCLUDED.title,kind=EXCLUDED.kind,position_seconds=EXCLUDED.position_seconds,duration_seconds=EXCLUDED.duration_seconds,updated_at=now() RETURNING updated_at`, s.User.ID, input.ContentKey, input.Title, input.Kind, input.Position, input.Duration).Scan(&updated)
	if err != nil {
		writeError(w, 500, "INTERNAL", "Could not save progress")
		return
	}
	writeJSON(w, 200, map[string]any{"contentKey": input.ContentKey, "title": input.Title, "kind": input.Kind, "positionSeconds": input.Position, "durationSeconds": input.Duration, "updatedAt": updated})
}

func validLibraryKind(v string) bool { return v == "live" || v == "movie" || v == "episode" }
