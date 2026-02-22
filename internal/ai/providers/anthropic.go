package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/AhmedTheGeek/fastsql/internal/ai"
)

// AnthropicProvider implements the Provider interface for Anthropic Claude.
type AnthropicProvider struct {
	*baseProvider
}

// NewAnthropic creates a new Anthropic provider.
func NewAnthropic(config *ai.ProviderConfig) (*AnthropicProvider, error) {
	base := newBaseProvider("anthropic", "Anthropic", config, "https://api.anthropic.com", true)
	if base.config.Model == "" {
		base.config.Model = "claude-sonnet-4-20250514"
	}
	return &AnthropicProvider{baseProvider: base}, nil
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Stream      bool               `json:"stream,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Text string `json:"text"`
	} `json:"delta"`
}

func (p *AnthropicProvider) GenerateSQL(ctx context.Context, req *ai.GenerateRequest) (*ai.GenerateResponse, error) {
	apiReq := anthropicRequest{
		Model:       p.getModel(),
		MaxTokens:   p.getMaxTokens(req),
		System:      buildSystemPrompt(req),
		Temperature: p.getTemperature(req),
		Messages: []anthropicMessage{
			{Role: "user", Content: req.Prompt},
		},
	}

	headers := map[string]string{
		"x-api-key":         p.config.APIKey,
		"anthropic-version": "2023-06-01",
	}

	resp, err := p.doRequest(ctx, "POST", "/v1/messages", apiReq, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, p.handleErrorResponse(resp)
	}

	var apiResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("no response from Anthropic")
	}

	return &ai.GenerateResponse{
		SQL:        extractSQL(apiResp.Content[0].Text),
		TokensUsed: apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens,
	}, nil
}

func (p *AnthropicProvider) StreamSQL(ctx context.Context, req *ai.GenerateRequest) (<-chan ai.StreamChunk, error) {
	ch := make(chan ai.StreamChunk, 100)

	go func() {
		defer close(ch)

		apiReq := anthropicRequest{
			Model:       p.getModel(),
			MaxTokens:   p.getMaxTokens(req),
			System:      buildSystemPrompt(req),
			Temperature: p.getTemperature(req),
			Stream:      true,
			Messages: []anthropicMessage{
				{Role: "user", Content: req.Prompt},
			},
		}

		headers := map[string]string{
			"x-api-key":         p.config.APIKey,
			"anthropic-version": "2023-06-01",
		}

		resp, err := p.doRequest(ctx, "POST", "/v1/messages", apiReq, headers)
		if err != nil {
			ch <- ai.StreamChunk{Error: err, Done: true}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			ch <- ai.StreamChunk{Error: p.handleErrorResponse(resp), Done: true}
			return
		}

		reader := bufio.NewReader(resp.Body)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				ch <- ai.StreamChunk{Error: err, Done: true}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			if event.Type == "content_block_delta" && event.Delta.Text != "" {
				ch <- ai.StreamChunk{Text: event.Delta.Text}
			}
			if event.Type == "message_stop" {
				ch <- ai.StreamChunk{Done: true}
				return
			}
		}

		ch <- ai.StreamChunk{Done: true}
	}()

	return ch, nil
}
