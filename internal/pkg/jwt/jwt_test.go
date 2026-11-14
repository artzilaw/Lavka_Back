package jwt

import (
	"testing"
	"time"

	"lavka/internal/domain/models"
)

func TestNewAndParseToken(t *testing.T) {
	secret := "test-secret"
	u := models.User{ID: 42, Email: "test@lavka.ru"}

	tokenStr, err := NewToken(u, time.Hour, secret)
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}

	tok, err := ParseToken(tokenStr, secret)
	if err != nil || !tok.Valid {
		t.Fatalf("expected valid token, got err: %v", err)
	}

	if _, err := ParseToken(tokenStr, "wrong-secret"); err == nil {
		t.Error("expected error when parsing with wrong secret")
	}
}
