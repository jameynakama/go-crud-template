# Go CRUD Template Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a minimal, reusable Go backend skeleton with full CRUD for a placeholder `MyModel`, Postgres via docker-compose, sqlc-generated store layer, chi router, and integration tests.

**Architecture:** Request enters chi router → middleware chain (logger, recoverer, requestID) → handler method on `Handler` struct → sqlc `Queries` → Postgres. No auth layer. All config from env vars loaded at startup.

**Tech Stack:** Go 1.22+, chi v5, pgx/v5, sqlc, golang-migrate, docker-compose, just

> **Note for guided sessions:** Jamey writes all the code. Each task has a brief "why" at the top -- read it before writing. Verify after each task before moving on.

---

## File Map

| File | Responsibility |
|---|---|
| `cmd/server/main.go` | Entry point: load config, connect DB, wire router, start server |
| `internal/api/router.go` | Handler struct, chi setup, middleware, route registration, `writeJSON`, `writeError` |
| `internal/api/handlers.go` | One method per endpoint (list, get, create, update, delete) |
| `internal/api/handlers_test.go` | Integration test stubs, `TestMain`, DB pool setup |
| `internal/store/queries/mymodels.sql` | Hand-written SQL; sqlc reads this |
| `internal/store/*.go` | sqlc-generated -- do not edit by hand |
| `migrations/001_initial.up.sql` | Schema + `update_time` trigger |
| `migrations/001_initial.down.sql` | Rollback |
| `sqlc.yaml` | Tells sqlc where to find queries/schema and where to emit Go |
| `docker-compose.yml` | Local Postgres service |
| `Justfile` | All dev commands |
| `.env.example` | Documents required env vars |
| `.gitignore` | Keeps `.env` and binaries out of git |
| `README.md` | Setup and usage |

---

## Task 1: Project Scaffold

**Why:** Go requires a module declaration before any code. The directory structure follows Go conventions: `cmd/` for executables, `internal/` for packages that shouldn't be imported by outside projects.

**Files:** `go.mod`, `.gitignore`, directory skeleton

- [ ] **Step 1: Create the directory tree**

```bash
mkdir -p cmd/server internal/api internal/store/queries migrations
```

- [ ] **Step 2: Initialize the Go module**

```bash
go mod init github.com/jameynakama/APPNAME
```

This creates `go.mod`. Every import in the project will be relative to this module path. When you copy the template, you'll find-replace `APPNAME`.

- [ ] **Step 3: Create `.gitignore`**

```
.env
bin/
```

- [ ] **Step 4: Verify**

```bash
cat go.mod
```

Expected output:
```
module github.com/jameynakama/APPNAME

go 1.XX.X
```

- [ ] **Step 5: Commit**

```bash
jj describe -m "Scaffold project structure and go module" && jj new
```

---

## Task 2: Infrastructure Files

**Why:** Docker-compose gives you a one-command local Postgres with no host install fuss. The `.env.example` documents every required variable -- future-you will thank present-you.

**Files:** `docker-compose.yml`, `.env.example`, `.env`

- [ ] **Step 1: Create `docker-compose.yml`**

```yaml
services:
  db:
    image: postgres:17
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: appname_dev
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

- [ ] **Step 2: Create `.env.example`**

```
DATABASE_URL=postgres://postgres:postgres@localhost:5432/appname_dev?sslmode=disable
PORT=8080
```

- [ ] **Step 3: Create `.env`** (copy from example, this is gitignored)

```bash
cp .env.example .env
```

- [ ] **Step 4: Start Postgres**

```bash
docker compose up -d
```

- [ ] **Step 5: Verify Postgres is accepting connections**

```bash
docker compose ps
```

Expected: `db` service shows `running`.

- [ ] **Step 6: Commit**

```bash
jj describe -m "Add docker-compose and env files" && jj new
```

---

## Task 3: Justfile

**Why:** `just` is a task runner -- like `make` but without the footguns. `set dotenv-load` means just reads your `.env` automatically, so every recipe has `DATABASE_URL` in scope without you having to export anything.

**Files:** `Justfile`

- [ ] **Step 1: Create `Justfile`**

```justfile
set dotenv-load

default: test

# Run all tests
test:
    go test ./...

# Start the dev server
run:
    go run ./cmd/server

# Build binary
build:
    go build -o bin/APPNAME ./cmd/server

# Run pending migrations
migrate-up:
    migrate -path migrations -database "$DATABASE_URL" up

# Roll back one migration
migrate-down:
    migrate -path migrations -database "$DATABASE_URL" down 1

# Regenerate sqlc types after query changes
generate:
    sqlc generate

# Create a new migration pair (usage: just migration name=add_something)
migration name:
    migrate create -ext sql -dir migrations -seq {{ name }}
```

- [ ] **Step 2: Verify just reads the file**

```bash
just --list
```

Expected: lists all recipes.

- [ ] **Step 3: Commit**

```bash
jj describe -m "Add Justfile" && jj new
```

---

## Task 4: sqlc Configuration

**Why:** sqlc reads your SQL files and emits type-safe Go. You tell it where your queries live, where your schema lives (the migrations directory), and where to write the output. `emit_interface: true` generates a `Querier` interface alongside the concrete `Queries` struct -- useful for testing.

**Files:** `sqlc.yaml`

- [ ] **Step 1: Create `sqlc.yaml`**

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/store/queries"
    schema: "migrations"
    gen:
      go:
        package: "store"
        out: "internal/store"
        emit_json_tags: true
        emit_db_tags: true
        emit_prepared_queries: false
        emit_interface: true
        emit_exact_table_names: false
        sql_package: "pgx/v5"
```

- [ ] **Step 2: Commit**

```bash
jj describe -m "Add sqlc config" && jj new
```

---

## Task 5: Database Migrations

**Why:** Migrations are versioned, reversible schema changes. golang-migrate applies `.up.sql` files in sequence and tracks which ones have run. The `update_time` trigger lives here -- Postgres maintains it automatically on every UPDATE, so handlers never have to set it manually.

**Files:** `migrations/001_initial.up.sql`, `migrations/001_initial.down.sql`

- [ ] **Step 1: Create `migrations/001_initial.up.sql`**

```sql
CREATE OR REPLACE FUNCTION set_update_time()
RETURNS TRIGGER AS $$
BEGIN
    NEW.update_time = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE my_models (
    id          BIGSERIAL   PRIMARY KEY,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    create_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    update_time TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER set_update_time_before_update
    BEFORE UPDATE ON my_models
    FOR EACH ROW
    EXECUTE FUNCTION set_update_time();
```

- [ ] **Step 2: Create `migrations/001_initial.down.sql`**

```sql
DROP TABLE IF EXISTS my_models;
DROP FUNCTION IF EXISTS set_update_time;
```

- [ ] **Step 3: Run the migration**

```bash
just migrate-up
```

Expected output ends with: `1/u 001_initial (Xms)`

- [ ] **Step 4: Verify the table exists**

```bash
docker exec -it go-crud-template-db-1 psql -U postgres -d appname_dev -c "\dt"
```

Expected: `my_models` appears in the table list.

- [ ] **Step 5: Commit**

```bash
jj describe -m "Add initial migration for my_models" && jj new
```

---

## Task 6: SQL Queries + Generate

**Why:** sqlc reads these SQL files and generates Go functions. The `-- name:` comment tells sqlc what to name the function and how many rows it returns (`:one`, `:many`, `:exec`). After running `just generate`, you'll have a full `store` package with typed structs and methods -- no SQL strings in your Go code.

**Files:** `internal/store/queries/mymodels.sql`, then `internal/store/*.go` (generated)

- [ ] **Step 1: Create `internal/store/queries/mymodels.sql`**

```sql
-- name: ListMyModels :many
SELECT * FROM my_models
ORDER BY create_time DESC
LIMIT $1 OFFSET $2;

-- name: GetMyModel :one
SELECT * FROM my_models WHERE id = $1;

-- name: CreateMyModel :one
INSERT INTO my_models (name, description)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateMyModel :one
UPDATE my_models
SET name = $2, description = $3
WHERE id = $1
RETURNING *;

-- name: DeleteMyModel :exec
DELETE FROM my_models WHERE id = $1;
```

- [ ] **Step 2: Run sqlc**

```bash
just generate
```

Expected: no output (silent success). Four files appear in `internal/store/`: `db.go`, `models.go`, `querier.go`, `mymodels.sql.go`.

- [ ] **Step 3: Inspect what was generated**

```bash
cat internal/store/models.go
```

You'll see a `MyModel` struct with fields matching the table. Note the field names -- sqlc converts `snake_case` columns to `CamelCase` Go fields: `create_time` → `CreateTime`, etc.

```bash
cat internal/store/querier.go
```

You'll see the `Querier` interface with one method per query.

- [ ] **Step 4: Commit**

```bash
jj describe -m "Add SQL queries and generate store package" && jj new
```

---

## Task 7: Router

**Why:** chi is a lightweight router. The `Handler` struct holds your dependencies (the DB query layer). Passing them in via a config struct -- rather than using globals -- makes the code testable: in tests you'll create a `Handler` with a test DB pool, not the real one.

`writeJSON` and `writeError` are package-level helpers in the `api` package, usable by every handler.

**Files:** `internal/api/router.go`

- [ ] **Step 1: Create `internal/api/router.go`**

```go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jameynakama/APPNAME/internal/store"
)

type RouterConfig struct {
	Queries *store.Queries
}

type Handler struct {
	queries *store.Queries
}

func NewRouter(cfg RouterConfig) http.Handler {
	h := &Handler{
		queries: cfg.Queries,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Get("/health", h.healthCheck)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/my-models", h.listMyModels)
		r.Post("/my-models", h.createMyModel)
		r.Get("/my-models/{id}", h.getMyModel)
		r.Put("/my-models/{id}", h.updateMyModel)
		r.Delete("/my-models/{id}", h.deleteMyModel)
	})

	return r
}

func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
```

- [ ] **Step 2: Commit**

```bash
jj describe -m "Add chi router with health endpoint and route stubs" && jj new
```

---

## Task 8: Handlers

**Why:** Each handler follows the same pattern: parse input → call the store → write response. `chi.URLParam` extracts path parameters like `{id}`. Request bodies are decoded with `json.NewDecoder`. All errors go through `writeError` for a consistent JSON shape.

Note on DELETE: sqlc's `:exec` queries don't tell you how many rows were affected, so `DELETE /my-models/999` returns 204 even if the row doesn't exist. That's intentional -- idempotent deletes are a valid REST pattern.

**Files:** `internal/api/handlers.go`

- [ ] **Step 1: Create `internal/api/handlers.go`**

```go
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jameynakama/APPNAME/internal/store"
)

func (h *Handler) listMyModels(w http.ResponseWriter, r *http.Request) {
	limit := int32(20)
	offset := int32(0)

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = int32(v)
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = int32(v)
		}
	}

	rows, err := h.queries.ListMyModels(r.Context(), store.ListMyModelsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		log.Printf("listMyModels: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *Handler) getMyModel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	row, err := h.queries.GetMyModel(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

type createMyModelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *Handler) createMyModel(w http.ResponseWriter, r *http.Request) {
	var req createMyModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	row, err := h.queries.CreateMyModel(r.Context(), store.CreateMyModelParams{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		log.Printf("createMyModel: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

type updateMyModelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *Handler) updateMyModel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req updateMyModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	row, err := h.queries.UpdateMyModel(r.Context(), store.UpdateMyModelParams{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *Handler) deleteMyModel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.queries.DeleteMyModel(r.Context(), id); err != nil {
		log.Printf("deleteMyModel: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Commit**

```bash
jj describe -m "Add CRUD handlers for MyModel" && jj new
```

---

## Task 9: Entry Point

**Why:** `main.go` is the wiring layer -- it reads config, opens the DB pool, builds the router with dependencies injected, and starts the HTTP server. It doesn't contain business logic; it just assembles the pieces. `pgxpool` maintains a pool of connections so concurrent requests don't block on each other.

**Files:** `cmd/server/main.go`

- [ ] **Step 1: Create `cmd/server/main.go`**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jameynakama/APPNAME/internal/api"
	"github.com/jameynakama/APPNAME/internal/store"
)

type config struct {
	databaseURL string
	port        string
}

func loadConfig() config {
	required := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			log.Fatalf("%s is required", key)
		}
		return v
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return config{
		databaseURL: required("DATABASE_URL"),
		port:        port,
	}
}

func main() {
	cfg := loadConfig()

	pool, err := pgxpool.New(context.Background(), cfg.databaseURL)
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}
	log.Println("database connected")

	queries := store.New(pool)

	router := api.NewRouter(api.RouterConfig{
		Queries: queries,
	})

	addr := fmt.Sprintf(":%s", cfg.port)
	log.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
```

- [ ] **Step 2: Fetch dependencies**

```bash
go get github.com/go-chi/chi/v5
go get github.com/jackc/pgx/v5
go mod tidy
```

- [ ] **Step 3: Verify the project compiles**

```bash
go build ./...
```

Expected: no output (silent success).

- [ ] **Step 4: Start the server and smoke-test the health endpoint**

```bash
just run &
curl http://localhost:8080/health
```

Expected: `{"status":"ok"}`

Kill the server with `fg` then `Ctrl+C`.

- [ ] **Step 5: Commit**

```bash
jj describe -m "Add entry point, wire dependencies, verify build" && jj new
```

---

## Task 10: Integration Tests

**Why:** `net/http/httptest` lets you fire real HTTP requests through your actual router -- no mocking, no fakes -- against a real test DB. `TestMain` runs once before all tests: it sets up the DB pool and calls `m.Run()` to execute the test suite. Each test calls `truncate()` first to start with a clean slate.

`package api_test` (note the `_test` suffix) is an external test package -- it can only use exported identifiers from `api`. This is a good habit: it tests the package from the outside, the same way any consumer would.

**Files:** `internal/api/handlers_test.go`

- [ ] **Step 1: Create `internal/api/handlers_test.go`**

```go
package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jameynakama/APPNAME/internal/api"
	"github.com/jameynakama/APPNAME/internal/store"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		panic("TEST_DATABASE_URL or DATABASE_URL must be set")
	}

	var err error
	testPool, err = pgxpool.New(context.Background(), dbURL)
	if err != nil {
		panic(err)
	}
	defer testPool.Close()

	os.Exit(m.Run())
}

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	return api.NewRouter(api.RouterConfig{Queries: store.New(testPool)})
}

func truncate(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "TRUNCATE my_models RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func TestListMyModels(t *testing.T) {
	truncate(t)
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/my-models", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetMyModel(t *testing.T) {
	truncate(t)
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/my-models/999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestCreateMyModel(t *testing.T) {
	truncate(t)
	srv := newTestServer(t)

	body := strings.NewReader(`{"name":"sparrow","description":"a small bird"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/my-models", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateMyModel(t *testing.T) {
	truncate(t)
	srv := newTestServer(t)

	// nonexistent ID returns 404
	body := strings.NewReader(`{"name":"updated"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/my-models/999", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteMyModel(t *testing.T) {
	truncate(t)
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/my-models/999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// DELETE is idempotent: 204 even for nonexistent IDs
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run the tests**

```bash
just test
```

Expected: all 5 tests pass.

- [ ] **Step 3: Commit**

```bash
jj describe -m "Add integration test stubs for CRUD endpoints" && jj new
```

---

## Task 11: README

**Why:** The README is the entry point for future-you. The find-replace step is the only annoying part of using this template -- making it explicit saves confusion.

**Files:** `README.md`

- [ ] **Step 1: Write `README.md`**

```markdown
# APPNAME

> Replace all occurrences of `APPNAME` and `go-crud-template` with your project name before first run.

A Go backend with full CRUD for a placeholder resource (`MyModel`), ready to clone and adapt.

## Stack

- **Go** + **chi** -- HTTP server and router
- **pgx/v5** -- Postgres driver
- **sqlc** -- type-safe SQL → Go codegen
- **golang-migrate** -- versioned migrations
- **docker-compose** -- local Postgres

## Prerequisites

```bash
brew install go just sqlc golang-migrate
```

Docker Desktop (or equivalent) for Postgres.

## First Run

```bash
git clone https://github.com/jameynakama/go-crud-template
cd go-crud-template

# 1. Replace the placeholder module name throughout
grep -rl APPNAME . --include="*.go" --include="*.mod" | xargs sed -i '' 's/APPNAME/yourappname/g'

# 2. Set up environment
cp .env.example .env   # edit DATABASE_URL if needed

# 3. Start Postgres
docker compose up -d

# 4. Run migrations
just migrate-up

# 5. Start the server
just run
```

Server starts on `http://localhost:8080`.

## Commands

| Command | Description |
|---|---|
| `just` | Run tests (default) |
| `just run` | Start dev server |
| `just build` | Build binary to `bin/APPNAME` |
| `just migrate-up` | Apply pending migrations |
| `just migrate-down` | Roll back one migration |
| `just generate` | Regenerate sqlc types after SQL changes |
| `just migration name=X` | Create a new migration pair |

## Adding a New Resource

1. `just migration name=add_things` -- creates `migrations/NNN_add_things.{up,down}.sql`
2. Write the schema in the `.up.sql` file (add the `update_time` trigger if needed)
3. `just migrate-up`
4. Create `internal/store/queries/things.sql` with your queries
5. `just generate` -- sqlc emits the Go types and methods
6. Add handler methods in `internal/api/handlers.go`
7. Register routes in `internal/api/router.go`
8. Write tests in `internal/api/handlers_test.go`

## API

```
GET    /health
GET    /api/v1/my-models?limit=20&offset=0
GET    /api/v1/my-models/{id}
POST   /api/v1/my-models
PUT    /api/v1/my-models/{id}
DELETE /api/v1/my-models/{id}
```
```

- [ ] **Step 2: Commit**

```bash
jj describe -m "Add README" && jj new
```

---

## Task 12: Push

- [ ] **Step 1: Push to GitHub**

```bash
jj git push
```

---

## Self-Review Checklist

- [x] **Spec coverage:** all routes ✓, MyModel fields ✓, trigger ✓, pagination ✓, middleware ✓, writeJSON/writeError ✓, all 5 test stubs ✓, README sections ✓, Justfile targets ✓
- [x] **Placeholder scan:** no TBDs or TODOs in code steps
- [x] **Type consistency:** `store.ListMyModelsParams`, `store.CreateMyModelParams`, `store.UpdateMyModelParams` used consistently across Tasks 6 and 8; `Handler.queries` type is `*store.Queries` throughout
- [x] **Note on spec:** `writeJSON`/`writeError` are placed in `router.go` (not `handlers.go` as the spec states) -- both files are in the same `api` package so this is correct; the spec description was slightly off
