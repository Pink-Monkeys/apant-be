package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"apant_be/internal/domain"
)

type userModel struct {
	ID           string    `gorm:"column:id;type:text;primaryKey"`
	Username     string    `gorm:"column:username;type:text;uniqueIndex;not null"`
	Email        string    `gorm:"column:email;type:text"`
	PasswordHash string    `gorm:"column:password_hash;type:text;not null"`
	Role         string    `gorm:"column:role;type:text;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

func (userModel) TableName() string {
	return "users"
}

type refreshTokenModel struct {
	ID        string     `gorm:"column:id;type:text;primaryKey"`
	UserID    string     `gorm:"column:user_id;type:text;index;not null"`
	TokenHash string     `gorm:"column:token_hash;type:text;uniqueIndex;not null"`
	ExpiresAt time.Time  `gorm:"column:expires_at;index;not null"`
	RevokedAt *time.Time `gorm:"column:revoked_at;index"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt time.Time  `gorm:"column:updated_at;not null"`
}

func (refreshTokenModel) TableName() string {
	return "refresh_tokens"
}

type PostgresUserRepository struct {
	db *Postgres
}

func NewPostgresUserRepository(db *Postgres) (*PostgresUserRepository, error) {
	if db == nil || db.DB == nil {
		return nil, fmt.Errorf("postgres db is not initialized")
	}

	return &PostgresUserRepository{db: db}, nil
}

func (r *PostgresUserRepository) Create(ctx context.Context, user domain.User) error {
	model := userModel{
		ID:           user.ID,
		Username:     strings.ToLower(strings.TrimSpace(user.Username)),
		Email:        strings.ToLower(strings.TrimSpace(user.Email)),
		PasswordHash: user.PasswordHash,
		Role:         user.Role,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}

	if err := r.db.DB.WithContext(ctx).Create(&model).Error; err != nil {
		return err
	}
	return nil
}

func (r *PostgresUserRepository) FindByUsername(ctx context.Context, username string) (domain.User, bool, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return domain.User{}, false, nil
	}

	var model userModel
	if err := r.db.DB.WithContext(ctx).Where("username = ?", username).First(&model).Error; err != nil {
		if isNotFound(err) {
			return domain.User{}, false, nil
		}
		return domain.User{}, false, err
	}

	return toDomainUser(model), true, nil
}

func (r *PostgresUserRepository) FindByEmail(ctx context.Context, email string) (domain.User, bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return domain.User{}, false, nil
	}

	var model userModel
	if err := r.db.DB.WithContext(ctx).Where("email = ?", email).First(&model).Error; err != nil {
		if isNotFound(err) {
			return domain.User{}, false, nil
		}
		return domain.User{}, false, err
	}

	return toDomainUser(model), true, nil
}

func (r *PostgresUserRepository) FindByID(ctx context.Context, id string) (domain.User, bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.User{}, false, nil
	}

	var model userModel
	if err := r.db.DB.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		if isNotFound(err) {
			return domain.User{}, false, nil
		}
		return domain.User{}, false, err
	}

	return toDomainUser(model), true, nil
}

func (r *PostgresUserRepository) CreateRefreshToken(ctx context.Context, token domain.RefreshToken) error {
	model := refreshTokenModel{
		ID:        token.ID,
		UserID:    token.UserID,
		TokenHash: token.TokenHash,
		ExpiresAt: token.ExpiresAt,
		RevokedAt: token.RevokedAt,
		CreatedAt: token.CreatedAt,
		UpdatedAt: token.UpdatedAt,
	}

	return r.db.DB.WithContext(ctx).Create(&model).Error
}

func (r *PostgresUserRepository) FindRefreshTokenByHash(ctx context.Context, tokenHash string) (domain.RefreshToken, bool, error) {
	tokenHash = strings.TrimSpace(tokenHash)
	if tokenHash == "" {
		return domain.RefreshToken{}, false, nil
	}

	var model refreshTokenModel
	if err := r.db.DB.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&model).Error; err != nil {
		if isNotFound(err) {
			return domain.RefreshToken{}, false, nil
		}
		return domain.RefreshToken{}, false, err
	}

	return toDomainRefreshToken(model), true, nil
}

func (r *PostgresUserRepository) RevokeRefreshToken(ctx context.Context, tokenID string) error {
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil
	}

	now := time.Now()
	return r.db.DB.WithContext(ctx).
		Model(&refreshTokenModel{}).
		Where("id = ? AND revoked_at IS NULL", tokenID).
		Updates(map[string]any{"revoked_at": now, "updated_at": now}).
		Error
}

func (r *PostgresUserRepository) RevokeUserRefreshTokens(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}

	now := time.Now()
	return r.db.DB.WithContext(ctx).
		Model(&refreshTokenModel{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]any{"revoked_at": now, "updated_at": now}).
		Error
}

func toDomainUser(model userModel) domain.User {
	return domain.User{
		ID:           model.ID,
		Username:     model.Username,
		Email:        model.Email,
		PasswordHash: model.PasswordHash,
		Role:         model.Role,
		CreatedAt:    model.CreatedAt,
		UpdatedAt:    model.UpdatedAt,
	}
}

func toDomainRefreshToken(model refreshTokenModel) domain.RefreshToken {
	return domain.RefreshToken{
		ID:        model.ID,
		UserID:    model.UserID,
		TokenHash: model.TokenHash,
		ExpiresAt: model.ExpiresAt,
		RevokedAt: model.RevokedAt,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}
