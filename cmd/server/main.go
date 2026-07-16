package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"nirvaan-backend/internal/auth"
	"nirvaan-backend/internal/config"
	"nirvaan-backend/internal/database"
	"nirvaan-backend/internal/handlers"
	"nirvaan-backend/internal/repositories"
	"nirvaan-backend/internal/routes"
	"nirvaan-backend/internal/services"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	// ── Auth Layer (Register + Login + JWT) ──────────────────────────────────
	jwtService := auth.NewJWTService(cfg.JWTSecret, cfg.JWTExpiration)
	authService := auth.NewService(db, jwtService)
	authHandler := auth.NewHandler(authService)

	// ── OTP Layer (Generate-OTP + Verify-OTP) ──────────────────────────────
	otpRepo := repositories.NewOTPRepository(db)
	otpService := services.NewOTPService(
		otpRepo,
		jwtService,
	)
	otpHandler := handlers.NewOTPHandler(otpService, authService)

	// ── Core Engine Initialization ──────────────────────────────────────────
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	// Health Checks
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Nirvaan Backend Running 🚀"})
	})

	// ── Mounting the Route Groups ───────────────────────────────────────────
	routes.RegisterAuthRoutes(router, authHandler, otpHandler)

	// Initializes your updated flight handler
	flightController := handlers.NewFlightHandler()
	// Mounts the new dynamic /api/flights/search endpoint seamlessly
	routes.RegisterFlightRoutes(router, flightController)

	log.Printf("Nirvaan backend listening on :%s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
