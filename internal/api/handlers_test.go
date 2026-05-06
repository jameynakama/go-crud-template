package api_test

import (
	"context"
	"encoding/json"
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
	"github.com/GHUSER/APPNAME/internal/api"
	"github.com/GHUSER/APPNAME/internal/store"
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
	return db
}

func TestMain(m *testing.M) {
	testDBURL := getRequiredEnvVar("TEST_DATABASE_URL")
	testDBName := getDBName(testDBURL)

	ctx := context.Background()

	pgDB := getDBConn(ctx, swapDBName(testDBURL, "postgres"))
	pgDB.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
	if _, err := pgDB.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", testDBName)); err != nil {
		log.Fatal("could not create test database")
	}

	migrateURL := strings.Replace(testDBURL, "postgres://", "pgx5://", 1)
	mig, err := migrate.New("file://../../migrations", migrateURL)
	if err != nil {
		log.Fatalf("could not create migrate instance: %v", err)
	}
	if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("could not migrate test db: %v", err)
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
	pgDB.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
	pgDB.Close()

	os.Exit(code)
}

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	return api.NewRouter(api.RouterConfig{Queries: store.New(testPool)})
}

func truncate(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "TRUNCATE users RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func createTestUser(t *testing.T, username string, isAdmin bool) store.User {
	t.Helper()
	user, err := store.New(testPool).CreateUser(context.Background(), store.CreateUserParams{
		Username: username,
		IsAdmin:  isAdmin,
	})
	if err != nil {
		t.Fatalf("createTestUser: %v", err)
	}
	return user
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
	if r := swapDBName("pg://hello:moto@some.place/meow?one=1&two=2", "woof"); r != expected {
		t.Errorf("wanted %s but got %s", expected, r)
	}
}

func TestGetDBName(t *testing.T) {
	if r := getDBName("pg://hello:moto@some.place/woof?one=1&two=2"); r != "woof" {
		t.Errorf("wanted woof but got %s", r)
	}
}

func TestListUsers(t *testing.T) {
	truncate(t)

	for i := range 3 {
		createTestUser(t, fmt.Sprintf("user%d", i), false)
	}

	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200; got %d", w.Result().StatusCode)
	}

	var users []store.User
	if err := json.NewDecoder(w.Body).Decode(&users); err != nil {
		t.Fatalf("error decoding response: %v", err)
	}
	if len(users) != 3 {
		t.Errorf("expected 3 users; got %d", len(users))
	}
}

func TestGetUser404(t *testing.T) {
	truncate(t)
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/666", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected 404; got %d", w.Result().StatusCode)
	}
}

func TestGetUser(t *testing.T) {
	truncate(t)
	user := createTestUser(t, "jamey", false)

	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/users/%d", user.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200; got %d", w.Result().StatusCode)
	}

	if err := json.NewDecoder(w.Body).Decode(&user); err != nil {
		t.Fatalf("error decoding response: %v", err)
	}
	if user.Username != "jamey" {
		t.Errorf("expected username 'jamey'; got '%s'", user.Username)
	}
}

func TestCreateUser(t *testing.T) {
	truncate(t)
	srv := newTestServer(t)

	body := strings.NewReader(`{"username":"newuser","is_admin":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusCreated {
		t.Fatalf("expected 201; got %d: %s", w.Result().StatusCode, w.Body.String())
	}

	var created store.User
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("error decoding response: %v", err)
	}
	if _, err := store.New(testPool).GetUser(context.Background(), created.ID); err != nil {
		t.Errorf("created user not found in db: %v", err)
	}
}

func TestUpdateUser404(t *testing.T) {
	truncate(t)
	srv := newTestServer(t)

	body := strings.NewReader(`{"username":"updated","is_admin":false}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/666", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected 404; got %d", w.Result().StatusCode)
	}
}

func TestUpdateUser(t *testing.T) {
	truncate(t)
	user := createTestUser(t, "original", false)

	srv := newTestServer(t)
	body := strings.NewReader(`{"username":"updated","is_admin":true}`)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/users/%d", user.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200; got %d", w.Result().StatusCode)
	}

	if err := json.NewDecoder(w.Body).Decode(&user); err != nil {
		t.Fatalf("error decoding response: %v", err)
	}
	if user.Username != "updated" {
		t.Errorf("expected username 'updated'; got '%s'", user.Username)
	}
	if !user.IsAdmin {
		t.Errorf("expected is_admin true; got false")
	}
}

func TestDeleteUser(t *testing.T) {
	truncate(t)
	user := createTestUser(t, "todelete", false)

	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/users/%d", user.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204; got %d", w.Result().StatusCode)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/users/%d", user.ID), nil)
	srv.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete; got %d", w.Result().StatusCode)
	}
}
