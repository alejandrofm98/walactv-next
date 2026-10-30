package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"walactv.local/api/internal/api"
)

func main() {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if err := run(command); err != nil {
		log.Printf("%s failed: %v", command, err)
		os.Exit(1)
	}
}

func run(command string) error {
	if command == "bootstrap-admin" {
		command = "bootstrap"
	}
	if command != "migrate" && command != "bootstrap" && command != "serve" {
		return fmt.Errorf("unknown command %q", command)
	}
	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return errors.New("could not initialize PostgreSQL pool")
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return errors.New("could not connect to PostgreSQL")
	}
	switch command {
	case "migrate":
		return migrate(ctx, pool)
	case "bootstrap":
		return bootstrap(ctx, pool)
	default:
		return serve(pool)
	}
}

func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock(731904281)"); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock(731904281)")
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return err
	}
	entries, err := os.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	versions := make([]int, 0, len(entries))
	filenames := map[int]string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			continue
		}
		version, parseErr := strconv.Atoi(prefix)
		if parseErr != nil || version < 1 {
			continue
		}
		versions = append(versions, version)
		filenames[version] = entry.Name()
	}
	sort.Ints(versions)
	for _, version := range versions {
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		schema, err := os.ReadFile("migrations/" + filenames[version])
		if err != nil {
			return fmt.Errorf("read migration %d: %w", version, err)
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(schema)); err != nil {
			_ = tx.Rollback(context.Background())
			return fmt.Errorf("apply migration %d: %w", version, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES ($1)", version); err != nil {
			_ = tx.Rollback(context.Background())
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func bootstrap(ctx context.Context, pool *pgxpool.Pool) error {
	username := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_USERNAME"))
	password := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	if username == "" || password == "" {
		return errors.New("BOOTSTRAP_ADMIN_USERNAME and BOOTSTRAP_ADMIN_PASSWORD are required")
	}
	if len(username) > 120 || len(password) < 12 || len(password) > 72 {
		return errors.New("bootstrap credentials do not meet length requirements")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return errors.New("could not create administrator")
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(731904282)"); err != nil {
		return errors.New("could not create administrator")
	}
	var count int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM users WHERE role='admin'").Scan(&count); err != nil {
		return errors.New("database is not initialized; run migrate first")
	}
	if count > 0 {
		return tx.Commit(ctx)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("could not hash bootstrap password")
	}
	id, err := randomID(18)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO users(id, username, password_hash, role) VALUES ($1,$2,$3,'admin')", id, username, string(hash)); err != nil {
		return errors.New("could not create administrator")
	}
	return tx.Commit(ctx)
}

func serve(pool *pgxpool.Pool) error {
	appOrigin := strings.TrimRight(strings.TrimSpace(os.Getenv("APP_ORIGIN")), "/")
	if appOrigin == "" || len(os.Getenv("SESSION_SECRET")) < 32 || len(os.Getenv("PROVIDER_IDENTITY_KEY")) < 32 {
		return errors.New("APP_ORIGIN and 32-byte SESSION_SECRET and PROVIDER_IDENTITY_KEY are required")
	}
	origin, err := url.Parse(appOrigin)
	if err != nil || origin.Host == "" || (origin.Scheme != "http" && origin.Scheme != "https") || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return errors.New("APP_ORIGIN must be an HTTP origin without a path")
	}
	if os.Getenv("PRIVATE_STATE_STORE") != "" && os.Getenv("PRIVATE_STATE_STORE") != "memory" {
		return errors.New("PRIVATE_STATE_STORE must be memory")
	}
	if replicas := os.Getenv("API_REPLICAS"); replicas != "" && replicas != "1" {
		return errors.New("private IPTV state requires API_REPLICAS=1")
	}
	encryptionKey, err := decodeEncryptionKey(os.Getenv("IPTV_ENCRYPTION_KEY"))
	if err != nil {
		return errors.New("IPTV_ENCRYPTION_KEY must be base64 encoding of 32 bytes")
	}
	app := api.New(api.Config{
		DB: pool, Origin: appOrigin, SessionSecret: []byte(os.Getenv("SESSION_SECRET")),
		IdentityKey: []byte(os.Getenv("PROVIDER_IDENTITY_KEY")), EncryptionKey: encryptionKey,
		AllowedHosts:          strings.Split(os.Getenv("IPTV_ALLOWED_HOSTS"), ","),
		TMDBToken:             os.Getenv("TMDB_API_TOKEN"),
		VODTranscodingEnabled: strings.EqualFold(strings.TrimSpace(os.Getenv("VOD_TRANSCODING_ENABLED")), "true"),
	})
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.Shutdown(shutdown); err != nil {
			log.Printf("media shutdown did not finish within the grace period")
		}
		_ = server.Shutdown(shutdown)
	}()
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func decodeEncryptionKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(key) != 32 {
		return nil, errors.New("invalid encryption key")
	}
	return key, nil
}

func randomID(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func tokenHash(value string) []byte {
	hash := sha256.Sum256([]byte(value))
	return hash[:]
}
