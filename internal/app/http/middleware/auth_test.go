package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lavka/internal/domain/models"
	"lavka/internal/pkg/jwt"
	"lavka/internal/storage/blacklist"
)

func TestAuthMiddleware(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	bl := blacklist.New(time.Minute)
	defer bl.Stop()
	secret := "test-secret"

	mw := AuthMiddleware(log, bl, secret)
	nextHandler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := r.Context().Value(UserIDKey).(int64)
		if uid != 7 {
			t.Errorf("expected uid 7 in context, got %d", uid)
		}
		w.WriteHeader(http.StatusOK)
	}))

	reqNoCookie := httptest.NewRequest(http.MethodPost, "/logout", nil)
	recNoCookie := httptest.NewRecorder()
	nextHandler.ServeHTTP(recNoCookie, reqNoCookie)
	if recNoCookie.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without cookie, got %d", recNoCookie.Code)
	}

	tok, _ := jwt.NewToken(models.User{ID: 7}, time.Hour, secret)
	reqValid := httptest.NewRequest(http.MethodPost, "/logout", nil)
	reqValid.AddCookie(&http.Cookie{Name: "token", Value: tok})
	recValid := httptest.NewRecorder()
	nextHandler.ServeHTTP(recValid, reqValid)
	if recValid.Code != http.StatusOK {
		t.Errorf("expected 200 with valid cookie, got %d", recValid.Code)
	}
}
