package services

import (
	"context"
	"fmt"
	"math/rand"
	"net/smtp"
	"os"
	"time"

	"nirvaan-backend/internal/auth"
	"nirvaan-backend/internal/models"
	"nirvaan-backend/internal/repositories"
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
// 6-digit code valid for 10 minutes, persists it, and sends the email.
func (s OTPService) GenerateOTP(ctx context.Context, email, purpose string) (string, error) {

	if err := validatePurpose(purpose); err != nil {
		return "", err
	}

	if err := s.repo.DeleteOldOTPs(ctx, email, purpose); err != nil {
		return "", fmt.Errorf("clear old otps: %w", err)
	}

	// Generate true 6-digit random code (000000 to 999999)
	code := fmt.Sprintf("%06d", rand.Intn(1000000))

	otp := &models.OTPCode{
		Email:     email,
		Code:      code,
		Purpose:   purpose,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		Used:      false,
	}

	if err := s.repo.Save(ctx, otp); err != nil {
		return "", fmt.Errorf("save otp: %w", err)
	}

	// Send the real email to the user
	if err := sendOTPEmail(email, code); err != nil {
		return "", fmt.Errorf("failed to send OTP email: %w", err)
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

// sendOTPEmail grabs credentials from .env and dispatches an HTML email
func sendOTPEmail(to string, code string) error {
	from := os.Getenv("SMTP_EMAIL")
	password := os.Getenv("SMTP_PASSWORD")
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")

	if from == "" || password == "" {
		return fmt.Errorf("SMTP credentials are not set in .env")
	}

	address := host + ":" + port

	// Define email headers and HTML body
	subject := "Subject: Your Nirvaan Verification Code\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #e0e0e0; border-radius: 10px;">
			<h2 style="color: #3D6B9E; text-align: center;">Nirvaan Verification</h2>
			<p style="color: #333; font-size: 16px;">Hello,</p>
			<p style="color: #333; font-size: 16px;">Here is your one-time password (OTP) to continue:</p>
			<div style="background-color: #f4f7fb; padding: 15px; text-align: center; border-radius: 8px; margin: 20px 0;">
				<span style="font-size: 28px; font-weight: bold; letter-spacing: 5px; color: #1a3a5c;">%s</span>
			</div>
			<p style="color: #7A8DAA; font-size: 14px;">This code will expire in 10 minutes.</p>
			<p style="color: #7A8DAA; font-size: 12px; margin-top: 30px;">If you didn't request this code, you can safely ignore this email.</p>
		</div>
	`, code)

	message := []byte(subject + mime + body)

	// Authenticate and send
	auth := smtp.PlainAuth("", from, password, host)
	return smtp.SendMail(address, auth, from, []string{to}, message)
}
