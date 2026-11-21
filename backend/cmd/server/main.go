package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/auth"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/config"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/db"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/httpapi"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL)
	authHandler, err := auth.NewHandler(auth.NewPGStore(pool), auth.BcryptHasher{}, tokens)
	if err != nil {
		log.Fatalf("auth: %v", err)
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.NewRouter(httpapi.Deps{DB: pool, Auth: authHandler, Tokens: tokens}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("triagely api listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
