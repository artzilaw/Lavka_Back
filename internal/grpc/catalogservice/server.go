package catalogservice

import (
	"context"
	"errors"
	"time"

	"lavka/internal/domain/models"
	"lavka/internal/storage/ads"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AdsStore interface {
	GetAll() []models.Ad
	Search(filter models.AdFilter) []models.Ad
	GetByID(id int64) (models.Ad, error)
	IncrementViews(id int64) (models.Ad, error)
	GetBySellerID(sellerID int64, includeClosed bool) []models.Ad
	CountBySeller(sellerID int64) (active int, closed int)
	Create(ad models.Ad) (models.Ad, error)
	Update(id int64, sellerID int64, patch models.Ad) (models.Ad, error)
	Close(id int64, sellerID int64) (models.Ad, error)
	Reopen(id int64, sellerID int64) (models.Ad, error)
	MarkSold(ids []int64) error
	Delete(id int64, sellerID int64) error
	GetPriceHistory(adID int64) []models.PriceHistoryItem
	Promote(id int64, sellerID int64, tier string, priority int, until time.Time) (models.Ad, error)
}

type FavoritesStore interface {
	Add(userID, adID int64) (int64, error)
	Remove(userID, adID int64) (int64, error)
	ListAdIDs(userID int64) []int64
	CountByUser(userID int64) int
}

type Server struct {
	adsStore AdsStore
	favStore FavoritesStore
}

func NewServer(adsStore AdsStore, favStore FavoritesStore) *Server {
	return &Server{
		adsStore: adsStore,
		favStore: favStore,
	}
}

func mapCatalogError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ads.ErrAdNotFound):
		return status.Error(codes.NotFound, ads.ErrAdNotFound.Error())
	case errors.Is(err, ads.ErrForbidden):
		return status.Error(codes.PermissionDenied, ads.ErrForbidden.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func (s *Server) GetAllAds(ctx context.Context, _ *EmptyRequest) (*AdsListResponse, error) {
	return &AdsListResponse{Ads: s.adsStore.GetAll()}, nil
}

func (s *Server) SearchAds(ctx context.Context, req *SearchAdsRequest) (*AdsListResponse, error) {
	return &AdsListResponse{Ads: s.adsStore.Search(req.Filter)}, nil
}

func (s *Server) GetAdByID(ctx context.Context, req *AdIDRequest) (*AdResponse, error) {
	ad, err := s.adsStore.GetByID(req.ID)
	if err != nil {
		return nil, mapCatalogError(err)
	}
	return &AdResponse{Ad: ad}, nil
}

func (s *Server) IncrementViews(ctx context.Context, req *AdIDRequest) (*AdResponse, error) {
	ad, err := s.adsStore.IncrementViews(req.ID)
	if err != nil {
		return nil, mapCatalogError(err)
	}
	return &AdResponse{Ad: ad}, nil
}

func (s *Server) GetAdsBySeller(ctx context.Context, req *SellerAdsRequest) (*AdsListResponse, error) {
	return &AdsListResponse{Ads: s.adsStore.GetBySellerID(req.SellerID, req.IncludeClosed)}, nil
}

func (s *Server) CountAdsBySeller(ctx context.Context, req *SellerIDRequest) (*SellerAdsCountResponse, error) {
	active, closed := s.adsStore.CountBySeller(req.SellerID)
	return &SellerAdsCountResponse{Active: active, Closed: closed}, nil
}

func (s *Server) CreateAd(ctx context.Context, req *CreateAdRequest) (*AdResponse, error) {
	ad, err := s.adsStore.Create(req.Ad)
	if err != nil {
		return nil, mapCatalogError(err)
	}
	return &AdResponse{Ad: ad}, nil
}

func (s *Server) UpdateAd(ctx context.Context, req *UpdateAdRequest) (*AdResponse, error) {
	ad, err := s.adsStore.Update(req.ID, req.SellerID, req.Patch)
	if err != nil {
		return nil, mapCatalogError(err)
	}
	return &AdResponse{Ad: ad}, nil
}

func (s *Server) CloseAd(ctx context.Context, req *SellerAdActionRequest) (*AdResponse, error) {
	ad, err := s.adsStore.Close(req.ID, req.SellerID)
	if err != nil {
		return nil, mapCatalogError(err)
	}
	return &AdResponse{Ad: ad}, nil
}

func (s *Server) ReopenAd(ctx context.Context, req *SellerAdActionRequest) (*AdResponse, error) {
	ad, err := s.adsStore.Reopen(req.ID, req.SellerID)
	if err != nil {
		return nil, mapCatalogError(err)
	}
	return &AdResponse{Ad: ad}, nil
}

func (s *Server) MarkSold(ctx context.Context, req *MarkSoldRequest) (*EmptyResponse, error) {
	if err := s.adsStore.MarkSold(req.IDs); err != nil {
		return nil, mapCatalogError(err)
	}
	return &EmptyResponse{}, nil
}

func (s *Server) DeleteAd(ctx context.Context, req *SellerAdActionRequest) (*EmptyResponse, error) {
	if err := s.adsStore.Delete(req.ID, req.SellerID); err != nil {
		return nil, mapCatalogError(err)
	}
	return &EmptyResponse{}, nil
}

func (s *Server) GetPriceHistory(ctx context.Context, req *AdIDRequest) (*PriceHistoryResponse, error) {
	return &PriceHistoryResponse{Items: s.adsStore.GetPriceHistory(req.ID)}, nil
}

func (s *Server) PromoteAd(ctx context.Context, req *PromoteAdRequest) (*AdResponse, error) {
	ad, err := s.adsStore.Promote(req.ID, req.SellerID, req.Tier, req.Priority, time.Unix(req.UntilUnix, 0))
	if err != nil {
		return nil, mapCatalogError(err)
	}
	return &AdResponse{Ad: ad}, nil
}

func (s *Server) AddFavorite(ctx context.Context, req *FavoriteRequest) (*FavoriteCountResponse, error) {
	cnt, err := s.favStore.Add(req.UserID, req.AdID)
	if err != nil {
		return nil, mapCatalogError(err)
	}
	return &FavoriteCountResponse{Count: cnt}, nil
}

func (s *Server) RemoveFavorite(ctx context.Context, req *FavoriteRequest) (*FavoriteCountResponse, error) {
	cnt, err := s.favStore.Remove(req.UserID, req.AdID)
	if err != nil {
		return nil, mapCatalogError(err)
	}
	return &FavoriteCountResponse{Count: cnt}, nil
}

func (s *Server) ListFavoriteAdIDs(ctx context.Context, req *UserIDRequest) (*FavoriteIDsResponse, error) {
	return &FavoriteIDsResponse{AdIDs: s.favStore.ListAdIDs(req.UserID)}, nil
}

func (s *Server) CountFavoritesByUser(ctx context.Context, req *UserIDRequest) (*FavoriteCountResponse, error) {
	return &FavoriteCountResponse{Count: int64(s.favStore.CountByUser(req.UserID))}, nil
}
