// Package user provides admin-only user management (list, change role, delete,
// reset password), separate from the auth package which owns login/session.
package user

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"apant_be/internal/domain"
	appErrors "apant_be/internal/shared/errors"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9_.-]{3,32}$`)

type Service struct {
	users domain.UserRepository
}

func NewService(users domain.UserRepository) *Service {
	return &Service{users: users}
}

// Create makes a new account on an admin's behalf. It validates, checks
// username/email uniqueness, hashes the password, and persists — but never
// issues a session, so the acting admin stays logged in as themselves.
func (s *Service) Create(ctx context.Context, req CreateUserRequest) (UserResponse, error) {
	username := strings.ToLower(strings.TrimSpace(req.Username))
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := strings.TrimSpace(req.Password)
	role := strings.ToLower(strings.TrimSpace(req.Role))

	if role == "" {
		role = domain.RolePentester
	}
	if role != domain.RoleAdmin && role != domain.RolePentester {
		return UserResponse{}, appErrors.New(http.StatusBadRequest, "role must be admin or pentester")
	}
	if err := validateUsername(username); err != nil {
		return UserResponse{}, err
	}
	if err := validateEmail(email); err != nil {
		return UserResponse{}, err
	}
	if err := validatePassword(password); err != nil {
		return UserResponse{}, err
	}

	if _, found, err := s.users.FindByUsername(ctx, username); err != nil {
		return UserResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to read user", err)
	} else if found {
		return UserResponse{}, appErrors.New(http.StatusConflict, "username is already used")
	}
	if _, found, err := s.users.FindByEmail(ctx, email); err != nil {
		return UserResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to read user", err)
	} else if found {
		return UserResponse{}, appErrors.New(http.StatusConflict, "email is already used")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return UserResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to secure password", err)
	}

	now := time.Now()
	newUser := domain.User{
		ID:           uuid.NewString(),
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.users.Create(ctx, newUser); err != nil {
		if errors.Is(err, domain.ErrUsernameConflict) {
			return UserResponse{}, appErrors.New(http.StatusConflict, "username is already used")
		}
		if errors.Is(err, domain.ErrEmailConflict) {
			return UserResponse{}, appErrors.New(http.StatusConflict, "email is already used")
		}
		return UserResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to create user", err)
	}

	return toUserResponse(newUser), nil
}

// List returns every user (passwords never included).
func (s *Service) List(ctx context.Context) ([]UserResponse, error) {
	users, err := s.users.ListUsers(ctx)
	if err != nil {
		return nil, appErrors.Wrap(http.StatusInternalServerError, "failed to list users", err)
	}
	out := make([]UserResponse, 0, len(users))
	for _, u := range users {
		out = append(out, toUserResponse(u))
	}
	return out, nil
}

// UpdateRole changes a user's role. Guards: the acting admin cannot change their
// own role (avoids self-lockout), the role must be valid, and the last admin
// cannot be demoted (keeps the system manageable).
func (s *Service) UpdateRole(ctx context.Context, actingAdminID, targetID, newRole string) (UserResponse, error) {
	targetID = strings.TrimSpace(targetID)
	newRole = strings.ToLower(strings.TrimSpace(newRole))

	if newRole != domain.RoleAdmin && newRole != domain.RolePentester {
		return UserResponse{}, appErrors.New(http.StatusBadRequest, "role must be admin or pentester")
	}
	if targetID == actingAdminID {
		return UserResponse{}, appErrors.New(http.StatusForbidden, "you cannot change your own role")
	}

	target, found, err := s.users.FindByID(ctx, targetID)
	if err != nil {
		return UserResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to load user", err)
	}
	if !found {
		return UserResponse{}, appErrors.New(http.StatusNotFound, "user not found")
	}

	// No-op change: return the current state without touching sessions.
	if target.Role == newRole {
		return toUserResponse(target), nil
	}

	// Demoting an admin: refuse if they are the last one.
	if target.Role == domain.RoleAdmin && newRole != domain.RoleAdmin {
		if err := s.ensureNotLastAdmin(ctx); err != nil {
			return UserResponse{}, err
		}
	}

	if err := s.users.UpdateRole(ctx, targetID, newRole); err != nil {
		return UserResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to update role", err)
	}

	// A role change is security-sensitive: revoke the target's sessions so their
	// old JWT (which still carries the previous role until it expires) cannot keep
	// its former privileges. They must log in again to get a token with the new role.
	_ = s.users.RevokeUserRefreshTokens(ctx, targetID)

	target.Role = newRole
	return toUserResponse(target), nil
}

// Delete removes a user. Guards: an admin cannot delete themselves, and the last
// admin cannot be deleted.
func (s *Service) Delete(ctx context.Context, actingAdminID, targetID string) error {
	targetID = strings.TrimSpace(targetID)
	if targetID == actingAdminID {
		return appErrors.New(http.StatusForbidden, "you cannot delete your own account")
	}

	target, found, err := s.users.FindByID(ctx, targetID)
	if err != nil {
		return appErrors.Wrap(http.StatusInternalServerError, "failed to load user", err)
	}
	if !found {
		return appErrors.New(http.StatusNotFound, "user not found")
	}

	if target.Role == domain.RoleAdmin {
		if err := s.ensureNotLastAdmin(ctx); err != nil {
			return err
		}
	}

	if err := s.users.DeleteUser(ctx, targetID); err != nil {
		return appErrors.Wrap(http.StatusInternalServerError, "failed to delete user", err)
	}
	return nil
}

// ResetPassword sets a new password for any user (admin action; no current
// password required). All the target's sessions are revoked so old tokens die.
func (s *Service) ResetPassword(ctx context.Context, targetID, newPassword string) error {
	targetID = strings.TrimSpace(targetID)
	newPassword = strings.TrimSpace(newPassword)

	if err := validatePassword(newPassword); err != nil {
		return err
	}

	_, found, err := s.users.FindByID(ctx, targetID)
	if err != nil {
		return appErrors.Wrap(http.StatusInternalServerError, "failed to load user", err)
	}
	if !found {
		return appErrors.New(http.StatusNotFound, "user not found")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return appErrors.Wrap(http.StatusInternalServerError, "failed to secure password", err)
	}
	if err := s.users.UpdatePassword(ctx, targetID, string(hash)); err != nil {
		return appErrors.Wrap(http.StatusInternalServerError, "failed to update password", err)
	}

	// Force re-login everywhere with the new password.
	_ = s.users.RevokeUserRefreshTokens(ctx, targetID)
	return nil
}

// ensureNotLastAdmin returns a 409 when only one admin remains, blocking the
// caller from removing the last one and locking everyone out of management.
func (s *Service) ensureNotLastAdmin(ctx context.Context) error {
	count, err := s.users.CountByRole(ctx, domain.RoleAdmin)
	if err != nil {
		return appErrors.Wrap(http.StatusInternalServerError, "failed to count admins", err)
	}
	if count <= 1 {
		return appErrors.New(http.StatusConflict, "cannot remove the last remaining admin")
	}
	return nil
}

// validateUsername mirrors the auth package's rule (3-32 chars, a-z 0-9 _ . -).
func validateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return appErrors.New(http.StatusBadRequest, "username must be 3-32 chars and only a-z 0-9 _ . -")
	}
	return nil
}

func validateEmail(email string) error {
	if email == "" {
		return appErrors.New(http.StatusBadRequest, "email is required")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return appErrors.New(http.StatusBadRequest, "email is invalid")
	}
	return nil
}

// validatePassword mirrors the auth package's rule (8-72 chars, at least one
// letter and one digit).
func validatePassword(password string) error {
	if len(password) < 8 || len(password) > 72 {
		return appErrors.New(http.StatusBadRequest, "password must be 8-72 characters")
	}
	hasLetter, hasDigit := false, false
	for _, r := range password {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return appErrors.New(http.StatusBadRequest, "password must contain letters and digits")
	}
	return nil
}
