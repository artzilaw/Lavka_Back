package models

import "time"

//go:generate easyjson -all user.go

//easyjson:json
type User struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	PassHash       []byte    `json:"-"`
	AvatarPath     string    `json:"avatar_path"`
	Balance        int64     `json:"balance"`
	Rating         float64   `json:"rating"`
	ReviewsCount   int       `json:"reviews_count"`
	AdsCount       int       `json:"ads_count"`
	FavoritesCount int       `json:"favorites_count"`
	CartCount      int       `json:"cart_count"`
	IsAdmin        bool      `json:"is_admin,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

//easyjson:json
type WalletTransaction struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Amount      int64     `json:"amount"`
	Type        string    `json:"type"` // "topup" | "promotion"
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}
