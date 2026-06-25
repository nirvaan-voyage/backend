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

func (h Handler) RegisterRoutes(router *gin.Engine) {
	group := router.Group("/auth")
	group.POST("/register", h.Register)
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
