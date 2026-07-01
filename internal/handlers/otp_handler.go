package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"nirvaan-backend/internal/auth"
	"nirvaan-backend/internal/models"
	"nirvaan-backend/internal/services"
)

type OTPHandler struct {
	otpService  services.OTPService
	authService auth.Service
}

func NewOTPHandler(otpService services.OTPService, authService auth.Service) OTPHandler {
	return OTPHandler{
		otpService:  otpService,
		authService: authService,
	}
}

// POST /auth/generate-otp
// Body: { "email": "...", "purpose": "signup" | "forgot_password" }
//
// DEV NOTE: the response currently includes the OTP code in plain text
// for testing. Before production, remove "otp" from the response and
// send the code via email/SMS instead.
func (h OTPHandler) GenerateOTP(c *gin.Context) {
	var req models.OTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Invalid request — email and purpose are required",
		})
		return
	}

	code, err := h.otpService.GenerateOTP(c.Request.Context(), req.Email, req.Purpose)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: err.Error(),
		})
		return
	}

	// TODO (Vaanya): replace with actual email/SMS send before production
	c.JSON(http.StatusOK, gin.H{
		"message": "OTP generated successfully",
		"otp":     code, // remove this line in production
	})
}

// POST /auth/verify-otp
// Body: { "email": "...", "purpose": "...", "code": "123456" }
func (h OTPHandler) VerifyOTP(c *gin.Context) {
	var req models.VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Invalid request — email, purpose, and 6-digit code are required",
		})
		return
	}

	verificationToken, err := h.otpService.VerifyOTP(
	c.Request.Context(),
	req.Email,
	req.Purpose,
	req.Code,
)

if err != nil {
	c.JSON(http.StatusBadRequest, gin.H{
		"message": err.Error(),
	})
	return
}

c.JSON(http.StatusOK, gin.H{
	"message": "OTP verified successfully",
	"verification_token": verificationToken,
})
}

// POST /auth/reset-password
// Body: { "email": "...", "code": "...", "password": "..." }
func (h OTPHandler) ResetPassword(c *gin.Context) {
	var req models.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Invalid request — email, code, and new password are required",
		})
		return
	}

	// 1. Verify OTP (purpose is always forgot_password for this endpoint)
	if _, err := h.otpService.VerifyOTP(
    c.Request.Context(),
    req.Email,
    "forgot_password",
    req.Code,
); err != nil {

	// 2. Update password
	if err := h.authService.ResetPassword(c.Request.Context(), req.Email, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Could not reset password: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Password reset successfully",
	})

}} 

