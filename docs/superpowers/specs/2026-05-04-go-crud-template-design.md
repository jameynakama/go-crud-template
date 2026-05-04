# Go CRUD Template -- Design Spec

**Date:** 2026-05-04
**Goal:** A minimal, opinionated Go backend skeleton for personal projects. Extracting the structural patterns from `lifer` into a reusable starting point. No auth. One sample resource (`MyModel`) with full CRUD. The module name contains a placeholder (`APPNAME`) so new projects just find-replace it.

---

## Stack

| Tool | Purpose |
|---|---|
| Go (1.22+) | Language |
| [chi](https://github.com/go-chi/chi) | HTTP router |
| [pgx/v5](https://github.com/jackc/pgx) | Postgres driver + connection pool |
| [sqlc](https://sqlc.dev) | Generates type-safe Go from SQL queries |
| [golang-migrate](https://github.com/golang-migrate/migrate) | SQL migrations |
| docker-compose | Local Postgres (no host install required) |
| just | Task runner |

---

## Directory Structure

```
APPNAME/
├── cmd/
│   └── server/
│       └── main.go              # entry point: config, DB, router, http.ListenAndServe
├── internal/
│   ├── api/
│   │   ├── router.go            # chi setup, middleware, route registration
│   │   └── handlers.go          # handler methods, writeJSON, writeError
│   └── store/
│       ├── queries/
│       │   └── mymodels.sql     # hand-written SQL queries for sqlc
│       ├── db.go                # sqlc-generated
│       ├── models.go            # sqlc-generated
│       ├── querier.go           # sqlc-generated interface
│       └── mymodels.sql.go      # sqlc-generated
├── migrations/
│   ├── 001_initial.up.sql
│   └── 001_initial.down.sql
├── sqlc.yaml
├── go.mod                       # module: github.com/jameynakama/APPNAME
├── Justfile
├── docker-compose.yml
├── .env                         # gitignored
├── .env.example
├── .gitignore
└── README.md
```

---

## MyModel

### Schema (`migrations/001_initial.up.sql`)

```sql
CREATE OR REPLACE FUNCTION set_update_time()
RETURNS TRIGGER AS $$
BEGIN
    NEW.update_time = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE my_models (
    id          BIGSERIAL    PRIMARY KEY,
    name        TEXT         NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    create_time TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    update_time TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TRIGGER set_update_time_before_update
    BEFORE UPDATE ON my_models
    FOR EACH ROW
    EXECUTE FUNCTION set_update_time();
```

Field naming follows [Google AIP-0142](https://aip.dev/142): `create_time`, `update_time`.
`update_time` is maintained automatically by the Postgres trigger -- no application-level stamp needed.

### SQL Queries (`internal/store/queries/mymodels.sql`)

```sql
-- name: ListMyModels :many
-- name: GetMyModel :one
-- name: CreateMyModel :one
-- name: UpdateMyModel :one
-- name: DeleteMyModel :exec
```

Full pagination via `LIMIT` / `OFFSET` on the list query.

---

## API Routes

```
GET    /health                            → 200 {"status":"ok"}
GET    /api/v1/my-models?limit=N&offset=N → paginated list
GET    /api/v1/my-models/{id}             → single record or 404
POST   /api/v1/my-models                  → create, 201 + created record
PUT    /api/v1/my-models/{id}             → full update, 200 + updated record
DELETE /api/v1/my-models/{id}             → 204 No Content
```

---

## Middleware

Chi built-ins only -- no custom middleware needed for this skeleton:

- `middleware.Logger` -- request/response logging
- `middleware.Recoverer` -- catches panics, returns 500
- `middleware.RequestID` -- attaches a unique ID to each request context

---

## Response Helpers

Two functions in `handlers.go`:

```go
func writeJSON(w http.ResponseWriter, status int, v any)
func writeError(w http.ResponseWriter, status int, msg string)
```

`writeError` writes `{"error": "<msg>"}` -- consistent shape for all error responses.

---

## Tests

File: `internal/api/handlers_test.go`

Integration tests -- real test DB, no mocks. Each test function uses `net/http/httptest` to fire real HTTP requests through the full router stack.

Stubs (exist and compile, minimal assertions to start):

- `TestListMyModels`
- `TestGetMyModel`
- `TestCreateMyModel`
- `TestUpdateMyModel`
- `TestDeleteMyModel`

Test DB setup: read `TEST_DATABASE_URL` from env (falls back to `DATABASE_URL`), connect a pool, run migrations in `TestMain`, truncate `my_models` in each test's cleanup.

---

## Config

Loaded from environment in `main.go`:

```
DATABASE_URL   required
PORT           optional, default 8080
```

---

## Justfile Targets

```
just            → run tests (default)
just run        → start server
just build      → build binary to bin/APPNAME
just migrate-up
just migrate-down
just generate   → re-run sqlc
just migration name=X  → create new migration pair
```

---

## README

Covers:
1. Prerequisites (Go, just, sqlc, golang-migrate, Docker)
2. First-run steps (clone, find-replace `APPNAME`, copy `.env.example`, `docker-compose up -d`, `just migrate-up`, `just run`)
3. Commands reference
4. How to add a new resource (the workflow: migration → SQL queries → `just generate` → handler)
