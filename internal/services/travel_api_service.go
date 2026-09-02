package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"nirvaan-backend/internal/models"
)

type TravelAPIService struct {
	client       *http.Client
	amadeusLock  sync.Mutex
	amadeusToken string
	tokenExpiry  time.Time
}

func NewTravelAPIService() *TravelAPIService {
	return &TravelAPIService{client: &http.Client{Timeout: 25 * time.Second}}
}

func (s *TravelAPIService) Search(ctx context.Context, category, from, to, date, city string) (models.TravelSearchResponse, error) {
	category = strings.ToLower(strings.TrimSpace(category))
	if city == "" {
		city = to
	}

	var (
		items  []models.TravelSearchItem
		source string
		err    error
	)

	switch category {
	case "flights":
		items, source, err = s.searchFlights(ctx, from, to, date)
	case "hotels":
		items, source, err = s.searchHotels(ctx, city)
	case "shows":
		items, source, err = s.searchShows(ctx, city)
	case "trains":
		items, source, err = s.searchTrains(ctx, from, to, date)
	case "buses":
		items, source, err = s.searchBuses(ctx, from, to, date)
	default:
		return models.TravelSearchResponse{}, fmt.Errorf("unsupported category")
	}

	if err != nil || len(items) == 0 {
		fallbackReason := "provider returned no India results"
		if err != nil {
			fallbackReason = err.Error()
		}
		items = fallbackTravelItems(category, from, to, date, city)
		source = source + "-fallback: " + fallbackReason
	}

	return models.TravelSearchResponse{
		Success:  true,
		Source:   source,
		Country:  "IN",
		Category: category,
		Count:    len(items),
		Data:     items,
	}, nil
}

func (s *TravelAPIService) searchFlights(ctx context.Context, from, to, date string) ([]models.TravelSearchItem, string, error) {
	token, err := s.amadeusAccessToken(ctx)
	if err != nil {
		return nil, "amadeus", err
	}

	params := url.Values{}
	params.Set("originLocationCode", strings.ToUpper(defaultString(from, "DEL")))
	params.Set("destinationLocationCode", strings.ToUpper(defaultString(to, "BOM")))
	params.Set("departureDate", defaultString(date, time.Now().AddDate(0, 0, 14).Format("2006-01-02")))
	params.Set("adults", "1")
	params.Set("currencyCode", "INR")
	params.Set("max", "10")

	var raw struct {
		Data []struct {
			ID    string `json:"id"`
			Price struct {
				Total    string `json:"total"`
				Currency string `json:"currency"`
			} `json:"price"`
			Itineraries []struct {
				Segments []struct {
					CarrierCode string `json:"carrierCode"`
					Number      string `json:"number"`
				} `json:"segments"`
			} `json:"itineraries"`
		} `json:"data"`
	}
	if err := s.getJSON(ctx, "https://test.api.amadeus.com/v2/shopping/flight-offers?"+params.Encode(), token, &raw); err != nil {
		return nil, "amadeus", err
	}

	items := []models.TravelSearchItem{}
	for _, offer := range raw.Data {
		airline := "Indian flight"
		if len(offer.Itineraries) > 0 && len(offer.Itineraries[0].Segments) > 0 {
			seg := offer.Itineraries[0].Segments[0]
			airline = strings.TrimSpace(seg.CarrierCode + " " + seg.Number)
		}
		items = append(items, models.TravelSearchItem{
			ID: offer.ID, Type: "flight", Title: airline, Subtitle: "Flight offer in India",
			Provider: "Amadeus", Origin: from, Destination: to, Date: date,
			Price: offer.Price.Total, Currency: offer.Price.Currency,
		})
	}
	return items, "amadeus", nil
}

func (s *TravelAPIService) searchHotels(ctx context.Context, city string) ([]models.TravelSearchItem, string, error) {
	key := strings.TrimSpace(os.Getenv("HOTELS_API_KEY"))
	if key == "" {
		return nil, "hotels-api", fmt.Errorf("HOTELS_API_KEY is missing")
	}

	city = defaultString(city, "Delhi")
	params := url.Values{}
	params.Set("city", city)
	params.Set("country", "India")
	params.Set("country_code", "IN")
	params.Set("rating", "3")
	params.Set("limit", "10")

	var raw struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Error   string `json:"error"`
		Data    []struct {
			ID          interface{} `json:"id"`
			Name        string      `json:"name"`
			City        string      `json:"city"`
			Country     string      `json:"country"`
			CountryCode string      `json:"country_code"`
			Address     string      `json:"address"`
			Rating      interface{} `json:"rating"`
			Amenities   []string    `json:"amenities"`
		} `json:"data"`
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.hotels-api.com/v2/hotels/search?"+params.Encode(), nil)
	if err != nil {
		return nil, "hotels-api", err
	}
	req.Header.Set("X-API-KEY", key)

	if err := s.doJSON(req, &raw); err != nil {
		return nil, "hotels-api", err
	}
	if !raw.Success && raw.Message != "" {
		return nil, "hotels-api", fmt.Errorf("hotels api error: %s", raw.Message)
	}
	if raw.Error != "" {
		return nil, "hotels-api", fmt.Errorf("hotels api error: %s", raw.Error)
	}

	items := []models.TravelSearchItem{}
	for _, hotel := range raw.Data {
		if hotel.CountryCode != "" && !strings.EqualFold(hotel.CountryCode, "IN") {
			continue
		}
		subtitle := hotel.Address
		if subtitle == "" {
			subtitle = strings.TrimSpace(strings.Join([]string{hotel.City, hotel.Country}, ", "))
		}
		if hotel.Rating != nil {
			rating := strings.TrimSpace(fmt.Sprint(hotel.Rating))
			if rating != "" && rating != "<nil>" {
				subtitle = strings.TrimSpace(subtitle + " - " + rating + " rating")
			}
		}
		items = append(items, models.TravelSearchItem{
			ID: fmt.Sprint(hotel.ID), Type: "hotel", Title: defaultString(hotel.Name, "Hotel in "+city), Subtitle: subtitle,
			Provider: "Hotels API", Destination: city, Currency: "INR",
		})
		if len(items) == 10 {
			break
		}
	}
	return items, "hotels-api", nil
}

func (s *TravelAPIService) searchShows(ctx context.Context, city string) ([]models.TravelSearchItem, string, error) {
	key := strings.TrimSpace(os.Getenv("TICKETMASTER_API_KEY"))
	if key == "" {
		return nil, "ticketmaster", fmt.Errorf("TICKETMASTER_API_KEY is missing")
	}

	params := url.Values{}
	params.Set("apikey", key)
	params.Set("countryCode", "IN")
	params.Set("size", "10")
	if city != "" {
		params.Set("city", city)
	}

	var raw struct {
		Embedded struct {
			Events []struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				URL   string `json:"url"`
				Dates struct {
					Start struct {
						LocalDate string `json:"localDate"`
					} `json:"start"`
				} `json:"dates"`
				Images []struct {
					URL string `json:"url"`
				} `json:"images"`
				Venues struct {
					Venues []struct {
						Name string `json:"name"`
					} `json:"venues"`
				} `json:"_embedded"`
			} `json:"events"`
		} `json:"_embedded"`
	}
	if err := s.getJSON(ctx, "https://app.ticketmaster.com/discovery/v2/events.json?"+params.Encode(), "", &raw); err != nil {
		return nil, "ticketmaster", err
	}

	items := []models.TravelSearchItem{}
	for _, event := range raw.Embedded.Events {
		image := ""
		if len(event.Images) > 0 {
			image = event.Images[0].URL
		}
		venue := city
		if len(event.Venues.Venues) > 0 {
			venue = event.Venues.Venues[0].Name
		}
		items = append(items, models.TravelSearchItem{
			ID: event.ID, Type: "show", Title: event.Name, Subtitle: venue,
			Provider: "Ticketmaster", Destination: city, Date: event.Dates.Start.LocalDate,
			BookingURL: event.URL, ImageURL: image, Currency: "INR",
		})
	}
	return items, "ticketmaster", nil
}

func (s *TravelAPIService) searchTrains(ctx context.Context, from, to, date string) ([]models.TravelSearchItem, string, error) {
	key := strings.TrimSpace(os.Getenv("RAPIDAPI_KEY"))
	if key == "" {
		return nil, "irctc-rapidapi", fmt.Errorf("RAPIDAPI_KEY is missing")
	}

	host := defaultString(os.Getenv("IRCTC_RAPIDAPI_HOST"), "irctc1.p.rapidapi.com")
	params := url.Values{}
	params.Set("fromStationCode", strings.ToUpper(defaultString(from, "NDLS")))
	params.Set("toStationCode", strings.ToUpper(defaultString(to, "BCT")))
	params.Set("dateOfJourney", defaultString(date, time.Now().AddDate(0, 0, 14).Format("2006-01-02")))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+host+"/api/v3/trainBetweenStations?"+params.Encode(), nil)
	if err != nil {
		return nil, "irctc-rapidapi", err
	}
	req.Header.Set("X-RapidAPI-Key", key)
	req.Header.Set("X-RapidAPI-Host", host)

	var raw map[string]interface{}
	if err := s.doJSON(req, &raw); err != nil {
		return nil, "irctc-rapidapi", err
	}

	items := []models.TravelSearchItem{}
	for i, train := range mapSlice(raw["data"]) {
		number := firstString(train, "train_number", "trainNumber", "number", "trainNo")
		name := firstString(train, "train_name", "trainName", "name")
		departure := firstString(train, "from_std", "departureTime", "departure", "fromDepartureTime")
		arrival := firstString(train, "to_sta", "arrivalTime", "arrival", "toArrivalTime")
		duration := firstString(train, "duration", "travelTime")
		title := strings.TrimSpace(strings.Join([]string{number, name}, " "))
		if title == "" {
			title = fmt.Sprintf("Train option %d", i+1)
		}
		subtitle := strings.TrimSpace(strings.Join(nonEmptyStrings(departure, arrival, duration), " - "))
		items = append(items, models.TravelSearchItem{
			ID: defaultString(number, fmt.Sprintf("train-%d", i+1)), Type: "train", Title: title, Subtitle: defaultString(subtitle, "Indian Railways listing"),
			Provider: "IRCTC RapidAPI", Origin: strings.ToUpper(defaultString(from, "NDLS")), Destination: strings.ToUpper(defaultString(to, "BCT")), Date: date, Currency: "INR",
		})
		if len(items) == 10 {
			break
		}
	}
	return items, "irctc-rapidapi", nil
}

func (s *TravelAPIService) searchBuses(ctx context.Context, from, to, date string) ([]models.TravelSearchItem, string, error) {
	key := strings.TrimSpace(os.Getenv("AOPAY_API_KEY"))
	if key == "" {
		return nil, "aopay-bus", fmt.Errorf("AOPAY_API_KEY is missing")
	}

	body, err := json.Marshal(map[string]interface{}{
		"origin":      defaultString(from, "Delhi"),
		"destination": defaultString(to, "Jaipur"),
		"date":        defaultString(date, time.Now().AddDate(0, 0, 14).Format("2006-01-02")),
		"passengers":  1,
	})
	if err != nil {
		return nil, "aopay-bus", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.aopay.in/v2/bus/search", bytes.NewReader(body))
	if err != nil {
		return nil, "aopay-bus", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", key)

	var raw map[string]interface{}
	if err := s.doJSON(req, &raw); err != nil {
		return nil, "aopay-bus", err
	}

	data := mapSlice(raw["data"])
	if len(data) == 0 {
		data = mapSlice(raw["buses"])
	}
	items := []models.TravelSearchItem{}
	for i, bus := range data {
		operator := firstString(bus, "operator", "operatorName", "name", "travels")
		busType := firstString(bus, "busType", "type", "coachType")
		departure := firstString(bus, "departureTime", "departure", "startTime")
		arrival := firstString(bus, "arrivalTime", "arrival", "endTime")
		price := firstString(bus, "fare", "price", "amount")
		subtitle := strings.TrimSpace(strings.Join(nonEmptyStrings(busType, departure, arrival), " - "))
		items = append(items, models.TravelSearchItem{
			ID: defaultString(firstString(bus, "id", "tripId", "routeId"), fmt.Sprintf("bus-%d", i+1)), Type: "bus", Title: defaultString(operator, "Bus option"), Subtitle: defaultString(subtitle, "India bus listing"),
			Provider: "AOPAY Bus API", Origin: defaultString(from, "Delhi"), Destination: defaultString(to, "Jaipur"), Date: date, Price: price, Currency: "INR",
		})
		if len(items) == 10 {
			break
		}
	}
	return items, "aopay-bus", nil
}

func (s *TravelAPIService) amadeusAccessToken(ctx context.Context) (string, error) {
	s.amadeusLock.Lock()
	defer s.amadeusLock.Unlock()
	if s.amadeusToken != "" && time.Now().Before(s.tokenExpiry) {
		return s.amadeusToken, nil
	}

	clientID := strings.TrimSpace(os.Getenv("AMADEUS_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("AMADEUS_CLIENT_SECRET"))
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("AMADEUS_CLIENT_ID or AMADEUS_CLIENT_SECRET is missing")
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://test.api.amadeus.com/v1/security/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || body.AccessToken == "" {
		return "", fmt.Errorf("amadeus auth failed")
	}
	s.amadeusToken = body.AccessToken
	s.tokenExpiry = time.Now().Add(time.Duration(body.ExpiresIn-60) * time.Second)
	return s.amadeusToken, nil
}

func (s *TravelAPIService) getJSON(ctx context.Context, endpoint, token string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return s.doJSON(req, target)
}

func (s *TravelAPIService) doJSON(req *http.Request, target interface{}) error {
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("provider status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func fallbackTravelItems(category, from, to, date, city string) []models.TravelSearchItem {
	switch category {
	case "flights":
		return sampleFlights(from, to, date)
	case "hotels":
		return sampleHotels(city)
	case "shows":
		return sampleShows(city)
	case "trains":
		return sampleTrains(from, to, date)
	case "buses":
		return sampleBuses(from, to, date)
	default:
		return nil
	}
}

func sampleFlights(from, to, date string) []models.TravelSearchItem {
	origin := strings.ToUpper(defaultString(from, "DEL"))
	destination := strings.ToUpper(defaultString(to, "BOM"))
	return []models.TravelSearchItem{
		{ID: "flight-1", Type: "flight", Title: "IndiGo 6E 204", Subtitle: "Non-stop - morning departure - cabin baggage included", Provider: "India test data", Origin: origin, Destination: destination, Date: date, Price: "4599", Currency: "INR"},
		{ID: "flight-2", Type: "flight", Title: "Air India AI 865", Subtitle: "Non-stop - full service - meal included", Provider: "India test data", Origin: origin, Destination: destination, Date: date, Price: "5299", Currency: "INR"},
		{ID: "flight-3", Type: "flight", Title: "Akasa Air QP 1127", Subtitle: "Direct flight - evening departure", Provider: "India test data", Origin: origin, Destination: destination, Date: date, Price: "4875", Currency: "INR"},
		{ID: "flight-4", Type: "flight", Title: "Vistara UK 981", Subtitle: "Premium economy available - flexible fare", Provider: "India test data", Origin: origin, Destination: destination, Date: date, Price: "6420", Currency: "INR"},
		{ID: "flight-5", Type: "flight", Title: "SpiceJet SG 8193", Subtitle: "Budget fare - afternoon departure", Provider: "India test data", Origin: origin, Destination: destination, Date: date, Price: "3999", Currency: "INR"},
	}
}

func sampleHotels(city string) []models.TravelSearchItem {
	city = defaultString(city, "Delhi")
	return []models.TravelSearchItem{
		{ID: "hotel-1", Type: "hotel", Title: "Nirvaan Stay " + city, Subtitle: "Central location - breakfast included - 4.4 rating", Provider: "India test data", Destination: city, Price: "3200", Currency: "INR"},
		{ID: "hotel-2", Type: "hotel", Title: "Heritage Comfort " + city, Subtitle: "Family friendly - near market - 4.2 rating", Provider: "India test data", Destination: city, Price: "4100", Currency: "INR"},
		{ID: "hotel-3", Type: "hotel", Title: "Backpacker Nest " + city, Subtitle: "Budget hostel - female dorms - 4.1 rating", Provider: "India test data", Destination: city, Price: "899", Currency: "INR"},
		{ID: "hotel-4", Type: "hotel", Title: "The City Courtyard " + city, Subtitle: "Boutique hotel - cafe - 4.6 rating", Provider: "India test data", Destination: city, Price: "5600", Currency: "INR"},
		{ID: "hotel-5", Type: "hotel", Title: "Lakeview Retreat " + city, Subtitle: "Quiet stay - airport pickup - 4.5 rating", Provider: "India test data", Destination: city, Price: "6800", Currency: "INR"},
	}
}

func sampleShows(city string) []models.TravelSearchItem {
	city = defaultString(city, "Mumbai")
	return []models.TravelSearchItem{
		{ID: "show-1", Type: "show", Title: "Stand-up Night", Subtitle: city + " comedy club - 8:00 PM", Provider: "India test data", Destination: city, Price: "799", Currency: "INR"},
		{ID: "show-2", Type: "show", Title: "Weekend Movie Premiere", Subtitle: city + " theatre listing - recliner seats", Provider: "India test data", Destination: city, Price: "350", Currency: "INR"},
		{ID: "show-3", Type: "show", Title: "Indie Music Evening", Subtitle: city + " live venue - local artists", Provider: "India test data", Destination: city, Price: "999", Currency: "INR"},
		{ID: "show-4", Type: "show", Title: "Classical Dance Showcase", Subtitle: city + " cultural centre - family friendly", Provider: "India test data", Destination: city, Price: "650", Currency: "INR"},
		{ID: "show-5", Type: "show", Title: "Food Festival Entry", Subtitle: city + " weekend event - tasting counters", Provider: "India test data", Destination: city, Price: "499", Currency: "INR"},
	}
}

func sampleTrains(from, to, date string) []models.TravelSearchItem {
	origin := strings.ToUpper(defaultString(from, "NDLS"))
	destination := strings.ToUpper(defaultString(to, "BCT"))
	return []models.TravelSearchItem{
		{ID: "train-1", Type: "train", Title: "12952 Mumbai Rajdhani", Subtitle: "Fast premium train - pantry available", Provider: "India test data", Origin: origin, Destination: destination, Date: date, Price: "1850", Currency: "INR"},
		{ID: "train-2", Type: "train", Title: "12002 Shatabdi Express", Subtitle: "Chair car - daytime journey", Provider: "India test data", Origin: origin, Destination: destination, Date: date, Price: "1450", Currency: "INR"},
		{ID: "train-3", Type: "train", Title: "12216 Garib Rath Express", Subtitle: "AC economy - overnight option", Provider: "India test data", Origin: origin, Destination: destination, Date: date, Price: "990", Currency: "INR"},
		{ID: "train-4", Type: "train", Title: "12904 Golden Temple Mail", Subtitle: "Sleeper and AC coaches - scenic route", Provider: "India test data", Origin: origin, Destination: destination, Date: date, Price: "760", Currency: "INR"},
		{ID: "train-5", Type: "train", Title: "12432 Intercity Express", Subtitle: "Short route - reserved seating", Provider: "India test data", Origin: origin, Destination: destination, Date: date, Price: "420", Currency: "INR"},
	}
}

func sampleBuses(from, to, date string) []models.TravelSearchItem {
	origin := defaultString(from, "Delhi")
	destination := defaultString(to, "Jaipur")
	return []models.TravelSearchItem{
		{ID: "bus-1", Type: "bus", Title: "Volvo AC Sleeper", Subtitle: "Overnight - charging point - blanket", Provider: "India test data", Origin: origin, Destination: destination, Date: date, Price: "899", Currency: "INR"},
		{ID: "bus-2", Type: "bus", Title: "Bharat Benz Seater", Subtitle: "Evening departure - semi sleeper", Provider: "India test data", Origin: origin, Destination: destination, Date: date, Price: "649", Currency: "INR"},
		{ID: "bus-3", Type: "bus", Title: "IntrCity SmartBus", Subtitle: "Live tracking - lounge access", Provider: "India test data", Origin: origin, Destination: destination, Date: date, Price: "1049", Currency: "INR"},
		{ID: "bus-4", Type: "bus", Title: "Zingbus Premium", Subtitle: "AC sleeper - hygienic rest stop", Provider: "India test data", Origin: origin, Destination: destination, Date: date, Price: "1199", Currency: "INR"},
		{ID: "bus-5", Type: "bus", Title: "RSRTC Express", Subtitle: "Budget government bus - frequent service", Provider: "India test data", Origin: origin, Destination: destination, Date: date, Price: "380", Currency: "INR"},
	}
}

func cityToIATA(city string) string {
	switch strings.ToLower(strings.TrimSpace(city)) {
	case "mumbai", "bombay":
		return "BOM"
	case "bengaluru", "bangalore":
		return "BLR"
	case "chennai":
		return "MAA"
	case "kolkata":
		return "CCU"
	case "hyderabad":
		return "HYD"
	case "goa":
		return "GOI"
	case "jaipur":
		return "JAI"
	case "kochi", "cochin":
		return "COK"
	default:
		return "DEL"
	}
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func mapSlice(value interface{}) []map[string]interface{} {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if mapped, ok := item.(map[string]interface{}); ok {
			result = append(result, mapped)
		}
	}
	return result
}

func firstString(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key]; ok && value != nil {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func nonEmptyStrings(values ...string) []string {
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
