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

// GoogleProvider implements the Provider interface for Google Gemini.
type GoogleProvider struct {
	*baseProvider
}

// NewGoogle creates a new Google Gemini provider.
func NewGoogle(config *ai.ProviderConfig) (*GoogleProvider, error) {
	base := newBaseProvider("google", "Google", config, "https://generativelanguage.googleapis.com/v1beta", true)
	if base.config.Model == "" {
		base.config.Model = "gemini-1.5-flash"
	}
	return &GoogleProvider{baseProvider: base}, nil
}

type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	SystemInstruction *geminiContent        `json:"systemInstruction,omitempty"`
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func (p *GoogleProvider) GenerateSQL(ctx context.Context, req *ai.GenerateRequest) (*ai.GenerateResponse, error) {
	apiReq := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: buildSystemPrompt(req)}},
		},
		Contents: []geminiContent{
			{
				Role:  "user",
				Parts: []geminiPart{{Text: req.Prompt}},
			},
		},
		GenerationConfig: &geminiGenerationConfig{
			Temperature:     p.getTemperature(req),
			MaxOutputTokens: p.getMaxTokens(req),
		},
	}

	path := fmt.Sprintf("/models/%s:generateContent?key=%s", p.getModel(), p.config.APIKey)

	resp, err := p.doRequest(ctx, "POST", path, apiReq, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, p.handleErrorResponse(resp)
	}

	var apiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Candidates) == 0 || len(apiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	return &ai.GenerateResponse{
		SQL:        extractSQL(apiResp.Candidates[0].Content.Parts[0].Text),
		TokensUsed: apiResp.UsageMetadata.TotalTokenCount,
	}, nil
}

func (p *GoogleProvider) StreamSQL(ctx context.Context, req *ai.GenerateRequest) (<-chan ai.StreamChunk, error) {
	ch := make(chan ai.StreamChunk, 100)

	go func() {
		defer close(ch)

		apiReq := geminiRequest{
			SystemInstruction: &geminiContent{
				Parts: []geminiPart{{Text: buildSystemPrompt(req)}},
			},
			Contents: []geminiContent{
				{
					Role:  "user",
					Parts: []geminiPart{{Text: req.Prompt}},
				},
			},
			GenerationConfig: &geminiGenerationConfig{
				Temperature:     p.getTemperature(req),
				MaxOutputTokens: p.getMaxTokens(req),
			},
		}

		path := fmt.Sprintf("/models/%s:streamGenerateContent?key=%s&alt=sse", p.getModel(), p.config.APIKey)

		resp, err := p.doRequest(ctx, "POST", path, apiReq, nil)
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
		var totalTokens int

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
			var streamResp geminiResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}

			if len(streamResp.Candidates) > 0 && len(streamResp.Candidates[0].Content.Parts) > 0 {
				text := streamResp.Candidates[0].Content.Parts[0].Text
				if text != "" {
					ch <- ai.StreamChunk{Text: text}
				}
			}
			if streamResp.UsageMetadata.TotalTokenCount > 0 {
				totalTokens = streamResp.UsageMetadata.TotalTokenCount
			}
		}

		ch <- ai.StreamChunk{Done: true, TokensUsed: totalTokens}
	}()

	return ch, nil
}
