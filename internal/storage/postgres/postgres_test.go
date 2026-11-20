package postgres

import (
	"context"
	"io"
	"log/slog"
	"regexp"
	"testing"
	"time"

	"lavka/internal/domain/models"

	"github.com/DATA-DOG/go-sqlmock"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestUserRepository_SQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	repo := NewUserRepository(db, testLogger())
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO users (name, email, pass_hash) VALUES ($1, $2, $3) RETURNING id`)).
		WithArgs("Alice", "alice@lavka.ru", []byte("hash")).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))

	id, err := repo.SaveUser(ctx, "alice@lavka.ru", []byte("hash"), "Alice")
	if err != nil || id != 7 {
		t.Fatalf("SaveUser = (%d, %v), want (7, nil)", id, err)
	}

	userCols := []string{"id", "name", "email", "pass_hash", "avatar_path", "rating", "reviews_count", "is_admin", "created_at", "updated_at"}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, name, email, pass_hash, avatar_path, rating, reviews_count, is_admin, created_at, updated_at FROM users WHERE id = $1`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows(userCols).AddRow(int64(7), "Alice", "alice@lavka.ru", []byte("hash"), "", 4.9, 5, false, now, now))

	u, err := repo.UserByID(ctx, 7)
	if err != nil || u.Email != "alice@lavka.ru" {
		t.Fatalf("UserByID = (%+v, %v)", u, err)
	}

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE users SET pass_hash = $1, updated_at = NOW() WHERE id = $2`)).
		WithArgs([]byte("newhash"), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.UpdatePassword(ctx, 7, []byte("newhash")); err != nil {
		t.Fatalf("UpdatePassword error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAdsRepository_SQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	repo := NewAdsRepository(db, testLogger())
	now := time.Now()
	adCols := []string{"id", "seller_id", "category_id", "title", "description", "price", "status", "location", "views_count", "favorites_count", "created_at", "updated_at", "deleted_at"}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO ads (seller_id, category_id, title, description, price, status, location) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, seller_id, category_id, title, description, price, status, location, views_count, favorites_count, created_at, updated_at, deleted_at`)).
		WithArgs(int64(3), int64(1), "iPhone 15", "Хорошее состояние", int64(70000), models.AdStatusActive, "Москва").
		WillReturnRows(sqlmock.NewRows(adCols).AddRow(int64(10), int64(3), int64(1), "iPhone 15", "Хорошее состояние", int64(70000), "active", "Москва", int64(0), int64(0), now, now, nil))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO ad_photos (ad_id, url, position) VALUES ($1, $2, $3)`)).
		WithArgs(int64(10), "/static/img/ads/test.jpg", 0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	created, err := repo.Create(models.Ad{
		SellerID:    3,
		CategoryID:  1,
		Title:       "iPhone 15",
		Description: "Хорошее состояние",
		Price:       70000,
		Location:    "Москва",
		Photos:      []string{"/static/img/ads/test.jpg"},
	})
	if err != nil || created.ID != 10 || len(created.Photos) != 1 {
		t.Fatalf("Create = (%+v, %v)", created, err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN status IN ('closed', 'sold') THEN 1 ELSE 0 END), 0) FROM ads WHERE seller_id = $1 AND status <> 'deleted'`)).
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"active", "closed"}).AddRow(2, 1))

	active, closed := repo.CountBySeller(3)
	if active != 2 || closed != 1 {
		t.Fatalf("CountBySeller = (%d, %d), want (2, 1)", active, closed)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCartRepository_SQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	repo := NewCartRepository(db, testLogger())
	now := time.Now()

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO cart_items (user_id, ad_id, quantity) VALUES ($1, $2, $3) ON CONFLICT (user_id, ad_id) DO UPDATE SET quantity = LEAST(cart_items.quantity + EXCLUDED.quantity, 99)`)).
		WithArgs(int64(2), int64(10), 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT ad_id, quantity, added_at FROM cart_items WHERE user_id = $1 ORDER BY added_at ASC`)).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"ad_id", "quantity", "added_at"}).AddRow(int64(10), 1, now))

	items, err := repo.AddItem(2, 10, 1)
	if err != nil || len(items) != 1 || items[0].AdID != 10 {
		t.Fatalf("AddItem = (%+v, %v)", items, err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO orders (buyer_id, total_price, delivery_method, delivery_address, payment_method, phone, comment, status) VALUES ($1, $2, $3, $4, $5, $6, $7, 'confirmed') RETURNING id, status, created_at`)).
		WithArgs(int64(2), int64(70000), "delivery", "Москва, Тверская 1", "card", "+79991234567", "").
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "created_at"}).AddRow(int64(1), "confirmed", now))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO order_items (order_id, ad_id, seller_id, seller_name, title, price, quantity, photo) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`)).
		WithArgs(int64(1), int64(10), int64(3), "Seller", "iPhone 15", int64(70000), 1, "").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	order, err := repo.CreateOrder(models.Order{
		BuyerID:         2,
		TotalPrice:      70000,
		DeliveryMethod:  "delivery",
		DeliveryAddress: "Москва, Тверская 1",
		PaymentMethod:   "card",
		Phone:           "+79991234567",
		Items: []models.OrderItem{
			{AdID: 10, SellerID: 3, SellerName: "Seller", Title: "iPhone 15", Price: 70000, Quantity: 1},
		},
	})
	if err != nil || order.ID != 1 {
		t.Fatalf("CreateOrder = (%+v, %v)", order, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
