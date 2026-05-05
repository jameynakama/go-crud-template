package api_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jameynakama/APPNAME/internal/api"
	"github.com/jameynakama/APPNAME/internal/store"
)

var testPool *pgxpool.Pool

func getRequiredEnvVar(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s must be set", key)
	}
	return v
}

func getDBConn(ctx context.Context, dbURL string) *pgxpool.Pool {
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("error establishing test database connection: %v", err)
	}

	if err := db.Ping(ctx); err != nil {
		log.Fatalf("cannot ping test database %s: %v", dbURL, err)
	}
	log.Println("test database connected")

	return db
}

func TestMain(m *testing.M) {
	testDBURL := getRequiredEnvVar("TEST_DATABASE_URL")
	testDBName := getDBName(testDBURL)

	ctx := context.Background()

	pgDB := getDBConn(ctx, swapDBName(testDBURL, "postgres"))
	_, err := pgDB.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
	if err != nil {
		log.Println("Could not drop test database during setup")
	}
	_, err = pgDB.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", testDBName))
	if err != nil {
		log.Fatal("Could not create test database")
	}

	migrateURL := strings.Replace(testDBURL, "postgres://", "pgx5://", 1)
	mig, err := migrate.New("file://../../migrations", migrateURL)
	if err != nil {
		log.Fatalf("could not create migrate instance for %s: %v", migrateURL, err)
	}
	if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("could not migrate test db %s: %v", migrateURL, err)
	}

	testPool = getDBConn(ctx, testDBURL)

	code := m.Run()

	testPool.Close()
	// Postgres refuses to drop a DB with active connections -- force evictions first
	pgDB.Exec(ctx, `
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = $1 AND pid <> pg_backend_pid()
	`, testDBName)
	_, err = pgDB.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
	if err != nil {
		log.Println("Could not drop test database")
	}
	pgDB.Close()

	os.Exit(code)
}

// newTestServer returns a fresh router wired to the test DB
func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	return api.NewRouter(api.RouterConfig{Queries: store.New(testPool)})
}

// truncate wipes the things table between tests
func truncate(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "TRUNCATE things RESTART IDENTITY CASCADE")
	if err != nil {
		log.Fatalf("failed to truncate: %v", err)
	}
}

func getDBName(dbURL string) string {
	u, _ := url.Parse(dbURL)
	return u.Path[1:]
}

func swapDBName(oldDB, newDB string) string {
	u, _ := url.Parse(oldDB)
	u.Path = "/" + newDB
	return u.String()
}

func TestSwapDBName(t *testing.T) {
	expected := "pg://hello:moto@some.place/woof?one=1&two=2"

	url := "pg://hello:moto@some.place/meow?one=1&two=2"
	replacement := "woof"

	if r := swapDBName(url, replacement); r != expected {
		t.Errorf("wanted %s but got %s", expected, r)
	}
}

func TestGetDBName(t *testing.T) {
	expected := "woof"
	url := "pg://hello:moto@some.place/woof?one=1&two=2"

	if r := getDBName(url); r != expected {
		t.Errorf("wanted %s but got %s", expected, r)
	}
}

func TestListThings(t *testing.T) {
	truncate(t)

	expected := http.StatusOK

	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/things", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	status := w.Result().StatusCode

	if status != expected {
		t.Errorf("expected %d; got %d", expected, status)
	}
}

func TestGetThing404(t *testing.T) {
	truncate(t)

	expected := http.StatusNotFound

	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/things/666", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	status := w.Result().StatusCode

	if status != expected {
		t.Errorf("expected %d; got %d", expected, status)
	}
}

func TestCreateThing(t *testing.T) {
	truncate(t)

	expected := http.StatusCreated

	srv := newTestServer(t)
	payload := strings.NewReader(`{"name": "apple", "description": "juice"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/things", payload)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	status := w.Result().StatusCode

	if status != expected {
		t.Errorf("expected %d; got %d", expected, status)
	}
}

func TestUpdateThing404(t *testing.T) {
	truncate(t)

	expected := http.StatusNotFound

	srv := newTestServer(t)
	payload := strings.NewReader(`{"name": "apple", "description": "juice"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/things/666", payload)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	status := w.Result().StatusCode

	if status != expected {
		t.Errorf("expected %d; got %d", expected, status)
	}
}

func TestDeleteThing(t *testing.T) {
	truncate(t)

	expected := http.StatusNoContent

	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/things/666", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	status := w.Result().StatusCode

	if status != expected {
		t.Errorf("expected %d; got %d", expected, status)
	}
}
