package services

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"nirvaan-backend/internal/models"
	"nirvaan-backend/internal/repositories"
	"nirvaan-backend/internal/auth"
)

type OTPService struct {
	repo repositories.OTPRepository
	jwt  auth.JWTService
}

func NewOTPService(
	repo repositories.OTPRepository,
	jwt auth.JWTService,
) OTPService {

	return OTPService{
		repo: repo,
		jwt:  jwt,
	}
}

// GenerateOTP deletes any old OTP for this email+purpose, creates a new
// 6-digit code valid for 10 minutes, persists it, and returns the code.
//
// In development the code is returned directly in the API response.
// Before production: replace the return value with an email/SMS send,
func (s OTPService) GenerateOTP(ctx context.Context, email, purpose string) (string, error) {

	if err := validatePurpose(purpose); err != nil {
		return "", err
	}

	if err := s.repo.DeleteOldOTPs(ctx, email, purpose); err != nil {
		return "", fmt.Errorf("clear old otps: %w", err)
	}

	code := fmt.Sprintf("%06d", rand.Intn(1000000))

	otp := &models.OTPCode{
		Email:     email,
		Code:      code,
		Purpose:   purpose,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		Used:      false,
	}

	// ⭐ THIS IS THE IMPORTANT PART
	if err := s.repo.Save(ctx, otp); err != nil {
		return "", fmt.Errorf("save otp: %w", err)
	}

	verificationToken, err := s.jwt.GenerateVerificationToken(email)
	if err != nil {
		return "", fmt.Errorf("generate verification token: %w", err)
	}

	return verificationToken, nil
}

// VerifyOTP checks that the supplied code is valid, unexpired, and unused,
// then marks it as used so it cannot be replayed.
func (s OTPService) VerifyOTP(
	ctx context.Context,
	email string,
	purpose string,
	code string,
) (string, error) {
	if err := validatePurpose(purpose); err != nil {
		return "", err
	}

	otp, err := s.repo.GetLatest(ctx, email, purpose)
	if err != nil {
		return "", fmt.Errorf("invalid OTP")
	}

	if otp.Used {
		return "", fmt.Errorf("OTP has already been used")
	}
	if time.Now().After(otp.ExpiresAt) {
		return "", fmt.Errorf("OTP has expired")
	}
	if otp.Code != code {
		return "", fmt.Errorf("incorrect OTP")
	}

	if err := s.repo.MarkUsed(ctx, otp.ID); err != nil {
		return "", fmt.Errorf("mark otp used: %w", err)
	}

	verificationToken, err := s.jwt.GenerateVerificationToken(email)
if err != nil {
    return "", err
}

return verificationToken, nil
}

func validatePurpose(purpose string) error {
	switch purpose {
	case "signup", "forgot_password":
		return nil
	default:
		return fmt.Errorf("invalid purpose: must be 'signup' or 'forgot_password'")
	}
}
