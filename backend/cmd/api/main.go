// Command api is the entry point of the workshop portal HTTP API.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"workshop/api/internal/auth"
	"workshop/api/internal/config"
	"workshop/api/internal/httpapi"
	"workshop/api/internal/queue"
	"workshop/api/internal/store"
)

// migrationFile is applied on every start. It is idempotent, so a restart does
// not disturb existing data.
const migrationFile = "migrations/0001_init.sql"

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	if err := run(); err != nil {
		log.Printf("api: fatal: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := st.Ping(pingCtx); err != nil {
		return fmt.Errorf("DATABASE_URL is not reachable: %w", err)
	}
	if err := applyMigrations(ctx, st); err != nil {
		return err
	}
	if err := auth.SeedFirstEmployee(ctx, st); err != nil {
		return fmt.Errorf("seed first employee: %w", err)
	}

	q, err := queue.New(cfg.ValkeyURL)
	if err != nil {
		return fmt.Errorf("VALKEY_URL is invalid: %w", err)
	}
	defer q.Close()

	handler := httpapi.Router(cfg, st, q)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("api: listening on %s", server.Addr)
	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Printf("api: shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

// applyMigrations executes the schema file. Reading the path here keeps the
// service independent of any external migration tool.
func applyMigrations(ctx context.Context, st *store.Store) error {
	sqlBytes, err := os.ReadFile(filepath.Clean(migrationFile))
	if err != nil {
		return fmt.Errorf("read migration %s: %w", migrationFile, err)
	}
	execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := st.Exec(execCtx, string(sqlBytes)); err != nil {
		return fmt.Errorf("apply migration %s: %w", migrationFile, err)
	}
	return nil
}
