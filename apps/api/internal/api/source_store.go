package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var errSourceStore = errors.New("source store unavailable")
var errSourceSessionRevoked = errors.New("source session no longer active")
var errInvalidInput = errors.New("invalid input")

type persistedCatalogItem struct {
	ID                 string `json:"id"`
	Title              string `json:"title"`
	Kind               string `json:"kind"`
	ContainerExtension string `json:"containerExtension,omitempty"`
	CategoryID         string `json:"categoryId,omitempty"`
	SeriesID           string `json:"seriesId,omitempty"`
	ContentKey         string `json:"contentKey,omitempty"`
}

type persistedCatalog struct {
	Categories map[string][]category             `json:"categories,omitempty"`
	Items      map[string][]persistedCatalogItem `json:"items,omitempty"`
	Episodes   map[string][]Episode              `json:"episodes,omitempty"`
}

func (s *privateState) saveSource(ctx context.Context, src *source, sessionTokenHash []byte) error {
	if s.cfg.DB == nil {
		return nil
	}
	ciphertext, err := encryptSourceSecret(s.cfg.EncryptionKey, src.UserID, src.ID, sourceSecret{OriginalURL: src.PlaylistURL, Base: src.Base, Username: src.Username, Password: src.Password})
	if err != nil {
		return errSourceStore
	}
	tx, err := s.cfg.DB.Begin(ctx)
	if err != nil {
		return errSourceStore
	}
	defer tx.Rollback(context.Background())
	var lockedUserID string
	if err = tx.QueryRow(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", src.UserID).Scan(&lockedUserID); err != nil {
		return errSourceStore
	}
	if len(sessionTokenHash) > 0 {
		var activeSession string
		err = tx.QueryRow(ctx, `SELECT user_id FROM sessions WHERE token_hash=$1 AND user_id=$2
			AND idle_expires_at>now() AND absolute_expires_at>now() FOR SHARE`, sessionTokenHash, src.UserID).Scan(&activeSession)
		if errors.Is(err, pgx.ErrNoRows) {
			return errSourceSessionRevoked
		}
		if err != nil {
			return errSourceStore
		}
	}
	var oldID string
	var oldCiphertext []byte
	err = tx.QueryRow(ctx, "SELECT id,secret_ciphertext FROM sources WHERE user_id=$1 FOR UPDATE", src.UserID).Scan(&oldID, &oldCiphertext)
	if err == nil {
		if _, err := decryptSourceSecret(s.cfg.EncryptionKey, src.UserID, oldID, oldCiphertext); err != nil {
			return errSourceStore
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return errSourceStore
	}
	if _, err = tx.Exec(ctx, "DELETE FROM sources WHERE user_id=$1", src.UserID); err != nil {
		return errSourceStore
	}
	_, err = tx.Exec(ctx, `INSERT INTO sources(id,user_id,label,account_key,fingerprint,max_connections,secret_ciphertext)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, src.ID, src.UserID, src.Label, src.AccountKey, src.Fingerprint, src.MaxConnections, ciphertext)
	if err != nil {
		return errSourceStore
	}
	if err := tx.Commit(ctx); err != nil {
		return errSourceStore
	}
	return nil
}

func (s *privateState) checkSourceSession(ctx context.Context, sess session) error {
	s.mu.Lock()
	generation := s.generation[sess.User.ID]
	s.mu.Unlock()
	if generation != sess.Generation {
		return errSourceSessionRevoked
	}
	if s.cfg.DB == nil {
		return nil
	}
	var active bool
	err := s.cfg.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE token_hash=$1 AND user_id=$2
		AND idle_expires_at>now() AND absolute_expires_at>now())`, hashToken(sess.Token), sess.User.ID).Scan(&active)
	if err != nil {
		return errSourceStore
	}
	if !active {
		return errSourceSessionRevoked
	}
	return nil
}

func (s *privateState) loadSource(ctx context.Context, userID, sourceID string, absolute time.Time) (*source, error) {
	if s.cfg.DB == nil {
		return nil, nil
	}
	var src source
	var ciphertext []byte
	err := s.cfg.DB.QueryRow(ctx, `SELECT id,user_id,label,account_key,fingerprint,max_connections,secret_ciphertext
		FROM sources WHERE id=$1 AND user_id=$2`, sourceID, userID).Scan(&src.ID, &src.UserID, &src.Label, &src.AccountKey, &src.Fingerprint, &src.MaxConnections, &ciphertext)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, errSourceStore
	}
	secret, err := decryptSourceSecret(s.cfg.EncryptionKey, userID, sourceID, ciphertext)
	if err != nil {
		return nil, errSourceStore
	}
	base, username, password, host, err := parsePlaylistURL(secret.OriginalURL)
	if err != nil || !s.allowedHost(host) || base != secret.Base || username != secret.Username || password != secret.Password {
		return nil, errSourceStore
	}
	if src.AccountKey != s.identityKey(base, username) || src.Fingerprint != s.fingerprint(base, username) {
		return nil, errSourceStore
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed == nil || parsed.Host == "" {
		return nil, errSourceStore
	}
	src.Base, src.Username, src.Password = base, username, password
	src.PlaylistURL = secret.OriginalURL
	if src.MaxConnections < 1 || src.MaxConnections > 64 {
		src.MaxConnections = 1
	}
	now := time.Now().UTC()
	src.Activity = now
	src.AbsoluteExpiresAt = absolute
	src.ExpiresAt = minTime(now.Add(30*time.Minute), absolute)
	src.Saved = true
	return &src, nil
}

func (s *privateState) loadSources(ctx context.Context, userID string, absolute time.Time) ([]*source, error) {
	if s.cfg.DB == nil {
		return nil, nil
	}
	rows, err := s.cfg.DB.Query(ctx, `SELECT id FROM sources WHERE user_id=$1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, errSourceStore
	}
	ids := make([]string, 0, 2)
	for rows.Next() {
		var id string
		if rows.Scan(&id) != nil {
			rows.Close()
			return nil, errSourceStore
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		rows.Close()
		return nil, errSourceStore
	}
	rows.Close()
	out := make([]*source, 0, len(ids))
	for _, id := range ids {
		src, err := s.loadSource(ctx, userID, id, absolute)
		if err != nil {
			return nil, err
		}
		if src != nil {
			out = append(out, src)
		}
	}
	return out, nil
}

func (s *privateState) deleteSavedSource(ctx context.Context, userID, sourceID string) (bool, error) {
	if s.cfg.DB == nil {
		return true, nil
	}
	tag, err := s.cfg.DB.Exec(ctx, "DELETE FROM sources WHERE id=$1 AND user_id=$2", sourceID, userID)
	if err != nil {
		return false, errSourceStore
	}
	return tag.RowsAffected() > 0, nil
}

func catalogStoreKey(typ, id string) string {
	return typ + ":" + id
}

func episodesStoreKey(seriesID string) string { return "episodes:" + seriesID }

func (s *privateState) saveCatalog(ctx context.Context, src *source, key string, data catalogData) error {
	if s.cfg.DB == nil {
		return nil
	}
	encoded, err := encodeCatalog(data)
	if err != nil {
		return errSourceStore
	}
	tag, err := s.cfg.DB.Exec(ctx, `INSERT INTO source_catalogs(source_id,catalog_key,payload,expires_at)
		SELECT id,$3,$4::jsonb,$5 FROM sources WHERE id=$1 AND user_id=$2
		ON CONFLICT(source_id,catalog_key)
		DO UPDATE SET payload=EXCLUDED.payload,expires_at=EXCLUDED.expires_at,updated_at=now()`, src.ID, src.UserID, key, string(encoded), data.expires)
	if err != nil || tag.RowsAffected() != 1 {
		return errSourceStore
	}
	return nil
}

func encodeCatalog(data catalogData) ([]byte, error) {
	doc := persistedCatalog{Categories: data.categories, Items: map[string][]persistedCatalogItem{}, Episodes: data.episodes}
	for itemKey, items := range data.items {
		stored := make([]persistedCatalogItem, 0, len(items))
		for _, item := range items {
			stored = append(stored, persistedCatalogItem{ID: item.ID, Title: item.Title, Kind: item.Kind, ContainerExtension: item.ContainerExtension, CategoryID: item.CategoryID, SeriesID: item.SeriesID, ContentKey: item.ContentKey})
		}
		doc.Items[itemKey] = stored
	}
	encoded, err := json.Marshal(doc)
	return encoded, err
}

func (s *privateState) loadCatalog(ctx context.Context, src *source, key string) (catalogData, bool, error) {
	if s.cfg.DB == nil {
		return catalogData{}, false, nil
	}
	if !validateCatalogKey(key) {
		return catalogData{}, false, errSourceStore
	}
	var encoded []byte
	var expires time.Time
	err := s.cfg.DB.QueryRow(ctx, `SELECT c.payload,c.expires_at FROM source_catalogs c
		JOIN sources s ON s.id=c.source_id WHERE c.source_id=$1 AND s.user_id=$2 AND c.catalog_key=$3`, src.ID, src.UserID, key).Scan(&encoded, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalogData{}, false, nil
	}
	if err != nil {
		return catalogData{}, false, errSourceStore
	}
	if !time.Now().Before(expires) {
		return catalogData{}, false, nil
	}
	var doc persistedCatalog
	if json.Unmarshal(encoded, &doc) != nil {
		return catalogData{}, false, errSourceStore
	}
	data := catalogData{expires: expires, categories: map[string][]category{}, items: map[string][]CatalogItem{}, episodes: map[string][]Episode{}}
	for typ, rows := range doc.Categories {
		if !validCatalogType(typ) {
			continue
		}
		for _, row := range rows {
			if numericID(row.ID) && row.Name != "" {
				data.categories[typ] = append(data.categories[typ], row)
			}
		}
	}
	for itemKey, rows := range doc.Items {
		parts := strings.Split(itemKey, ":")
		if len(parts) != 2 || !validCatalogType(parts[0]) || !numericID(parts[1]) {
			continue
		}
		for _, row := range rows {
			if !numericID(row.ID) || row.Title == "" || row.Kind != parts[0] {
				continue
			}
			categoryID := row.CategoryID
			if categoryID == "" {
				categoryID = parts[1]
			}
			if !numericID(categoryID) {
				continue
			}
			item := CatalogItem{ID: row.ID, Title: row.Title, Kind: row.Kind, ContainerExtension: validatedExtension(row.ContainerExtension), CategoryID: categoryID, SeriesID: row.SeriesID}
			item.ContentKey = stableContentKey(s.cfg.IdentityKey, src.AccountKey, item.Kind, item.ID)
			data.items[itemKey] = append(data.items[itemKey], item)
		}
	}
	for seriesID, rows := range doc.Episodes {
		if !numericID(seriesID) {
			continue
		}
		for _, row := range rows {
			if !numericID(row.ID) || row.Title == "" || row.Season < 0 || row.Episode < 0 {
				continue
			}
			row.ContainerExtension = validatedExtension(row.ContainerExtension)
			row.URL = ""
			row.ContentKey = stableContentKey(s.cfg.IdentityKey, src.AccountKey, "episode", row.ID)
			data.episodes[seriesID] = append(data.episodes[seriesID], row)
		}
	}
	return data, true, nil
}

func catalogKeyParts(key string) (string, string, bool) {
	parts := strings.Split(key, ":")
	if len(parts) != 2 || !validCatalogType(parts[0]) || parts[1] != "" && !numericID(parts[1]) {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func validateCatalogKey(key string) bool {
	if strings.HasPrefix(key, "episodes:") {
		return numericID(strings.TrimPrefix(key, "episodes:"))
	}
	_, _, ok := catalogKeyParts(key)
	return ok
}
