package httpapp

import (
	"bytes"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"lavka/internal/app/http/middleware"
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
	t.Cleanup(func() {
		bl.Stop()
		_ = os.RemoveAll("static")
	})

	authSvc := auth.New(log, st, bl, time.Hour, "test-secret")
	adsSvc := ads.NewAdsRepository()

	return New(log, Services{Ads: adsSvc, Auth: authSvc}, bl, 8000, time.Hour, "test-secret")
}

func TestHTTP_RegisterLoginProfileAdsLogout(t *testing.T) {
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

	cookies := recReg.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected auth cookie after register")
	}
	var authCookie, csrfCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "token" {
			authCookie = c
		}
		if c.Name == middleware.CSRFCookieName {
			csrfCookie = c
		}
	}
	if authCookie == nil || csrfCookie == nil {
		t.Fatalf("expected token and csrf_token cookies, got %+v", cookies)
	}
	withSession := func(req *http.Request) {
		req.AddCookie(authCookie)
		req.AddCookie(csrfCookie)
		req.Header.Set(middleware.CSRFHeaderName, csrfCookie.Value)
	}

	// 4. GET /api/v1/profile
	reqProf := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	withSession(reqProf)
	recProf := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recProf, reqProf)
	if recProf.Code != http.StatusOK {
		t.Fatalf("GET /profile status = %d, want 200", recProf.Code)
	}

	// 5. PATCH /api/v1/profile (обновление имени и почты)
	updBody := []byte(`{"name":"UpdatedName","email":"updated@lavka.ru"}`)
	reqUpd := httptest.NewRequest(http.MethodPatch, "/api/v1/profile", bytes.NewReader(updBody))
	withSession(reqUpd)
	recUpd := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recUpd, reqUpd)
	if recUpd.Code != http.StatusOK {
		t.Fatalf("PATCH /profile status = %d, body = %s", recUpd.Code, recUpd.Body.String())
	}

	// 6. POST /api/v1/profile/password (смена пароля)
	pwdBody := []byte(`{"current_password":"Password123","new_password":"NewPassword456"}`)
	reqPwd := httptest.NewRequest(http.MethodPost, "/api/v1/profile/password", bytes.NewReader(pwdBody))
	withSession(reqPwd)
	recPwd := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recPwd, reqPwd)
	if recPwd.Code != http.StatusOK {
		t.Fatalf("POST /profile/password status = %d, body = %s", recPwd.Code, recPwd.Body.String())
	}

	// 7. POST /api/v1/profile/avatar (загрузка аватарки)
	var bodyBuf bytes.Buffer
	mw := multipart.NewWriter(&bodyBuf)
	fw, _ := mw.CreateFormFile("avatar", "avatar.png")
	_, _ = fw.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D})
	_ = mw.Close()

	reqAvatar := httptest.NewRequest(http.MethodPost, "/api/v1/profile/avatar", &bodyBuf)
	reqAvatar.Header.Set("Content-Type", mw.FormDataContentType())
	withSession(reqAvatar)
	recAvatar := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recAvatar, reqAvatar)
	if recAvatar.Code != http.StatusOK {
		t.Fatalf("POST /profile/avatar status = %d, body = %s", recAvatar.Code, recAvatar.Body.String())
	}

	// 8. GET /api/v1/users/1 (публичный профиль)
	reqPub := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	recPub := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recPub, reqPub)
	if recPub.Code != http.StatusOK {
		t.Fatalf("GET /users/1 status = %d, want 200", recPub.Code)
	}

	// 9. POST /api/v1/auth/login с обновлённым email и паролем
	loginBody := []byte(`{"email":"updated@lavka.ru","password":"NewPassword456"}`)
	reqLoginCreds := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	recLoginCreds := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recLoginCreds, reqLoginCreds)
	if recLoginCreds.Code != http.StatusOK {
		t.Fatalf("POST /auth/login (new credentials) status = %d, want 200", recLoginCreds.Code)
	}

	// 10. POST /api/v1/auth/logout
	reqLogout := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	withSession(reqLogout)
	recLogout := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recLogout, reqLogout)
	if recLogout.Code != http.StatusOK {
		t.Fatalf("POST /auth/logout status = %d, want 200", recLogout.Code)
	}
}
