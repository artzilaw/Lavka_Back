package httpapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lavka/internal/domain/models"
	"lavka/internal/storage/favorites"
)

func TestHTTP_RK3_SearchFavoritesAndMetrics(t *testing.T) {
	app := setupRK2TestApp(t)
	app.services.Favorites = favorites.NewInMemoryRepository()

	sess := registerAndGetSession(t, app, "FavUser", "fav@lavka.ru", "Password123")

	// 1. Search with query, price range and sort
	reqSearch := httptest.NewRequest(http.MethodGet, "/api/v1/ads?q=MacBook&min_price=100000&sort=price_desc", nil)
	recSearch := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recSearch, reqSearch)
	if recSearch.Code != http.StatusOK {
		t.Fatalf("GET /ads search status = %d", recSearch.Code)
	}
	var found []models.Ad
	_ = json.Unmarshal(recSearch.Body.Bytes(), &found)
	if len(found) != 1 || found[0].ID != 1 {
		t.Fatalf("unexpected search results: %+v", found)
	}

	// 2. Add to favorites
	reqAddFav := httptest.NewRequest(http.MethodPost, "/api/v1/favorites/1", nil)
	sess.apply(reqAddFav)
	recAddFav := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recAddFav, reqAddFav)
	if recAddFav.Code != http.StatusOK {
		t.Fatalf("POST /favorites/1 status = %d, body = %s", recAddFav.Code, recAddFav.Body.String())
	}

	// 3. List favorites
	reqListFav := httptest.NewRequest(http.MethodGet, "/api/v1/favorites", nil)
	sess.apply(reqListFav)
	recListFav := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recListFav, reqListFav)
	if recListFav.Code != http.StatusOK {
		t.Fatalf("GET /favorites status = %d", recListFav.Code)
	}
	var favAds []models.Ad
	_ = json.Unmarshal(recListFav.Body.Bytes(), &favAds)
	if len(favAds) != 1 || favAds[0].ID != 1 {
		t.Fatalf("unexpected favorites: %+v", favAds)
	}

	// 4. Remove from favorites
	reqDelFav := httptest.NewRequest(http.MethodDelete, "/api/v1/favorites/1", nil)
	sess.apply(reqDelFav)
	recDelFav := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recDelFav, reqDelFav)
	if recDelFav.Code != http.StatusOK {
		t.Fatalf("DELETE /favorites/1 status = %d", recDelFav.Code)
	}

	// 5. Prometheus /metrics endpoint
	reqMetrics := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recMetrics := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recMetrics, reqMetrics)
	if recMetrics.Code != http.StatusOK {
		t.Fatalf("GET /metrics status = %d", recMetrics.Code)
	}
	if !strings.Contains(recMetrics.Body.String(), "lavka_http_requests_total") {
		t.Fatalf("expected prometheus metrics, got: %s", recMetrics.Body.String())
	}
}
