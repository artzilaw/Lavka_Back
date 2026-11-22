package wallet

import (
	"errors"
	"sync"
	"time"

	"lavka/internal/domain/models"
)

const DefaultStartingBalance int64 = 1000

var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrInvalidAmount     = errors.New("invalid amount")
)

type InMemoryRepository struct {
	mu           sync.RWMutex
	balances     map[int64]int64
	transactions map[int64][]models.WalletTransaction
	nextTxID     int64
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		balances:     make(map[int64]int64),
		transactions: make(map[int64][]models.WalletTransaction),
	}
}

func (r *InMemoryRepository) ensureBalanceLocked(userID int64) int64 {
	bal, ok := r.balances[userID]
	if !ok {
		r.balances[userID] = DefaultStartingBalance
		return DefaultStartingBalance
	}
	return bal
}

func (r *InMemoryRepository) GetBalance(userID int64) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ensureBalanceLocked(userID), nil
}

func (r *InMemoryRepository) ListTransactions(userID int64) ([]models.WalletTransaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	src := r.transactions[userID]
	out := make([]models.WalletTransaction, len(src))
	copy(out, src)
	return out, nil
}

func (r *InMemoryRepository) TopUp(userID int64, amount int64, description string) (int64, models.WalletTransaction, error) {
	if amount <= 0 {
		return 0, models.WalletTransaction{}, ErrInvalidAmount
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	bal := r.ensureBalanceLocked(userID) + amount
	r.balances[userID] = bal

	r.nextTxID++
	tx := models.WalletTransaction{
		ID:          r.nextTxID,
		UserID:      userID,
		Amount:      amount,
		Type:        "topup",
		Description: description,
		CreatedAt:   time.Now(),
	}
	r.transactions[userID] = append([]models.WalletTransaction{tx}, r.transactions[userID]...)
	return bal, tx, nil
}

func (r *InMemoryRepository) Charge(userID int64, amount int64, description string) (int64, models.WalletTransaction, error) {
	if amount <= 0 {
		return 0, models.WalletTransaction{}, ErrInvalidAmount
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	bal := r.ensureBalanceLocked(userID)
	if bal < amount {
		return bal, models.WalletTransaction{}, ErrInsufficientFunds
	}

	bal -= amount
	r.balances[userID] = bal

	r.nextTxID++
	tx := models.WalletTransaction{
		ID:          r.nextTxID,
		UserID:      userID,
		Amount:      -amount,
		Type:        "promotion",
		Description: description,
		CreatedAt:   time.Now(),
	}
	r.transactions[userID] = append([]models.WalletTransaction{tx}, r.transactions[userID]...)
	return bal, tx, nil
}
