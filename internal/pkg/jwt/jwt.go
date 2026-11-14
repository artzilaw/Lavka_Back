package jwt

import (
	"fmt"
	"time"

	"lavka/internal/domain/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func NewToken(user models.User, duration time.Duration, secret string) (string, error) {
	claims := jwt.MapClaims{
		"uid": user.ID,
		"exp": time.Now().Add(duration).Unix(),
		"jti": uuid.New().String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseToken(tokenString string, secret string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("token is invalid")
	}

	return token, nil
}
