package auth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"lavka/internal/storage"
	"lavka/internal/storage/blacklist"
)

func TestAuth_RegisterLoginValidateLogoutProfile(t *testing.T) {
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

	// GetProfile & UpdateProfile
	prof, err := svc.GetProfile(ctx, uid)
	if err != nil || prof.Name != "Alex" {
		t.Fatalf("GetProfile failed: %v", err)
	}

	updatedProf, err := svc.UpdateProfile(ctx, uid, "Alexander", "alex.new@lavka.ru")
	if err != nil || updatedProf.Name != "Alexander" || updatedProf.Email != "alex.new@lavka.ru" {
		t.Fatalf("UpdateProfile failed: %v", err)
	}

	// ChangePassword
	if err := svc.ChangePassword(ctx, uid, "WrongOld1", "NewPass123"); !errors.Is(err, ErrInvalidCurrentPassword) {
		t.Errorf("expected ErrInvalidCurrentPassword, got %v", err)
	}
	if err := svc.ChangePassword(ctx, uid, "Pass1234", "Pass1234"); !errors.Is(err, ErrSamePassword) {
		t.Errorf("expected ErrSamePassword, got %v", err)
	}
	if err := svc.ChangePassword(ctx, uid, "Pass1234", "NewPass123"); err != nil {
		t.Fatalf("ChangePassword failed: %v", err)
	}

	// Вход с новым email и новым паролем
	if _, _, err := svc.Login(ctx, "alex.new@lavka.ru", "NewPass123"); err != nil {
		t.Fatalf("Login with new password failed: %v", err)
	}

	// UpdateAvatar (валидный PNG-заголовок)
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	userWithAvatar, err := svc.UpdateAvatar(ctx, uid, bytes.NewReader(pngHeader), "avatar.png")
	if err != nil || userWithAvatar.AvatarPath == "" {
		t.Fatalf("UpdateAvatar failed: %v", err)
	}
	_ = os.RemoveAll("static")

	if err := svc.Logout(ctx, "some-jti", time.Now().Add(time.Hour)); err != nil {
		t.Errorf("Logout failed: %v", err)
	}
	if !bl.Check("some-jti") {
		t.Error("expected jti to be in blacklist after Logout")
	}
}
