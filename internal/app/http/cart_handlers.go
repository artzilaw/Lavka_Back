package httpapp

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lavka/internal/app/http/middleware"
	"lavka/internal/domain/models"
	"lavka/internal/pkg/responser"
	"lavka/internal/pkg/validator"
	"lavka/internal/storage/ads"
)

type AddCartItemRequest struct {
	AdID     int64 `json:"ad_id"`
	Quantity int   `json:"quantity"`
}

type UpdateCartItemRequest struct {
	Quantity int `json:"quantity"`
}

type CheckoutRequest struct {
	DeliveryMethod  string `json:"delivery_method"`
	DeliveryAddress string `json:"delivery_address"`
	PaymentMethod   string `json:"payment_method"`
	Phone           string `json:"phone"`
	Comment         string `json:"comment"`
}

type CheckoutValidationErrors struct {
	DeliveryAddress string `json:"delivery_address,omitempty"`
	Phone           string `json:"phone,omitempty"`
	Error           string `json:"error,omitempty"`
}

type CartItemView struct {
	AdID       int64     `json:"ad_id"`
	Quantity   int       `json:"quantity"`
	AddedAt    time.Time `json:"added_at"`
	Ad         models.Ad `json:"ad"`
	SellerName string    `json:"seller_name"`
	Available  bool      `json:"available"`
}

type CartResponse struct {
	Items      []CartItemView `json:"items"`
	TotalItems int            `json:"total_items"`
	TotalPrice int64          `json:"total_price"`
}

func (a *App) buildCartResponse(r *http.Request, userID int64) CartResponse {
	rawItems := a.services.Cart.GetCart(userID)
	views := make([]CartItemView, 0, len(rawItems))
	totalItems := 0
	var totalPrice int64

	for _, item := range rawItems {
		ad, err := a.services.Ads.GetByID(item.AdID)
		if err != nil {
			continue
		}
		available := ad.Status == models.AdStatusActive
		sellerName := fmtSellerName(r, a, ad.SellerID)
		views = append(views, CartItemView{
			AdID:       item.AdID,
			Quantity:   item.Quantity,
			AddedAt:    item.AddedAt,
			Ad:         ad,
			SellerName: sellerName,
			Available:  available,
		})
		totalItems += item.Quantity
		if available {
			totalPrice += ad.Price * int64(item.Quantity)
		}
	}

	return CartResponse{
		Items:      views,
		TotalItems: totalItems,
		TotalPrice: totalPrice,
	}
}

func fmtSellerName(r *http.Request, a *App, sellerID int64) string {
	if a.services.Auth == nil {
		return "Продавец"
	}
	u, err := a.services.Auth.GetProfile(r.Context(), sellerID)
	if err != nil || strings.TrimSpace(u.Name) == "" {
		return "Продавец"
	}
	return u.Name
}

func (a *App) handleGetCart(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	if a.services.Cart == nil {
		responser.RespondWithJSON(w, http.StatusOK, CartResponse{Items: []CartItemView{}})
		return
	}

	responser.RespondWithJSON(w, http.StatusOK, a.buildCartResponse(r, userID))
}

func (a *App) handleAddCartItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	var req AddCartItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AdID <= 0 {
		responser.RespondWithError(w, http.StatusBadRequest, ErrInvalidRequestBody)
		return
	}

	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	ad, err := a.services.Ads.GetByID(req.AdID)
	if err != nil {
		if errors.Is(err, ads.ErrAdNotFound) {
			responser.RespondWithError(w, http.StatusNotFound, "ad not found")
			return
		}
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	if ad.Status != models.AdStatusActive {
		responser.RespondWithError(w, http.StatusBadRequest, "ad is not active")
		return
	}

	if ad.SellerID == userID {
		responser.RespondWithError(w, http.StatusBadRequest, "cannot add own ad to cart")
		return
	}

	if _, err := a.services.Cart.AddItem(userID, req.AdID, req.Quantity); err != nil {
		a.log.Error("failed to add item to cart", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusOK, a.buildCartResponse(r, userID))
}

func (a *App) handleUpdateCartItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	adID, err := strconv.ParseInt(r.PathValue("adId"), 10, 64)
	if err != nil || adID <= 0 {
		responser.RespondWithError(w, http.StatusBadRequest, "invalid ad id")
		return
	}

	var req UpdateCartItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responser.RespondWithError(w, http.StatusBadRequest, ErrInvalidRequestBody)
		return
	}

	if req.Quantity > 99 {
		req.Quantity = 99
	}

	if _, err := a.services.Cart.UpdateItemQuantity(userID, adID, req.Quantity); err != nil {
		a.log.Error("failed to update cart item", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusOK, a.buildCartResponse(r, userID))
}

func (a *App) handleRemoveCartItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	adID, err := strconv.ParseInt(r.PathValue("adId"), 10, 64)
	if err != nil || adID <= 0 {
		responser.RespondWithError(w, http.StatusBadRequest, "invalid ad id")
		return
	}

	if _, err := a.services.Cart.RemoveItem(userID, adID); err != nil {
		a.log.Error("failed to remove cart item", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusOK, a.buildCartResponse(r, userID))
}

func (a *App) handleClearCart(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	if err := a.services.Cart.ClearCart(userID); err != nil {
		a.log.Error("failed to clear cart", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusOK, CartResponse{Items: []CartItemView{}})
}

func (a *App) handleCheckout(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	var req CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responser.RespondWithError(w, http.StatusBadRequest, ErrInvalidRequestBody)
		return
	}

	req.DeliveryMethod = strings.TrimSpace(req.DeliveryMethod)
	if req.DeliveryMethod != "pickup" {
		req.DeliveryMethod = "delivery"
	}
	req.PaymentMethod = strings.TrimSpace(req.PaymentMethod)
	if req.PaymentMethod != "cash" {
		req.PaymentMethod = "card"
	}
	req.DeliveryAddress = strings.TrimSpace(req.DeliveryAddress)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Comment = strings.TrimSpace(req.Comment)

	var vErrs CheckoutValidationErrors
	if req.DeliveryMethod == "delivery" {
		if err := validator.ValidateDeliveryAddress(req.DeliveryAddress); err != nil {
			vErrs.DeliveryAddress = err.Error()
		}
	}
	if err := validator.ValidatePhone(req.Phone); err != nil {
		vErrs.Phone = err.Error()
	}
	if vErrs.DeliveryAddress != "" || vErrs.Phone != "" {
		vErrs.Error = "validation failed"
		responser.RespondWithJSON(w, http.StatusBadRequest, vErrs)
		return
	}

	cartItems := a.services.Cart.GetCart(userID)
	if len(cartItems) == 0 {
		responser.RespondWithError(w, http.StatusBadRequest, "cart is empty")
		return
	}

	orderItems := make([]models.OrderItem, 0, len(cartItems))
	soldAdIDs := make([]int64, 0, len(cartItems))
	var totalPrice int64

	for _, item := range cartItems {
		ad, err := a.services.Ads.GetByID(item.AdID)
		if err != nil || ad.Status != models.AdStatusActive {
			responser.RespondWithError(w, http.StatusBadRequest, "some items in cart are no longer available")
			return
		}
		photo := ""
		if len(ad.Photos) > 0 {
			photo = ad.Photos[0]
		}
		orderItems = append(orderItems, models.OrderItem{
			AdID:       ad.ID,
			SellerID:   ad.SellerID,
			SellerName: fmtSellerName(r, a, ad.SellerID),
			Title:      ad.Title,
			Price:      ad.Price,
			Quantity:   item.Quantity,
			Photo:      photo,
		})
		soldAdIDs = append(soldAdIDs, ad.ID)
		totalPrice += ad.Price * int64(item.Quantity)
	}

	order, err := a.services.Cart.CreateOrder(models.Order{
		BuyerID:         userID,
		Items:           orderItems,
		TotalPrice:      totalPrice,
		DeliveryMethod:  req.DeliveryMethod,
		DeliveryAddress: req.DeliveryAddress,
		PaymentMethod:   req.PaymentMethod,
		Phone:           req.Phone,
		Comment:         req.Comment,
	})
	if err != nil {
		a.log.Error("failed to create order", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	_ = a.services.Ads.MarkSold(soldAdIDs)
	for _, adID := range soldAdIDs {
		_ = a.services.Cart.RemoveAdFromAllCarts(adID)
	}
	_ = a.services.Cart.ClearCart(userID)

	responser.RespondWithJSON(w, http.StatusCreated, order)
}

func (a *App) handleGetOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	if a.services.Cart == nil {
		responser.RespondWithJSON(w, http.StatusOK, []models.Order{})
		return
	}

	orders := a.services.Cart.GetOrdersByBuyer(userID)
	responser.RespondWithJSON(w, http.StatusOK, orders)
}
