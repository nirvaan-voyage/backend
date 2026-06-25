package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port          string
	DatabaseURL   string
	JWTSecret     string
	JWTExpiration time.Duration
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found; using environment variables")
	}

	expiresInHours := getInt("JWT_EXPIRES_IN_HOURS", 168)

	return Config{
		Port:          getString("PORT", "8080"),
		DatabaseURL:   required("DATABASE_URL"),
		JWTSecret:     required("JWT_SECRET"),
		JWTExpiration: time.Duration(expiresInHours) * time.Hour,
	}
}

func required(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("%s is required", key)
	}
	return value
}

func getString(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
