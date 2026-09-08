package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultModel = "gemini-3.5-flash-lite"
	maxRetries   = 2
	timeout      = 45 * time.Second
)

var (
	apiKey    string
	modelName string
)

func Load() error {
	godotenv.Load()
	apiKey = os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}
	modelName = os.Getenv("GEMINI_MODEL")
	if modelName == "" {
		modelName = defaultModel
	}
	return nil
}

type ClientInterface interface {
	GetMovieSuggestions(ctx context.Context, systemPrompt string, userPrompt string) (*RecommendationsResponse, error)
	GetWeeklyMovieSuggestions(ctx context.Context, systemPrompt string, userPrompt string) (*RecommendationsResponse, error)
}

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
	}
}

type Content struct {
	Parts []Part `json:"parts"`
}

type Part struct {
	Text string `json:"text"`
}

type GenerationConfig struct {
	ResponseMimeType string          `json:"responseMimeType"`
	ResponseSchema   *ResponseSchema `json:"responseSchema,omitempty"`
}

type ResponseSchema struct {
	Type       string                `json:"type"`
	Properties map[string]SchemaProp `json:"properties,omitempty"`
	Required   []string              `json:"required,omitempty"`
}

type SchemaProp struct {
	Type        string       `json:"type"`
	Description string       `json:"description,omitempty"`
	Items       *SchemaItems `json:"items,omitempty"`
}

type SchemaItems struct {
	Type       string                `json:"type"`
	Properties map[string]SchemaProp `json:"properties,omitempty"`
}

type GenerateRequest struct {
	Contents          []Content        `json:"contents"`
	SystemInstruction *Content         `json:"systemInstruction,omitempty"`
	GenerationConfig  GenerationConfig `json:"generationConfig"`
}

type Candidate struct {
	Content Content `json:"content"`
}

type GenerateResponse struct {
	Candidates []Candidate `json:"candidates"`
}

type RecommendationsResponse struct {
	Recommendations []Recommendation `json:"recommendations"`
}

type Recommendation struct {
	TMDBID      int         `json:"tmdb_id"`
	Title       string      `json:"title"`
	Year        interface{} `json:"year"`
	Genre       interface{} `json:"genre"`
	MatchReason string      `json:"match_reason"`
}

func (c *Client) generateContent(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	model := modelName
	if model == "" {
		model = defaultModel
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		bodyReader := bytes.NewReader(jsonData)
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(time.Duration(1<<attempt) * 500 * time.Millisecond) // 1s, 2s
				continue
			}
			return nil, fmt.Errorf("failed to execute request after %d attempts: %w", maxRetries, lastErr)
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			if attempt < maxRetries {
				time.Sleep(time.Duration(1<<attempt) * 500 * time.Millisecond)
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("Gemini API returned status %d, body: %s", resp.StatusCode, string(bodyBytes))
			// Do not retry on client errors like 400 Bad Request, 401 Unauthorized, 404 Not Found
			if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound {
				return nil, lastErr
			}
			if attempt < maxRetries {
				time.Sleep(time.Duration(1<<attempt) * 1000 * time.Millisecond) // 2s, 4s for server/rate limit errors
				continue
			}
			return nil, lastErr
		}

		var result GenerateResponse
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(time.Duration(1<<attempt) * 500 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return &result, nil
	}

	return nil, lastErr
}

func (c *Client) GetMovieSuggestions(ctx context.Context, systemPrompt string, userPrompt string) (*RecommendationsResponse, error) {
	req := GenerateRequest{
		Contents: []Content{
			{
				Parts: []Part{
					{Text: userPrompt},
				},
			},
		},
		SystemInstruction: &Content{Parts: []Part{{Text: systemPrompt}}},
		GenerationConfig: GenerationConfig{
			ResponseMimeType: "application/json",
			ResponseSchema: &ResponseSchema{
				Type: "OBJECT",
				Properties: map[string]SchemaProp{
					"recommendations": {
						Type:        "ARRAY",
						Description: "Array of movie recommendations",
						Items: &SchemaItems{
							Type: "OBJECT",
							Properties: map[string]SchemaProp{
								"tmdb_id": {
									Type:        "INTEGER",
									Description: "TMDB movie ID (must be positive integer)",
								},
								"title": {
									Type:        "STRING",
									Description: "Movie title",
								},
								"year": {
									Type:        "INTEGER",
									Description: "4-digit release year e.g. 2023",
								},
								"genre": {
									Type:        "ARRAY",
									Description: "Array of 1-3 genre names",
									Items: &SchemaItems{
										Type: "STRING",
									},
								},
								"match_reason": {
									Type:        "STRING",
									Description: "1-2 sentence explanation of cinematic connections (10-50 words)",
								},
							},
						},
					},
				},
				Required: []string{"recommendations"},
			},
		},
	}

	resp, err := c.generateContent(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates returned from Gemini")
	}

	if len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no parts in Gemini response")
	}

	text := resp.Candidates[0].Content.Parts[0].Text

	var result RecommendationsResponse
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal recommendations: %w", err)
	}

	return &result, nil
}

// GetWeeklyMovieSuggestions requests exactly 5 movie recommendations for weekly suggestions
func (c *Client) GetWeeklyMovieSuggestions(ctx context.Context, systemPrompt string, userPrompt string) (*RecommendationsResponse, error) {
	req := GenerateRequest{
		Contents: []Content{
			{
				Parts: []Part{
					{Text: userPrompt},
				},
			},
		},
		SystemInstruction: &Content{Parts: []Part{{Text: systemPrompt}}},
		GenerationConfig: GenerationConfig{
			ResponseMimeType: "application/json",
			ResponseSchema: &ResponseSchema{
				Type: "OBJECT",
				Properties: map[string]SchemaProp{
					"recommendations": {
						Type:        "ARRAY",
						Description: "Array of exactly 5 movie recommendations",
						Items: &SchemaItems{
							Type: "OBJECT",
							Properties: map[string]SchemaProp{
								"tmdb_id": {
									Type:        "INTEGER",
									Description: "TMDB movie ID (must be positive integer)",
								},
								"title": {
									Type:        "STRING",
									Description: "Movie title",
								},
								"year": {
									Type:        "INTEGER",
									Description: "4-digit release year e.g. 2023",
								},
								"genre": {
									Type:        "ARRAY",
									Description: "Array of 1-3 genre names",
									Items: &SchemaItems{
										Type: "STRING",
									},
								},
								"match_reason": {
									Type:        "STRING",
									Description: "1-2 sentence explanation of cinematic connections (10-50 words)",
								},
							},
						},
					},
				},
				Required: []string{"recommendations"},
			},
		},
	}

	resp, err := c.generateContent(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates returned from Gemini")
	}

	if len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no parts in Gemini response")
	}

	text := resp.Candidates[0].Content.Parts[0].Text

	var result RecommendationsResponse
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal recommendations: %w", err)
	}

	return &result, nil
}