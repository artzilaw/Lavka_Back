package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lavka/internal/domain/models"
	"lavka/internal/logger"
	"lavka/internal/pkg/jwt"
	"lavka/internal/storage"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var allowedAvatarTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

type TokenRevoker interface {
	Add(jti string, exp time.Time)
	Check(jti string) bool
}

type UserProviderSaver interface {
	SaveUser(ctx context.Context, email string, passHash []byte, name string) (uid int64, err error)
	User(ctx context.Context, email string) (models.User, error)
	UserByID(ctx context.Context, userID int64) (models.User, error)
	IsAdmin(ctx context.Context, userID int64) (bool, error)
	UpdateUser(ctx context.Context, userID int64, name string, email string) (models.User, error)
	UpdatePassword(ctx context.Context, userID int64, passHash []byte) error
	UpdateAvatarPath(ctx context.Context, userID int64, avatarPath string) (models.User, error)
}

type Auth struct {
	log          *slog.Logger
	userStorage  UserProviderSaver
	tokenRevoker TokenRevoker
	tokenTTL     time.Duration
	secret       string
}

var (
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrTokenRevoked           = errors.New("token has been revoked")
	ErrInvalidCurrentPassword = errors.New("invalid current password")
	ErrSamePassword           = errors.New("new password must be different")
	ErrUnsupportedFileType    = errors.New("unsupported file type")
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

	log := logger.FromContext(ctx, a.log).With(slog.String("op", op), slog.String("email", email))
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
	log := logger.FromContext(ctx, a.log).With(slog.String("email", email))
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

func (a *Auth) GetProfile(ctx context.Context, userID int64) (models.User, error) {
	const op = "auth.GetProfile"
	user, err := a.userStorage.UserByID(ctx, userID)
	if err != nil {
		return models.User{}, fmt.Errorf("%s: %w", op, err)
	}
	return user, nil
}

func (a *Auth) UpdateProfile(ctx context.Context, userID int64, name string, email string) (models.User, error) {
	const op = "auth.UpdateProfile"
	user, err := a.userStorage.UpdateUser(ctx, userID, name, email)
	if err != nil {
		return models.User{}, fmt.Errorf("%s: %w", op, err)
	}
	return user, nil
}

func (a *Auth) ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error {
	const op = "auth.ChangePassword"

	user, err := a.userStorage.UserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if err := bcrypt.CompareHashAndPassword(user.PassHash, []byte(currentPassword)); err != nil {
		return ErrInvalidCurrentPassword
	}

	if currentPassword == newPassword {
		return ErrSamePassword
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if err := a.userStorage.UpdatePassword(ctx, userID, newHash); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (a *Auth) UpdateAvatar(ctx context.Context, userID int64, file io.ReadSeeker, filename string) (models.User, error) {
	const op = "auth.UpdateAvatar"

	buff := make([]byte, 512)
	n, err := file.Read(buff)
	if err != nil && !errors.Is(err, io.EOF) {
		return models.User{}, fmt.Errorf("%s: failed to read file header: %w", op, err)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return models.User{}, fmt.Errorf("%s: failed to seek file: %w", op, err)
	}

	contentType := http.DetectContentType(buff[:n])
	defaultExt, ok := allowedAvatarTypes[contentType]
	if !ok {
		return models.User{}, ErrUnsupportedFileType
	}

	currentUser, err := a.userStorage.UserByID(ctx, userID)
	if err != nil {
		return models.User{}, fmt.Errorf("%s: %w", op, err)
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" && ext != ".gif" {
		ext = defaultExt
	}

	newFileName := fmt.Sprintf("%d_%d%s", userID, time.Now().UnixNano(), ext)
	storageDir := filepath.Join("static", "img", "avatars")
	storagePath := filepath.Join(storageDir, newFileName)
	dbPath := "/static/img/avatars/" + newFileName

	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		return models.User{}, fmt.Errorf("%s: failed to create avatar dir: %w", op, err)
	}

	dst, err := os.Create(storagePath)
	if err != nil {
		return models.User{}, fmt.Errorf("%s: failed to create file: %w", op, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		_ = os.Remove(storagePath)
		return models.User{}, fmt.Errorf("%s: failed to copy file: %w", op, err)
	}

	updatedUser, err := a.userStorage.UpdateAvatarPath(ctx, userID, dbPath)
	if err != nil {
		_ = os.Remove(storagePath)
		return models.User{}, fmt.Errorf("%s: %w", op, err)
	}

	if strings.HasPrefix(currentUser.AvatarPath, "/static/img/avatars/") {
		oldFilePath := filepath.Clean(strings.TrimPrefix(currentUser.AvatarPath, "/"))
		_ = os.Remove(oldFilePath)
	}

	return updatedUser, nil
}
