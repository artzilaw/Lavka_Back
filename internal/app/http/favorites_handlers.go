package httpapp

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"lavka/internal/app/http/middleware"
	"lavka/internal/domain/models"
	"lavka/internal/pkg/responser"
	"lavka/internal/storage/ads"
)

func (a *App) handleGetFavorites(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	if a.services.Favorites == nil {
		responser.RespondWithJSON(w, http.StatusOK, []models.Ad{})
		return
	}

	adIDs := a.services.Favorites.ListAdIDs(userID)
	result := make([]models.Ad, 0, len(adIDs))
	for _, adID := range adIDs {
		ad, err := a.services.Ads.GetByID(adID)
		if err != nil {
			continue
		}
		result = append(result, ad)
	}

	responser.RespondWithJSON(w, http.StatusOK, result)
}

func (a *App) handleAddFavorite(w http.ResponseWriter, r *http.Request) {
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

	if _, err := a.services.Ads.GetByID(adID); err != nil {
		if errors.Is(err, ads.ErrAdNotFound) {
			responser.RespondWithError(w, http.StatusNotFound, "ad not found")
			return
		}
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	if a.services.Favorites == nil {
		responser.RespondWithJSON(w, http.StatusOK, map[string]any{"status": "ok", "favorites_count": 0})
		return
	}

	count, err := a.services.Favorites.Add(userID, adID)
	if err != nil {
		a.log.Error("failed to add favorite", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"ad_id":           adID,
		"favorites_count": count,
	})
}

func (a *App) handleRemoveFavorite(w http.ResponseWriter, r *http.Request) {
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

	if a.services.Favorites == nil {
		responser.RespondWithJSON(w, http.StatusOK, map[string]any{"status": "ok", "favorites_count": 0})
		return
	}

	count, err := a.services.Favorites.Remove(userID, adID)
	if err != nil {
		a.log.Error("failed to remove favorite", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"ad_id":           adID,
		"favorites_count": count,
	})
}
