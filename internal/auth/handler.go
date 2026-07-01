package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"nirvaan-backend/internal/models"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) Handler {
	return Handler{service: service}
}

// RegisterRoutes mounts all auth endpoints under /auth.
func (h Handler) RegisterRoutes(router *gin.Engine) {
	group := router.Group("/auth")
	group.POST("/register", h.Register)
	group.POST("/login", h.Login)
}

// POST /auth/register
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
		// Return the actual error message for debugging
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Could not create account: " + err.Error(),
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
