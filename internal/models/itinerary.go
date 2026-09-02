package models

type ItineraryRequest struct {
	Destination string                 `json:"destination"`
	Answers     map[string]interface{} `json:"answers"`
}

type ItineraryDay struct {
	Day       int      `json:"day"`
	Title     string   `json:"title"`
	Morning   string   `json:"morning"`
	Afternoon string   `json:"afternoon"`
	Evening   string   `json:"evening"`
	Notes     []string `json:"notes"`
}

type MatchedGuide struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	City         string   `json:"city"`
	Specialties  []string `json:"specialties"`
	Languages    []string `json:"languages"`
	Rating       float64  `json:"rating"`
	Reviews      int      `json:"reviews"`
	PricePerDay  int      `json:"pricePerDay"`
	MatchScore   int      `json:"matchScore"`
	MatchReasons []string `json:"matchReasons"`
	Available    bool     `json:"available"`
	Image        string   `json:"image"`
	About        string   `json:"about"`
}

type ItineraryResponse struct {
	PromptUsed    string         `json:"promptUsed"`
	Summary       string         `json:"summary"`
	Days          []ItineraryDay `json:"days"`
	MatchedGuides []MatchedGuide `json:"matchedGuides"`
	AIProvider    string         `json:"aiProvider"`
}
