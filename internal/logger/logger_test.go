package logger

import (
	"context"
	"log/slog"
	"testing"
)

func TestSetupLogger(t *testing.T) {
	tests := []struct {
		name         string
		env          string
		debugEnabled bool
	}{
		{name: "local env enables debug", env: envLocal, debugEnabled: true},
		{name: "dev env enables debug", env: envDev, debugEnabled: true},
		{name: "prod env disables debug", env: envProd, debugEnabled: false},
		{name: "default env", env: "unknown", debugEnabled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := SetupLogger(tt.env)
			if log == nil {
				t.Fatal("expected non-nil logger")
			}

			got := log.Enabled(context.Background(), slog.LevelDebug)
			if got != tt.debugEnabled {
				t.Errorf("SetupLogger(%q) debug enabled = %v, want %v", tt.env, got, tt.debugEnabled)
			}
		})
	}
}
