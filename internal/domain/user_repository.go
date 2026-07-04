package domain

import "context"

// UserRepository defines persistence contract for auth users.
type UserRepository interface {
	Create(ctx context.Context, user User) error
	FindByUsername(ctx context.Context, username string) (User, bool, error)
	FindByEmail(ctx context.Context, email string) (User, bool, error)
	FindByID(ctx context.Context, id string) (User, bool, error)
	UpdateProfile(ctx context.Context, userID, username, email string) error
	UpdatePassword(ctx context.Context, userID, passwordHash string) error
	// Admin user management.
	ListUsers(ctx context.Context) ([]User, error)
	UpdateRole(ctx context.Context, userID, role string) error
	DeleteUser(ctx context.Context, userID string) error
	// CountByRole supports the "at least one admin must remain" guard.
	CountByRole(ctx context.Context, role string) (int64, error)
	CreateRefreshToken(ctx context.Context, token RefreshToken) error
	FindRefreshTokenByHash(ctx context.Context, tokenHash string) (RefreshToken, bool, error)
	RevokeRefreshToken(ctx context.Context, tokenID string) error
	RevokeUserRefreshTokens(ctx context.Context, userID string) error
}
