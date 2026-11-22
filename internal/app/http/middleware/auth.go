package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"lavka/internal/pkg/responser"

	"github.com/golang-jwt/jwt/v5"
)

type TokenChecker interface {
	Check(jti string) bool
}

type contextKey string

const (
	UserIDKey contextKey = "userID"
	JtiKey    contextKey = "jti"
)

var (
	ErrMissingTokenCookie = "missing token cookie"
	ErrInvalidToken       = "invalid token"
	ErrInvalidTokenClaims = "invalid token claims"
	ErrTokenRevoked       = "token has been revoked"
	ErrInvalidUidClaim    = "invalid uid claim"
)

func AuthMiddleware(log *slog.Logger, bl TokenChecker, secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("token")
			if err != nil {
				responser.RespondWithError(w, http.StatusUnauthorized, ErrMissingTokenCookie)
				return
			}

			tokenString := cookie.Value
			token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
				return []byte(secret), nil
			})

			if err != nil || !token.Valid {
				responser.RespondWithError(w, http.StatusUnauthorized, ErrInvalidToken)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				responser.RespondWithError(w, http.StatusUnauthorized, ErrInvalidTokenClaims)
				return
			}

			jti, _ := claims["jti"].(string)
			if bl.Check(jti) {
				log.Info("attempt to use revoked token", slog.String("jti", jti))
				responser.RespondWithError(w, http.StatusUnauthorized, ErrTokenRevoked)
				return
			}

			uidRaw, ok := claims["uid"].(float64)
			if !ok {
				responser.RespondWithError(w, http.StatusUnauthorized, ErrInvalidUidClaim)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, int64(uidRaw))
			ctx = context.WithValue(ctx, JtiKey, jti)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
