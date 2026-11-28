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

	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/assistant"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/auth"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/config"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/db"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/httpapi"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/intake"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/llm"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/triage"
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

	// AI is optional. With no key, rules and fixed questions still work.
	var aiClient llm.Client
	if cfg.LLMAPIKey != "" {
		aiClient, err = llm.New(llm.Config{
			Provider: cfg.LLMProvider, APIKey: cfg.LLMAPIKey, Model: cfg.LLMModel, Timeout: cfg.LLMTimeout,
		})
		if err != nil {
			log.Fatalf("llm: %v", err)
		}
		log.Printf("AI on: provider=%s model=%q", cfg.LLMProvider, cfg.LLMModel)
	} else {
		log.Println("AI off (no LLM_API_KEY): using rules and fixed questions only")
	}

	intakeStore := intake.NewPGStore(pool)
	triageSvc := triage.NewService(triage.NewPGStore(pool), aiClient)
	chatAssistant := assistant.New(aiClient)

	intakeHandler := intake.NewHandler(intakeStore, cfg.FrontendURL, cfg.IntakeTTL).WithHooks(intake.Hooks{
		Emergency:      triage.IsEmergency,
		EmergencyReply: assistant.EmergencyReply,
		Reply:          chatAssistant.Reply,
		Triage: func(ctx context.Context, id string, msgs []intake.Message) {
			if _, err := triageSvc.Run(ctx, id, msgs); err != nil {
				log.Printf("triage: intake %s: %v", id, err)
			}
		},
	})
	triageHandler := triage.NewHandler(triageSvc, intakeStore)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.NewRouter(httpapi.Deps{DB: pool, Auth: authHandler, Tokens: tokens, Intake: intakeHandler, Triage: triageHandler, CORSOrigins: cfg.CORSOrigins}),
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
