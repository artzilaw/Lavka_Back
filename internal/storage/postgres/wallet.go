package postgres

import (
	"context"
	"database/sql"
	"log/slog"

	"lavka/internal/domain/models"
	"lavka/internal/storage/wallet"
)

type WalletRepository struct {
	db  *sql.DB
	log *slog.Logger
}

func NewWalletRepository(db *sql.DB, log *slog.Logger) *WalletRepository {
	return &WalletRepository{db: db, log: log}
}

func (r *WalletRepository) GetBalance(userID int64) (int64, error) {
	ctx := context.Background()
	var bal int64
	err := r.db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id = $1`, userID).Scan(&bal)
	if err != nil {
		return 0, err
	}
	return bal, nil
}

func (r *WalletRepository) ListTransactions(userID int64) ([]models.WalletTransaction, error) {
	ctx := context.Background()
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, user_id, amount, type, description, created_at
		 FROM wallet_transactions
		 WHERE user_id = $1
		 ORDER BY created_at DESC, id DESC
		 LIMIT 50`,
		userID,
	)
	if err != nil {
		return []models.WalletTransaction{}, err
	}
	defer rows.Close()

	res := make([]models.WalletTransaction, 0)
	for rows.Next() {
		var tx models.WalletTransaction
		if err := rows.Scan(&tx.ID, &tx.UserID, &tx.Amount, &tx.Type, &tx.Description, &tx.CreatedAt); err != nil {
			continue
		}
		res = append(res, tx)
	}
	return res, nil
}

func (r *WalletRepository) TopUp(userID int64, amount int64, description string) (int64, models.WalletTransaction, error) {
	if amount <= 0 {
		return 0, models.WalletTransaction{}, wallet.ErrInvalidAmount
	}

	ctx := context.Background()
	sqlTx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, models.WalletTransaction{}, err
	}
	defer func() { _ = sqlTx.Rollback() }()

	var newBalance int64
	err = sqlTx.QueryRowContext(
		ctx,
		`UPDATE users SET balance = balance + $1, updated_at = NOW() WHERE id = $2 RETURNING balance`,
		amount, userID,
	).Scan(&newBalance)
	if err != nil {
		return 0, models.WalletTransaction{}, err
	}

	var tx models.WalletTransaction
	err = sqlTx.QueryRowContext(
		ctx,
		`INSERT INTO wallet_transactions (user_id, amount, type, description)
		 VALUES ($1, $2, 'topup', $3)
		 RETURNING id, user_id, amount, type, description, created_at`,
		userID, amount, description,
	).Scan(&tx.ID, &tx.UserID, &tx.Amount, &tx.Type, &tx.Description, &tx.CreatedAt)
	if err != nil {
		return 0, models.WalletTransaction{}, err
	}

	if err := sqlTx.Commit(); err != nil {
		return 0, models.WalletTransaction{}, err
	}
	return newBalance, tx, nil
}

func (r *WalletRepository) Charge(userID int64, amount int64, description string) (int64, models.WalletTransaction, error) {
	if amount <= 0 {
		return 0, models.WalletTransaction{}, wallet.ErrInvalidAmount
	}

	ctx := context.Background()
	sqlTx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, models.WalletTransaction{}, err
	}
	defer func() { _ = sqlTx.Rollback() }()

	// Пессимистичная блокировка строки пользователя (защита от double-spending)
	var currentBalance int64
	if err := sqlTx.QueryRowContext(
		ctx,
		`SELECT balance FROM users WHERE id = $1 FOR UPDATE`,
		userID,
	).Scan(&currentBalance); err != nil {
		return 0, models.WalletTransaction{}, err
	}

	if currentBalance < amount {
		return currentBalance, models.WalletTransaction{}, wallet.ErrInsufficientFunds
	}

	newBalance := currentBalance - amount
	if _, err := sqlTx.ExecContext(
		ctx,
		`UPDATE users SET balance = $1, updated_at = NOW() WHERE id = $2`,
		newBalance, userID,
	); err != nil {
		return 0, models.WalletTransaction{}, err
	}

	var tx models.WalletTransaction
	err = sqlTx.QueryRowContext(
		ctx,
		`INSERT INTO wallet_transactions (user_id, amount, type, description)
		 VALUES ($1, $2, 'promotion', $3)
		 RETURNING id, user_id, amount, type, description, created_at`,
		userID, -amount, description,
	).Scan(&tx.ID, &tx.UserID, &tx.Amount, &tx.Type, &tx.Description, &tx.CreatedAt)
	if err != nil {
		return 0, models.WalletTransaction{}, err
	}

	if err := sqlTx.Commit(); err != nil {
		return 0, models.WalletTransaction{}, err
	}
	return newBalance, tx, nil
}
