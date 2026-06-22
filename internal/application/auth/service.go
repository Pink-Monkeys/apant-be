package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"apant_be/internal/domain"
	appErrors "apant_be/internal/shared/errors"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9_.-]{3,32}$`)

type Service struct {
	users              domain.UserRepository
	jwtSecret          []byte
	accessTokenTTL     time.Duration
	refreshTokenTTL    time.Duration
	absoluteSessionTTL time.Duration
}

func NewService(users domain.UserRepository, jwtSecret string, accessTokenTTL, refreshTokenTTL, absoluteSessionTTL time.Duration) *Service {
	if accessTokenTTL <= 0 {
		accessTokenTTL = 15 * time.Minute
	}

	if refreshTokenTTL <= 0 {
		refreshTokenTTL = 7 * 24 * time.Hour
	}

	if absoluteSessionTTL <= 0 {
		absoluteSessionTTL = 30 * 24 * time.Hour
	}

	return &Service{
		users:              users,
		jwtSecret:          []byte(strings.TrimSpace(jwtSecret)),
		accessTokenTTL:     accessTokenTTL,
		refreshTokenTTL:    refreshTokenTTL,
		absoluteSessionTTL: absoluteSessionTTL,
	}
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (AuthResponse, error) {
	username := strings.ToLower(strings.TrimSpace(req.Username))
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := strings.TrimSpace(req.Password)

	if err := validateRegisterCredentials(username, email, password); err != nil {
		return AuthResponse{}, err
	}

	if _, found, err := s.users.FindByUsername(ctx, username); err != nil {
		return AuthResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to read user", err)
	} else if found {
		return AuthResponse{}, appErrors.New(http.StatusConflict, "username is already used")
	}

	if email != "" {
		if _, found, err := s.users.FindByEmail(ctx, email); err != nil {
			return AuthResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to read user", err)
		} else if found {
			return AuthResponse{}, appErrors.New(http.StatusConflict, "email is already used")
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to secure password", err)
	}

	now := time.Now()
	user := domain.User{
		ID:           uuid.NewString(),
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Role:         domain.RolePentester,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.users.Create(ctx, user); err != nil {
		if errors.Is(err, domain.ErrEmailConflict) {
			return AuthResponse{}, appErrors.New(http.StatusConflict, "email is already used")
		}
		if errors.Is(err, domain.ErrUsernameConflict) {
			return AuthResponse{}, appErrors.New(http.StatusConflict, "username is already used")
		}
		return AuthResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to create user", err)
	}

	return s.issueTokenPair(ctx, user, time.Now())
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (AuthResponse, error) {
	username := strings.ToLower(strings.TrimSpace(req.Username))
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := strings.TrimSpace(req.Password)
	if err := validateLoginCredentials(username, email, password); err != nil {
		return AuthResponse{}, appErrors.New(http.StatusBadRequest, err.Error())
	}

	var (
		user  domain.User
		found bool
		err   error
	)
	if username != "" {
		user, found, err = s.users.FindByUsername(ctx, username)
	} else {
		user, found, err = s.users.FindByEmail(ctx, email)
	}
	if err != nil {
		return AuthResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to read user", err)
	}
	if !found {
		return AuthResponse{}, appErrors.New(http.StatusUnauthorized, "invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return AuthResponse{}, appErrors.New(http.StatusUnauthorized, "invalid credentials")
	}

	return s.issueTokenPair(ctx, user, time.Now())
}

func (s *Service) RefreshToken(ctx context.Context, req RefreshTokenRequest) (AuthResponse, error) {
	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		return AuthResponse{}, appErrors.New(http.StatusBadRequest, "refresh_token is required")
	}

	hash := hashToken(refreshToken)
	storedToken, found, err := s.users.FindRefreshTokenByHash(ctx, hash)
	if err != nil {
		return AuthResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to verify refresh token", err)
	}
	if !found {
		return AuthResponse{}, appErrors.New(http.StatusUnauthorized, "invalid refresh token")
	}

	if storedToken.RevokedAt != nil {
		// A revoked token presented again means it was already rotated — a strong
		// signal of theft. Revoke the whole token family so both the attacker and
		// the legitimate user are forced to log in again (OWASP reuse detection).
		_ = s.users.RevokeUserRefreshTokens(ctx, storedToken.UserID)
		return AuthResponse{}, appErrors.New(http.StatusUnauthorized, "refresh token has been revoked")
	}

	if time.Now().After(storedToken.ExpiresAt) {
		_ = s.users.RevokeRefreshToken(ctx, storedToken.ID)
		return AuthResponse{}, appErrors.New(http.StatusUnauthorized, "refresh token has expired")
	}

	// Absolute session cap: the refresh window may slide, but never beyond a hard
	// limit measured from the original login, forcing a periodic re-login.
	sessionStart := storedToken.SessionStartedAt
	if sessionStart.IsZero() {
		sessionStart = storedToken.CreatedAt
	}
	if time.Since(sessionStart) > s.absoluteSessionTTL {
		_ = s.users.RevokeUserRefreshTokens(ctx, storedToken.UserID)
		return AuthResponse{}, appErrors.New(http.StatusUnauthorized, "session expired, please log in again")
	}

	user, found, err := s.users.FindByID(ctx, storedToken.UserID)
	if err != nil {
		return AuthResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to load user", err)
	}
	if !found {
		return AuthResponse{}, appErrors.New(http.StatusUnauthorized, "invalid refresh token")
	}

	if err := s.users.RevokeRefreshToken(ctx, storedToken.ID); err != nil {
		return AuthResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to rotate refresh token", err)
	}

	return s.issueTokenPair(ctx, user, sessionStart)
}

func (s *Service) Logout(ctx context.Context, userID string, req LogoutRequest) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return appErrors.New(http.StatusUnauthorized, "invalid token subject")
	}

	if err := s.users.RevokeUserRefreshTokens(ctx, userID); err != nil {
		return appErrors.Wrap(http.StatusInternalServerError, "failed to revoke user sessions", err)
	}

	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken != "" {
		hash := hashToken(refreshToken)
		storedToken, found, err := s.users.FindRefreshTokenByHash(ctx, hash)
		if err != nil {
			return appErrors.Wrap(http.StatusInternalServerError, "failed to process refresh token", err)
		}
		if found && storedToken.UserID == userID {
			_ = s.users.RevokeRefreshToken(ctx, storedToken.ID)
		}
	}

	return nil
}

func (s *Service) Me(ctx context.Context, userID string) (AuthUser, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return AuthUser{}, appErrors.New(http.StatusUnauthorized, "invalid token subject")
	}

	user, found, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return AuthUser{}, appErrors.Wrap(http.StatusInternalServerError, "failed to load user", err)
	}
	if !found {
		return AuthUser{}, appErrors.New(http.StatusNotFound, "user not found")
	}

	return AuthUser{ID: user.ID, Username: user.Username, Email: user.Email, Role: user.Role}, nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID string, req UpdateProfileRequest) (AuthUser, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return AuthUser{}, appErrors.New(http.StatusUnauthorized, "invalid token subject")
	}

	username := strings.ToLower(strings.TrimSpace(req.Username))
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if username == "" || email == "" {
		return AuthUser{}, appErrors.New(http.StatusBadRequest, "username and email are required")
	}
	if !usernamePattern.MatchString(username) {
		return AuthUser{}, appErrors.New(http.StatusBadRequest, "username must be 3-32 chars and only a-z 0-9 _ . -")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return AuthUser{}, appErrors.New(http.StatusBadRequest, "email is invalid")
	}

	current, found, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return AuthUser{}, appErrors.Wrap(http.StatusInternalServerError, "failed to load user", err)
	}
	if !found {
		return AuthUser{}, appErrors.New(http.StatusNotFound, "user not found")
	}

	if other, found, err := s.users.FindByUsername(ctx, username); err != nil {
		return AuthUser{}, appErrors.Wrap(http.StatusInternalServerError, "failed to read user", err)
	} else if found && other.ID != userID {
		return AuthUser{}, appErrors.New(http.StatusConflict, "username is already used")
	}
	if other, found, err := s.users.FindByEmail(ctx, email); err != nil {
		return AuthUser{}, appErrors.Wrap(http.StatusInternalServerError, "failed to read user", err)
	} else if found && other.ID != userID {
		return AuthUser{}, appErrors.New(http.StatusConflict, "email is already used")
	}

	if err := s.users.UpdateProfile(ctx, userID, username, email); err != nil {
		if errors.Is(err, domain.ErrUsernameConflict) {
			return AuthUser{}, appErrors.New(http.StatusConflict, "username is already used")
		}
		if errors.Is(err, domain.ErrEmailConflict) {
			return AuthUser{}, appErrors.New(http.StatusConflict, "email is already used")
		}
		return AuthUser{}, appErrors.Wrap(http.StatusInternalServerError, "failed to update profile", err)
	}

	return AuthUser{ID: userID, Username: username, Email: email, Role: current.Role}, nil
}

// ChangePassword verifies the current password, sets a new one, revokes every
// existing refresh token (logging out all other devices), then issues a fresh
// token pair so the current device stays authenticated.
func (s *Service) ChangePassword(ctx context.Context, userID string, req ChangePasswordRequest) (AuthResponse, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return AuthResponse{}, appErrors.New(http.StatusUnauthorized, "invalid token subject")
	}

	currentPassword := strings.TrimSpace(req.CurrentPassword)
	newPassword := strings.TrimSpace(req.NewPassword)
	if currentPassword == "" || newPassword == "" {
		return AuthResponse{}, appErrors.New(http.StatusBadRequest, "current_password and new_password are required")
	}
	if err := validatePassword(newPassword); err != nil {
		return AuthResponse{}, err
	}

	user, found, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return AuthResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to load user", err)
	}
	if !found {
		return AuthResponse{}, appErrors.New(http.StatusNotFound, "user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return AuthResponse{}, appErrors.New(http.StatusUnauthorized, "current password is incorrect")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(newPassword)) == nil {
		return AuthResponse{}, appErrors.New(http.StatusBadRequest, "new password must be different from the current password")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return AuthResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to secure password", err)
	}

	if err := s.users.UpdatePassword(ctx, userID, string(hash)); err != nil {
		return AuthResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to update password", err)
	}

	if err := s.users.RevokeUserRefreshTokens(ctx, userID); err != nil {
		return AuthResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to revoke sessions", err)
	}

	user.PasswordHash = string(hash)
	return s.issueTokenPair(ctx, user, time.Now())
}

func (s *Service) issueTokenPair(ctx context.Context, user domain.User, sessionStartedAt time.Time) (AuthResponse, error) {
	if len(s.jwtSecret) == 0 {
		return AuthResponse{}, appErrors.New(http.StatusInternalServerError, "jwt secret is not configured")
	}

	now := time.Now()
	if sessionStartedAt.IsZero() {
		sessionStartedAt = now
	}
	expiresAt := now.Add(s.accessTokenTTL)
	claims := jwt.MapClaims{
		"sub":  user.ID,
		"usr":  user.Username,
		"role": user.Role,
		"iat":  now.Unix(),
		"exp":  expiresAt.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return AuthResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to sign token", err)
	}

	rawRefreshToken, err := generateRefreshToken()
	if err != nil {
		return AuthResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to generate refresh token", err)
	}

	refreshToken := domain.RefreshToken{
		ID:               uuid.NewString(),
		UserID:           user.ID,
		TokenHash:        hashToken(rawRefreshToken),
		ExpiresAt:        now.Add(s.refreshTokenTTL),
		SessionStartedAt: sessionStartedAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.users.CreateRefreshToken(ctx, refreshToken); err != nil {
		return AuthResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to store refresh token", err)
	}

	return AuthResponse{
		AccessToken:  signed,
		RefreshToken: rawRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.accessTokenTTL.Seconds()),
		User: AuthUser{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		},
	}, nil
}

func hashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func validateRegisterCredentials(username, email, password string) error {
	if username == "" || email == "" || password == "" {
		return appErrors.New(http.StatusBadRequest, "username, email and password are required")
	}
	if username != "" && !usernamePattern.MatchString(username) {
		return appErrors.New(http.StatusBadRequest, "username must be 3-32 chars and only a-z 0-9 _ . -")
	}
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			return appErrors.New(http.StatusBadRequest, "email is invalid")
		}
	}
	if err := validatePassword(password); err != nil {
		return err
	}

	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 || len(password) > 72 {
		return appErrors.New(http.StatusBadRequest, "password must be 8-72 characters")
	}

	hasLetter := false
	hasDigit := false
	for _, r := range password {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			hasLetter = true
		}
		if r >= '0' && r <= '9' {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return appErrors.New(http.StatusBadRequest, "password must contain letters and digits")
	}

	return nil
}

func validateLoginCredentials(username, email, password string) error {
	if (username == "" && email == "") || password == "" {
		return appErrors.New(http.StatusBadRequest, "username or email and password are required")
	}
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			return appErrors.New(http.StatusBadRequest, "email is invalid")
		}
	}

	return nil
}
