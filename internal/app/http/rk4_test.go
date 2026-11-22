package httpapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"lavka/internal/domain/models"
	"lavka/internal/storage/wallet"
)

func TestHTTP_RK4_PriceHistoryWalletAndPromotion(t *testing.T) {
	app := setupRK2TestApp(t)
	app.services.Wallet = wallet.NewInMemoryRepository()

	sess := registerAndGetSession(t, app, "PromoSeller", "promo@lavka.ru", "Password123")

	// 1. Создаём объявление с начальной ценой 50000
	createBody := []byte(`{"title":"Игровой ноутбук","description":"Мощный ноутбук для игр и работы","price":50000,"category_id":1,"location":"Москва","photos":[]}`)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/ads", bytes.NewReader(createBody))
	sess.apply(reqCreate)
	recCreate := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("POST /ads status = %d, body = %s", recCreate.Code, recCreate.Body.String())
	}
	var created models.Ad
	_ = json.Unmarshal(recCreate.Body.Bytes(), &created)

	// 2. Меняем цену на 45000 -> должна появиться вторая запись в истории цены
	updateBody := []byte(`{"title":"Игровой ноутбук","description":"Мощный ноутбук для игр и работы","price":45000,"category_id":1,"location":"Москва","photos":[]}`)
	reqUpd := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/ads/%d", created.ID), bytes.NewReader(updateBody))
	sess.apply(reqUpd)
	recUpd := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recUpd, reqUpd)
	if recUpd.Code != http.StatusOK {
		t.Fatalf("PUT /ads/%d status = %d, body = %s", created.ID, recUpd.Code, recUpd.Body.String())
	}

	// 3. Проверяем GET /api/v1/ads/{id}/price-history
	reqHist := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/ads/%d/price-history", created.ID), nil)
	recHist := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recHist, reqHist)
	if recHist.Code != http.StatusOK {
		t.Fatalf("GET /price-history status = %d", recHist.Code)
	}
	var history []models.PriceHistoryItem
	_ = json.Unmarshal(recHist.Body.Bytes(), &history)
	if len(history) != 2 || history[0].NewPrice != 50000 || history[1].OldPrice != 50000 || history[1].NewPrice != 45000 {
		t.Fatalf("unexpected price history: %+v", history)
	}

	// 4. Проверяем тарифы продвижения
	reqTariffs := httptest.NewRequest(http.MethodGet, "/api/v1/promotion/tariffs", nil)
	recTariffs := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recTariffs, reqTariffs)
	if recTariffs.Code != http.StatusOK {
		t.Fatalf("GET /promotion/tariffs status = %d", recTariffs.Code)
	}

	// 5. Проверяем баланс и моковое пополнение кошелька
	reqTopUp := httptest.NewRequest(http.MethodPost, "/api/v1/wallet/topup", bytes.NewReader([]byte(`{"amount":500}`)))
	sess.apply(reqTopUp)
	recTopUp := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recTopUp, reqTopUp)
	if recTopUp.Code != http.StatusOK {
		t.Fatalf("POST /wallet/topup status = %d, body = %s", recTopUp.Code, recTopUp.Body.String())
	}

	// 6. Покупаем VIP-продвижение (599 ₽)
	reqPromo := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/ads/%d/promote", created.ID), bytes.NewReader([]byte(`{"tier":"vip"}`)))
	sess.apply(reqPromo)
	recPromo := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recPromo, reqPromo)
	if recPromo.Code != http.StatusOK {
		t.Fatalf("POST /ads/%d/promote status = %d, body = %s", created.ID, recPromo.Code, recPromo.Body.String())
	}

	// 7. Проверяем, что VIP-объявление поднялось на 1-е место в выдаче GET /api/v1/ads
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/ads", nil)
	recList := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recList, reqList)
	var list []models.Ad
	_ = json.Unmarshal(recList.Body.Bytes(), &list)
	if len(list) == 0 || list[0].ID != created.ID || list[0].PromotionTier != models.PromoTierVIP {
		t.Fatalf("expected promoted VIP ad at top of feed, got first=%+v", list[0])
	}

	// 8. Проверяем GET /api/v1/wallet (должно быть 2 транзакции: topup + promotion)
	reqWallet := httptest.NewRequest(http.MethodGet, "/api/v1/wallet", nil)
	sess.apply(reqWallet)
	recWallet := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recWallet, reqWallet)
	if recWallet.Code != http.StatusOK {
		t.Fatalf("GET /wallet status = %d", recWallet.Code)
	}
	var wResp WalletResponse
	_ = json.Unmarshal(recWallet.Body.Bytes(), &wResp)
	if wResp.Balance != 1000+500-599 || len(wResp.Transactions) != 2 {
		t.Fatalf("unexpected wallet state: %+v", wResp)
	}
}
