package models

import "time"

type User struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	PassHash       []byte    `json:"-"`
	AvatarPath     string    `json:"avatar_path"`
	Rating         float64   `json:"rating"`
	ReviewsCount   int       `json:"reviews_count"`
	AdsCount       int       `json:"ads_count"`
	FavoritesCount int       `json:"favorites_count"`
	CartCount      int       `json:"cart_count"`
	IsAdmin        bool      `json:"is_admin,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
