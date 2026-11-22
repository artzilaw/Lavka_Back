package grpc_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lavka/internal/domain/models"
	"lavka/internal/grpc/authservice"
	"lavka/internal/grpc/catalogservice"
	"lavka/internal/grpc/interceptors"
	"lavka/internal/logger"
	"lavka/internal/services/auth"
	"lavka/internal/storage"
	"lavka/internal/storage/ads"
	"lavka/internal/storage/blacklist"
	"lavka/internal/storage/favorites"

	"google.golang.org/grpc"
)

func TestAuthServiceGRPC_FullFlowAndMetrics(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, err := storage.New("")
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	bl := blacklist.New(time.Minute)
	defer bl.Stop()

	authSvc := auth.New(log, st, bl, time.Hour, "test-secret-grpc")
	metricsReg := interceptors.NewGRPCMetricsRegistry("auth-service")

	srv := grpc.NewServer(grpc.UnaryInterceptor(metricsReg.UnaryServerInterceptor(log)))
	authservice.RegisterAuthServiceServer(srv, authservice.NewServer(authSvc, bl))

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	client, err := authservice.NewClient(lis.Addr().String())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	ctx := logger.WithRequestID(context.Background(), "req-grpc-auth-1")

	// 1. Register
	uid, err := client.RegisterNewUser(ctx, "grpc@lavka.ru", "Password123", "GRPC User")
	if err != nil || uid <= 0 {
		t.Fatalf("RegisterNewUser failed: uid=%d err=%v", uid, err)
	}

	// Дубликат -> ErrUserExists
	if _, err := client.RegisterNewUser(ctx, "grpc@lavka.ru", "Password123", "GRPC User"); !errors.Is(err, storage.ErrUserExists) {
		t.Fatalf("expected ErrUserExists, got %v", err)
	}

	// 2. Login
	token, u, err := client.Login(ctx, "grpc@lavka.ru", "Password123")
	if err != nil || token == "" || u.ID != uid {
		t.Fatalf("Login failed: token=%q user=%+v err=%v", token, u, err)
	}

	// Неверный пароль -> ErrInvalidCredentials
	if _, _, err := client.Login(ctx, "grpc@lavka.ru", "WrongPass999"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	// 3. ValidateTokenAndGetUser
	validated, err := client.ValidateTokenAndGetUser(ctx, token)
	if err != nil || validated.Email != "grpc@lavka.ru" {
		t.Fatalf("ValidateTokenAndGetUser failed: %+v err=%v", validated, err)
	}

	// 4. GetProfile & UpdateProfile
	prof, err := client.GetProfile(ctx, uid)
	if err != nil || prof.Name != "GRPC User" {
		t.Fatalf("GetProfile failed: %+v err=%v", prof, err)
	}

	updated, err := client.UpdateProfile(ctx, uid, "Updated GRPC", "grpc2@lavka.ru")
	if err != nil || updated.Name != "Updated GRPC" || updated.Email != "grpc2@lavka.ru" {
		t.Fatalf("UpdateProfile failed: %+v err=%v", updated, err)
	}

	// 5. ChangePassword
	if err := client.ChangePassword(ctx, uid, "Password123", "NewPass456"); err != nil {
		t.Fatalf("ChangePassword failed: %v", err)
	}
	if _, _, err := client.Login(ctx, "grpc2@lavka.ru", "NewPass456"); err != nil {
		t.Fatalf("Login after ChangePassword failed: %v", err)
	}

	// 6. Logout & IsTokenRevoked (TokenChecker.Check)
	if err := client.Logout(ctx, "jti-123", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}
	if !client.Check("jti-123") {
		t.Fatal("expected jti-123 to be revoked via gRPC Check")
	}
	if client.Check("jti-unknown") {
		t.Fatal("expected jti-unknown not to be revoked")
	}

	// 7. Проверка Prometheus-метрик gRPC-сервиса
	rec := httptest.NewRecorder()
	metricsReg.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	if !strings.Contains(body, `lavka_grpc_requests_total{service="auth-service"`) {
		t.Fatalf("expected lavka_grpc_requests_total in metrics, got:\n%s", body)
	}
	if !strings.Contains(body, `lavka_grpc_errors_total{service="auth-service"`) {
		t.Fatalf("expected lavka_grpc_errors_total in metrics, got:\n%s", body)
	}
}

func TestCatalogServiceGRPC_FullFlowAndMetrics(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	adsRepo := ads.NewAdsRepository()
	favRepo := favorites.NewInMemoryRepository()
	metricsReg := interceptors.NewGRPCMetricsRegistry("catalog-service")

	srv := grpc.NewServer(grpc.UnaryInterceptor(metricsReg.UnaryServerInterceptor(log)))
	catalogservice.RegisterCatalogServiceServer(srv, catalogservice.NewServer(adsRepo, favRepo))

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	rawClient, err := catalogservice.NewClient(lis.Addr().String())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer rawClient.Close()

	adsClient := catalogservice.NewAdsClient(rawClient)

	// 1. GetAll & Search
	all := adsClient.GetAll()
	if len(all) == 0 {
		t.Fatal("expected non-empty ads list")
	}

	found := adsClient.Search(models.AdFilter{Query: "MacBook"})
	if len(found) == 0 {
		t.Fatal("expected Search('MacBook') to find at least 1 ad")
	}

	// 2. Create, GetByID, IncrementViews, Update, Close, Reopen, Delete
	created, err := adsClient.Create(models.Ad{
		SellerID:    42,
		CategoryID:  1,
		Title:       "gRPC Test Laptop",
		Description: "Brand new laptop via gRPC",
		Price:       99000,
		Location:    "Москва",
	})
	if err != nil || created.ID == 0 {
		t.Fatalf("Create failed: %+v err=%v", created, err)
	}

	viewed, err := adsClient.IncrementViews(created.ID)
	if err != nil || viewed.ViewsCount != 1 {
		t.Fatalf("IncrementViews failed: %+v err=%v", viewed, err)
	}

	updated, err := adsClient.Update(created.ID, 42, models.Ad{
		CategoryID:  1,
		Title:       "gRPC Test Laptop Updated",
		Description: "Updated description",
		Price:       89000,
		Location:    "Казань",
	})
	if err != nil || updated.Price != 89000 {
		t.Fatalf("Update failed: %+v err=%v", updated, err)
	}

	// Чужой seller -> ErrForbidden
	if _, err := adsClient.Close(created.ID, 999); !errors.Is(err, ads.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	closed, err := adsClient.Close(created.ID, 42)
	if err != nil || closed.Status != models.AdStatusClosed {
		t.Fatalf("Close failed: %+v err=%v", closed, err)
	}

	reopened, err := adsClient.Reopen(created.ID, 42)
	if err != nil || reopened.Status != models.AdStatusActive {
		t.Fatalf("Reopen failed: %+v err=%v", reopened, err)
	}

	// 3. Favorites via gRPC
	cnt, err := rawClient.Add(42, created.ID)
	if err != nil || cnt != 1 {
		t.Fatalf("Add favorite failed: cnt=%d err=%v", cnt, err)
	}
	ids := rawClient.ListAdIDs(42)
	if len(ids) != 1 || ids[0] != created.ID {
		t.Fatalf("ListAdIDs unexpected: %v", ids)
	}
	if rawClient.CountByUser(42) != 1 {
		t.Fatalf("CountByUser expected 1, got %d", rawClient.CountByUser(42))
	}
	if _, err := rawClient.Remove(42, created.ID); err != nil {
		t.Fatalf("Remove favorite failed: %v", err)
	}

	// 4. Delete
	if err := adsClient.Delete(created.ID, 42); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := adsClient.GetByID(created.ID); !errors.Is(err, ads.ErrAdNotFound) {
		t.Fatalf("expected ErrAdNotFound after delete, got %v", err)
	}

	// 5. Проверка Prometheus-метрик
	rec := httptest.NewRecorder()
	metricsReg.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	if !strings.Contains(body, `lavka_grpc_requests_total{service="catalog-service"`) {
		t.Fatalf("expected catalog-service metrics, got:\n%s", body)
	}
}
