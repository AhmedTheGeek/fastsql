package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/AhmedTheGeek/fastsql/internal/ai"
)

// baseProvider provides common functionality for HTTP-based providers.
type baseProvider struct {
	id          string
	name        string
	config      *ai.ProviderConfig
	client      *http.Client
	baseURL     string
	requiresKey bool
}

func newBaseProvider(id, name string, config *ai.ProviderConfig, defaultURL string, requiresKey bool) *baseProvider {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = defaultURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &baseProvider{
		id:          id,
		name:        name,
		config:      config,
		baseURL:     baseURL,
		requiresKey: requiresKey,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (p *baseProvider) ID() string {
	return p.id
}

func (p *baseProvider) Name() string {
	return p.name
}

func (p *baseProvider) ValidateConfig() error {
	if p.requiresKey && p.config.APIKey == "" {
		return fmt.Errorf("%s requires an API key", p.name)
	}
	return nil
}

func (p *baseProvider) Close() error {
	p.client.CloseIdleConnections()
	return nil
}

func (p *baseProvider) SupportsStreaming() bool {
	return true
}

func (p *baseProvider) doRequest(ctx context.Context, method, path string, body interface{}, headers map[string]string) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	url := p.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

func (p *baseProvider) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return &ai.ProviderError{
		Provider:   p.name,
		StatusCode: resp.StatusCode,
		Message:    fmt.Sprintf("API error %d: %s", resp.StatusCode, string(body)),
	}
}

func (p *baseProvider) getModel() string {
	if p.config.Model != "" {
		return p.config.Model
	}
	return ""
}

func (p *baseProvider) getTemperature(req *ai.GenerateRequest) float64 {
	if req.Temperature > 0 {
		return req.Temperature
	}
	if p.config.Temperature > 0 {
		return p.config.Temperature
	}
	return 0.2 // Default low temperature for SQL generation
}

func (p *baseProvider) getMaxTokens(req *ai.GenerateRequest) int {
	if req.MaxTokens > 0 {
		return req.MaxTokens
	}
	if p.config.MaxTokens > 0 {
		return p.config.MaxTokens
	}
	return 2000 // Default
}

// buildSystemPrompt creates the system prompt for SQL generation.
func buildSystemPrompt(req *ai.GenerateRequest) string {
	var sb strings.Builder
	sb.WriteString("You are an expert SQL developer. Generate valid ")
	if req.Dialect != "" {
		sb.WriteString(req.Dialect)
		sb.WriteString(" ")
	}
	sb.WriteString("SQL based on the user's request.\n\n")

	if req.Schema != nil {
		builder := ai.NewContextBuilder()
		sb.WriteString("DATABASE SCHEMA:\n")
		sb.WriteString(builder.FormatSchema(req.Schema))
		sb.WriteString("\n\n")
	}

	sb.WriteString("RULES:\n")
	sb.WriteString("- Output ONLY the SQL query, no explanations or markdown\n")
	sb.WriteString("- Use table and column names exactly as shown\n")
	sb.WriteString("- Use explicit JOINs\n")
	sb.WriteString("- Make reasonable assumptions if the request is ambiguous\n")

	return sb.String()
}

// extractSQL extracts SQL from a response that might have markdown fences.
func extractSQL(text string) string {
	text = strings.TrimSpace(text)
	
	// Remove markdown code fences if present
	sqlFenceRe := regexp.MustCompile("(?s)```(?:sql)?\\s*\\n?(.*?)\\n?```")
	if matches := sqlFenceRe.FindStringSubmatch(text); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	
	// Remove generic code fences
	fenceRe := regexp.MustCompile("(?s)```\\s*\\n?(.*?)\\n?```")
	if matches := fenceRe.FindStringSubmatch(text); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	
	return text
}
