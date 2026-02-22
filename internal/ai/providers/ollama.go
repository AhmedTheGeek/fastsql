package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/AhmedTheGeek/fastsql/internal/ai"
)

// OllamaProvider implements the Provider interface for Ollama.
type OllamaProvider struct {
	*baseProvider
}

// NewOllama creates a new Ollama provider.
func NewOllama(config *ai.ProviderConfig) (*OllamaProvider, error) {
	base := newBaseProvider("ollama", "Ollama", config, "http://localhost:11434", false)
	if base.config.Model == "" {
		base.config.Model = "llama3"
	}
	return &OllamaProvider{baseProvider: base}, nil
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done            bool `json:"done"`
	PromptEvalCount int  `json:"prompt_eval_count"`
	EvalCount       int  `json:"eval_count"`
}

func (p *OllamaProvider) GenerateSQL(ctx context.Context, req *ai.GenerateRequest) (*ai.GenerateResponse, error) {
	apiReq := ollamaRequest{
		Model:  p.getModel(),
		Stream: false,
		Messages: []ollamaMessage{
			{Role: "system", Content: buildSystemPrompt(req)},
			{Role: "user", Content: req.Prompt},
		},
		Options: &ollamaOptions{
			Temperature: p.getTemperature(req),
			NumPredict:  p.getMaxTokens(req),
		},
	}

	resp, err := p.doRequest(ctx, "POST", "/api/chat", apiReq, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, p.handleErrorResponse(resp)
	}

	var apiResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &ai.GenerateResponse{
		SQL:        extractSQL(apiResp.Message.Content),
		TokensUsed: apiResp.PromptEvalCount + apiResp.EvalCount,
	}, nil
}

func (p *OllamaProvider) StreamSQL(ctx context.Context, req *ai.GenerateRequest) (<-chan ai.StreamChunk, error) {
	ch := make(chan ai.StreamChunk, 100)

	go func() {
		defer close(ch)

		apiReq := ollamaRequest{
			Model:  p.getModel(),
			Stream: true,
			Messages: []ollamaMessage{
				{Role: "system", Content: buildSystemPrompt(req)},
				{Role: "user", Content: req.Prompt},
			},
			Options: &ollamaOptions{
				Temperature: p.getTemperature(req),
				NumPredict:  p.getMaxTokens(req),
			},
		}

		resp, err := p.doRequest(ctx, "POST", "/api/chat", apiReq, nil)
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
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				ch <- ai.StreamChunk{Error: err, Done: true}
				return
			}

			var streamResp ollamaResponse
			if err := json.Unmarshal(line, &streamResp); err != nil {
				continue
			}

			if streamResp.Message.Content != "" {
				ch <- ai.StreamChunk{Text: streamResp.Message.Content}
			}
			if streamResp.Done {
				ch <- ai.StreamChunk{Done: true, TokensUsed: streamResp.PromptEvalCount + streamResp.EvalCount}
				return
			}
		}

		ch <- ai.StreamChunk{Done: true}
	}()

	return ch, nil
}

func (p *OllamaProvider) ValidateConfig() error {
	// Ollama doesn't require an API key
	return nil
}
