package cart

import (
	"path/filepath"
	"testing"

	"lavka/internal/domain/models"
)

func TestCartRepository_FlowAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cart.json")
	repo, err := NewCartRepository(path)
	if err != nil {
		t.Fatalf("NewCartRepository error: %v", err)
	}

	items, err := repo.AddItem(1, 10, 1)
	if err != nil || len(items) != 1 || items[0].Quantity != 1 {
		t.Fatalf("AddItem failed: items=%+v err=%v", items, err)
	}

	items, err = repo.AddItem(1, 10, 2)
	if err != nil || len(items) != 1 || items[0].Quantity != 3 {
		t.Fatalf("AddItem increment failed: items=%+v err=%v", items, err)
	}

	if got := repo.CountItems(1); got != 3 {
		t.Fatalf("CountItems = %d, want 3", got)
	}

	items, err = repo.UpdateItemQuantity(1, 10, 2)
	if err != nil || len(items) != 1 || items[0].Quantity != 2 {
		t.Fatalf("UpdateItemQuantity failed: items=%+v err=%v", items, err)
	}

	_, err = repo.AddItem(2, 10, 1)
	if err != nil {
		t.Fatalf("AddItem user 2 error: %v", err)
	}

	if err := repo.RemoveAdFromAllCarts(10); err != nil {
		t.Fatalf("RemoveAdFromAllCarts error: %v", err)
	}
	if len(repo.GetCart(1)) != 0 || len(repo.GetCart(2)) != 0 {
		t.Fatal("expected ad 10 to be removed from all carts")
	}

	order, err := repo.CreateOrder(models.Order{
		BuyerID:    1,
		TotalPrice: 50000,
		Items: []models.OrderItem{
			{AdID: 10, SellerID: 101, Title: "Item", Price: 50000, Quantity: 1},
		},
	})
	if err != nil || order.ID == 0 || order.Status != "confirmed" {
		t.Fatalf("CreateOrder failed: order=%+v err=%v", order, err)
	}

	reloaded, err := NewCartRepository(path)
	if err != nil {
		t.Fatalf("reload CartRepository error: %v", err)
	}
	orders := reloaded.GetOrdersByBuyer(1)
	if len(orders) != 1 || orders[0].ID != order.ID {
		t.Fatalf("reloaded orders mismatch: %+v", orders)
	}
}
