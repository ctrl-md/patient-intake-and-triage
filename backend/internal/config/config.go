package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	JWTTTL      time.Duration

	FrontendURL string        // where the patient chat page lives (used to build intake links)
	IntakeTTL   time.Duration // how long an intake link works
}

// Load reads settings from the environment. If a .env file exists in the
// current folder, it is read first. Real environment variables win over .env.
func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, err
	}

	ttl, err := time.ParseDuration(getenv("JWT_TTL", "12h"))
	if err != nil || ttl <= 0 {
		return Config{}, errors.New("JWT_TTL must be a duration like 12h or 30m")
	}

	intakeTTL, err := time.ParseDuration(getenv("INTAKE_TTL", "24h"))
	if err != nil || intakeTTL <= 0 {
		return Config{}, errors.New("INTAKE_TTL must be a duration like 24h or 90m")
	}

	cfg := Config{
		Port:        getenv("PORT", "8080"),
		DatabaseURL: getenv("DATABASE_URL", "postgres://triagely:triagely@localhost:5432/triagely?sslmode=disable"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		JWTTTL:      ttl,
		FrontendURL: strings.TrimRight(getenv("FRONTEND_URL", "http://localhost:3000"), "/"),
		IntakeTTL:   intakeTTL,
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("JWT_SECRET must be at least 32 characters (make one with: openssl rand -hex 32)")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv reads KEY=VALUE lines from a file into the environment.
// A missing file is fine. Variables that are already set are left alone.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			line = strings.TrimPrefix(line, "\ufeff") // Windows editors add this
			first = false
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)

		if _, set := os.LookupEnv(key); !set {
			os.Setenv(key, value)
		}
	}
	return sc.Err()
}
