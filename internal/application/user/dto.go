package user

import (
	"time"

	"apant_be/internal/domain"
)

// UserResponse is the admin-facing view of a user. It never includes the
// password hash.
type UserResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateUserRequest is an admin creating an account for someone else. Unlike
// self-registration it does not log anyone in; the admin stays authenticated as
// themselves.
type CreateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type UpdateRoleRequest struct {
	Role string `json:"role"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

func toUserResponse(u domain.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
