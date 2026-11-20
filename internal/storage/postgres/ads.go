package postgres

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"lavka/internal/domain/models"
	"lavka/internal/storage/ads"
)

type AdsRepository struct {
	db  *sql.DB
	log *slog.Logger
}

func NewAdsRepository(db *sql.DB, log *slog.Logger) *AdsRepository {
	return &AdsRepository{db: db, log: log}
}

func (r *AdsRepository) loadPhotos(ctx context.Context, adID int64) []string {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT url FROM ad_photos WHERE ad_id = $1 ORDER BY position ASC, id ASC`,
		adID,
	)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	photos := make([]string, 0)
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err == nil {
			photos = append(photos, u)
		}
	}
	return photos
}

func scanAdRow(scanner interface{ Scan(dest ...any) error }) (models.Ad, error) {
	var ad models.Ad
	var deletedAt sql.NullTime
	err := scanner.Scan(
		&ad.ID, &ad.SellerID, &ad.CategoryID, &ad.Title, &ad.Description,
		&ad.Price, &ad.Status, &ad.Location, &ad.ViewsCount, &ad.FavoritesCount,
		&ad.CreatedAt, &ad.UpdatedAt, &deletedAt,
	)
	if deletedAt.Valid {
		ad.DeletedAt = deletedAt.Time
	}
	return ad, err
}

func (r *AdsRepository) GetAll() []models.Ad {
	ctx := context.Background()
	r.log.Debug("db query: Ads.GetAll")

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, seller_id, category_id, title, description, price, status, location,
		        views_count, favorites_count, created_at, updated_at, deleted_at
		 FROM ads
		 WHERE status = $1
		 ORDER BY created_at DESC, id DESC`,
		models.AdStatusActive,
	)
	if err != nil {
		r.log.Error("db error: Ads.GetAll", slog.String("error", err.Error()))
		return []models.Ad{}
	}
	defer rows.Close()

	result := make([]models.Ad, 0)
	for rows.Next() {
		ad, err := scanAdRow(rows)
		if err != nil {
			continue
		}
		result = append(result, ad)
	}
	for i := range result {
		result[i].Photos = r.loadPhotos(ctx, result[i].ID)
	}
	return result
}

func (r *AdsRepository) GetByID(id int64) (models.Ad, error) {
	ctx := context.Background()
	r.log.Debug("db query: Ads.GetByID", slog.Int64("ad_id", id))

	row := r.db.QueryRowContext(
		ctx,
		`SELECT id, seller_id, category_id, title, description, price, status, location,
		        views_count, favorites_count, created_at, updated_at, deleted_at
		 FROM ads
		 WHERE id = $1 AND status <> $2`,
		id, models.AdStatusDeleted,
	)
	ad, err := scanAdRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Ad{}, ads.ErrAdNotFound
		}
		return models.Ad{}, err
	}
	ad.Photos = r.loadPhotos(ctx, ad.ID)
	return ad, nil
}

func (r *AdsRepository) IncrementViews(id int64) (models.Ad, error) {
	ctx := context.Background()
	r.log.Debug("db query: Ads.IncrementViews", slog.Int64("ad_id", id))

	row := r.db.QueryRowContext(
		ctx,
		`UPDATE ads
		 SET views_count = views_count + 1
		 WHERE id = $1 AND status <> $2
		 RETURNING id, seller_id, category_id, title, description, price, status, location,
		           views_count, favorites_count, created_at, updated_at, deleted_at`,
		id, models.AdStatusDeleted,
	)
	ad, err := scanAdRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Ad{}, ads.ErrAdNotFound
		}
		return models.Ad{}, err
	}
	ad.Photos = r.loadPhotos(ctx, ad.ID)
	return ad, nil
}

func (r *AdsRepository) GetBySellerID(sellerID int64, includeClosed bool) []models.Ad {
	ctx := context.Background()
	r.log.Debug("db query: Ads.GetBySellerID", slog.Int64("seller_id", sellerID))

	query := `SELECT id, seller_id, category_id, title, description, price, status, location,
	                 views_count, favorites_count, created_at, updated_at, deleted_at
	          FROM ads
	          WHERE seller_id = $1 AND status <> $2`
	args := []any{sellerID, models.AdStatusDeleted}
	if !includeClosed {
		query += ` AND status = $3`
		args = append(args, models.AdStatusActive)
	}
	query += ` ORDER BY created_at DESC, id DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		r.log.Error("db error: Ads.GetBySellerID", slog.String("error", err.Error()))
		return []models.Ad{}
	}
	defer rows.Close()

	result := make([]models.Ad, 0)
	for rows.Next() {
		ad, err := scanAdRow(rows)
		if err != nil {
			continue
		}
		result = append(result, ad)
	}
	for i := range result {
		result[i].Photos = r.loadPhotos(ctx, result[i].ID)
	}
	return result
}

func (r *AdsRepository) CountBySeller(sellerID int64) (active int, closed int) {
	ctx := context.Background()
	_ = r.db.QueryRowContext(
		ctx,
		`SELECT
		   COALESCE(SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END), 0),
		   COALESCE(SUM(CASE WHEN status IN ('closed', 'sold') THEN 1 ELSE 0 END), 0)
		 FROM ads
		 WHERE seller_id = $1 AND status <> 'deleted'`,
		sellerID,
	).Scan(&active, &closed)
	return active, closed
}

func (r *AdsRepository) Create(ad models.Ad) (models.Ad, error) {
	ctx := context.Background()
	r.log.Debug("db query: Ads.Create", slog.Int64("seller_id", ad.SellerID))

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Ad{}, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(
		ctx,
		`INSERT INTO ads (seller_id, category_id, title, description, price, status, location)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, seller_id, category_id, title, description, price, status, location,
		           views_count, favorites_count, created_at, updated_at, deleted_at`,
		ad.SellerID, ad.CategoryID, ad.Title, ad.Description, ad.Price, models.AdStatusActive, ad.Location,
	)
	created, err := scanAdRow(row)
	if err != nil {
		return models.Ad{}, err
	}

	created.Photos = make([]string, 0, len(ad.Photos))
	for idx, photoURL := range ad.Photos {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO ad_photos (ad_id, url, position) VALUES ($1, $2, $3)`,
			created.ID, photoURL, idx,
		); err != nil {
			return models.Ad{}, err
		}
		created.Photos = append(created.Photos, photoURL)
	}

	if err := tx.Commit(); err != nil {
		return models.Ad{}, err
	}
	return created, nil
}

func (r *AdsRepository) checkOwner(ctx context.Context, id, sellerID int64) error {
	var ownerID int64
	err := r.db.QueryRowContext(
		ctx,
		`SELECT seller_id FROM ads WHERE id = $1 AND status <> $2`,
		id, models.AdStatusDeleted,
	).Scan(&ownerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ads.ErrAdNotFound
		}
		return err
	}
	if ownerID != sellerID {
		return ads.ErrForbidden
	}
	return nil
}

func (r *AdsRepository) Update(id int64, sellerID int64, patch models.Ad) (models.Ad, error) {
	ctx := context.Background()
	r.log.Debug("db query: Ads.Update", slog.Int64("ad_id", id), slog.Int64("seller_id", sellerID))

	if err := r.checkOwner(ctx, id, sellerID); err != nil {
		return models.Ad{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Ad{}, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(
		ctx,
		`UPDATE ads
		 SET title = $1, description = $2, price = $3, category_id = $4, location = $5, updated_at = NOW()
		 WHERE id = $6 AND seller_id = $7 AND status <> $8
		 RETURNING id, seller_id, category_id, title, description, price, status, location,
		           views_count, favorites_count, created_at, updated_at, deleted_at`,
		patch.Title, patch.Description, patch.Price, patch.CategoryID, patch.Location,
		id, sellerID, models.AdStatusDeleted,
	)
	updated, err := scanAdRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Ad{}, ads.ErrAdNotFound
		}
		return models.Ad{}, err
	}

	if patch.Photos != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM ad_photos WHERE ad_id = $1`, id); err != nil {
			return models.Ad{}, err
		}
		for idx, photoURL := range patch.Photos {
			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO ad_photos (ad_id, url, position) VALUES ($1, $2, $3)`,
				id, photoURL, idx,
			); err != nil {
				return models.Ad{}, err
			}
		}
		updated.Photos = patch.Photos
	}

	if err := tx.Commit(); err != nil {
		return models.Ad{}, err
	}

	if patch.Photos == nil {
		updated.Photos = r.loadPhotos(ctx, id)
	}
	return updated, nil
}

func (r *AdsRepository) updateStatus(id, sellerID int64, status string) (models.Ad, error) {
	ctx := context.Background()
	if err := r.checkOwner(ctx, id, sellerID); err != nil {
		return models.Ad{}, err
	}

	row := r.db.QueryRowContext(
		ctx,
		`UPDATE ads
		 SET status = $1, updated_at = NOW()
		 WHERE id = $2 AND seller_id = $3 AND status <> $4
		 RETURNING id, seller_id, category_id, title, description, price, status, location,
		           views_count, favorites_count, created_at, updated_at, deleted_at`,
		status, id, sellerID, models.AdStatusDeleted,
	)
	updated, err := scanAdRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Ad{}, ads.ErrAdNotFound
		}
		return models.Ad{}, err
	}
	updated.Photos = r.loadPhotos(ctx, id)
	return updated, nil
}

func (r *AdsRepository) Close(id int64, sellerID int64) (models.Ad, error) {
	r.log.Debug("db query: Ads.Close", slog.Int64("ad_id", id))
	return r.updateStatus(id, sellerID, models.AdStatusClosed)
}

func (r *AdsRepository) Reopen(id int64, sellerID int64) (models.Ad, error) {
	r.log.Debug("db query: Ads.Reopen", slog.Int64("ad_id", id))
	return r.updateStatus(id, sellerID, models.AdStatusActive)
}

func (r *AdsRepository) MarkSold(ids []int64) error {
	ctx := context.Background()
	r.log.Debug("db query: Ads.MarkSold", slog.Int("count", len(ids)))

	for _, id := range ids {
		if _, err := r.db.ExecContext(
			ctx,
			`UPDATE ads SET status = $1, updated_at = NOW() WHERE id = $2 AND status <> $3`,
			models.AdStatusSold, id, models.AdStatusDeleted,
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *AdsRepository) Delete(id int64, sellerID int64) error {
	ctx := context.Background()
	r.log.Debug("db query: Ads.Delete", slog.Int64("ad_id", id))

	if err := r.checkOwner(ctx, id, sellerID); err != nil {
		return err
	}

	_, err := r.db.ExecContext(
		ctx,
		`UPDATE ads SET status = $1, deleted_at = NOW(), updated_at = NOW() WHERE id = $2 AND seller_id = $3`,
		models.AdStatusDeleted, id, sellerID,
	)
	return err
}
