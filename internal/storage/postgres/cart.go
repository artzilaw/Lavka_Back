package postgres

import (
	"context"
	"database/sql"
	"log/slog"

	"lavka/internal/domain/models"
)

type CartRepository struct {
	db  *sql.DB
	log *slog.Logger
}

func NewCartRepository(db *sql.DB, log *slog.Logger) *CartRepository {
	return &CartRepository{db: db, log: log}
}

func (r *CartRepository) GetCart(userID int64) []models.CartItem {
	ctx := context.Background()
	r.log.Debug("db query: Cart.GetCart", slog.Int64("user_id", userID))

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT ad_id, quantity, added_at FROM cart_items WHERE user_id = $1 ORDER BY added_at ASC`,
		userID,
	)
	if err != nil {
		r.log.Error("db error: Cart.GetCart", slog.String("error", err.Error()))
		return []models.CartItem{}
	}
	defer rows.Close()

	items := make([]models.CartItem, 0)
	for rows.Next() {
		var item models.CartItem
		if err := rows.Scan(&item.AdID, &item.Quantity, &item.AddedAt); err == nil {
			items = append(items, item)
		}
	}
	return items
}

func (r *CartRepository) CountItems(userID int64) int {
	ctx := context.Background()
	var total int
	_ = r.db.QueryRowContext(
		ctx,
		`SELECT COALESCE(SUM(quantity), 0) FROM cart_items WHERE user_id = $1`,
		userID,
	).Scan(&total)
	return total
}

func (r *CartRepository) AddItem(userID, adID int64, quantity int) ([]models.CartItem, error) {
	if quantity <= 0 {
		quantity = 1
	}
	ctx := context.Background()
	r.log.Debug("db query: Cart.AddItem", slog.Int64("user_id", userID), slog.Int64("ad_id", adID))

	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO cart_items (user_id, ad_id, quantity)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, ad_id)
		 DO UPDATE SET quantity = LEAST(cart_items.quantity + EXCLUDED.quantity, 99)`,
		userID, adID, quantity,
	)
	if err != nil {
		return nil, err
	}
	return r.GetCart(userID), nil
}

func (r *CartRepository) UpdateItemQuantity(userID, adID int64, quantity int) ([]models.CartItem, error) {
	ctx := context.Background()
	r.log.Debug("db query: Cart.UpdateItemQuantity", slog.Int64("user_id", userID), slog.Int64("ad_id", adID))

	if quantity <= 0 {
		_, err := r.db.ExecContext(ctx, `DELETE FROM cart_items WHERE user_id = $1 AND ad_id = $2`, userID, adID)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := r.db.ExecContext(
			ctx,
			`UPDATE cart_items SET quantity = $1 WHERE user_id = $2 AND ad_id = $3`,
			quantity, userID, adID,
		)
		if err != nil {
			return nil, err
		}
	}
	return r.GetCart(userID), nil
}

func (r *CartRepository) RemoveItem(userID, adID int64) ([]models.CartItem, error) {
	return r.UpdateItemQuantity(userID, adID, 0)
}

func (r *CartRepository) RemoveAdFromAllCarts(adID int64) error {
	ctx := context.Background()
	_, err := r.db.ExecContext(ctx, `DELETE FROM cart_items WHERE ad_id = $1`, adID)
	return err
}

func (r *CartRepository) ClearCart(userID int64) error {
	ctx := context.Background()
	_, err := r.db.ExecContext(ctx, `DELETE FROM cart_items WHERE user_id = $1`, userID)
	return err
}

func (r *CartRepository) CreateOrder(order models.Order) (models.Order, error) {
	ctx := context.Background()
	r.log.Debug("db query: Cart.CreateOrder", slog.Int64("buyer_id", order.BuyerID))

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Order{}, err
	}
	defer func() { _ = tx.Rollback() }()

	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO orders (buyer_id, total_price, delivery_method, delivery_address, payment_method, phone, comment, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'confirmed')
		 RETURNING id, status, created_at`,
		order.BuyerID, order.TotalPrice, order.DeliveryMethod, order.DeliveryAddress,
		order.PaymentMethod, order.Phone, order.Comment,
	).Scan(&order.ID, &order.Status, &order.CreatedAt)
	if err != nil {
		return models.Order{}, err
	}

	for _, it := range order.Items {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO order_items (order_id, ad_id, seller_id, seller_name, title, price, quantity, photo)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			order.ID, it.AdID, it.SellerID, it.SellerName, it.Title, it.Price, it.Quantity, it.Photo,
		); err != nil {
			return models.Order{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return models.Order{}, err
	}
	return order, nil
}

func (r *CartRepository) GetOrdersByBuyer(buyerID int64) []models.Order {
	ctx := context.Background()
	r.log.Debug("db query: Cart.GetOrdersByBuyer", slog.Int64("buyer_id", buyerID))

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, buyer_id, total_price, delivery_method, delivery_address, payment_method, phone, comment, status, created_at
		 FROM orders
		 WHERE buyer_id = $1
		 ORDER BY created_at DESC, id DESC`,
		buyerID,
	)
	if err != nil {
		r.log.Error("db error: Cart.GetOrdersByBuyer", slog.String("error", err.Error()))
		return []models.Order{}
	}
	defer rows.Close()

	orders := make([]models.Order, 0)
	for rows.Next() {
		var o models.Order
		if err := rows.Scan(
			&o.ID, &o.BuyerID, &o.TotalPrice, &o.DeliveryMethod, &o.DeliveryAddress,
			&o.PaymentMethod, &o.Phone, &o.Comment, &o.Status, &o.CreatedAt,
		); err == nil {
			orders = append(orders, o)
		}
	}

	for i := range orders {
		itemRows, err := r.db.QueryContext(
			ctx,
			`SELECT ad_id, seller_id, seller_name, title, price, quantity, photo
			 FROM order_items
			 WHERE order_id = $1
			 ORDER BY id ASC`,
			orders[i].ID,
		)
		if err != nil {
			orders[i].Items = []models.OrderItem{}
			continue
		}
		items := make([]models.OrderItem, 0)
		for itemRows.Next() {
			var it models.OrderItem
			if err := itemRows.Scan(&it.AdID, &it.SellerID, &it.SellerName, &it.Title, &it.Price, &it.Quantity, &it.Photo); err == nil {
				items = append(items, it)
			}
		}
		itemRows.Close()
		orders[i].Items = items
	}

	return orders
}
