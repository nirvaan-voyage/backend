package models

type TravelSearchItem struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	Provider    string `json:"provider"`
	Origin      string `json:"origin,omitempty"`
	Destination string `json:"destination,omitempty"`
	Date        string `json:"date,omitempty"`
	Price       string `json:"price,omitempty"`
	Currency    string `json:"currency,omitempty"`
	BookingURL  string `json:"bookingUrl,omitempty"`
	ImageURL    string `json:"imageUrl,omitempty"`
}

type TravelSearchResponse struct {
	Success  bool               `json:"success"`
	Source   string             `json:"source"`
	Country  string             `json:"country"`
	Category string             `json:"category"`
	Count    int                `json:"count"`
	Data     []TravelSearchItem `json:"data"`
}
