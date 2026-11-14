package httpapp

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lavka/internal/services/auth"
	"lavka/internal/storage"
	"lavka/internal/storage/ads"
	"lavka/internal/storage/blacklist"
)

func setupTestApp(t *testing.T) *App {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, _ := storage.New("")
	bl := blacklist.New(time.Minute)
	t.Cleanup(func() { bl.Stop() })

	authSvc := auth.New(log, st, bl, time.Hour, "test-secret")
	adsSvc := ads.NewAdsRepository()

	return New(log, Services{Ads: adsSvc, Auth: authSvc}, bl, 8000, time.Hour, "test-secret")
}

func TestHTTP_RegisterLoginAdsLogout(t *testing.T) {
	app := setupTestApp(t)

	// 1. GET /api/v1/ads
	reqAds := httptest.NewRequest(http.MethodGet, "/api/v1/ads", nil)
	recAds := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recAds, reqAds)
	if recAds.Code != http.StatusOK {
		t.Fatalf("GET /ads status = %d, want 200", recAds.Code)
	}

	// 2. POST /api/v1/auth/register (ошибка валидации)
	badRegBody := []byte(`{"name":"","email":"bad","password":"123"}`)
	reqBadReg := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(badRegBody))
	recBadReg := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recBadReg, reqBadReg)
	if recBadReg.Code != http.StatusBadRequest {
		t.Fatalf("POST /auth/register bad input status = %d, want 400", recBadReg.Code)
	}

	// 3. POST /api/v1/auth/register (успешная регистрация)
	regBody := []byte(`{"name":"TestUser","email":"test@lavka.ru","password":"Password123"}`)
	reqReg := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regBody))
	recReg := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recReg, reqReg)
	if recReg.Code != http.StatusOK {
		t.Fatalf("POST /auth/register status = %d, body = %s", recReg.Code, recReg.Body.String())
	}

	// Дубликат регистрации
	reqDup := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regBody))
	recDup := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recDup, reqDup)
	if recDup.Code != http.StatusBadRequest {
		t.Fatalf("POST /auth/register duplicate status = %d, want 400", recDup.Code)
	}

	// 4. POST /api/v1/auth/login по логину и паролю (без куки)
	loginBody := []byte(`{"email":"test@lavka.ru","password":"Password123"}`)
	reqLoginCreds := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	recLoginCreds := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recLoginCreds, reqLoginCreds)
	if recLoginCreds.Code != http.StatusOK {
		t.Fatalf("POST /auth/login (credentials) status = %d, want 200", recLoginCreds.Code)
	}

	// Вход с неверным паролем
	wrongLoginBody := []byte(`{"email":"test@lavka.ru","password":"WrongPassword1"}`)
	reqWrongLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(wrongLoginBody))
	recWrongLogin := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recWrongLogin, reqWrongLogin)
	if recWrongLogin.Code != http.StatusUnauthorized {
		t.Fatalf("POST /auth/login (wrong password) status = %d, want 401", recWrongLogin.Code)
	}

	// Вход с пустыми полями
	emptyLoginBody := []byte(`{"email":"","password":""}`)
	reqEmptyLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(emptyLoginBody))
	recEmptyLogin := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recEmptyLogin, reqEmptyLogin)
	if recEmptyLogin.Code != http.StatusUnauthorized {
		t.Fatalf("POST /auth/login (empty fields) status = %d, want 401", recEmptyLogin.Code)
	}

	cookies := recReg.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected auth cookie after register")
	}

	// 5. POST /api/v1/auth/login по куке
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	reqCheck.AddCookie(cookies[0])
	recCheck := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("POST /auth/login (cookie) status = %d, want 200", recCheck.Code)
	}

	// 6. POST /api/v1/auth/logout
	reqLogout := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	reqLogout.AddCookie(cookies[0])
	recLogout := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recLogout, reqLogout)
	if recLogout.Code != http.StatusOK {
		t.Fatalf("POST /auth/logout status = %d, want 200", recLogout.Code)
	}

	// 7. Повторный вход по отозванному токену должен вернуть 401
	reqRevoked := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	reqRevoked.AddCookie(cookies[0])
	recRevoked := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recRevoked, reqRevoked)
	if recRevoked.Code != http.StatusUnauthorized {
		t.Fatalf("POST /auth/login with revoked cookie status = %d, want 401", recRevoked.Code)
	}
}
