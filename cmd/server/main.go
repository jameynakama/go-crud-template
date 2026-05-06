package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/GHUSER/APPNAME/internal/api"
	"github.com/GHUSER/APPNAME/internal/store"
)

type config struct {
	databaseURL string
	port        string
}

func loadConfig() config {
	required := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			log.Fatalf("must set %s env var", key)
		}
		return v
	}

	withDefault := func(key string, defaultV string) string {
		v := os.Getenv(key)
		if v == "" {
			return defaultV
		}
		return v
	}

	return config{
		databaseURL: required("DATABASE_URL"),
		port:        withDefault("PORT", "8080"),
	}
}

func main() {
	cfg := loadConfig()

	ctx := context.Background()

	db, err := pgxpool.New(ctx, cfg.databaseURL)
	if err != nil {
		log.Fatal("error establishing database connection")
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatal("cannot ping database")
	}
	log.Println("database connected")

	routerCfg := api.RouterConfig{Queries: store.New(db)}
	r := api.NewRouter(routerCfg)

	log.Printf("starting server at localhost:%s", cfg.port)
	if err := http.ListenAndServe(":"+cfg.port, r); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
