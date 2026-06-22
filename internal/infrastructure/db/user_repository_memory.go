package db

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"apant_be/internal/domain"
)

type MemoryUserRepository struct {
	mu              sync.RWMutex
	usersByUsername map[string]domain.User
	usersByEmail    map[string]domain.User
	usersByID       map[string]domain.User
	tokensByHash    map[string]domain.RefreshToken
	tokensByID      map[string]domain.RefreshToken
}

func NewMemoryUserRepository() *MemoryUserRepository {
	return &MemoryUserRepository{
		usersByUsername: make(map[string]domain.User),
		usersByEmail:    make(map[string]domain.User),
		usersByID:       make(map[string]domain.User),
		tokensByHash:    make(map[string]domain.RefreshToken),
		tokensByID:      make(map[string]domain.RefreshToken),
	}
}

func (r *MemoryUserRepository) Create(_ context.Context, user domain.User) error {
	username := strings.ToLower(strings.TrimSpace(user.Username))
	email := strings.ToLower(strings.TrimSpace(user.Email))
	if username == "" {
		return fmt.Errorf("username is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.usersByUsername[username]; exists {
		return domain.ErrUsernameConflict
	}
	if email != "" {
		if _, exists := r.usersByEmail[email]; exists {
			return domain.ErrEmailConflict
		}
	}

	user.Username = username
	user.Email = email
	r.usersByUsername[username] = user
	if email != "" {
		r.usersByEmail[email] = user
	}
	r.usersByID[user.ID] = user
	return nil
}

func (r *MemoryUserRepository) FindByUsername(_ context.Context, username string) (domain.User, bool, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return domain.User{}, false, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.usersByUsername[username]
	if !ok {
		return domain.User{}, false, nil
	}

	return user, true, nil
}

func (r *MemoryUserRepository) FindByEmail(_ context.Context, email string) (domain.User, bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return domain.User{}, false, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.usersByEmail[email]
	if !ok {
		return domain.User{}, false, nil
	}

	return user, true, nil
}

func (r *MemoryUserRepository) FindByID(_ context.Context, id string) (domain.User, bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.User{}, false, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.usersByID[id]
	if !ok {
		return domain.User{}, false, nil
	}

	return user, true, nil
}

func (r *MemoryUserRepository) UpdateProfile(_ context.Context, userID, username, email string) error {
	userID = strings.TrimSpace(userID)
	username = strings.ToLower(strings.TrimSpace(username))
	email = strings.ToLower(strings.TrimSpace(email))
	if userID == "" {
		return fmt.Errorf("user id is required")
	}
	if username == "" {
		return fmt.Errorf("username is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.usersByID[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}

	if existing, exists := r.usersByUsername[username]; exists && existing.ID != userID {
		return domain.ErrUsernameConflict
	}
	if email != "" {
		if existing, exists := r.usersByEmail[email]; exists && existing.ID != userID {
			return domain.ErrEmailConflict
		}
	}

	delete(r.usersByUsername, strings.ToLower(strings.TrimSpace(user.Username)))
	if oldEmail := strings.ToLower(strings.TrimSpace(user.Email)); oldEmail != "" {
		delete(r.usersByEmail, oldEmail)
	}

	user.Username = username
	user.Email = email
	user.UpdatedAt = time.Now()
	r.usersByUsername[username] = user
	if email != "" {
		r.usersByEmail[email] = user
	}
	r.usersByID[userID] = user
	return nil
}

func (r *MemoryUserRepository) UpdatePassword(_ context.Context, userID, passwordHash string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("user id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.usersByID[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}

	user.PasswordHash = passwordHash
	user.UpdatedAt = time.Now()
	r.usersByID[userID] = user
	if uname := strings.ToLower(strings.TrimSpace(user.Username)); uname != "" {
		r.usersByUsername[uname] = user
	}
	if email := strings.ToLower(strings.TrimSpace(user.Email)); email != "" {
		r.usersByEmail[email] = user
	}
	return nil
}

func (r *MemoryUserRepository) CreateRefreshToken(_ context.Context, token domain.RefreshToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tokensByHash[token.TokenHash]; exists {
		return fmt.Errorf("refresh token already exists")
	}

	r.tokensByHash[token.TokenHash] = token
	r.tokensByID[token.ID] = token
	return nil
}

func (r *MemoryUserRepository) FindRefreshTokenByHash(_ context.Context, tokenHash string) (domain.RefreshToken, bool, error) {
	tokenHash = strings.TrimSpace(tokenHash)
	if tokenHash == "" {
		return domain.RefreshToken{}, false, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	token, ok := r.tokensByHash[tokenHash]
	if !ok {
		return domain.RefreshToken{}, false, nil
	}

	return token, true, nil
}

func (r *MemoryUserRepository) RevokeRefreshToken(_ context.Context, tokenID string) error {
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	token, ok := r.tokensByID[tokenID]
	if !ok {
		return nil
	}

	now := time.Now()
	token.RevokedAt = &now
	token.UpdatedAt = now
	r.tokensByID[tokenID] = token
	r.tokensByHash[token.TokenHash] = token
	return nil
}

func (r *MemoryUserRepository) RevokeUserRefreshTokens(_ context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for id, token := range r.tokensByID {
		if token.UserID != userID || token.RevokedAt != nil {
			continue
		}
		token.RevokedAt = &now
		token.UpdatedAt = now
		r.tokensByID[id] = token
		r.tokensByHash[token.TokenHash] = token
	}

	return nil
}
