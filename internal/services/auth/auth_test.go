package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"lavka/internal/storage"
	"lavka/internal/storage/blacklist"
)

func TestAuth_RegisterLoginValidateLogout(t *testing.T) {
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, _ := storage.New("")
	bl := blacklist.New(time.Minute)
	defer bl.Stop()

	svc := New(log, st, bl, time.Hour, "secret-key")

	uid, err := svc.RegisterNewUser(ctx, "user@lavka.ru", "Pass1234", "Alex")
	if err != nil || uid == 0 {
		t.Fatalf("RegisterNewUser failed: %v", err)
	}

	if _, err := svc.RegisterNewUser(ctx, "user@lavka.ru", "Pass1234", "Alex"); !errors.Is(err, storage.ErrUserExists) {
		t.Errorf("expected ErrUserExists, got %v", err)
	}

	tok, user, err := svc.Login(ctx, "user@lavka.ru", "Pass1234")
	if err != nil || tok == "" || user.ID != uid {
		t.Fatalf("Login failed: %v", err)
	}

	if _, _, err := svc.Login(ctx, "user@lavka.ru", "WrongPass1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}

	validatedUser, err := svc.ValidateTokenAndGetUser(ctx, tok)
	if err != nil || validatedUser.Email != "user@lavka.ru" {
		t.Errorf("ValidateTokenAndGetUser failed: %v", err)
	}

	if err := svc.Logout(ctx, "some-jti", time.Now().Add(time.Hour)); err != nil {
		t.Errorf("Logout failed: %v", err)
	}
	if !bl.Check("some-jti") {
		t.Error("expected jti to be in blacklist after Logout")
	}
}
