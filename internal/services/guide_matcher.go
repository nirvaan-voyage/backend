package services

import (
	"sort"
	"strings"

	"nirvaan-backend/internal/models"
)

var testGuides = []models.MatchedGuide{
	{ID: 1, Name: "Priya Sharma", City: "Jaipur", Specialties: []string{"Heritage", "Historical", "Culture", "Shopping"}, Languages: []string{"English", "Hindi"}, Rating: 4.9, Reviews: 128, PricePerDay: 2200, Available: true, Image: "assets/guides/priya.jpg", About: "Certified heritage guide for forts, palaces, bazaars, and royal Rajasthan stories."},
	{ID: 2, Name: "Rahul Nair", City: "Kochi", Specialties: []string{"Nature", "Food", "Local Cuisine", "Relaxation", "Photography"}, Languages: []string{"English", "Hindi", "Malayalam"}, Rating: 4.8, Reviews: 94, PricePerDay: 2000, Available: true, Image: "assets/guides/rahul.jpg", About: "Kerala guide for backwaters, local food trails, calm routes, and culture walks."},
	{ID: 3, Name: "Tsering Angmo", City: "Leh", Specialties: []string{"Adventure", "Trekking", "Camping", "Mountains", "Photography"}, Languages: []string{"English", "Hindi", "Ladakhi"}, Rating: 4.95, Reviews: 156, PricePerDay: 2800, Available: true, Image: "assets/guides/tsering.jpg", About: "Mountain guide with trekking experience across Ladakh, Nubra, and high-altitude routes."},
	{ID: 4, Name: "Ananya Iyer", City: "Mysore", Specialties: []string{"Historical", "Religious", "Culture", "Senior Friendly", "Child Friendly"}, Languages: []string{"English", "Hindi", "Kannada"}, Rating: 4.7, Reviews: 72, PricePerDay: 1800, Available: true, Image: "assets/guides/ananya.jpg", About: "Slow-paced palace, temple, and family-friendly city guide across Mysore."},
	{ID: 5, Name: "Rohit Bisht", City: "Rishikesh", Specialties: []string{"Adventure", "Trekking", "Camping", "Boating", "Spiritual"}, Languages: []string{"English", "Hindi"}, Rating: 4.85, Reviews: 110, PricePerDay: 2400, Available: true, Image: "assets/guides/rohit.jpg", About: "Adventure guide for rafting, camping, treks, and spiritual riverside experiences."},
	{ID: 6, Name: "Sneha Desai", City: "Mumbai", Specialties: []string{"Street Food", "Nightlife", "Shopping", "Local Cuisine", "Photography"}, Languages: []string{"English", "Hindi", "Marathi"}, Rating: 4.75, Reviews: 89, PricePerDay: 2100, Available: true, Image: "assets/guides/sneha.jpg", About: "Food and city guide helping visitors experience Mumbai like a local."},
	{ID: 7, Name: "Kabir Khan", City: "Delhi", Specialties: []string{"Historical", "Food", "Religious", "Shopping", "Culture"}, Languages: []string{"English", "Hindi", "Urdu"}, Rating: 4.8, Reviews: 143, PricePerDay: 2300, Available: true, Image: "assets/guides/kabir.jpg", About: "Delhi guide for monuments, Old Delhi food walks, markets, and layered city history."},
	{ID: 8, Name: "Mira Fernandes", City: "Goa", Specialties: []string{"Relaxation", "Nature", "Nightlife", "Food", "Photography"}, Languages: []string{"English", "Hindi", "Konkani"}, Rating: 4.82, Reviews: 101, PricePerDay: 2500, Available: true, Image: "assets/guides/mira.jpg", About: "Goa guide for beaches, food, nightlife, Portuguese heritage, and easy scenic days."},
}

func MatchGuides(destination string, answers map[string]interface{}) []models.MatchedGuide {
	terms := answerTerms(destination, answers)
	matches := make([]models.MatchedGuide, 0, len(testGuides))

	for _, guide := range testGuides {
		score := 0
		reasons := []string{}
		city := strings.ToLower(guide.City)
		dest := strings.ToLower(destination)

		if dest != "" && (strings.Contains(dest, city) || strings.Contains(city, dest)) {
			score += 45
			reasons = append(reasons, "Matches your destination")
		}

		for _, specialty := range guide.Specialties {
			key := strings.ToLower(specialty)
			if terms[key] {
				score += 12
				reasons = append(reasons, "Fits "+specialty)
			}
		}

		if guide.Rating >= 4.8 {
			score += 8
			reasons = append(reasons, "Highly rated")
		}
		if guide.Available {
			score += 5
		}

		if score == 0 {
			score = int(guide.Rating * 10)
			reasons = append(reasons, "Good general match")
		}
		if score > 100 {
			score = 100
		}

		guide.MatchScore = score
		guide.MatchReasons = dedupe(reasons)
		matches = append(matches, guide)
	}

	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].MatchScore > matches[j].MatchScore
	})
	if len(matches) > 3 {
		return matches[:3]
	}
	return matches
}

func answerTerms(destination string, answers map[string]interface{}) map[string]bool {
	terms := map[string]bool{}
	add := func(value string) {
		value = strings.TrimSpace(strings.ToLower(value))
		if value != "" {
			terms[value] = true
		}
	}
	add(destination)
	for _, raw := range answers {
		switch value := raw.(type) {
		case string:
			add(value)
		case []interface{}:
			for _, item := range value {
				add(toString(item))
			}
		case []string:
			for _, item := range value {
				add(item)
			}
		}
	}
	return terms
}

func toString(value interface{}) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
