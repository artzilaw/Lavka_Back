package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMustLoadConfig_Success(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	content := `{
		"env": "local",
		"storage_path": "./storage/test.json",
		"token_ttl": "12h",
		"http": {"port": 8080},
		"cleanup_interval": "10m"
	}`

	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	t.Setenv("CONFIG_PATH", cfgPath)
	t.Setenv("TOKEN_SECRET", "test-secret")

	cfg := MustLoadConfig()

	if cfg.Env != "local" {
		t.Errorf("got Env %q, want local", cfg.Env)
	}
	if cfg.HTTP.Port != 8080 {
		t.Errorf("got Port %d, want 8080", cfg.HTTP.Port)
	}
	if cfg.TokenTTL != 12*time.Hour {
		t.Errorf("got TokenTTL %v, want 12h", cfg.TokenTTL)
	}
	if cfg.CleanupInterval != 10*time.Minute {
		t.Errorf("got CleanupInterval %v, want 10m", cfg.CleanupInterval)
	}
	if cfg.TokenSecret != "test-secret" {
		t.Errorf("got TokenSecret %q, want test-secret", cfg.TokenSecret)
	}
}

func TestMustLoadByPath_Panics(t *testing.T) {
	dir := t.TempDir()

	validJSON := `{"env":"local","storage_path":"","token_ttl":"1h","http":{"port":8000},"cleanup_interval":"1m"}`
	invalidDurationJSON := `{"env":"local","storage_path":"","token_ttl":"not-a-duration","http":{"port":8000},"cleanup_interval":"1m"}`
	brokenJSON := `{broken`

	validFile := filepath.Join(dir, "valid.json")
	invalidDurFile := filepath.Join(dir, "invalid_dur.json")
	brokenFile := filepath.Join(dir, "broken.json")

	_ = os.WriteFile(validFile, []byte(validJSON), 0o600)
	_ = os.WriteFile(invalidDurFile, []byte(invalidDurationJSON), 0o600)
	_ = os.WriteFile(brokenFile, []byte(brokenJSON), 0o600)

	tests := []struct {
		name   string
		path   string
		secret string
	}{
		{name: "non-existent file", path: filepath.Join(dir, "nope.json"), secret: "sec"},
		{name: "broken json", path: brokenFile, secret: "sec"},
		{name: "missing secret", path: validFile, secret: ""},
		{name: "invalid duration", path: invalidDurFile, secret: "sec"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TOKEN_SECRET", tt.secret)
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic for case %q, but did not panic", tt.name)
				}
			}()
			_ = MustLoadByPath(tt.path)
		})
	}
}
