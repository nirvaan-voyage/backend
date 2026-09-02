package handlers

import (
	"net/http"

	"nirvaan-backend/internal/services"

	"github.com/gin-gonic/gin"
)

type TravelHandler struct {
	service *services.TravelAPIService
}

func NewTravelHandler(service *services.TravelAPIService) TravelHandler {
	return TravelHandler{service: service}
}

func (h TravelHandler) Search(c *gin.Context) {
	category := c.Param("category")
	response, err := h.service.Search(
		c.Request.Context(),
		category,
		c.Query("from"),
		c.Query("to"),
		c.Query("date"),
		c.Query("city"),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}
