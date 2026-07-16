package routes

import (
	"github.com/gin-gonic/gin"
)

// RegisterAuthRoutes mounts all authentication endpoints.
//
// Meghna's endpoints:
//   POST /auth/register   — create account
//   POST /auth/login      — sign in (email or username)
//
// Tanya's endpoints:
//   POST /auth/generate-otp  — generate a 6-digit OTP (signup / forgot_password)
//   POST /auth/verify-otp    — verify the OTP code

func RegisterAuthRoutes(router *gin.Engine, authHandler interface {
	Register(*gin.Context)
	Login(*gin.Context)
	GoogleAuth(*gin.Context)
	CompleteProfile(*gin.Context)
}, otpHandler interface {
	GenerateOTP(*gin.Context)
	VerifyOTP(*gin.Context)
	ResetPassword(*gin.Context)
}) {
	auth := router.Group("/auth")
	{
		// Meghna
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/google", authHandler.GoogleAuth)
		auth.PUT("/profile", authHandler.CompleteProfile)

		// Tanya
		auth.POST("/generate-otp", otpHandler.GenerateOTP)
		auth.POST("/verify-otp", otpHandler.VerifyOTP)
		auth.POST("/reset-password", otpHandler.ResetPassword)
	}
}

// RegisterFlightRoutes mounts all external API endpoints for travel data.
func RegisterFlightRoutes(router *gin.Engine, flightHandler interface {
	SearchLiveFlights(*gin.Context) // Updated to match our new handler method!
}) {
	api := router.Group("/api")
	{
		// Attached directly to the 'api' group instance.
		// This automatically resolves the full URL path to: /api/flights/search
		api.GET("/flights/search", flightHandler.SearchLiveFlights)
	}
}
