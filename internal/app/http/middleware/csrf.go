package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"lavka/internal/pkg/responser"
)

const (
	CSRFCookieName = "csrf_token"
	CSRFHeaderName = "X-CSRF-Token"
)

var (
	ErrInvalidCSRFToken = "invalid or missing csrf token"

	csrfExemptPaths = map[string]bool{
		"/api/v1/auth/login":    true,
		"/api/v1/auth/register": true,
	}
)

func generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "fallback-csrf-token"
	}
	return hex.EncodeToString(b)
}

// EnsureCSRFCookie гарантирует наличие cookie csrf_token и возвращает его значение.
func EnsureCSRFCookie(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(CSRFCookieName); err == nil && strings.TrimSpace(c.Value) != "" {
		return c.Value
	}

	token := generateCSRFToken()
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false, // Должен читаться JS на фронтенде для Double-Submit Cookie
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
	return token
}

// CSRFMiddleware реализует защиту от CSRF по схеме Double-Submit Cookie.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			EnsureCSRFCookie(w, r)
			next.ServeHTTP(w, r)
			return
		}

		if csrfExemptPaths[r.URL.Path] {
			EnsureCSRFCookie(w, r)
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(CSRFCookieName)
		headerToken := strings.TrimSpace(r.Header.Get(CSRFHeaderName))

		if err != nil || cookie.Value == "" || headerToken == "" ||
			len(cookie.Value) != len(headerToken) ||
			subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(headerToken)) != 1 {
			responser.RespondWithError(w, http.StatusForbidden, ErrInvalidCSRFToken)
			return
		}

		next.ServeHTTP(w, r)
	})
}
