package auth

import (
	"fmt"
	"time"

	"nirvaan-backend/internal/models"

	"github.com/golang-jwt/jwt/v5"
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

// Generate creates a signed JWT for the given user.
// Claims include user_id, email, and username so the Flutter client
// can decode them without an extra profile API call.

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

func (s JWTService) UserIDFromToken(tokenString string) (int, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, fmt.Errorf("invalid token")
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok || userIDFloat <= 0 {
		return 0, fmt.Errorf("missing user_id claim")
	}

	return int(userIDFloat), nil
}

// GenerateVerificationToken creates a short-lived token after OTP verification.
func (s JWTService) GenerateVerificationToken(email string) (string, error) {

	claims := jwt.MapClaims{
		"email": email,
		"type":  "verification",
		"exp":   time.Now().Add(10 * time.Minute).Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(s.secret)
}
