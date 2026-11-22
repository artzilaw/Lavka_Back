package postgres

import (
	"context"
	"database/sql"
	"log/slog"
)

type FavoritesRepository struct {
	db  *sql.DB
	log *slog.Logger
}

func NewFavoritesRepository(db *sql.DB, log *slog.Logger) *FavoritesRepository {
	return &FavoritesRepository{db: db, log: log}
}

func (r *FavoritesRepository) Add(userID, adID int64) (int64, error) {
	ctx := context.Background()
	r.log.Debug("db query: Favorites.Add", slog.Int64("user_id", userID), slog.Int64("ad_id", adID))

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO favorites (user_id, ad_id) VALUES ($1, $2) ON CONFLICT (user_id, ad_id) DO NOTHING`,
		userID, adID,
	); err != nil {
		return 0, err
	}

	var count int64
	if err := tx.QueryRowContext(
		ctx,
		`UPDATE ads
		 SET favorites_count = (SELECT COUNT(*) FROM favorites WHERE ad_id = $1)
		 WHERE id = $1
		 RETURNING favorites_count`,
		adID,
	).Scan(&count); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *FavoritesRepository) Remove(userID, adID int64) (int64, error) {
	ctx := context.Background()
	r.log.Debug("db query: Favorites.Remove", slog.Int64("user_id", userID), slog.Int64("ad_id", adID))

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM favorites WHERE user_id = $1 AND ad_id = $2`,
		userID, adID,
	); err != nil {
		return 0, err
	}

	var count int64
	_ = tx.QueryRowContext(
		ctx,
		`UPDATE ads
		 SET favorites_count = (SELECT COUNT(*) FROM favorites WHERE ad_id = $1)
		 WHERE id = $1
		 RETURNING favorites_count`,
		adID,
	).Scan(&count)

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *FavoritesRepository) ListAdIDs(userID int64) []int64 {
	ctx := context.Background()
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT f.ad_id
		 FROM favorites f
		 JOIN ads a ON a.id = f.ad_id
		 WHERE f.user_id = $1 AND a.status <> 'deleted'
		 ORDER BY f.created_at DESC`,
		userID,
	)
	if err != nil {
		r.log.Error("db error: Favorites.ListAdIDs", slog.String("error", err.Error()))
		return []int64{}
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func (r *FavoritesRepository) CountByUser(userID int64) int {
	ctx := context.Background()
	var cnt int
	_ = r.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM favorites f JOIN ads a ON a.id = f.ad_id WHERE f.user_id = $1 AND a.status <> 'deleted'`,
		userID,
	).Scan(&cnt)
	return cnt
}
