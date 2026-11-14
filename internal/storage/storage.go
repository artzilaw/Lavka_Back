package storage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lavka/internal/domain/models"
)

var (
	ErrUserExists   = errors.New("user already exists")
	ErrUserNotFound = errors.New("user not found")
)

type Storage struct {
	mu           sync.RWMutex
	usersByEmail map[string]models.User
	usersByID    map[int64]models.User
	nextUserID   int64
	path         string
}

type dump struct {
	Users      map[string]models.User `json:"users"`
	UsersByID  map[int64]models.User  `json:"users_by_id"`
	PassHashes map[int64][]byte       `json:"pass_hashes"`
	NextUserID int64                  `json:"next_user_id"`
}

func New(path string) (*Storage, error) {
	s := &Storage{
		usersByEmail: make(map[string]models.User),
		usersByID:    make(map[int64]models.User),
		path:         path,
	}

	if path == "" {
		return s, nil
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(path); err == nil {
		if err := s.load(); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	return s, nil
}

func (s *Storage) load() error {
	f, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer f.Close()

	var d dump
	if err := json.NewDecoder(f).Decode(&d); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}

	if d.UsersByID != nil {
		s.usersByID = d.UsersByID
	}
	if d.Users != nil {
		s.usersByEmail = d.Users
	}
	for id, hash := range d.PassHashes {
		if u, ok := s.usersByID[id]; ok {
			u.PassHash = hash
			s.usersByID[id] = u
			s.usersByEmail[u.Email] = u
		}
	}
	s.nextUserID = d.NextUserID

	return nil
}

func (s *Storage) persist() error {
	if s.path == "" {
		return nil
	}

	passHashes := make(map[int64][]byte, len(s.usersByID))
	for id, u := range s.usersByID {
		passHashes[id] = u.PassHash
	}

	d := dump{
		Users:      s.usersByEmail,
		UsersByID:  s.usersByID,
		PassHashes: passHashes,
		NextUserID: s.nextUserID,
	}

	f, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(&d)
}

func (s *Storage) SaveUser(ctx context.Context, email string, passHash []byte, name string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.usersByEmail[email]; exists {
		return 0, ErrUserExists
	}

	s.nextUserID++
	now := time.Now()
	u := models.User{
		ID:        s.nextUserID,
		Name:      name,
		Email:     email,
		PassHash:  passHash,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.usersByEmail[email] = u
	s.usersByID[u.ID] = u

	if err := s.persist(); err != nil {
		delete(s.usersByEmail, email)
		delete(s.usersByID, u.ID)
		s.nextUserID--
		return 0, err
	}

	return u.ID, nil
}

func (s *Storage) User(ctx context.Context, email string) (models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.usersByEmail[email]
	if !ok {
		return models.User{}, ErrUserNotFound
	}
	return u, nil
}

func (s *Storage) UserByID(ctx context.Context, userID int64) (models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.usersByID[userID]
	if !ok {
		return models.User{}, ErrUserNotFound
	}
	return u, nil
}

func (s *Storage) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.usersByID[userID]
	if !ok {
		return false, ErrUserNotFound
	}
	return u.IsAdmin, nil
}

func (s *Storage) UpdateUser(ctx context.Context, userID int64, name string, email string) (models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.usersByID[userID]
	if !ok {
		return models.User{}, ErrUserNotFound
	}

	oldEmail := u.Email
	if email != "" && email != oldEmail {
		if existing, exists := s.usersByEmail[email]; exists && existing.ID != userID {
			return models.User{}, ErrUserExists
		}
		delete(s.usersByEmail, oldEmail)
		u.Email = email
	}

	if name != "" {
		u.Name = name
	}

	u.UpdatedAt = time.Now()
	s.usersByID[userID] = u
	s.usersByEmail[u.Email] = u

	if err := s.persist(); err != nil {
		return models.User{}, err
	}

	return u, nil
}

func (s *Storage) UpdatePassword(ctx context.Context, userID int64, passHash []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.usersByID[userID]
	if !ok {
		return ErrUserNotFound
	}

	u.PassHash = passHash
	u.UpdatedAt = time.Now()
	s.usersByID[userID] = u
	s.usersByEmail[u.Email] = u

	return s.persist()
}

func (s *Storage) UpdateAvatarPath(ctx context.Context, userID int64, avatarPath string) (models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.usersByID[userID]
	if !ok {
		return models.User{}, ErrUserNotFound
	}

	u.AvatarPath = avatarPath
	u.UpdatedAt = time.Now()
	s.usersByID[userID] = u
	s.usersByEmail[u.Email] = u

	if err := s.persist(); err != nil {
		return models.User{}, err
	}

	return u, nil
}
