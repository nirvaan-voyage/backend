package models

type BookableFlight struct {
	ID        string `json:"id"`
	Airline   string `json:"airline"`
	Departure string `json:"departure"`
	Arrival   string `json:"arrival"`
	Price     string `json:"price"`
	Currency  string `json:"currency"`
}

// AviationstackRawResponse maps the incoming structure from the external API
type AviationstackRawResponse struct {
	Data []struct {
		FlightDate   string `json:"flight_date"`
		FlightStatus string `json:"flight_status"`
		Departure    struct {
			Airport string `json:"airport"`
			Iata    string `json:"iata"`
		} `json:"departure"`
		Arrival struct {
			Airport string `json:"airport"`
			Iata    string `json:"iata"`
		} `json:"arrival"`
		Flight struct {
			Number string `json:"number"`
		} `json:"flight"`
	} `json:"data"`
}
