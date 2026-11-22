package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open открывает пул подключений к PostgreSQL через database/sql + pgx и проверяет доступность БД.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres.Open: %w", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres.Open ping: %w", err)
	}

	_ = ensureRK4Schema(pingCtx, db)

	return db, nil
}

func ensureRK4Schema(ctx context.Context, db *sql.DB) error {
	const ddl = `
ALTER TABLE users ADD COLUMN IF NOT EXISTS balance BIGINT NOT NULL DEFAULT 1000;
ALTER TABLE ads ADD COLUMN IF NOT EXISTS promotion_tier VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE ads ADD COLUMN IF NOT EXISTS promotion_priority INT NOT NULL DEFAULT 0;
ALTER TABLE ads ADD COLUMN IF NOT EXISTS promoted_until TIMESTAMPTZ;
CREATE TABLE IF NOT EXISTS ad_price_history (
    id BIGSERIAL PRIMARY KEY,
    ad_id BIGINT NOT NULL REFERENCES ads(id) ON DELETE CASCADE,
    old_price BIGINT NOT NULL,
    new_price BIGINT NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS wallet_transactions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount BIGINT NOT NULL,
    type VARCHAR(32) NOT NULL CHECK (type IN ('topup', 'promotion')),
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	_, err := db.ExecContext(ctx, ddl)
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
