package models

import "time"

// OTPCode stores one-time passwords for email verification and password reset.
// Purpose values: "signup" | "forgot_password"
type OTPCode struct {
	ID        int
	Email     string
	Code      string    // 6 digits
	Purpose   string    // "signup" or "forgot_password"
	ExpiresAt time.Time // 10 minutes from creation
	Used      bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// OTPRequest is the body for POST /auth/generate-otp
type OTPRequest struct {
	Email   string `json:"email"   binding:"required,email"`
	Purpose string `json:"purpose" binding:"required"`
}

// VerifyOTPRequest is the body for POST /auth/verify-otp
type VerifyOTPRequest struct {
	Email   string `json:"email"   binding:"required,email"`
	Purpose string `json:"purpose" binding:"required"`
	Code    string `json:"code"    binding:"required,len=6"`
}
