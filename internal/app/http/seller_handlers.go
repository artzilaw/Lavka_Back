package httpapp

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"lavka/internal/domain/models"
	"lavka/internal/pkg/responser"
	"lavka/internal/storage"
)

type SellerProfileResponse struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	AvatarPath     string    `json:"avatar_path"`
	Rating         float64   `json:"rating"`
	ReviewsCount   int       `json:"reviews_count"`
	AdsCount       int       `json:"ads_count"`
	ActiveAdsCount int       `json:"active_ads_count"`
	ClosedAdsCount int       `json:"closed_ads_count"`
	CreatedAt      time.Time `json:"created_at"`
}

type SellerPageResponse struct {
	Seller SellerProfileResponse `json:"seller"`
	Ads    []models.Ad           `json:"ads"`
}

func (a *App) handleGetSellerPage(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	sellerID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || sellerID <= 0 {
		responser.RespondWithError(w, http.StatusBadRequest, ErrInvalidUserID)
		return
	}

	user, err := a.services.Auth.GetProfile(r.Context(), sellerID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			responser.RespondWithError(w, http.StatusNotFound, ErrUserNotFound)
			return
		}
		a.log.Error("failed to get seller profile", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	sellerAds := a.services.Ads.GetBySellerID(sellerID, true)
	activeCount, closedCount := a.services.Ads.CountBySeller(sellerID)

	responser.RespondWithJSON(w, http.StatusOK, SellerPageResponse{
		Seller: SellerProfileResponse{
			ID:             user.ID,
			Name:           user.Name,
			AvatarPath:     user.AvatarPath,
			Rating:         user.Rating,
			ReviewsCount:   user.ReviewsCount,
			AdsCount:       activeCount + closedCount,
			ActiveAdsCount: activeCount,
			ClosedAdsCount: closedCount,
			CreatedAt:      user.CreatedAt,
		},
		Ads: sellerAds,
	})
}
