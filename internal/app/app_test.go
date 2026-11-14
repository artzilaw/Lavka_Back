package app

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"lavka/internal/config"
)

func TestApp_NewAndStop(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{
		Env:             "local",
		StoragePath:     "",
		TokenTTL:        time.Hour,
		HTTP:            config.HTTPConfig{Port: 0},
		CleanupInterval: time.Minute,
		TokenSecret:     "secret",
	}

	a := New(log, cfg)
	if a == nil || a.HTTPServer == nil || a.Blacklist == nil {
		t.Fatal("expected initialized App")
	}
	a.Stop()
}
