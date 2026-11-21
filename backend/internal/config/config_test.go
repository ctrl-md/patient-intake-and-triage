package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const goodSecret = "0123456789abcdef0123456789abcdef"

func TestLoadNeedsJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "too-short")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error for a short JWT_SECRET")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("JWT_SECRET", goodSecret)
	t.Setenv("JWT_TTL", "")
	t.Setenv("PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8080" || cfg.JWTTTL != 12*time.Hour {
		t.Fatalf("bad defaults: %+v", cfg)
	}
}

func TestLoadBadTTL(t *testing.T) {
	t.Setenv("JWT_SECRET", goodSecret)
	t.Setenv("JWT_TTL", "soon")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error for a bad JWT_TTL")
	}
}

func TestLoadDotEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "\ufeff# comment\n" +
		"DOTENV_A=from-file\r\n" +
		"DOTENV_B=\"quoted value\"\n" +
		"export DOTENV_C=exported\n" +
		"DOTENV_KEEP=file-value\n" +
		"\n" +
		"no equals sign here\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, k := range []string{"DOTENV_A", "DOTENV_B", "DOTENV_C"} {
		t.Setenv(k, "") // register for cleanup
		os.Unsetenv(k)
	}
	t.Setenv("DOTENV_KEEP", "real-env-value")

	if err := loadDotEnv(path); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"DOTENV_A":    "from-file",
		"DOTENV_B":    "quoted value",
		"DOTENV_C":    "exported",
		"DOTENV_KEEP": "real-env-value", // real environment wins
	}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestLoadDotEnvMissingFileIsFine(t *testing.T) {
	if err := loadDotEnv(filepath.Join(t.TempDir(), "nope.env")); err != nil {
		t.Fatal(err)
	}
}
