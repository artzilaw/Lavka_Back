package httpapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lavka/internal/app/http/middleware"
	"lavka/internal/domain/models"
	"lavka/internal/pkg/jwt"
	"lavka/internal/pkg/responser"
	"lavka/internal/pkg/validator"
	"lavka/internal/storage/ads"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

var categoryNames = map[int64]string{
	1:  "Электроника",
	2:  "Авто и гаражи",
	3:  "Недвижимость",
	4:  "Одежда и обувь",
	5:  "Хобби и отдых",
	6:  "Для дома",
	7:  "Работа",
	8:  "Животные",
	9:  "Запчасти",
	10: "Детям",
	11: "Красота",
}

var allowedAdPhotoTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

type AdUpsertRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Price       int64    `json:"price"`
	CategoryID  int64    `json:"category_id"`
	Location    string   `json:"location"`
	Photos      []string `json:"photos"`
}

type AdValidationErrors struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Price       string `json:"price,omitempty"`
	CategoryID  string `json:"category_id,omitempty"`
	Location    string `json:"location,omitempty"`
	Error       string `json:"error,omitempty"`
}

type AdDetailResponse struct {
	models.Ad
	CategoryName string                    `json:"category_name,omitempty"`
	PriceHistory []models.PriceHistoryItem `json:"price_history,omitempty"`
	Seller       *PublicUserResponse       `json:"seller,omitempty"`
}

func (r AdDetailResponse) MarshalJSON() ([]byte, error) {
	type adAlias models.Ad
	return json.Marshal(struct {
		adAlias
		CategoryName string                    `json:"category_name,omitempty"`
		PriceHistory []models.PriceHistoryItem `json:"price_history,omitempty"`
		Seller       *PublicUserResponse       `json:"seller,omitempty"`
	}{
		adAlias:      adAlias(r.Ad),
		CategoryName: r.CategoryName,
		PriceHistory: r.PriceHistory,
		Seller:       r.Seller,
	})
}

func (r *AdDetailResponse) UnmarshalJSON(data []byte) error {
	type adAlias models.Ad
	aux := struct {
		*adAlias
		CategoryName string                    `json:"category_name,omitempty"`
		PriceHistory []models.PriceHistoryItem `json:"price_history,omitempty"`
		Seller       *PublicUserResponse       `json:"seller,omitempty"`
	}{
		adAlias: (*adAlias)(&r.Ad),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	r.CategoryName = aux.CategoryName
	r.PriceHistory = aux.PriceHistory
	r.Seller = aux.Seller
	return nil
}

func validateAdRequest(req *AdUpsertRequest) AdValidationErrors {
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.Location = strings.TrimSpace(req.Location)

	var errs AdValidationErrors
	if err := validator.ValidateAdTitle(req.Title); err != nil {
		errs.Title = err.Error()
	}
	if err := validator.ValidateAdDescription(req.Description); err != nil {
		errs.Description = err.Error()
	}
	if err := validator.ValidateAdPrice(req.Price); err != nil {
		errs.Price = err.Error()
	}
	if err := validator.ValidateAdCategory(req.CategoryID); err != nil {
		errs.CategoryID = err.Error()
	}
	if err := validator.ValidateAdLocation(req.Location); err != nil {
		errs.Location = err.Error()
	}
	return errs
}

func (e AdValidationErrors) hasErrors() bool {
	return e.Title != "" || e.Description != "" || e.Price != "" || e.CategoryID != "" || e.Location != ""
}

func (a *App) buildPublicSeller(r *http.Request, sellerID int64) *PublicUserResponse {
	if a.services.Auth == nil {
		return nil
	}
	u, err := a.services.Auth.GetProfile(r.Context(), sellerID)
	if err != nil {
		return nil
	}
	active, closed := 0, 0
	if a.services.Ads != nil {
		active, closed = a.services.Ads.CountBySeller(sellerID)
	}
	return &PublicUserResponse{
		ID:           u.ID,
		Name:         u.Name,
		AvatarPath:   u.AvatarPath,
		Rating:       u.Rating,
		ReviewsCount: u.ReviewsCount,
		AdsCount:     active + closed,
		CreatedAt:    u.CreatedAt,
	}
}

func (a *App) extractOptionalUserID(r *http.Request) int64 {
	cookie, err := r.Cookie("token")
	if err != nil || cookie.Value == "" {
		return 0
	}
	tok, err := jwt.ParseToken(cookie.Value, a.secret)
	if err != nil || !tok.Valid {
		return 0
	}
	claims, ok := tok.Claims.(jwtlib.MapClaims)
	if !ok {
		return 0
	}
	if uid, ok := claims["uid"].(float64); ok {
		return int64(uid)
	}
	return 0
}

func (a *App) handleGetAdByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		responser.RespondWithError(w, http.StatusBadRequest, "invalid ad id")
		return
	}

	ad, err := a.services.Ads.GetByID(id)
	if err != nil {
		if errors.Is(err, ads.ErrAdNotFound) {
			responser.RespondWithError(w, http.StatusNotFound, "ad not found")
			return
		}
		a.log.Error("failed to get ad", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	trackView := r.URL.Query().Get("track_view") != "false"
	if trackView {
		viewerID := a.extractOptionalUserID(r)
		// Не засчитываем просмотр от самого продавца
		if viewerID == 0 || viewerID != ad.SellerID {
			viewerKey := "ip:" + clientIPFromRequest(r)
			if viewerID > 0 {
				viewerKey = fmt.Sprintf("u:%d", viewerID)
			}
			if a.viewTracker == nil || a.viewTracker.shouldCount(viewerKey, ad.ID) {
				if updated, incErr := a.services.Ads.IncrementViews(id); incErr == nil {
					ad = updated
				}
			}
		}
	}

	seller := a.buildPublicSeller(r, ad.SellerID)
	priceHistory := a.services.Ads.GetPriceHistory(ad.ID)
	responser.RespondWithJSON(w, http.StatusOK, AdDetailResponse{
		Ad:           ad,
		CategoryName: categoryNames[ad.CategoryID],
		PriceHistory: priceHistory,
		Seller:       seller,
	})
}

func (a *App) handleGetMyAds(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	list := a.services.Ads.GetBySellerID(userID, true)
	responser.RespondWithJSON(w, http.StatusOK, list)
}

func (a *App) handleCreateAd(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	var req AdUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responser.RespondWithError(w, http.StatusBadRequest, ErrInvalidRequestBody)
		return
	}

	if vErrs := validateAdRequest(&req); vErrs.hasErrors() {
		vErrs.Error = "validation failed"
		responser.RespondWithJSON(w, http.StatusBadRequest, vErrs)
		return
	}

	photos := sanitizePhotos(req.Photos)
	created, err := a.services.Ads.Create(models.Ad{
		SellerID:    userID,
		CategoryID:  req.CategoryID,
		Title:       req.Title,
		Description: req.Description,
		Price:       req.Price,
		Location:    req.Location,
		Photos:      photos,
	})
	if err != nil {
		a.log.Error("failed to create ad", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusCreated, created)
}

func (a *App) handleUpdateAd(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		responser.RespondWithError(w, http.StatusBadRequest, "invalid ad id")
		return
	}

	var req AdUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responser.RespondWithError(w, http.StatusBadRequest, ErrInvalidRequestBody)
		return
	}

	if vErrs := validateAdRequest(&req); vErrs.hasErrors() {
		vErrs.Error = "validation failed"
		responser.RespondWithJSON(w, http.StatusBadRequest, vErrs)
		return
	}

	var photos []string
	if req.Photos != nil {
		photos = sanitizePhotos(req.Photos)
	}

	updated, err := a.services.Ads.Update(id, userID, models.Ad{
		CategoryID:  req.CategoryID,
		Title:       req.Title,
		Description: req.Description,
		Price:       req.Price,
		Location:    req.Location,
		Photos:      photos,
	})
	if err != nil {
		if errors.Is(err, ads.ErrAdNotFound) {
			responser.RespondWithError(w, http.StatusNotFound, "ad not found")
			return
		}
		if errors.Is(err, ads.ErrForbidden) {
			responser.RespondWithError(w, http.StatusForbidden, "forbidden")
			return
		}
		a.log.Error("failed to update ad", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusOK, updated)
}

func (a *App) handleCloseAd(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		responser.RespondWithError(w, http.StatusBadRequest, "invalid ad id")
		return
	}

	updated, err := a.services.Ads.Close(id, userID)
	if err != nil {
		if errors.Is(err, ads.ErrAdNotFound) {
			responser.RespondWithError(w, http.StatusNotFound, "ad not found")
			return
		}
		if errors.Is(err, ads.ErrForbidden) {
			responser.RespondWithError(w, http.StatusForbidden, "forbidden")
			return
		}
		a.log.Error("failed to close ad", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	if a.services.Cart != nil {
		_ = a.services.Cart.RemoveAdFromAllCarts(id)
	}

	responser.RespondWithJSON(w, http.StatusOK, updated)
}

func (a *App) handleReopenAd(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		responser.RespondWithError(w, http.StatusBadRequest, "invalid ad id")
		return
	}

	updated, err := a.services.Ads.Reopen(id, userID)
	if err != nil {
		if errors.Is(err, ads.ErrAdNotFound) {
			responser.RespondWithError(w, http.StatusNotFound, "ad not found")
			return
		}
		if errors.Is(err, ads.ErrForbidden) {
			responser.RespondWithError(w, http.StatusForbidden, "forbidden")
			return
		}
		a.log.Error("failed to reopen ad", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusOK, updated)
}

func (a *App) handleDeleteAd(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		responser.RespondWithError(w, http.StatusBadRequest, "invalid ad id")
		return
	}

	if err := a.services.Ads.Delete(id, userID); err != nil {
		if errors.Is(err, ads.ErrAdNotFound) {
			responser.RespondWithError(w, http.StatusNotFound, "ad not found")
			return
		}
		if errors.Is(err, ads.ErrForbidden) {
			responser.RespondWithError(w, http.StatusForbidden, "forbidden")
			return
		}
		a.log.Error("failed to delete ad", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	if a.services.Cart != nil {
		_ = a.services.Cart.RemoveAdFromAllCarts(id)
	}

	responser.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleUploadAdPhoto(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok {
		responser.RespondWithError(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize)
	if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
		responser.RespondWithError(w, http.StatusBadRequest, ErrFileTooBig)
		return
	}

	file, header, err := r.FormFile("photo")
	if err != nil {
		responser.RespondWithError(w, http.StatusBadRequest, ErrFailedToGetFile)
		return
	}
	defer file.Close()

	buff := make([]byte, 512)
	n, err := file.Read(buff)
	if err != nil && !errors.Is(err, io.EOF) {
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	contentType := http.DetectContentType(buff[:n])
	defaultExt, ok := allowedAdPhotoTypes[contentType]
	if !ok {
		responser.RespondWithError(w, http.StatusBadRequest, "unsupported file type")
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" && ext != ".gif" {
		ext = defaultExt
	}

	newFileName := fmt.Sprintf("ad_%d_%d%s", userID, time.Now().UnixNano(), ext)
	storageDir := filepath.Join("static", "img", "ads")
	storagePath := filepath.Join(storageDir, newFileName)
	publicURL := "/static/img/ads/" + newFileName

	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		a.log.Error("failed to create ad photo dir", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	dst, err := os.Create(storagePath)
	if err != nil {
		a.log.Error("failed to create ad photo file", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		_ = os.Remove(storagePath)
		a.log.Error("failed to save ad photo file", slog.String("error", err.Error()))
		responser.RespondWithError(w, http.StatusInternalServerError, ErrInternalError)
		return
	}

	responser.RespondWithJSON(w, http.StatusCreated, map[string]string{"url": publicURL})
}

func sanitizePhotos(raw []string) []string {
	result := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "/static/") || strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
			result = append(result, p)
		}
	}
	return result
}
