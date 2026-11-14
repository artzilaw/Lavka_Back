package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestStorage_SaveAndGet(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "users.json")

	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	uid, err := s.SaveUser(ctx, "test@mail.ru", []byte("hash123"), "Ivan")
	if err != nil {
		t.Fatalf("unexpected error on SaveUser: %v", err)
	}

	// Повторное сохранение того же email должно вернуть ErrUserExists
	_, err = s.SaveUser(ctx, "test@mail.ru", []byte("hash123"), "Ivan")
	if !errors.Is(err, ErrUserExists) {
		t.Errorf("expected ErrUserExists, got %v", err)
	}

	// Поиск по email
	u, err := s.User(ctx, "test@mail.ru")
	if err != nil || u.ID != uid || u.Name != "Ivan" {
		t.Errorf("unexpected user by email: %+v, err: %v", u, err)
	}

	// Поиск по ID
	uByID, err := s.UserByID(ctx, uid)
	if err != nil || uByID.Email != "test@mail.ru" {
		t.Errorf("unexpected user by ID: %+v, err: %v", uByID, err)
	}

	// Обновление имени и email
	updated, err := s.UpdateUser(ctx, uid, "Ivan Petrov", "ivan.petrov@mail.ru")
	if err != nil || updated.Name != "Ivan Petrov" || updated.Email != "ivan.petrov@mail.ru" {
		t.Fatalf("UpdateUser failed: %+v, err: %v", updated, err)
	}

	// Старый email больше не должен находиться
	if _, err := s.User(ctx, "test@mail.ru"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound for old email, got %v", err)
	}

	// Обновление пароля и аватарки
	if err := s.UpdatePassword(ctx, uid, []byte("newhash456")); err != nil {
		t.Fatalf("UpdatePassword failed: %v", err)
	}
	withAvatar, err := s.UpdateAvatarPath(ctx, uid, "/static/img/avatars/1.webp")
	if err != nil || withAvatar.AvatarPath != "/static/img/avatars/1.webp" {
		t.Fatalf("UpdateAvatarPath failed: %+v, err: %v", withAvatar, err)
	}

	// Проверка перезагрузки из файла (persistence)
	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to reload storage: %v", err)
	}
	uReloaded, err := s2.User(ctx, "ivan.petrov@mail.ru")
	if err != nil || uReloaded.ID != uid || string(uReloaded.PassHash) != "newhash456" {
		t.Errorf("expected user to persist across reloads, got %+v, err: %v", uReloaded, err)
	}

	// Несуществующий пользователь
	_, err = s.User(ctx, "nonexistent@mail.ru")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}
