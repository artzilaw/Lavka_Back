package catalogservice

import (
	"context"
	"time"

	"lavka/internal/domain/models"
	"lavka/internal/grpc/codec"
	"lavka/internal/storage/ads"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const defaultTimeout = 5 * time.Second

// Client реализует интерфейсы httpapp.Ads и httpapp.Favorites поверх gRPC.
type Client struct {
	conn *grpc.ClientConn
}

func NewClient(target string) (*Client, error) {
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(codec.Name)),
	)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

func NewClientWithConn(conn *grpc.ClientConn) *Client {
	return &Client{conn: conn}
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) invoke(method string, in, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	return c.conn.Invoke(ctx, "/"+ServiceName+"/"+method, in, out, grpc.CallContentSubtype(codec.Name))
}

func unmapCatalogError(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch st.Code() {
	case codes.NotFound:
		return ads.ErrAdNotFound
	case codes.PermissionDenied:
		return ads.ErrForbidden
	default:
		return err
	}
}

func (c *Client) GetAll() []models.Ad {
	var resp AdsListResponse
	if err := c.invoke("GetAllAds", &EmptyRequest{}, &resp); err != nil {
		return []models.Ad{}
	}
	if resp.Ads == nil {
		return []models.Ad{}
	}
	return resp.Ads
}

func (c *Client) Search(filter models.AdFilter) []models.Ad {
	var resp AdsListResponse
	if err := c.invoke("SearchAds", &SearchAdsRequest{Filter: filter}, &resp); err != nil {
		return []models.Ad{}
	}
	if resp.Ads == nil {
		return []models.Ad{}
	}
	return resp.Ads
}

func (c *Client) GetByID(id int64) (models.Ad, error) {
	var resp AdResponse
	if err := c.invoke("GetAdByID", &AdIDRequest{ID: id}, &resp); err != nil {
		return models.Ad{}, unmapCatalogError(err)
	}
	return resp.Ad, nil
}

func (c *Client) IncrementViews(id int64) (models.Ad, error) {
	var resp AdResponse
	if err := c.invoke("IncrementViews", &AdIDRequest{ID: id}, &resp); err != nil {
		return models.Ad{}, unmapCatalogError(err)
	}
	return resp.Ad, nil
}

func (c *Client) GetBySellerID(sellerID int64, includeClosed bool) []models.Ad {
	var resp AdsListResponse
	if err := c.invoke("GetAdsBySeller", &SellerAdsRequest{SellerID: sellerID, IncludeClosed: includeClosed}, &resp); err != nil {
		return []models.Ad{}
	}
	if resp.Ads == nil {
		return []models.Ad{}
	}
	return resp.Ads
}

func (c *Client) CountBySeller(sellerID int64) (int, int) {
	var resp SellerAdsCountResponse
	if err := c.invoke("CountAdsBySeller", &SellerIDRequest{SellerID: sellerID}, &resp); err != nil {
		return 0, 0
	}
	return resp.Active, resp.Closed
}

func (c *Client) Create(ad models.Ad) (models.Ad, error) {
	var resp AdResponse
	if err := c.invoke("CreateAd", &CreateAdRequest{Ad: ad}, &resp); err != nil {
		return models.Ad{}, unmapCatalogError(err)
	}
	return resp.Ad, nil
}

func (c *Client) Update(id int64, sellerID int64, patch models.Ad) (models.Ad, error) {
	var resp AdResponse
	if err := c.invoke("UpdateAd", &UpdateAdRequest{ID: id, SellerID: sellerID, Patch: patch}, &resp); err != nil {
		return models.Ad{}, unmapCatalogError(err)
	}
	return resp.Ad, nil
}

func (c *Client) CloseAd(id int64, sellerID int64) (models.Ad, error) {
	var resp AdResponse
	if err := c.invoke("CloseAd", &SellerAdActionRequest{ID: id, SellerID: sellerID}, &resp); err != nil {
		return models.Ad{}, unmapCatalogError(err)
	}
	return resp.Ad, nil
}

// AdsClient обёртка, чтобы метод Close() клиента gRPC не конфликтовал с Close(id, sellerID) интерфейса Ads.
type AdsClient struct {
	*Client
}

func NewAdsClient(c *Client) *AdsClient {
	return &AdsClient{Client: c}
}

func (a *AdsClient) Close(id int64, sellerID int64) (models.Ad, error) {
	return a.Client.CloseAd(id, sellerID)
}

func (c *Client) Reopen(id int64, sellerID int64) (models.Ad, error) {
	var resp AdResponse
	if err := c.invoke("ReopenAd", &SellerAdActionRequest{ID: id, SellerID: sellerID}, &resp); err != nil {
		return models.Ad{}, unmapCatalogError(err)
	}
	return resp.Ad, nil
}

func (c *Client) MarkSold(ids []int64) error {
	var resp EmptyResponse
	return unmapCatalogError(c.invoke("MarkSold", &MarkSoldRequest{IDs: ids}, &resp))
}

func (c *Client) Delete(id int64, sellerID int64) error {
	var resp EmptyResponse
	return unmapCatalogError(c.invoke("DeleteAd", &SellerAdActionRequest{ID: id, SellerID: sellerID}, &resp))
}

func (c *Client) GetPriceHistory(adID int64) []models.PriceHistoryItem {
	var resp PriceHistoryResponse
	if err := c.invoke("GetPriceHistory", &AdIDRequest{ID: adID}, &resp); err != nil {
		return []models.PriceHistoryItem{}
	}
	if resp.Items == nil {
		return []models.PriceHistoryItem{}
	}
	return resp.Items
}

func (c *Client) Promote(id int64, sellerID int64, tier string, priority int, until time.Time) (models.Ad, error) {
	var resp AdResponse
	if err := c.invoke("PromoteAd", &PromoteAdRequest{
		ID:        id,
		SellerID:  sellerID,
		Tier:      tier,
		Priority:  priority,
		UntilUnix: until.Unix(),
	}, &resp); err != nil {
		return models.Ad{}, unmapCatalogError(err)
	}
	return resp.Ad, nil
}

func (c *Client) Add(userID, adID int64) (int64, error) {
	var resp FavoriteCountResponse
	if err := c.invoke("AddFavorite", &FavoriteRequest{UserID: userID, AdID: adID}, &resp); err != nil {
		return 0, unmapCatalogError(err)
	}
	return resp.Count, nil
}

func (c *Client) Remove(userID, adID int64) (int64, error) {
	var resp FavoriteCountResponse
	if err := c.invoke("RemoveFavorite", &FavoriteRequest{UserID: userID, AdID: adID}, &resp); err != nil {
		return 0, unmapCatalogError(err)
	}
	return resp.Count, nil
}

func (c *Client) ListAdIDs(userID int64) []int64 {
	var resp FavoriteIDsResponse
	if err := c.invoke("ListFavoriteAdIDs", &UserIDRequest{UserID: userID}, &resp); err != nil {
		return []int64{}
	}
	if resp.AdIDs == nil {
		return []int64{}
	}
	return resp.AdIDs
}

func (c *Client) CountByUser(userID int64) int {
	var resp FavoriteCountResponse
	if err := c.invoke("CountFavoritesByUser", &UserIDRequest{UserID: userID}, &resp); err != nil {
		return 0
	}
	return int(resp.Count)
}
