package auth

import (
	"errors"
	"net/http"
	"strings"

	"nirvaan-backend/internal/models"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) Handler {
	return Handler{service: service}
}

func (h Handler) RegisterRoutes(router *gin.Engine) {
	group := router.Group("/auth")
	group.POST("/register", h.Register)
	group.POST("/login", h.Login)
	group.POST("/google", h.GoogleAuth)
	group.PUT("/profile", h.CompleteProfile)
}

func (h Handler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Invalid request body",
		})
		return
	}

	response, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		var fieldErr FieldError
		if errors.As(err, &fieldErr) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Message:     fieldErr.Message,
				Field:       fieldErr.Field,
				Suggestions: fieldErr.Suggestions,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Could not create account",
		})
		return
	}

	c.JSON(http.StatusCreated, response)
}

// POST /auth/login
func (h Handler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Invalid request body",
		})
		return
	}

	response, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		var fieldErr FieldError
		if errors.As(err, &fieldErr) {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Message: fieldErr.Message,
				Field:   fieldErr.Field,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Login failed",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h Handler) GoogleAuth(c *gin.Context) {
	var req models.GoogleAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Invalid request body",
		})
		return
	}

	response, err := h.service.GoogleAuth(c.Request.Context(), req)
	if err != nil {
		var fieldErr FieldError
		if errors.As(err, &fieldErr) {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Message: fieldErr.Message,
				Field:   fieldErr.Field,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Google sign-in failed",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h Handler) CompleteProfile(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" || token == authHeader {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Message: "Authorization token is required",
			Field:   "token",
		})
		return
	}

	userID, err := h.service.jwt.UserIDFromToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Message: "Invalid or expired token",
			Field:   "token",
		})
		return
	}

	var req models.CompleteProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Invalid request body",
		})
		return
	}

	response, err := h.service.CompleteProfile(c.Request.Context(), userID, req)
	if err != nil {
		var fieldErr FieldError
		if errors.As(err, &fieldErr) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Message:     fieldErr.Message,
				Field:       fieldErr.Field,
				Suggestions: fieldErr.Suggestions,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Could not update profile",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}
