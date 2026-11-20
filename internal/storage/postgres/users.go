package postgres

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"lavka/internal/domain/models"
	"lavka/internal/logger"
	"lavka/internal/storage"
)

type UserRepository struct {
	db  *sql.DB
	log *slog.Logger
}

func NewUserRepository(db *sql.DB, log *slog.Logger) *UserRepository {
	return &UserRepository{db: db, log: log}
}

func (r *UserRepository) SaveUser(ctx context.Context, email string, passHash []byte, name string) (int64, error) {
	log := logger.FromContext(ctx, r.log)
	log.Debug("db query: SaveUser", slog.String("email", email))

	var id int64
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO users (name, email, pass_hash) VALUES ($1, $2, $3) RETURNING id`,
		name, email, passHash,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, storage.ErrUserExists
		}
		log.Error("db error: SaveUser", slog.String("error", err.Error()))
		return 0, err
	}
	return id, nil
}

func (r *UserRepository) User(ctx context.Context, email string) (models.User, error) {
	log := logger.FromContext(ctx, r.log)
	log.Debug("db query: User", slog.String("email", email))

	var u models.User
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, name, email, pass_hash, avatar_path, rating, reviews_count, is_admin, created_at, updated_at
		 FROM users WHERE email = $1`,
		email,
	).Scan(
		&u.ID, &u.Name, &u.Email, &u.PassHash, &u.AvatarPath,
		&u.Rating, &u.ReviewsCount, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, storage.ErrUserNotFound
		}
		log.Error("db error: User", slog.String("error", err.Error()))
		return models.User{}, err
	}
	return u, nil
}

func (r *UserRepository) UserByID(ctx context.Context, userID int64) (models.User, error) {
	log := logger.FromContext(ctx, r.log)
	log.Debug("db query: UserByID", slog.Int64("user_id", userID))

	var u models.User
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, name, email, pass_hash, avatar_path, rating, reviews_count, is_admin, created_at, updated_at
		 FROM users WHERE id = $1`,
		userID,
	).Scan(
		&u.ID, &u.Name, &u.Email, &u.PassHash, &u.AvatarPath,
		&u.Rating, &u.ReviewsCount, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, storage.ErrUserNotFound
		}
		log.Error("db error: UserByID", slog.String("error", err.Error()))
		return models.User{}, err
	}
	return u, nil
}

func (r *UserRepository) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	log := logger.FromContext(ctx, r.log)
	log.Debug("db query: IsAdmin", slog.Int64("user_id", userID))

	var isAdmin bool
	err := r.db.QueryRowContext(ctx, `SELECT is_admin FROM users WHERE id = $1`, userID).Scan(&isAdmin)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, storage.ErrUserNotFound
		}
		return false, err
	}
	return isAdmin, nil
}

func (r *UserRepository) UpdateUser(ctx context.Context, userID int64, name string, email string) (models.User, error) {
	log := logger.FromContext(ctx, r.log)
	log.Debug("db query: UpdateUser", slog.Int64("user_id", userID))

	var u models.User
	err := r.db.QueryRowContext(
		ctx,
		`UPDATE users
		 SET name = COALESCE(NULLIF($1, ''), name),
		     email = COALESCE(NULLIF($2, ''), email),
		     updated_at = NOW()
		 WHERE id = $3
		 RETURNING id, name, email, pass_hash, avatar_path, rating, reviews_count, is_admin, created_at, updated_at`,
		name, email, userID,
	).Scan(
		&u.ID, &u.Name, &u.Email, &u.PassHash, &u.AvatarPath,
		&u.Rating, &u.ReviewsCount, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return models.User{}, storage.ErrUserExists
		}
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, storage.ErrUserNotFound
		}
		log.Error("db error: UpdateUser", slog.String("error", err.Error()))
		return models.User{}, err
	}
	return u, nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID int64, passHash []byte) error {
	log := logger.FromContext(ctx, r.log)
	log.Debug("db query: UpdatePassword", slog.Int64("user_id", userID))

	res, err := r.db.ExecContext(
		ctx,
		`UPDATE users SET pass_hash = $1, updated_at = NOW() WHERE id = $2`,
		passHash, userID,
	)
	if err != nil {
		log.Error("db error: UpdatePassword", slog.String("error", err.Error()))
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return storage.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) UpdateAvatarPath(ctx context.Context, userID int64, avatarPath string) (models.User, error) {
	log := logger.FromContext(ctx, r.log)
	log.Debug("db query: UpdateAvatarPath", slog.Int64("user_id", userID))

	var u models.User
	err := r.db.QueryRowContext(
		ctx,
		`UPDATE users
		 SET avatar_path = $1, updated_at = NOW()
		 WHERE id = $2
		 RETURNING id, name, email, pass_hash, avatar_path, rating, reviews_count, is_admin, created_at, updated_at`,
		avatarPath, userID,
	).Scan(
		&u.ID, &u.Name, &u.Email, &u.PassHash, &u.AvatarPath,
		&u.Rating, &u.ReviewsCount, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, storage.ErrUserNotFound
		}
		log.Error("db error: UpdateAvatarPath", slog.String("error", err.Error()))
		return models.User{}, err
	}
	return u, nil
}
