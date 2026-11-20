package cart

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lavka/internal/domain/models"
)

type CartRepository struct {
	mu          sync.RWMutex
	carts       map[int64][]models.CartItem
	orders      []models.Order
	nextOrderID int64
	path        string
}

type cartDump struct {
	Carts       map[int64][]models.CartItem `json:"carts"`
	Orders      []models.Order              `json:"orders"`
	NextOrderID int64                       `json:"next_order_id"`
}

func NewCartRepository(path string) (*CartRepository, error) {
	r := &CartRepository{
		carts:  make(map[int64][]models.CartItem),
		orders: make([]models.Order, 0),
		path:   path,
	}

	if path == "" {
		return r, nil
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(path); err == nil {
		if err := r.load(); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	return r, nil
}

func (r *CartRepository) load() error {
	f, err := os.Open(r.path)
	if err != nil {
		return err
	}
	defer f.Close()

	var d cartDump
	if err := json.NewDecoder(f).Decode(&d); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}

	if d.Carts != nil {
		r.carts = d.Carts
	}
	if d.Orders != nil {
		r.orders = d.Orders
	}
	r.nextOrderID = d.NextOrderID
	return nil
}

func (r *CartRepository) persist() error {
	if r.path == "" {
		return nil
	}

	d := cartDump{
		Carts:       r.carts,
		Orders:      r.orders,
		NextOrderID: r.nextOrderID,
	}

	f, err := os.Create(r.path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(&d)
}

func (r *CartRepository) GetCart(userID int64) []models.CartItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := r.carts[userID]
	result := make([]models.CartItem, len(items))
	copy(result, items)
	return result
}

func (r *CartRepository) CountItems(userID int64) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := 0
	for _, item := range r.carts[userID] {
		total += item.Quantity
	}
	return total
}

func (r *CartRepository) AddItem(userID, adID int64, quantity int) ([]models.CartItem, error) {
	if quantity <= 0 {
		quantity = 1
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	items := r.carts[userID]
	found := false
	for i := range items {
		if items[i].AdID == adID {
			items[i].Quantity += quantity
			found = true
			break
		}
	}
	if !found {
		items = append(items, models.CartItem{
			AdID:     adID,
			Quantity: quantity,
			AddedAt:  time.Now(),
		})
	}
	r.carts[userID] = items

	if err := r.persist(); err != nil {
		return nil, err
	}

	res := make([]models.CartItem, len(items))
	copy(res, items)
	return res, nil
}

func (r *CartRepository) UpdateItemQuantity(userID, adID int64, quantity int) ([]models.CartItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := r.carts[userID]
	if quantity <= 0 {
		filtered := make([]models.CartItem, 0, len(items))
		for _, item := range items {
			if item.AdID != adID {
				filtered = append(filtered, item)
			}
		}
		r.carts[userID] = filtered
	} else {
		for i := range items {
			if items[i].AdID == adID {
				items[i].Quantity = quantity
				break
			}
		}
		r.carts[userID] = items
	}

	if err := r.persist(); err != nil {
		return nil, err
	}

	res := make([]models.CartItem, len(r.carts[userID]))
	copy(res, r.carts[userID])
	return res, nil
}

func (r *CartRepository) RemoveItem(userID, adID int64) ([]models.CartItem, error) {
	return r.UpdateItemQuantity(userID, adID, 0)
}

func (r *CartRepository) RemoveAdFromAllCarts(adID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	changed := false
	for uid, items := range r.carts {
		filtered := make([]models.CartItem, 0, len(items))
		for _, item := range items {
			if item.AdID == adID {
				changed = true
				continue
			}
			filtered = append(filtered, item)
		}
		r.carts[uid] = filtered
	}

	if changed {
		return r.persist()
	}
	return nil
}

func (r *CartRepository) ClearCart(userID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.carts, userID)
	return r.persist()
}

func (r *CartRepository) CreateOrder(order models.Order) (models.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextOrderID++
	order.ID = r.nextOrderID
	order.Status = "confirmed"
	order.CreatedAt = time.Now()

	r.orders = append([]models.Order{order}, r.orders...)
	if err := r.persist(); err != nil {
		return models.Order{}, err
	}
	return order, nil
}

func (r *CartRepository) GetOrdersByBuyer(buyerID int64) []models.Order {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]models.Order, 0)
	for _, o := range r.orders {
		if o.BuyerID == buyerID {
			result = append(result, o)
		}
	}
	return result
}
