package models

import "time"

type CartItem struct {
	AdID     int64     `json:"ad_id"`
	Quantity int       `json:"quantity"`
	AddedAt  time.Time `json:"added_at"`
}

type OrderItem struct {
	AdID       int64  `json:"ad_id"`
	SellerID   int64  `json:"seller_id"`
	SellerName string `json:"seller_name"`
	Title      string `json:"title"`
	Price      int64  `json:"price"`
	Quantity   int    `json:"quantity"`
	Photo      string `json:"photo,omitempty"`
}

type Order struct {
	ID              int64       `json:"id"`
	BuyerID         int64       `json:"buyer_id"`
	Items           []OrderItem `json:"items"`
	TotalPrice      int64       `json:"total_price"`
	DeliveryMethod  string      `json:"delivery_method"`
	DeliveryAddress string      `json:"delivery_address"`
	PaymentMethod   string      `json:"payment_method"`
	Phone           string      `json:"phone"`
	Comment         string      `json:"comment,omitempty"`
	Status          string      `json:"status"`
	CreatedAt       time.Time   `json:"created_at"`
}
