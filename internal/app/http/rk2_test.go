package httpapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"lavka/internal/app/http/middleware"
	"lavka/internal/domain/models"
	"lavka/internal/services/auth"
	"lavka/internal/storage"
	"lavka/internal/storage/ads"
	"lavka/internal/storage/blacklist"
	"lavka/internal/storage/cart"
)

func setupRK2TestApp(t *testing.T) *App {
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
	cartSvc, _ := cart.NewCartRepository("")

	return New(log, Services{Ads: adsSvc, Auth: authSvc, Cart: cartSvc}, bl, 8000, time.Hour, "test-secret")
}

type testSession struct {
	auth *http.Cookie
	csrf *http.Cookie
}

func (s testSession) apply(req *http.Request) {
	req.AddCookie(s.auth)
	req.AddCookie(s.csrf)
	req.Header.Set(middleware.CSRFHeaderName, s.csrf.Value)
}

func registerAndGetSession(t *testing.T, app *App, name, email, password string) testSession {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q,"email":%q,"password":%q}`, name, email, password)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("register %s failed: %d %s", email, rec.Code, rec.Body.String())
	}
	var sess testSession
	for _, c := range rec.Result().Cookies() {
		if c.Name == "token" {
			sess.auth = c
		}
		if c.Name == middleware.CSRFCookieName {
			sess.csrf = c
		}
	}
	if sess.auth == nil || sess.csrf == nil {
		t.Fatalf("expected auth and csrf cookies, got %+v", rec.Result().Cookies())
	}
	return sess
}

func TestHTTP_RK2_AdsCartAndSellerFlow(t *testing.T) {
	app := setupRK2TestApp(t)
	sellerSess := registerAndGetSession(t, app, "SellerOne", "seller1@lavka.ru", "Password123")
	buyerSess := registerAndGetSession(t, app, "BuyerOne", "buyer1@lavka.ru", "Password123")

	// 1. Upload ad photo
	var photoBuf bytes.Buffer
	mw := multipart.NewWriter(&photoBuf)
	fw, _ := mw.CreateFormFile("photo", "item.png")
	_, _ = fw.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D})
	_ = mw.Close()

	reqPhoto := httptest.NewRequest(http.MethodPost, "/api/v1/ads/photos", &photoBuf)
	reqPhoto.Header.Set("Content-Type", mw.FormDataContentType())
	sellerSess.apply(reqPhoto)
	recPhoto := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recPhoto, reqPhoto)
	if recPhoto.Code != http.StatusCreated {
		t.Fatalf("POST /ads/photos status = %d, body = %s", recPhoto.Code, recPhoto.Body.String())
	}
	var photoResp map[string]string
	_ = json.Unmarshal(recPhoto.Body.Bytes(), &photoResp)
	if photoResp["url"] == "" {
		t.Fatal("expected uploaded photo url")
	}

	// 2. Create ad
	createBody := fmt.Sprintf(`{"title":"PlayStation 5 Slim","description":"Отличная приставка с двумя геймпадами","price":54000,"category_id":1,"location":"Москва","photos":[%q]}`, photoResp["url"])
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/ads", bytes.NewBufferString(createBody))
	sellerSess.apply(reqCreate)
	recCreate := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("POST /ads status = %d, body = %s", recCreate.Code, recCreate.Body.String())
	}
	var createdAd models.Ad
	_ = json.Unmarshal(recCreate.Body.Bytes(), &createdAd)
	if createdAd.ID == 0 {
		t.Fatal("expected created ad ID")
	}

	// 3. Get ad by ID (increments views & includes seller)
	reqGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/ads/%d", createdAd.ID), nil)
	recGet := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("GET /ads/%d status = %d", createdAd.ID, recGet.Code)
	}
	var detail AdDetailResponse
	_ = json.Unmarshal(recGet.Body.Bytes(), &detail)
	if detail.ViewsCount != 1 || detail.Seller == nil || detail.Seller.Name != "SellerOne" {
		t.Fatalf("unexpected detail response: %+v", detail)
	}

	// 4. Update ad (forbidden for buyer, ok for seller)
	updateBody := `{"title":"PlayStation 5 Slim 1TB","description":"Обновлённое описание приставки","price":52000,"category_id":1,"location":"Москва"}`
	reqForbid := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/ads/%d", createdAd.ID), bytes.NewBufferString(updateBody))
	buyerSess.apply(reqForbid)
	recForbid := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recForbid, reqForbid)
	if recForbid.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for buyer updating seller ad, got %d", recForbid.Code)
	}

	reqUpd := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/ads/%d", createdAd.ID), bytes.NewBufferString(updateBody))
	sellerSess.apply(reqUpd)
	recUpd := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recUpd, reqUpd)
	if recUpd.Code != http.StatusOK {
		t.Fatalf("PUT /ads/%d status = %d, body = %s", createdAd.ID, recUpd.Code, recUpd.Body.String())
	}

	// 5. Close and reopen ad
	reqClose := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/ads/%d/close", createdAd.ID), nil)
	sellerSess.apply(reqClose)
	recClose := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recClose, reqClose)
	if recClose.Code != http.StatusOK {
		t.Fatalf("POST /ads/%d/close status = %d", createdAd.ID, recClose.Code)
	}

	reqReopen := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/ads/%d/reopen", createdAd.ID), nil)
	sellerSess.apply(reqReopen)
	recReopen := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recReopen, reqReopen)
	if recReopen.Code != http.StatusOK {
		t.Fatalf("POST /ads/%d/reopen status = %d", createdAd.ID, recReopen.Code)
	}

	// 6. Seller page
	reqSeller := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/sellers/%d", createdAd.SellerID), nil)
	recSeller := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recSeller, reqSeller)
	if recSeller.Code != http.StatusOK {
		t.Fatalf("GET /sellers/%d status = %d, body = %s", createdAd.SellerID, recSeller.Code, recSeller.Body.String())
	}
	var sellerPage SellerPageResponse
	_ = json.Unmarshal(recSeller.Body.Bytes(), &sellerPage)
	if sellerPage.Seller.Name != "SellerOne" || len(sellerPage.Ads) != 1 {
		t.Fatalf("unexpected seller page: %+v", sellerPage)
	}

	// 7. Cart: seller cannot add own ad, buyer can add, update quantity, checkout
	addBody := fmt.Sprintf(`{"ad_id":%d,"quantity":1}`, createdAd.ID)
	reqOwnCart := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewBufferString(addBody))
	sellerSess.apply(reqOwnCart)
	recOwnCart := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recOwnCart, reqOwnCart)
	if recOwnCart.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when adding own ad to cart, got %d", recOwnCart.Code)
	}

	reqAddCart := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewBufferString(addBody))
	buyerSess.apply(reqAddCart)
	recAddCart := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recAddCart, reqAddCart)
	if recAddCart.Code != http.StatusOK {
		t.Fatalf("POST /cart/items status = %d, body = %s", recAddCart.Code, recAddCart.Body.String())
	}

	reqQty := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/cart/items/%d", createdAd.ID), bytes.NewBufferString(`{"quantity":2}`))
	buyerSess.apply(reqQty)
	recQty := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recQty, reqQty)
	if recQty.Code != http.StatusOK {
		t.Fatalf("PATCH /cart/items/%d status = %d", createdAd.ID, recQty.Code)
	}
	var cartResp CartResponse
	_ = json.Unmarshal(recQty.Body.Bytes(), &cartResp)
	if cartResp.TotalItems != 2 || cartResp.TotalPrice != 104000 {
		t.Fatalf("unexpected cart totals: %+v", cartResp)
	}

	// 8. Checkout
	checkoutBody := `{"delivery_method":"delivery","delivery_address":"Москва, ул. Тверская, д. 10","payment_method":"card","phone":"+7 (999) 123-45-67","comment":"Позвоните за час"}`
	reqCheckout := httptest.NewRequest(http.MethodPost, "/api/v1/cart/checkout", bytes.NewBufferString(checkoutBody))
	buyerSess.apply(reqCheckout)
	recCheckout := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recCheckout, reqCheckout)
	if recCheckout.Code != http.StatusCreated {
		t.Fatalf("POST /cart/checkout status = %d, body = %s", recCheckout.Code, recCheckout.Body.String())
	}

	// Verify orders history
	reqOrders := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	buyerSess.apply(reqOrders)
	recOrders := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recOrders, reqOrders)
	if recOrders.Code != http.StatusOK {
		t.Fatalf("GET /orders status = %d", recOrders.Code)
	}
	var orders []models.Order
	_ = json.Unmarshal(recOrders.Body.Bytes(), &orders)
	if len(orders) != 1 || orders[0].TotalPrice != 104000 {
		t.Fatalf("unexpected orders: %+v", orders)
	}

	// 9. Delete ad
	reqDel := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/ads/%d", createdAd.ID), nil)
	sellerSess.apply(reqDel)
	recDel := httptest.NewRecorder()
	app.srv.Handler.ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Fatalf("DELETE /ads/%d status = %d", createdAd.ID, recDel.Code)
	}
}
