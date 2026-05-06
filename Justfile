set dotenv-load := true

alias mu := migrate-up
alias md := migrate-down

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
    rm -f internal/store/*.sql.go
    sqlc generate
