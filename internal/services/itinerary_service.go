package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"nirvaan-backend/internal/models"
)

type ItineraryService struct {
	client *http.Client
}

func NewItineraryService() ItineraryService {
	return ItineraryService{
		client: &http.Client{Timeout: 35 * time.Second},
	}
}

func (s ItineraryService) Generate(ctx context.Context, req models.ItineraryRequest) (models.ItineraryResponse, error) {
	req.Destination = strings.TrimSpace(req.Destination)
	if req.Destination == "" {
		return models.ItineraryResponse{}, fmt.Errorf("destination is required")
	}
	if req.Answers == nil {
		req.Answers = map[string]interface{}{}
	}

	prompt := buildItineraryPrompt(req)
	guides := MatchGuides(req.Destination, req.Answers)

	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
		response := fallbackItinerary(req, prompt, guides)
		response.AIProvider = "local-fallback"
		return response, nil
	}

	response, err := s.generateWithOpenAI(ctx, prompt, guides)
	if err != nil {
		fallback := fallbackItinerary(req, prompt, guides)
		fallback.AIProvider = "local-fallback-after-ai-error"
		return fallback, nil
	}
	response.PromptUsed = prompt
	response.MatchedGuides = guides
	response.AIProvider = "openai"
	return response, nil
}

func buildItineraryPrompt(req models.ItineraryRequest) string {
	return fmt.Sprintf(`Create a practical India travel itinerary.

Destination: %s
Age group: %s
Travelling with: %s
Trip type: %s
Accommodation: %s
Transportation: %s
Food preference: %s
Pace: %s
Hidden gems: %s
Local guide preference: %s
Accessibility: %s
Favourite attractions: %s
Activities: %s
Daily start time: %s

Return only JSON with:
summary: short paragraph
days: array of 3 objects with day, title, morning, afternoon, evening, notes array.
Keep suggestions realistic, safe, and beginner friendly.`,
		req.Destination,
		answer(req.Answers, "age_group"),
		answer(req.Answers, "companions"),
		answer(req.Answers, "trip_type"),
		answer(req.Answers, "accommodation"),
		answer(req.Answers, "transportation"),
		answer(req.Answers, "food"),
		answer(req.Answers, "pace"),
		answer(req.Answers, "hidden_gems"),
		answer(req.Answers, "guide"),
		answer(req.Answers, "accessibility"),
		answer(req.Answers, "attractions"),
		answer(req.Answers, "activities"),
		answer(req.Answers, "start_time"),
	)
}

func answer(answers map[string]interface{}, key string) string {
	value, ok := answers[key]
	if !ok || value == nil {
		return "Not specified"
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []interface{}:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprint(typed)
	}
}

func (s ItineraryService) generateWithOpenAI(ctx context.Context, prompt string, guides []models.MatchedGuide) (models.ItineraryResponse, error) {
	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	if model == "" {
		model = "gpt-5-mini"
	}

	body := map[string]interface{}{
		"model": model,
		"input": "You are Nirvaan's backend itinerary planner. Do not chat with the user. " +
			"Return only valid JSON for the requested itinerary.\n\n" + prompt,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return models.ItineraryResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(payload))
	if err != nil {
		return models.ItineraryResponse{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+os.Getenv("OPENAI_API_KEY"))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return models.ItineraryResponse{}, err
	}
	defer resp.Body.Close()

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return models.ItineraryResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return models.ItineraryResponse{}, fmt.Errorf("openai status %d", resp.StatusCode)
	}

	text := extractOutputText(raw)
	var parsed struct {
		Summary string                `json:"summary"`
		Days    []models.ItineraryDay `json:"days"`
	}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return models.ItineraryResponse{}, err
	}
	return models.ItineraryResponse{
		Summary:       parsed.Summary,
		Days:          parsed.Days,
		MatchedGuides: guides,
	}, nil
}

func extractOutputText(raw map[string]interface{}) string {
	if value, ok := raw["output_text"].(string); ok {
		return value
	}
	output, _ := raw["output"].([]interface{})
	for _, item := range output {
		message, _ := item.(map[string]interface{})
		content, _ := message["content"].([]interface{})
		for _, contentItem := range content {
			part, _ := contentItem.(map[string]interface{})
			if text, ok := part["text"].(string); ok {
				return text
			}
		}
	}
	return ""
}

func fallbackItinerary(req models.ItineraryRequest, prompt string, guides []models.MatchedGuide) models.ItineraryResponse {
	pace := answer(req.Answers, "pace")
	food := answer(req.Answers, "food")
	activities := answer(req.Answers, "activities")
	hiddenGems := answer(req.Answers, "hidden_gems")

	return models.ItineraryResponse{
		PromptUsed: prompt,
		Summary: fmt.Sprintf(
			"A %s itinerary for %s with %s food preferences, %s activities, and hidden gems set to %s.",
			strings.ToLower(pace),
			req.Destination,
			strings.ToLower(food),
			strings.ToLower(activities),
			strings.ToLower(hiddenGems),
		),
		Days: []models.ItineraryDay{
			{Day: 1, Title: "Arrival and local orientation", Morning: "Arrive, check in, and keep the first morning light.", Afternoon: "Explore the easiest nearby landmark and understand local transport.", Evening: "Try a local dinner spot and rest early.", Notes: []string{"Keep transfers simple on day one", "Confirm guide availability if needed"}},
			{Day: 2, Title: "Main attractions and culture", Morning: "Start with the most important attraction before crowds build.", Afternoon: "Add a local market, museum, or scenic viewpoint.", Evening: "Choose food or a relaxed walk based on energy.", Notes: []string{"Book timed entries where possible", "Carry water and offline maps"}},
			{Day: 3, Title: "Hidden gems and flexible wrap-up", Morning: "Visit one quieter local spot or nature stop.", Afternoon: "Leave time for shopping, photos, or an optional activity.", Evening: "Finish with a memorable local meal before departure.", Notes: []string{"Keep a buffer before travel", "Use the matched guides for local context"}},
		},
		MatchedGuides: guides,
	}
}
