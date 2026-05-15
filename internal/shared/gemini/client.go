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
	apiURL       = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent"
	maxRetries   = 3
	timeout      = 30 * time.Second
)

var apiKey string

func Load() error {
	godotenv.Load()
	apiKey = os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}
	return nil
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
	ResponseMimeType string `json:"responseMimeType"`
	ResponseJsonSchema *ResponseJsonSchema `json:"responseJsonSchema,omitempty"`
}

type ResponseJsonSchema struct {
	Type       string                  `json:"type"`
	Properties map[string]SchemaProp   `json:"properties"`
	Required   []string                `json:"required"`
}

type SchemaProp struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Items       *SchemaItems `json:"items,omitempty"`
}

type SchemaItems struct {
	Type string `json:"type"`
	Properties map[string]SchemaProp `json:"properties,omitempty"`
}

type GenerateRequest struct {
	Contents           []Content       `json:"contents"`
	SystemInstruction  *Content        `json:"systemInstruction,omitempty"`
	GenerationConfig   GenerationConfig `json:"generationConfig"`
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
	TMDBID      int      `json:"tmdb_id"`
	Title       string   `json:"title"`
	Year        int      `json:"year"`
	Genre       []string `json:"genre"`
	MatchReason string  `json:"match_reason"`
}

func (c *Client) generateContent(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	url := fmt.Sprintf("%s?key=%s", apiURL, apiKey)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create a fresh request body for each attempt
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
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("failed to execute request after %d attempts: %w", maxRetries, lastErr)
		}

		var result GenerateResponse
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		resp.Body = nil

		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("Gemini API returned status %d, body: %s", resp.StatusCode, string(bodyBytes))
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
				continue
			}
			return nil, lastErr
		}

		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
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
			ResponseJsonSchema: &ResponseJsonSchema{
				Type: "object",
				Properties: map[string]SchemaProp{
					"recommendations": {
						Type: "array",
						Description: "Array of 20 movie recommendations",
						Items: &SchemaItems{
							Type: "object",
							Properties: map[string]SchemaProp{
								"tmdb_id": {
									Type:        "integer",
									Description: "TMDB movie ID (must be positive integer)",
								},
								"title": {
									Type:        "string",
									Description: "Movie title",
								},
								"year": {
									Type:        "integer",
									Description: "Release year between 1970 and 2025",
								},
								"genre": {
									Type:        "array",
									Description: "Array of 1-3 genre names",
									Items: &SchemaItems{
										Type: "string",
									},
								},
								"match_reason": {
									Type:        "string",
									Description: "1-2 sentence explanation tying to user's psychology (10-50 words)",
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
			ResponseJsonSchema: &ResponseJsonSchema{
				Type: "object",
				Properties: map[string]SchemaProp{
					"recommendations": {
						Type:        "array",
						Description: "Array of exactly 5 movie recommendations",
						Items: &SchemaItems{
							Type: "object",
							Properties: map[string]SchemaProp{
								"tmdb_id": {
									Type:        "integer",
									Description: "TMDB movie ID (must be positive integer)",
								},
								"title": {
									Type:        "string",
									Description: "Movie title",
								},
								"year": {
									Type:        "integer",
									Description: "Release year between 1970 and 2026",
								},
								"genre": {
									Type:        "array",
									Description: "Array of 1-3 genre names",
									Items: &SchemaItems{
										Type: "string",
									},
								},
								"match_reason": {
									Type:        "string",
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