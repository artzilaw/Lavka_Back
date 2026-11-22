package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"lavka/internal/domain/models"
	"lavka/internal/storage/ads"
)

const adSelectCols = `id, seller_id, category_id, title, description, price, status, location,
	views_count, favorites_count, promotion_tier, promotion_priority, promoted_until,
	created_at, updated_at, deleted_at`

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
	var promotedUntil sql.NullTime
	var deletedAt sql.NullTime
	err := scanner.Scan(
		&ad.ID, &ad.SellerID, &ad.CategoryID, &ad.Title, &ad.Description,
		&ad.Price, &ad.Status, &ad.Location, &ad.ViewsCount, &ad.FavoritesCount,
		&ad.PromotionTier, &ad.PromotionPriority, &promotedUntil,
		&ad.CreatedAt, &ad.UpdatedAt, &deletedAt,
	)
	if promotedUntil.Valid {
		t := promotedUntil.Time
		ad.PromotedUntil = &t
	}
	if deletedAt.Valid {
		ad.DeletedAt = deletedAt.Time
	}
	return ad, err
}

func (r *AdsRepository) GetAll() []models.Ad {
	return r.Search(models.AdFilter{})
}

func (r *AdsRepository) Search(filter models.AdFilter) []models.Ad {
	ctx := context.Background()
	r.log.Debug("db query: Ads.Search", slog.String("q", filter.Query), slog.Int64("category_id", filter.CategoryID))

	where := []string{"status = $1"}
	args := []any{models.AdStatusActive}
	idx := 2

	if q := strings.TrimSpace(filter.Query); q != "" {
		where = append(where, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d OR location ILIKE $%d)", idx, idx, idx))
		args = append(args, "%"+q+"%")
		idx++
	}
	if filter.CategoryID > 0 {
		where = append(where, fmt.Sprintf("category_id = $%d", idx))
		args = append(args, filter.CategoryID)
		idx++
	}
	if loc := strings.TrimSpace(filter.Location); loc != "" {
		where = append(where, fmt.Sprintf("location ILIKE $%d", idx))
		args = append(args, "%"+loc+"%")
		idx++
	}
	if filter.MinPrice != nil {
		where = append(where, fmt.Sprintf("price >= $%d", idx))
		args = append(args, *filter.MinPrice)
		idx++
	}
	if filter.MaxPrice != nil {
		where = append(where, fmt.Sprintf("price <= $%d", idx))
		args = append(args, *filter.MaxPrice)
		idx++
	}

	orderBy := `CASE WHEN promoted_until IS NOT NULL AND promoted_until > NOW() THEN promotion_priority ELSE 0 END DESC, created_at DESC, id DESC`
	switch filter.Sort {
	case "price_asc":
		orderBy = "price ASC, created_at DESC"
	case "price_desc":
		orderBy = "price DESC, created_at DESC"
	case "popular":
		orderBy = "views_count DESC, created_at DESC"
	}

	query := fmt.Sprintf(
		`SELECT %s FROM ads WHERE %s ORDER BY %s`,
		adSelectCols,
		strings.Join(where, " AND "),
		orderBy,
	)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		r.log.Error("db error: Ads.Search", slog.String("error", err.Error()))
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
		`SELECT `+adSelectCols+` FROM ads WHERE id = $1 AND status <> $2`,
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
		`UPDATE ads SET views_count = views_count + 1 WHERE id = $1 AND status <> $2 RETURNING `+adSelectCols,
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

	query := `SELECT ` + adSelectCols + ` FROM ads WHERE seller_id = $1 AND status <> $2`
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
		`SELECT COALESCE(SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END), 0),
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
		 RETURNING `+adSelectCols,
		ad.SellerID, ad.CategoryID, ad.Title, ad.Description, ad.Price, models.AdStatusActive, ad.Location,
	)
	created, err := scanAdRow(row)
	if err != nil {
		return models.Ad{}, err
	}

	// Записываем стартовую точку истории цены
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO ad_price_history (ad_id, old_price, new_price) VALUES ($1, $2, $3)`,
		created.ID, created.Price, created.Price,
	); err != nil {
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

	var oldPrice int64
	if err := tx.QueryRowContext(
		ctx,
		`SELECT price FROM ads WHERE id = $1 AND seller_id = $2 AND status <> $3 FOR UPDATE`,
		id, sellerID, models.AdStatusDeleted,
	).Scan(&oldPrice); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Ad{}, ads.ErrAdNotFound
		}
		return models.Ad{}, err
	}

	row := tx.QueryRowContext(
		ctx,
		`UPDATE ads
		 SET title = $1, description = $2, price = $3, category_id = $4, location = $5, updated_at = NOW()
		 WHERE id = $6 AND seller_id = $7 AND status <> $8
		 RETURNING `+adSelectCols,
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

	if oldPrice != patch.Price {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO ad_price_history (ad_id, old_price, new_price) VALUES ($1, $2, $3)`,
			id, oldPrice, patch.Price,
		); err != nil {
			return models.Ad{}, err
		}
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
		`UPDATE ads SET status = $1, updated_at = NOW() WHERE id = $2 AND seller_id = $3 AND status <> $4 RETURNING `+adSelectCols,
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

func (r *AdsRepository) GetPriceHistory(adID int64) []models.PriceHistoryItem {
	ctx := context.Background()
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, ad_id, old_price, new_price, changed_at
		 FROM ad_price_history
		 WHERE ad_id = $1
		 ORDER BY changed_at ASC, id ASC`,
		adID,
	)
	if err != nil {
		return []models.PriceHistoryItem{}
	}
	defer rows.Close()

	res := make([]models.PriceHistoryItem, 0)
	for rows.Next() {
		var item models.PriceHistoryItem
		if err := rows.Scan(&item.ID, &item.AdID, &item.OldPrice, &item.NewPrice, &item.ChangedAt); err != nil {
			continue
		}
		res = append(res, item)
	}
	return res
}

func (r *AdsRepository) Promote(id int64, sellerID int64, tier string, priority int, until time.Time) (models.Ad, error) {
	ctx := context.Background()
	if err := r.checkOwner(ctx, id, sellerID); err != nil {
		return models.Ad{}, err
	}

	row := r.db.QueryRowContext(
		ctx,
		`UPDATE ads
		 SET promotion_tier = $1, promotion_priority = $2, promoted_until = $3, updated_at = NOW()
		 WHERE id = $4 AND seller_id = $5 AND status <> $6
		 RETURNING `+adSelectCols,
		tier, priority, until, id, sellerID, models.AdStatusDeleted,
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
