package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lavka/internal/logger"
)

func TestAccessLogMiddleware_RequestIDAndSecurityHeaders(t *testing.T) {
	var logBuf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logBuf, nil))

	var ctxReqID string
	handler := AccessLogMiddleware(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxReqID = logger.RequestIDFromContext(r.Context())
		logger.FromContext(r.Context(), log).Info("db query")
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ads", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	headerReqID := rec.Header().Get("X-Request-ID")
	if headerReqID == "" || headerReqID != ctxReqID {
		t.Fatalf("request_id mismatch: header=%q ctx=%q", headerReqID, ctxReqID)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing X-Content-Type-Options header")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("missing X-Frame-Options header")
	}
	if !strings.Contains(logBuf.String(), headerReqID) {
		t.Fatalf("expected request_id %q in logs: %s", headerReqID, logBuf.String())
	}
}

func TestCSRFMiddleware(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 1. GET sets csrf_token cookie
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/ads", nil)
	recGet := httptest.NewRecorder()
	handler.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", recGet.Code)
	}
	var csrfCookie *http.Cookie
	for _, c := range recGet.Result().Cookies() {
		if c.Name == CSRFCookieName {
			csrfCookie = c
		}
	}
	if csrfCookie == nil || csrfCookie.Value == "" {
		t.Fatal("expected csrf_token cookie on GET")
	}

	// 2. POST without CSRF header -> 403
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/ads", nil)
	reqBad.AddCookie(csrfCookie)
	recBad := httptest.NewRecorder()
	handler.ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusForbidden {
		t.Fatalf("POST without CSRF header status = %d, want 403", recBad.Code)
	}

	// 3. POST with matching CSRF cookie and header -> 200
	reqOk := httptest.NewRequest(http.MethodPost, "/api/v1/ads", nil)
	reqOk.AddCookie(csrfCookie)
	reqOk.Header.Set(CSRFHeaderName, csrfCookie.Value)
	recOk := httptest.NewRecorder()
	handler.ServeHTTP(recOk, reqOk)
	if recOk.Code != http.StatusOK {
		t.Fatalf("POST with valid CSRF status = %d, want 200", recOk.Code)
	}
}
