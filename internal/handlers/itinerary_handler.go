package handlers

import (
	"net/http"

	"nirvaan-backend/internal/models"
	"nirvaan-backend/internal/services"

	"github.com/gin-gonic/gin"
)

type ItineraryHandler struct {
	service services.ItineraryService
}

func NewItineraryHandler(service services.ItineraryService) ItineraryHandler {
	return ItineraryHandler{service: service}
}

func (h ItineraryHandler) Generate(c *gin.Context) {
	var req models.ItineraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid itinerary request"})
		return
	}

	response, err := h.service.Generate(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}
