package catalogservice

import (
	"context"

	"lavka/internal/domain/models"
	_ "lavka/internal/grpc/codec"

	"google.golang.org/grpc"
)

const ServiceName = "lavka.catalog.v1.CatalogService"

type EmptyRequest struct{}
type EmptyResponse struct{}

type AdIDRequest struct {
	ID int64 `json:"id"`
}

type SellerIDRequest struct {
	SellerID int64 `json:"seller_id"`
}

type UserIDRequest struct {
	UserID int64 `json:"user_id"`
}

type SellerAdsRequest struct {
	SellerID      int64 `json:"seller_id"`
	IncludeClosed bool  `json:"include_closed"`
}

type SellerAdsCountResponse struct {
	Active int `json:"active"`
	Closed int `json:"closed"`
}

type SearchAdsRequest struct {
	Filter models.AdFilter `json:"filter"`
}

type CreateAdRequest struct {
	Ad models.Ad `json:"ad"`
}

type UpdateAdRequest struct {
	ID       int64     `json:"id"`
	SellerID int64     `json:"seller_id"`
	Patch    models.Ad `json:"patch"`
}

type SellerAdActionRequest struct {
	ID       int64 `json:"id"`
	SellerID int64 `json:"seller_id"`
}

type MarkSoldRequest struct {
	IDs []int64 `json:"ids"`
}

type FavoriteRequest struct {
	UserID int64 `json:"user_id"`
	AdID   int64 `json:"ad_id"`
}

type FavoriteCountResponse struct {
	Count int64 `json:"count"`
}

type FavoriteIDsResponse struct {
	AdIDs []int64 `json:"ad_ids"`
}

type AdResponse struct {
	Ad models.Ad `json:"ad"`
}

type AdsListResponse struct {
	Ads []models.Ad `json:"ads"`
}

type PriceHistoryResponse struct {
	Items []models.PriceHistoryItem `json:"items"`
}

type PromoteAdRequest struct {
	ID        int64 `json:"id"`
	SellerID  int64 `json:"seller_id"`
	Tier      string `json:"tier"`
	Priority  int    `json:"priority"`
	UntilUnix int64  `json:"until_unix"`
}

type CatalogServiceServer interface {
	GetAllAds(context.Context, *EmptyRequest) (*AdsListResponse, error)
	SearchAds(context.Context, *SearchAdsRequest) (*AdsListResponse, error)
	GetAdByID(context.Context, *AdIDRequest) (*AdResponse, error)
	IncrementViews(context.Context, *AdIDRequest) (*AdResponse, error)
	GetAdsBySeller(context.Context, *SellerAdsRequest) (*AdsListResponse, error)
	CountAdsBySeller(context.Context, *SellerIDRequest) (*SellerAdsCountResponse, error)
	CreateAd(context.Context, *CreateAdRequest) (*AdResponse, error)
	UpdateAd(context.Context, *UpdateAdRequest) (*AdResponse, error)
	CloseAd(context.Context, *SellerAdActionRequest) (*AdResponse, error)
	ReopenAd(context.Context, *SellerAdActionRequest) (*AdResponse, error)
	MarkSold(context.Context, *MarkSoldRequest) (*EmptyResponse, error)
	DeleteAd(context.Context, *SellerAdActionRequest) (*EmptyResponse, error)
	GetPriceHistory(context.Context, *AdIDRequest) (*PriceHistoryResponse, error)
	PromoteAd(context.Context, *PromoteAdRequest) (*AdResponse, error)

	AddFavorite(context.Context, *FavoriteRequest) (*FavoriteCountResponse, error)
	RemoveFavorite(context.Context, *FavoriteRequest) (*FavoriteCountResponse, error)
	ListFavoriteAdIDs(context.Context, *UserIDRequest) (*FavoriteIDsResponse, error)
	CountFavoritesByUser(context.Context, *UserIDRequest) (*FavoriteCountResponse, error)
}

func RegisterCatalogServiceServer(s grpc.ServiceRegistrar, srv CatalogServiceServer) {
	s.RegisterService(&CatalogServiceDesc, srv)
}

func unaryHandler[Req any, Resp any](
	fn func(ctx context.Context, req *Req) (*Resp, error),
	fullMethod string,
) grpc.MethodHandler {
	return func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		in := new(Req)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return fn(ctx, in)
		}
		info := &grpc.UnaryServerInfo{
			Server:     srv,
			FullMethod: fullMethod,
		}
		handler := func(ctx context.Context, req any) (any, error) {
			return fn(ctx, req.(*Req))
		}
		return interceptor(ctx, in, info, handler)
	}
}

var CatalogServiceDesc = grpc.ServiceDesc{
	ServiceName: ServiceName,
	HandlerType: (*CatalogServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetAllAds",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).GetAllAds, "/"+ServiceName+"/GetAllAds")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "SearchAds",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).SearchAds, "/"+ServiceName+"/SearchAds")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "GetAdByID",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).GetAdByID, "/"+ServiceName+"/GetAdByID")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "IncrementViews",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).IncrementViews, "/"+ServiceName+"/IncrementViews")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "GetAdsBySeller",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).GetAdsBySeller, "/"+ServiceName+"/GetAdsBySeller")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "CountAdsBySeller",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).CountAdsBySeller, "/"+ServiceName+"/CountAdsBySeller")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "CreateAd",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).CreateAd, "/"+ServiceName+"/CreateAd")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "UpdateAd",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).UpdateAd, "/"+ServiceName+"/UpdateAd")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "CloseAd",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).CloseAd, "/"+ServiceName+"/CloseAd")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "ReopenAd",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).ReopenAd, "/"+ServiceName+"/ReopenAd")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "MarkSold",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).MarkSold, "/"+ServiceName+"/MarkSold")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "DeleteAd",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).DeleteAd, "/"+ServiceName+"/DeleteAd")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "GetPriceHistory",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).GetPriceHistory, "/"+ServiceName+"/GetPriceHistory")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "PromoteAd",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).PromoteAd, "/"+ServiceName+"/PromoteAd")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "AddFavorite",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).AddFavorite, "/"+ServiceName+"/AddFavorite")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "RemoveFavorite",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).RemoveFavorite, "/"+ServiceName+"/RemoveFavorite")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "ListFavoriteAdIDs",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).ListFavoriteAdIDs, "/"+ServiceName+"/ListFavoriteAdIDs")(srv, ctx, dec, interceptor)
			},
		},
		{
			MethodName: "CountFavoritesByUser",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return unaryHandler(srv.(CatalogServiceServer).CountFavoritesByUser, "/"+ServiceName+"/CountFavoritesByUser")(srv, ctx, dec, interceptor)
			},
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/catalog/catalog.proto",
}
