package handlers

import (
	"net/http"
	"nirvaan-backend/internal/services" // Make sure this matches your module name!

	"github.com/gin-gonic/gin"
)

// FlightHandler is the controller struct for flight routes
type FlightHandler struct{}

// NewFlightHandler acts as a constructor to initialize the controller in main.go
func NewFlightHandler() *FlightHandler {
	return &FlightHandler{}
}

// SearchLiveFlights is now properly attached to the FlightHandler using (h *FlightHandler)
func (h *FlightHandler) SearchLiveFlights(c *gin.Context) {
	// Grab the query parameters sent by Flutter (or Postman)
	origin := c.DefaultQuery("from", "DEL")      // Defaults to Delhi
	destination := c.DefaultQuery("to", "BOM")   // Defaults to Mumbai
	date := c.DefaultQuery("date", "2026-12-15") // Defaults to a future date

	// Pass the dynamic queries into our Duffel service
	flights, err := services.SearchDuffelFlights(origin, destination, date)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Return the clean JSON to the frontend
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   len(flights),
		"data":    flights,
	})
}
