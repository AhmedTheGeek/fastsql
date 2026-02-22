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

// OpenAIProvider implements the Provider interface for OpenAI.
type OpenAIProvider struct {
	*baseProvider
}

// NewOpenAI creates a new OpenAI provider.
func NewOpenAI(config *ai.ProviderConfig) (*OpenAIProvider, error) {
	base := newBaseProvider("openai", "OpenAI", config, "https://api.openai.com/v1", true)
	if base.config.Model == "" {
		base.config.Model = "gpt-4o"
	}
	return &OpenAIProvider{baseProvider: base}, nil
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIStreamResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func (p *OpenAIProvider) GenerateSQL(ctx context.Context, req *ai.GenerateRequest) (*ai.GenerateResponse, error) {
	messages := []openAIMessage{
		{Role: "system", Content: buildSystemPrompt(req)},
		{Role: "user", Content: req.Prompt},
	}

	// Add history if present
	for _, msg := range req.History {
		messages = append(messages, openAIMessage{Role: msg.Role, Content: msg.Content})
	}

	apiReq := openAIRequest{
		Model:       p.getModel(),
		Messages:    messages,
		Temperature: p.getTemperature(req),
		MaxTokens:   p.getMaxTokens(req),
	}

	headers := map[string]string{
		"Authorization": "Bearer " + p.config.APIKey,
	}

	resp, err := p.doRequest(ctx, "POST", "/chat/completions", apiReq, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, p.handleErrorResponse(resp)
	}

	var apiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	return &ai.GenerateResponse{
		SQL:        extractSQL(apiResp.Choices[0].Message.Content),
		TokensUsed: apiResp.Usage.TotalTokens,
	}, nil
}

func (p *OpenAIProvider) StreamSQL(ctx context.Context, req *ai.GenerateRequest) (<-chan ai.StreamChunk, error) {
	ch := make(chan ai.StreamChunk, 100)

	go func() {
		defer close(ch)

		messages := []openAIMessage{
			{Role: "system", Content: buildSystemPrompt(req)},
			{Role: "user", Content: req.Prompt},
		}

		apiReq := openAIRequest{
			Model:       p.getModel(),
			Messages:    messages,
			Temperature: p.getTemperature(req),
			MaxTokens:   p.getMaxTokens(req),
			Stream:      true,
		}

		headers := map[string]string{
			"Authorization": "Bearer " + p.config.APIKey,
		}

		resp, err := p.doRequest(ctx, "POST", "/chat/completions", apiReq, headers)
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
		var fullText strings.Builder

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
			if line == "" || line == "data: [DONE]" {
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			var streamResp openAIStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}

			if len(streamResp.Choices) > 0 {
				content := streamResp.Choices[0].Delta.Content
				if content != "" {
					fullText.WriteString(content)
					ch <- ai.StreamChunk{Text: content}
				}
				if streamResp.Choices[0].FinishReason == "stop" {
					ch <- ai.StreamChunk{Done: true}
					return
				}
			}
		}

		ch <- ai.StreamChunk{Done: true}
	}()

	return ch, nil
}
