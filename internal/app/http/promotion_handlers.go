package httpapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	"lavka/internal/app/http/middleware"
	"lavka/internal/domain/models"
	"lavka/internal/pkg/responser"
	"lavka/internal/storage/ads"
	"lavka/internal/storage/wallet"
)

type WalletTopUpRequest struct {
	Amount int64 `json:"amount"`
}

type PromoteAdRequest struct {
	Tier string `json:"tier"`
}

type WalletResponse struct {
	Balance      int64                      `json:"balance"`
	Transactions []models.WalletTransaction `json:"transactions"`
}

func (a *App) handleGetPriceHistory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		responser.RespondWithError(w, http.StatusBadRequest, "invalid ad id")
		return
	}

	if _, err := a.services.Ads.GetByID(id); err != nil {
		if errors.Is(err, ads.ErrAdNotFound) {
			responser.RespondWithError(w, http.StatusNotFound, "ad not found")
			return
		}
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	history := a.services.Ads.GetPriceHistory(id)
	responser.RespondWithJSON(w, http.StatusOK, history)
}

func (a *App) handleGetPromotionTariffs(w http.ResponseWriter, r *http.Request) {
	list := make([]models.PromotionTariff, 0, len(models.PromotionTariffs))
	for _, t := range models.PromotionTariffs {
		list = append(list, t)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Priority < list[j].Priority })
	responser.RespondWithJSON(w, http.StatusOK, list)
}

func (a *App) handleGetWallet(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}
	if a.services.Wallet == nil {
		responser.RespondWithJSON(w, http.StatusOK, WalletResponse{Balance: 0, Transactions: []models.WalletTransaction{}})
		return
	}

	bal, err := a.services.Wallet.GetBalance(userID)
	if err != nil {
		a.log.Error("failed to get wallet balance", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}
	txs, err := a.services.Wallet.ListTransactions(userID)
	if err != nil {
		txs = []models.WalletTransaction{}
	}

	responser.RespondWithJSON(w, http.StatusOK, WalletResponse{
		Balance:      bal,
		Transactions: txs,
	})
}

func (a *App) handleWalletTopUp(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}
	if a.services.Wallet == nil {
		responser.RespondWithError(w, http.StatusServiceUnavailable, "wallet unavailable")
		return
	}

	var req WalletTopUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responser.RespondWithError(w, http.StatusBadRequest, ErrInvalidRequestBody)
		return
	}

	if req.Amount < 100 || req.Amount > 100000 {
		responser.RespondWithError(w, http.StatusBadRequest, "amount must be between 100 and 100000")
		return
	}

	bal, tx, err := a.services.Wallet.TopUp(userID, req.Amount, fmt.Sprintf("Демо-пополнение кошелька на %d ₽", req.Amount))
	if err != nil {
		a.log.Error("failed to top up wallet", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusOK, map[string]any{
		"balance":     bal,
		"transaction": tx,
	})
}

func (a *App) handlePromoteAd(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}
	if a.services.Wallet == nil {
		responser.RespondWithError(w, http.StatusServiceUnavailable, "wallet unavailable")
		return
	}

	adID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || adID <= 0 {
		responser.RespondWithError(w, http.StatusBadRequest, "invalid ad id")
		return
	}

	var req PromoteAdRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responser.RespondWithError(w, http.StatusBadRequest, ErrInvalidRequestBody)
		return
	}

	tariff, exists := models.PromotionTariffs[req.Tier]
	if !exists {
		responser.RespondWithError(w, http.StatusBadRequest, "unknown promotion tier")
		return
	}

	ad, err := a.services.Ads.GetByID(adID)
	if err != nil {
		if errors.Is(err, ads.ErrAdNotFound) {
			responser.RespondWithError(w, http.StatusNotFound, "ad not found")
			return
		}
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	if ad.SellerID != userID {
		responser.RespondWithError(w, http.StatusForbidden, "forbidden")
		return
	}

	desc := fmt.Sprintf("Продвижение «%s» для объявления №%d", tariff.Title, adID)
	newBalance, tx, err := a.services.Wallet.Charge(userID, tariff.Price, desc)
	if err != nil {
		if errors.Is(err, wallet.ErrInsufficientFunds) {
			responser.RespondWithError(w, http.StatusBadRequest, "insufficient funds")
			return
		}
		a.log.Error("failed to charge wallet", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	until := time.Now().Add(tariff.Duration)
	promotedAd, err := a.services.Ads.Promote(adID, userID, tariff.Tier, tariff.Priority, until)
	if err != nil {
		// Откатываем списание, если не удалось активировать продвижение
		_, _, _ = a.services.Wallet.TopUp(userID, tariff.Price, fmt.Sprintf("Возврат средств за объявление №%d", adID))
		a.log.Error("failed to promote ad", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusOK, map[string]any{
		"ad":          promotedAd,
		"balance":     newBalance,
		"transaction": tx,
	})
}
