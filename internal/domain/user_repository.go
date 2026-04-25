package domain

import "context"

// UserRepository defines persistence contract for auth users.
type UserRepository interface {
	Create(ctx context.Context, user User) error
	FindByUsername(ctx context.Context, username string) (User, bool, error)
	FindByID(ctx context.Context, id string) (User, bool, error)
	CreateRefreshToken(ctx context.Context, token RefreshToken) error
	FindRefreshTokenByHash(ctx context.Context, tokenHash string) (RefreshToken, bool, error)
	RevokeRefreshToken(ctx context.Context, tokenID string) error
	RevokeUserRefreshTokens(ctx context.Context, userID string) error
}
