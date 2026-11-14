package httpapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lavka/internal/app/http/middleware"
	"lavka/internal/domain/models"
	"lavka/internal/pkg/responser"
	"lavka/internal/pkg/validator"
	"lavka/internal/services/auth"
	"lavka/internal/storage"
	"lavka/internal/storage/blacklist"
)

const (
	opRun         = "httpapp.Run"
	opStop        = "httpapp.Stop"
	apiPrefix     = "/api/v1"
	MaxUploadSize = 5 << 20 // 5 MB
)

var (
	ErrInvalidRequestBody   = "invalid request body"
	ErrUserAlreadyExists    = "user already exists"
	ErrFailedToRegisterUser = "failed to register user"
	ErrAutoLoginFailed      = "registered, but failed to login"
	ErrFailedToLogin        = "failed to login"
	ErrInternalError        = "internal error"
	ErrFailedToLogout       = "failed to logout"
	ErrMethodNotAllowed     = "Method not allowed"
	ErrUnauthorized         = "unauthorized"
	ErrInvalidUserID        = "invalid user id"
	ErrUserNotFound         = "user not found"
	ErrFileTooBig           = "file too big"
	ErrFailedToGetFile      = "failed to get file"
)

type App struct {
	log       *slog.Logger
	router    *http.ServeMux
	port      int
	srv       *http.Server
	services  Services
	blacklist *blacklist.InMemory
	tokenTTL  time.Duration
	secret    string
}

type Ads interface {
	GetAll() []models.Ad
}

type Auth interface {
	Login(ctx context.Context, email string, password string) (string, models.User, error)
	ValidateTokenAndGetUser(ctx context.Context, tokenString string) (models.User, error)
	RegisterNewUser(ctx context.Context, email string, password string, name string) (userID int64, err error)
	Logout(ctx context.Context, jti string, exp time.Time) error
	GetProfile(ctx context.Context, userID int64) (models.User, error)
	UpdateProfile(ctx context.Context, userID int64, name string, email string) (models.User, error)
	ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error
	UpdateAvatar(ctx context.Context, userID int64, file io.ReadSeeker, filename string) (models.User, error)
}

type Services struct {
	Ads  Ads
	Auth Auth
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	AvatarPath string    `json:"avatar_path"`
	CreatedAt  time.Time `json:"created_at"`
}

type UpdateProfileRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type PublicUserResponse struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	AvatarPath   string    `json:"avatar_path"`
	Rating       float64   `json:"rating"`
	ReviewsCount int       `json:"reviews_count"`
	AdsCount     int       `json:"ads_count"`
	CreatedAt    time.Time `json:"created_at"`
}

type ValidationErrors struct {
	Email           string `json:"email,omitempty"`
	Password        string `json:"password,omitempty"`
	Name            string `json:"name,omitempty"`
	CurrentPassword string `json:"current_password,omitempty"`
	NewPassword     string `json:"new_password,omitempty"`
	Error           string `json:"error,omitempty"`
}

func New(
	log *slog.Logger,
	services Services,
	bl *blacklist.InMemory,
	port int,
	tokenTTL time.Duration,
	secret string,
) *App {
	app := &App{
		log:       log,
		router:    http.NewServeMux(),
		port:      port,
		services:  services,
		tokenTTL:  tokenTTL,
		blacklist: bl,
		secret:    secret,
	}

	app.setupRoutes()

	handlerWithCORS := middleware.CORSMiddleware(app.router)

	app.srv = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handlerWithCORS,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return app
}

func (a *App) setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(a.tokenTTL.Seconds()),
	})
}

func (a *App) setupRoutes() {
	// Публичные маршруты
	a.router.HandleFunc("POST "+apiPrefix+"/auth/register", a.handleRegister)
	a.router.HandleFunc("POST "+apiPrefix+"/auth/login", a.handleLogin)
	a.router.HandleFunc("GET "+apiPrefix+"/ads", a.handleGetAds)
	a.router.HandleFunc("GET "+apiPrefix+"/users/{id}", a.handleGetPublicProfile)

	// Защищённые маршруты (требуют валидный JWT в куке token)
	authMW := middleware.AuthMiddleware(a.log, a.blacklist, a.secret)
	a.router.Handle("POST "+apiPrefix+"/auth/logout", authMW(http.HandlerFunc(a.handleLogout)))

	// Маршруты профиля
	a.router.Handle("GET "+apiPrefix+"/profile", authMW(http.HandlerFunc(a.handleGetProfile)))
	a.router.Handle("PATCH "+apiPrefix+"/profile", authMW(http.HandlerFunc(a.handleUpdateProfile)))
	a.router.Handle("PUT "+apiPrefix+"/profile", authMW(http.HandlerFunc(a.handleUpdateProfile)))
	a.router.Handle("POST "+apiPrefix+"/profile/password", authMW(http.HandlerFunc(a.handleChangePassword)))
	a.router.Handle("PATCH "+apiPrefix+"/profile/password", authMW(http.HandlerFunc(a.handleChangePassword)))
	a.router.Handle("POST "+apiPrefix+"/profile/avatar", authMW(http.HandlerFunc(a.handleUploadAvatar)))

	// Раздача статики (картинки объявлений и аватарки)
	fs := http.FileServer(http.Dir("static"))
	a.router.Handle("/static/", http.StripPrefix("/static/", fs))
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responser.RespondWithError(w, http.StatusBadRequest, ErrInvalidRequestBody)
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)

	validationErrors := ValidationErrors{}
	if err := validator.ValidateEmail(req.Email); err != nil {
		validationErrors.Email = err.Error()
	}
	if err := validator.ValidatePassword(req.Password); err != nil {
		validationErrors.Password = err.Error()
	}
	if err := validator.ValidateName(req.Name); err != nil {
		validationErrors.Name = err.Error()
	}

	if validationErrors.Email != "" || validationErrors.Password != "" || validationErrors.Name != "" {
		responser.RespondWithJSON(w, http.StatusBadRequest, validationErrors)
		return
	}

	userID, err := a.services.Auth.RegisterNewUser(r.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		if errors.Is(err, storage.ErrUserExists) {
			responser.RespondWithError(w, http.StatusBadRequest, ErrUserAlreadyExists)
			return
		}
		a.log.Error("failed to register user", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrFailedToRegisterUser)
		return
	}

	token, user, err := a.services.Auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		a.log.Error("auto-login failed", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrAutoLoginFailed)
		return
	}

	a.setAuthCookie(w, token)
	a.respondWithUser(w, userID, user)
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	hasCredentials := err == nil && (strings.TrimSpace(req.Email) != "" || req.Password != "")

	if !hasCredentials {
		if cookie, cookieErr := r.Cookie("token"); cookieErr == nil && cookie.Value != "" {
			a.handleTokenLogin(w, r, cookie.Value)
			return
		}
	}

	if err != nil {
		responser.RespondWithError(w, http.StatusBadRequest, ErrInvalidRequestBody)
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if req.Email == "" || req.Password == "" {
		validationErrors := ValidationErrors{}
		if req.Email == "" {
			validationErrors.Email = "email is required"
		}
		if req.Password == "" {
			validationErrors.Password = "password is required"
		}
		responser.RespondWithJSON(w, http.StatusUnauthorized, validationErrors)
		return
	}

	a.handleCredentialsLogin(w, r, req.Email, req.Password)
}

func (a *App) handleCredentialsLogin(w http.ResponseWriter, r *http.Request, email, password string) {
	validationErrors := ValidationErrors{}
	if err := validator.ValidateEmail(email); err != nil {
		validationErrors.Email = err.Error()
	}
	if err := validator.ValidatePassword(password); err != nil {
		validationErrors.Password = err.Error()
	}

	if validationErrors.Email != "" || validationErrors.Password != "" {
		responser.RespondWithJSON(w, http.StatusUnauthorized, validationErrors)
		return
	}

	token, user, err := a.services.Auth.Login(r.Context(), email, password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			responser.RespondWithError(w, http.StatusUnauthorized, err.Error())
			return
		}
		responser.RespondWithError(w, http.StatusInternalServerError, ErrFailedToLogin)
		return
	}

	a.setAuthCookie(w, token)
	a.respondWithUser(w, user.ID, user)
}

func (a *App) handleTokenLogin(w http.ResponseWriter, r *http.Request, tokenString string) {
	user, err := a.services.Auth.ValidateTokenAndGetUser(r.Context(), tokenString)
	if err != nil {
		responser.RespondWithError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	a.setAuthCookie(w, tokenString)
	a.respondWithUser(w, user.ID, user)
}

func (a *App) respondWithUser(w http.ResponseWriter, userID int64, user models.User) {
	responser.RespondWithJSON(w, http.StatusOK, LoginResponse{
		ID:         userID,
		UserID:     userID,
		Email:      user.Email,
		Name:       user.Name,
		AvatarPath: user.AvatarPath,
		CreatedAt:  user.CreatedAt,
	})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	jti, ok := r.Context().Value(middleware.JtiKey).(string)
	if !ok {
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	if err := a.services.Auth.Logout(r.Context(), jti, time.Now().Add(a.tokenTTL)); err != nil {
		responser.RespondWithError(w, http.StatusInternalServerError, ErrFailedToLogout)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	responser.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	user, err := a.services.Auth.GetProfile(r.Context(), userID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			responser.RespondWithError(w, http.StatusNotFound, ErrUserNotFound)
			return
		}
		a.log.Error("failed to get profile", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusOK, user)
}

func (a *App) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responser.RespondWithError(w, http.StatusBadRequest, ErrInvalidRequestBody)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if req.Name == "" && req.Email == "" {
		responser.RespondWithError(w, http.StatusBadRequest, ErrInvalidRequestBody)
		return
	}

	validationErrors := ValidationErrors{}
	if req.Name != "" {
		if err := validator.ValidateName(req.Name); err != nil {
			validationErrors.Name = err.Error()
		}
	}
	if req.Email != "" {
		if err := validator.ValidateEmail(req.Email); err != nil {
			validationErrors.Email = err.Error()
		}
	}

	if validationErrors.Name != "" || validationErrors.Email != "" {
		responser.RespondWithJSON(w, http.StatusBadRequest, validationErrors)
		return
	}

	updatedUser, err := a.services.Auth.UpdateProfile(r.Context(), userID, req.Name, req.Email)
	if err != nil {
		if errors.Is(err, storage.ErrUserExists) {
			responser.RespondWithJSON(w, http.StatusBadRequest, ValidationErrors{
				Email: ErrUserAlreadyExists,
				Error: ErrUserAlreadyExists,
			})
			return
		}
		if errors.Is(err, storage.ErrUserNotFound) {
			responser.RespondWithError(w, http.StatusNotFound, ErrUserNotFound)
			return
		}
		a.log.Error("failed to update profile", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusOK, updatedUser)
}

func (a *App) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responser.RespondWithError(w, http.StatusBadRequest, ErrInvalidRequestBody)
		return
	}

	validationErrors := ValidationErrors{}
	if req.CurrentPassword == "" {
		validationErrors.CurrentPassword = "current password is required"
	}
	if err := validator.ValidatePassword(req.NewPassword); err != nil {
		validationErrors.NewPassword = err.Error()
	}

	if validationErrors.CurrentPassword != "" || validationErrors.NewPassword != "" {
		responser.RespondWithJSON(w, http.StatusBadRequest, validationErrors)
		return
	}

	if err := a.services.Auth.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		if errors.Is(err, auth.ErrInvalidCurrentPassword) {
			responser.RespondWithJSON(w, http.StatusBadRequest, ValidationErrors{
				CurrentPassword: err.Error(),
				Error:           err.Error(),
			})
			return
		}
		if errors.Is(err, auth.ErrSamePassword) {
			responser.RespondWithJSON(w, http.StatusBadRequest, ValidationErrors{
				NewPassword: err.Error(),
				Error:       err.Error(),
			})
			return
		}
		a.log.Error("failed to change password", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize)
	if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
		responser.RespondWithError(w, http.StatusBadRequest, ErrFileTooBig)
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		responser.RespondWithError(w, http.StatusBadRequest, ErrFailedToGetFile)
		return
	}
	defer file.Close()

	updatedUser, err := a.services.Auth.UpdateAvatar(r.Context(), userID, file, header.Filename)
	if err != nil {
		if errors.Is(err, auth.ErrUnsupportedFileType) {
			responser.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		a.log.Error("failed to update avatar", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusOK, updatedUser)
}

func (a *App) handleGetPublicProfile(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || userID <= 0 {
		responser.RespondWithError(w, http.StatusBadRequest, ErrInvalidUserID)
		return
	}

	user, err := a.services.Auth.GetProfile(r.Context(), userID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			responser.RespondWithError(w, http.StatusNotFound, ErrUserNotFound)
			return
		}
		a.log.Error("failed to get public profile", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusOK, PublicUserResponse{
		ID:           user.ID,
		Name:         user.Name,
		AvatarPath:   user.AvatarPath,
		Rating:       user.Rating,
		ReviewsCount: user.ReviewsCount,
		AdsCount:     user.AdsCount,
		CreatedAt:    user.CreatedAt,
	})
}

func (a *App) handleGetAds(w http.ResponseWriter, r *http.Request) {
	adsList := a.services.Ads.GetAll()
	responser.RespondWithJSON(w, http.StatusOK, adsList)
}

func (a *App) MustRun() {
	if err := a.Run(); err != nil {
		panic(err)
	}
}

func (a *App) Run() error {
	a.log.Info("http server started", slog.String("addr", a.srv.Addr))
	if err := a.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("%s: %w", opRun, err)
	}
	return nil
}

func (a *App) Stop() {
	a.log.With(slog.String("op", opStop)).Info("stopping http server", slog.Int("port", a.port))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.srv.Shutdown(ctx); err != nil {
		a.log.Error("failed to shutdown http server gracefully", slog.String("error", err.Error()))
	}
}
