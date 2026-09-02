package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"nirvaan-backend/internal/models"
)

// SearchDuffelFlights connects to the live developer sandbox engine
func SearchDuffelFlights(origin, destination, date string) ([]models.BookableFlight, error) {
	apiKey := os.Getenv("DUFFEL_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("duffel API key is missing")
	}

	// Prepare payload mapping
	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"slices": []map[string]string{
				{
					"origin":         origin,
					"destination":    destination,
					"departure_date": date,
				},
			},
			"passengers": []map[string]string{
				{"type": "adult"},
			},
		},
	}

	jsonBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://api.duffel.com/air/offer_requests", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}

	// Add mandatory developer sandbox headers
	req.Header.Add("Authorization", "Bearer "+apiKey)
	req.Header.Add("Duffel-Version", "v2")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("duffel API error: %s", string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var cleanFlights []models.BookableFlight
	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return cleanFlights, nil
	}

	offers, ok := data["offers"].([]interface{})
	if !ok {
		return cleanFlights, nil
	}

	// Unmarshal data into the updated core models
	// Unmarshal data into the updated core models
	for i, offerData := range offers {
		if i >= 10 {
			break
		}
		offer := offerData.(map[string]interface{})
		owner := offer["owner"].(map[string]interface{})

		// Grab the airline name from Duffel
		airlineName := owner["name"].(string)

		// 🇮🇳 THE LOCALIZATION INTERCEPTOR 🇮🇳
		// If Duffel sends a generic test airline, we swap it with a real Indian carrier
		if airlineName == "Duffel Airways" || airlineName == "ZZ" {
			indianAirlines := []string{"IndiGo", "Air India", "Vistara", "Akasa Air", "SpiceJet"}
			// Distribute the airlines evenly across the search results
			airlineName = indianAirlines[i%len(indianAirlines)]
		}

		cleanFlights = append(cleanFlights, models.BookableFlight{
			ID:        offer["id"].(string),
			Airline:   airlineName, // Now guaranteed to look like a real Indian airline!
			Departure: origin,
			Arrival:   destination,
			Price:     offer["total_amount"].(string),
			Currency:  offer["total_currency"].(string), // Will return INR if you set it, or default base
		})
	}

	return cleanFlights, nil
}
