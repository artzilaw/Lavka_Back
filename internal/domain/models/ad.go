package models

import "time"

//go:generate easyjson -all ad.go

const (
	AdStatusActive  = "active"
	AdStatusClosed  = "closed"
	AdStatusSold    = "sold"
	AdStatusDeleted = "deleted"

	PromoTierNone      = ""
	PromoTierHighlight = "highlight"
	PromoTierBoost     = "boost"
	PromoTierVIP       = "vip"
)

type PromotionTariff struct {
	Tier     string        `json:"tier"`
	Title    string        `json:"title"`
	Price    int64         `json:"price"`
	Priority int           `json:"priority"`
	Duration time.Duration `json:"-"`
	Days     int           `json:"days"`
}

var PromotionTariffs = map[string]PromotionTariff{
	PromoTierHighlight: {
		Tier:     PromoTierHighlight,
		Title:    "Выделение цветом",
		Price:    199,
		Priority: 1,
		Duration: 7 * 24 * time.Hour,
		Days:     7,
	},
	PromoTierBoost: {
		Tier:     PromoTierBoost,
		Title:    "Поднятие в топ",
		Price:    299,
		Priority: 2,
		Duration: 7 * 24 * time.Hour,
		Days:     7,
	},
	PromoTierVIP: {
		Tier:     PromoTierVIP,
		Title:    "VIP-продвижение",
		Price:    599,
		Priority: 3,
		Duration: 14 * 24 * time.Hour,
		Days:     14,
	},
}

//easyjson:json
type Ad struct {
	ID                int64      `json:"id"`
	SellerID          int64      `json:"seller_id"`
	CategoryID        int64      `json:"category_id"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	Price             int64      `json:"price"`
	Status            string     `json:"status"`
	ViewsCount        int64      `json:"views_count"`
	FavoritesCount    int64      `json:"favorites_count"`
	PromotionTier     string     `json:"promotion_tier,omitempty"`
	PromotionPriority int        `json:"promotion_priority,omitempty"`
	PromotedUntil     *time.Time `json:"promoted_until,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         time.Time  `json:"deleted_at,omitempty"`
	Photos            []string   `json:"photos"`
	Location          string     `json:"location"`
}

//easyjson:json
type PriceHistoryItem struct {
	ID        int64     `json:"id"`
	AdID      int64     `json:"ad_id"`
	OldPrice  int64     `json:"old_price"`
	NewPrice  int64     `json:"new_price"`
	ChangedAt time.Time `json:"changed_at"`
}

type AdFilter struct {
	Query      string
	CategoryID int64
	Location   string
	MinPrice   *int64
	MaxPrice   *int64
	Sort       string // "newest" | "price_asc" | "price_desc" | "popular"
}
