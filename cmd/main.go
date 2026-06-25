package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"nirvaan-backend/internal/auth"
	"nirvaan-backend/internal/config"
	"nirvaan-backend/internal/database"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	jwtService := auth.NewJWTService(cfg.JWTSecret, cfg.JWTExpiration)
	authService := auth.NewService(db, jwtService)
	authHandler := auth.NewHandler(authService)
	authHandler.RegisterRoutes(router)

	log.Printf("Nirvaan backend listening on :%s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
