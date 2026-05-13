package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
	users           domain.UserRepository
	jwtSecret       []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func NewService(users domain.UserRepository, jwtSecret string, accessTokenTTL, refreshTokenTTL time.Duration) *Service {
	if accessTokenTTL <= 0 {
		accessTokenTTL = 24 * time.Hour
	}

	if refreshTokenTTL <= 0 {
		refreshTokenTTL = 7 * 24 * time.Hour
	}

	return &Service{
		users:           users,
		jwtSecret:       []byte(strings.TrimSpace(jwtSecret)),
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
	}
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (AuthResponse, error) {
	username := strings.ToLower(strings.TrimSpace(req.Username))
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := strings.TrimSpace(req.Password)

	if err := validateRegisterCredentials(username, email, password); err != nil {
		return AuthResponse{}, appErrors.New(http.StatusBadRequest, err.Error())
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
		return AuthResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to create user", err)
	}

	return s.issueTokenPair(ctx, user)
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

	return s.issueTokenPair(ctx, user)
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
		return AuthResponse{}, appErrors.New(http.StatusUnauthorized, "refresh token has been revoked")
	}

	if time.Now().After(storedToken.ExpiresAt) {
		_ = s.users.RevokeRefreshToken(ctx, storedToken.ID)
		return AuthResponse{}, appErrors.New(http.StatusUnauthorized, "refresh token has expired")
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

	return s.issueTokenPair(ctx, user)
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

func (s *Service) issueTokenPair(ctx context.Context, user domain.User) (AuthResponse, error) {
	if len(s.jwtSecret) == 0 {
		return AuthResponse{}, appErrors.New(http.StatusInternalServerError, "jwt secret is not configured")
	}

	now := time.Now()
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
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: hashToken(rawRefreshToken),
		ExpiresAt: now.Add(s.refreshTokenTTL),
		CreatedAt: now,
		UpdatedAt: now,
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
