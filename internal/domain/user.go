package domain

import (
	"errors"
	"time"
)

const RolePentester = "pentester"

// ErrEmailConflict is returned when a user with the same email already exists.
var ErrEmailConflict = errors.New("email already in use")

// ErrUsernameConflict is returned when a user with the same username already exists.
var ErrUsernameConflict = errors.New("username already in use")

// User is the core identity entity used for authentication and authorization.
type User struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RefreshToken stores revocable long-lived auth credentials.
type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}
