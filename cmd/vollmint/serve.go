package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/vollminlab/vollmint/internal/api"
	"github.com/vollminlab/vollmint/internal/migrate"
	"github.com/vollminlab/vollmint/internal/store"
)

func runServe(args []string) error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	ctx := context.Background()
	if err := migrate.Up(dbURL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	s, err := store.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer s.Close()

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.New(s).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Printf("vollmint serve listening on %s\n", addr)
	return srv.ListenAndServe()
}
