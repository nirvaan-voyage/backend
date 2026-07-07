package models

import "time"

// User is the core user record stored in PostgreSQL.
// JSON tags intentionally exclude PasswordHash from API responses.
type User struct {
	ID               int       `json:"id"`
	Name             string    `json:"name"`
	Username         string    `json:"username"`
	Email            string    `json:"email"`
	Phone            string    `json:"phone"`
	Age              *int      `json:"age,omitempty"`
	ProfilePhotoPath *string   `json:"profilePhotoPath,omitempty"`
	PasswordHash     string    `json:"-"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// RegisterRequest matches POST /auth/register body.
type RegisterRequest struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Age      *int   `json:"age"`
	Password string `json:"password"`
}

// LoginRequest matches POST /auth/login body.
// EmailOrUsername accepts either field — the service resolves which one.
type LoginRequest struct {
	EmailOrUsername string `json:"emailOrUsername"`
	Password        string `json:"password"`
}

// GoogleAuthRequest matches POST /auth/google body.
type GoogleAuthRequest struct {
	IDToken string `json:"idToken"`
}

// CompleteProfileRequest matches PUT /auth/profile body.
type CompleteProfileRequest struct {
	Username         string  `json:"username"`
	Phone            string  `json:"phone"`
	Age              *int    `json:"age"`
	ProfilePhotoPath *string `json:"profilePhotoPath"`
}

// ResetPasswordRequest matches POST /auth/reset-password body.
type ResetPasswordRequest struct {
	Email    string `json:"email"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

// AuthResponse is returned by both /register and /login on success.
type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// ErrorResponse is the standard error envelope for all failed requests.
type ErrorResponse struct {
	Message     string   `json:"message"`
	Field       string   `json:"field,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}
