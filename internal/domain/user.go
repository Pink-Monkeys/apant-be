package domain

import (
	"errors"
	"time"
)

const (
	RolePentester = "pentester"
	// RoleAdmin can manage LLM providers/models (CRUD) in addition to running
	// scans. Pentesters may only select an existing provider/model.
	RoleAdmin = "admin"
)

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
	// SessionStartedAt is the time the login session began. It is carried across
	// token rotations so an absolute session lifetime can be enforced regardless
	// of how often the token slides.
	SessionStartedAt time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
