package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"nirvaan-backend/internal/models"
)

type JWTService struct {
	secret     []byte
	expiration time.Duration
}

func NewJWTService(secret string, expiration time.Duration) JWTService {
	return JWTService{
		secret:     []byte(secret),
		expiration: expiration,
	}
}

func (s JWTService) Generate(user models.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"email":    user.Email,
		"username": user.Username,
		"exp":      time.Now().Add(s.expiration).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}
