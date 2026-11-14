package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"lavka/internal/domain/models"
	"lavka/internal/pkg/jwt"
	"lavka/internal/storage"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type TokenRevoker interface {
	Add(jti string, exp time.Time)
	Check(jti string) bool
}

type UserProviderSaver interface {
	SaveUser(ctx context.Context, email string, passHash []byte, name string) (uid int64, err error)
	User(ctx context.Context, email string) (models.User, error)
	UserByID(ctx context.Context, userID int64) (models.User, error)
	IsAdmin(ctx context.Context, userID int64) (bool, error)
}

type Auth struct {
	log          *slog.Logger
	userStorage  UserProviderSaver
	tokenRevoker TokenRevoker
	tokenTTL     time.Duration
	secret       string
}

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenRevoked       = errors.New("token has been revoked")
)

func New(
	log *slog.Logger,
	userStorage UserProviderSaver,
	tokenRevoker TokenRevoker,
	tokenTTL time.Duration,
	secret string,
) *Auth {
	return &Auth{
		log:          log,
		userStorage:  userStorage,
		tokenRevoker: tokenRevoker,
		tokenTTL:     tokenTTL,
		secret:       secret,
	}
}

func (a *Auth) RegisterNewUser(ctx context.Context, email, password, name string) (int64, error) {
	const op = "auth.RegisterNewUser"

	log := a.log.With(slog.String("op", op), slog.String("email", email))
	log.Info("registering user")

	passHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("failed to generate password hash")
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := a.userStorage.SaveUser(ctx, email, passHash, name)
	if err != nil {
		if errors.Is(err, storage.ErrUserExists) {
			log.Info("user already exists")
			return 0, fmt.Errorf("%s: %w", op, err)
		}
		log.Error("failed to save user")
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("user registered", slog.Int64("user_id", id))
	return id, nil
}

func (a *Auth) Login(ctx context.Context, email, password string) (string, models.User, error) {
	log := a.log.With(slog.String("email", email))
	log.Info("logging in user")

	user, err := a.userStorage.User(ctx, email)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			log.Info("user not found")
			return "", models.User{}, fmt.Errorf("%w", ErrInvalidCredentials)
		}
		log.Error("failed to get user")
		return "", models.User{}, fmt.Errorf("%w", err)
	}

	if err := bcrypt.CompareHashAndPassword(user.PassHash, []byte(password)); err != nil {
		log.Info("invalid credentials")
		return "", models.User{}, fmt.Errorf("%w", ErrInvalidCredentials)
	}

	token, err := jwt.NewToken(user, a.tokenTTL, a.secret)
	if err != nil {
		log.Error("failed to generate token")
		return "", models.User{}, fmt.Errorf("%w", err)
	}

	log.Info("user logged in")
	return token, user, nil
}

func (a *Auth) ValidateTokenAndGetUser(ctx context.Context, tokenString string) (models.User, error) {
	const op = "auth.ValidateTokenAndGetUser"
	log := a.log.With(slog.String("op", op))

	token, err := jwt.ParseToken(tokenString, a.secret)
	if err != nil {
		log.Info("failed to parse token", slog.String("error", err.Error()))
		return models.User{}, fmt.Errorf("%w", err)
	}

	claims, ok := token.Claims.(jwtlib.MapClaims)
	if !ok {
		return models.User{}, errors.New("invalid token claims")
	}

	jti, _ := claims["jti"].(string)
	if jti != "" && a.tokenRevoker != nil && a.tokenRevoker.Check(jti) {
		log.Info("attempt to use revoked token", slog.String("jti", jti))
		return models.User{}, ErrTokenRevoked
	}

	uidRaw, ok := claims["uid"].(float64)
	if !ok {
		return models.User{}, errors.New("invalid uid claim")
	}

	userID := int64(uidRaw)
	user, err := a.userStorage.UserByID(ctx, userID)
	if err != nil {
		return models.User{}, fmt.Errorf("user not found: %w", err)
	}

	return user, nil
}

func (a *Auth) Logout(ctx context.Context, jti string, exp time.Time) error {
	a.tokenRevoker.Add(jti, exp)
	return nil
}
