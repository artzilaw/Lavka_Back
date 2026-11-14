package httpapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
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
	opRun     = "httpapp.Run"
	opStop    = "httpapp.Stop"
	apiPrefix = "/api/v1"
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
	UserID    int64     `json:"user_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type ValidationErrors struct {
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
	Name     string `json:"name,omitempty"`
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
	a.router.HandleFunc("POST "+apiPrefix+"/auth/register", a.handleRegister)
	a.router.HandleFunc("POST "+apiPrefix+"/auth/login", a.handleLogin)

	authMW := middleware.AuthMiddleware(a.log, a.blacklist, a.secret)
	a.router.Handle("POST "+apiPrefix+"/auth/logout", authMW(http.HandlerFunc(a.handleLogout)))

	fs := http.FileServer(http.Dir("static"))
	a.router.Handle("/static/", http.StripPrefix("/static/", fs))

	a.router.HandleFunc("GET "+apiPrefix+"/ads", a.handleGetAds)
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
		UserID:    userID,
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt,
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
