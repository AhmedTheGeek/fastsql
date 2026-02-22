package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AhmedTheGeek/fastsql/internal/ai"
)

// mockOpenAIResponse returns a mock OpenAI API response
func mockOpenAIResponse() string {
	return `{
		"id": "chatcmpl-123",
		"object": "chat.completion",
		"created": 1677652288,
		"model": "gpt-4o",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "SELECT * FROM users WHERE id = 1"
			},
			"finish_reason": "stop"
		}],
		"usage": {
			"prompt_tokens": 9,
			"completion_tokens": 12,
			"total_tokens": 21
		}
	}`
}

// mockAnthropicResponse returns a mock Anthropic API response
func mockAnthropicResponse() string {
	return `{
		"id": "msg_123",
		"type": "message",
		"role": "assistant",
		"model": "claude-3-5-sonnet-20241022",
		"content": [{
			"type": "text",
			"text": "SELECT * FROM users WHERE id = 1"
		}],
		"stop_reason": "end_turn",
		"usage": {
			"input_tokens": 10,
			"output_tokens": 15
		}
	}`
}

// mockOllamaResponse returns a mock Ollama API response
func mockOllamaResponse() string {
	return `{
		"model": "llama3",
		"message": {
			"role": "assistant",
			"content": "SELECT * FROM users WHERE id = 1"
		},
		"done": true,
		"done_reason": "stop",
		"prompt_eval_count": 11,
		"eval_count": 13
	}`
}

// mockGeminiResponse returns a mock Google Gemini API response
func mockGeminiResponse() string {
	return `{
		"candidates": [{
			"content": {
				"parts": [{"text": "SELECT * FROM users WHERE id = 1"}],
				"role": "model"
			},
			"finishReason": "STOP"
		}],
		"usageMetadata": {
			"promptTokenCount": 8,
			"candidatesTokenCount": 14,
			"totalTokenCount": 22
		}
	}`
}

func createTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func TestOpenAIProvider_GenerateSQL(t *testing.T) {
	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing or wrong Authorization header")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockOpenAIResponse()))
	})
	defer server.Close()

	provider, err := NewOpenAI(&ai.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "gpt-4o",
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Close()

	resp, err := provider.GenerateSQL(context.Background(), &ai.GenerateRequest{
		Prompt: "Get user with id 1",
	})
	if err != nil {
		t.Fatalf("GenerateSQL failed: %v", err)
	}

	if resp.SQL != "SELECT * FROM users WHERE id = 1" {
		t.Errorf("unexpected SQL: %s", resp.SQL)
	}
	if resp.TokensUsed != 21 {
		t.Errorf("unexpected tokens: %d", resp.TokensUsed)
	}
}

func TestOpenAIProvider_ValidateConfig(t *testing.T) {
	provider, _ := NewOpenAI(&ai.ProviderConfig{})
	err := provider.ValidateConfig()
	if err == nil {
		t.Error("expected error for missing API key")
	}

	provider, _ = NewOpenAI(&ai.ProviderConfig{APIKey: "key"})
	err = provider.ValidateConfig()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAnthropicProvider_GenerateSQL(t *testing.T) {
	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/messages") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("missing or wrong x-api-key header")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Errorf("missing anthropic-version header")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockAnthropicResponse()))
	})
	defer server.Close()

	provider, err := NewAnthropic(&ai.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Close()

	resp, err := provider.GenerateSQL(context.Background(), &ai.GenerateRequest{
		Prompt: "Get user with id 1",
	})
	if err != nil {
		t.Fatalf("GenerateSQL failed: %v", err)
	}

	if resp.SQL != "SELECT * FROM users WHERE id = 1" {
		t.Errorf("unexpected SQL: %s", resp.SQL)
	}
}

func TestAnthropicProvider_ValidateConfig(t *testing.T) {
	provider, _ := NewAnthropic(&ai.ProviderConfig{})
	err := provider.ValidateConfig()
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestOllamaProvider_GenerateSQL(t *testing.T) {
	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/api/chat") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockOllamaResponse()))
	})
	defer server.Close()

	provider, err := NewOllama(&ai.ProviderConfig{
		BaseURL: server.URL,
		Model:   "llama3",
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Close()

	resp, err := provider.GenerateSQL(context.Background(), &ai.GenerateRequest{
		Prompt: "Get user with id 1",
	})
	if err != nil {
		t.Fatalf("GenerateSQL failed: %v", err)
	}

	if resp.SQL != "SELECT * FROM users WHERE id = 1" {
		t.Errorf("unexpected SQL: %s", resp.SQL)
	}
}

func TestOllamaProvider_NoKeyRequired(t *testing.T) {
	provider, err := NewOllama(&ai.ProviderConfig{})
	if err != nil {
		t.Errorf("should not fail to create: %v", err)
	}
	err = provider.ValidateConfig()
	if err != nil {
		t.Errorf("should not require API key: %v", err)
	}
}

func TestGoogleProvider_GenerateSQL(t *testing.T) {
	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, ":generateContent") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("key") != "test-key" {
			t.Errorf("missing or wrong API key in query")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockGeminiResponse()))
	})
	defer server.Close()

	provider, err := NewGoogle(&ai.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Close()

	resp, err := provider.GenerateSQL(context.Background(), &ai.GenerateRequest{
		Prompt: "Get user with id 1",
	})
	if err != nil {
		t.Fatalf("GenerateSQL failed: %v", err)
	}

	if resp.SQL != "SELECT * FROM users WHERE id = 1" {
		t.Errorf("unexpected SQL: %s", resp.SQL)
	}
}

func TestGoogleProvider_ValidateConfig(t *testing.T) {
	provider, _ := NewGoogle(&ai.ProviderConfig{})
	err := provider.ValidateConfig()
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestKimiProvider_GenerateSQL(t *testing.T) {
	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockOpenAIResponse()))
	})
	defer server.Close()

	provider, err := NewKimi(&ai.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Close()

	resp, err := provider.GenerateSQL(context.Background(), &ai.GenerateRequest{
		Prompt: "Get user with id 1",
	})
	if err != nil {
		t.Fatalf("GenerateSQL failed: %v", err)
	}

	if resp.SQL != "SELECT * FROM users WHERE id = 1" {
		t.Errorf("unexpected SQL: %s", resp.SQL)
	}
}

func TestDeepSeekProvider_GenerateSQL(t *testing.T) {
	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockOpenAIResponse()))
	})
	defer server.Close()

	provider, err := NewDeepSeek(&ai.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Close()

	resp, err := provider.GenerateSQL(context.Background(), &ai.GenerateRequest{
		Prompt: "Get user with id 1",
	})
	if err != nil {
		t.Fatalf("GenerateSQL failed: %v", err)
	}

	if resp.SQL != "SELECT * FROM users WHERE id = 1" {
		t.Errorf("unexpected SQL: %s", resp.SQL)
	}
}

func TestGroqProvider_GenerateSQL(t *testing.T) {
	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockOpenAIResponse()))
	})
	defer server.Close()

	provider, err := NewGroq(&ai.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Close()

	resp, err := provider.GenerateSQL(context.Background(), &ai.GenerateRequest{
		Prompt: "Get user with id 1",
	})
	if err != nil {
		t.Fatalf("GenerateSQL failed: %v", err)
	}

	if resp.SQL != "SELECT * FROM users WHERE id = 1" {
		t.Errorf("unexpected SQL: %s", resp.SQL)
	}
}

func TestOpenRouterProvider_GenerateSQL(t *testing.T) {
	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		// Check OpenRouter-specific headers
		if r.Header.Get("HTTP-Referer") == "" {
			t.Errorf("missing HTTP-Referer header")
		}
		if r.Header.Get("X-Title") == "" {
			t.Errorf("missing X-Title header")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockOpenAIResponse()))
	})
	defer server.Close()

	provider, err := NewOpenRouter(&ai.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Close()

	resp, err := provider.GenerateSQL(context.Background(), &ai.GenerateRequest{
		Prompt: "Get user with id 1",
	})
	if err != nil {
		t.Fatalf("GenerateSQL failed: %v", err)
	}

	if resp.SQL != "SELECT * FROM users WHERE id = 1" {
		t.Errorf("unexpected SQL: %s", resp.SQL)
	}
}

func TestGenericProvider_GenerateSQL(t *testing.T) {
	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockOpenAIResponse()))
	})
	defer server.Close()

	provider, err := NewGeneric(&ai.ProviderConfig{
		BaseURL: server.URL,
		Model:   "local-model",
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Close()

	resp, err := provider.GenerateSQL(context.Background(), &ai.GenerateRequest{
		Prompt: "Get user with id 1",
	})
	if err != nil {
		t.Fatalf("GenerateSQL failed: %v", err)
	}

	if resp.SQL != "SELECT * FROM users WHERE id = 1" {
		t.Errorf("unexpected SQL: %s", resp.SQL)
	}
}

func TestGenericProvider_ValidateConfig(t *testing.T) {
	provider, _ := NewGeneric(&ai.ProviderConfig{})
	err := provider.ValidateConfig()
	if err == nil {
		t.Error("expected error for missing endpoint")
	}
}

func TestGenericProvider_OptionalAPIKey(t *testing.T) {
	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockOpenAIResponse()))
	})
	defer server.Close()

	provider, err := NewGeneric(&ai.ProviderConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("should work without API key: %v", err)
	}
	defer provider.Close()

	_, err = provider.GenerateSQL(context.Background(), &ai.GenerateRequest{
		Prompt: "Get user with id 1",
	})
	if err != nil {
		t.Fatalf("GenerateSQL should work without API key: %v", err)
	}
}

func TestRegistry_NewProvider(t *testing.T) {
	tests := []struct {
		providerType ProviderType
		config       *ai.ProviderConfig
		wantErr      bool
	}{
		{TypeOpenAI, &ai.ProviderConfig{APIKey: "key"}, false},
		{TypeAnthropic, &ai.ProviderConfig{APIKey: "key"}, false},
		{TypeOllama, &ai.ProviderConfig{}, false},
		{TypeKimi, &ai.ProviderConfig{APIKey: "key"}, false},
		{TypeDeepSeek, &ai.ProviderConfig{APIKey: "key"}, false},
		{TypeGoogle, &ai.ProviderConfig{APIKey: "key"}, false},
		{TypeGroq, &ai.ProviderConfig{APIKey: "key"}, false},
		{TypeOpenRouter, &ai.ProviderConfig{APIKey: "key"}, false},
		{TypeGeneric, &ai.ProviderConfig{BaseURL: "http://localhost"}, false},
		{ProviderType("unknown"), &ai.ProviderConfig{}, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.providerType), func(t *testing.T) {
			provider, err := NewProvider(tt.providerType, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
			if provider != nil {
				provider.Close()
			}
		})
	}
}

func TestRegistry_NewProviderFromConfig(t *testing.T) {
	config := &ai.ProviderConfig{
		Type:   "openai",
		APIKey: "test-key",
	}

	provider, err := NewProviderFromConfig(config)
	if err != nil {
		t.Fatalf("NewProviderFromConfig failed: %v", err)
	}
	defer provider.Close()

	if provider.ID() != "openai" {
		t.Errorf("unexpected ID: %s", provider.ID())
	}
}

func TestRegistry_ListProviderTypes(t *testing.T) {
	types := ListProviderTypes()
	if len(types) != 9 {
		t.Errorf("expected 9 provider types, got %d", len(types))
	}
}

func TestRegistry_ProviderInfo(t *testing.T) {
	infos := ListProviderInfo()
	if len(infos) != 9 {
		t.Errorf("expected 9 provider infos, got %d", len(infos))
	}

	info, err := GetProviderInfo(TypeOpenAI)
	if err != nil {
		t.Fatalf("GetProviderInfo failed: %v", err)
	}
	if info.Name != "OpenAI" {
		t.Errorf("unexpected name: %s", info.Name)
	}
	if !info.RequiresKey {
		t.Error("OpenAI should require key")
	}
}

func TestProvider_ErrorHandling(t *testing.T) {
	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	})
	defer server.Close()

	provider, _ := NewOpenAI(&ai.ProviderConfig{
		APIKey:  "invalid-key",
		BaseURL: server.URL,
	})
	defer provider.Close()

	_, err := provider.GenerateSQL(context.Background(), &ai.GenerateRequest{
		Prompt: "Get user with id 1",
	})
	if err == nil {
		t.Error("expected error for unauthorized request")
	}
}

func TestProvider_ContextCancellation(t *testing.T) {
	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		select {
		case <-r.Context().Done():
			return
		}
	})
	defer server.Close()

	provider, _ := NewOpenAI(&ai.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	defer provider.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := provider.GenerateSQL(ctx, &ai.GenerateRequest{
		Prompt: "Get user with id 1",
	})
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestProvider_SupportsStreaming(t *testing.T) {
	tests := []struct {
		name     string
		provider ai.Provider
		expected bool
	}{
		{"OpenAI", must(NewOpenAI(&ai.ProviderConfig{APIKey: "key"})), true},
		{"Anthropic", must(NewAnthropic(&ai.ProviderConfig{APIKey: "key"})), true},
		{"Ollama", must(NewOllama(&ai.ProviderConfig{})), true},
		{"Google", must(NewGoogle(&ai.ProviderConfig{APIKey: "key"})), true},
		{"Generic", must(NewGeneric(&ai.ProviderConfig{BaseURL: "http://localhost"})), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.provider.SupportsStreaming() != tt.expected {
				t.Errorf("expected SupportsStreaming() = %v", tt.expected)
			}
			tt.provider.Close()
		})
	}
}

func TestProvider_IDAndName(t *testing.T) {
	provider, _ := NewOpenAI(&ai.ProviderConfig{APIKey: "key"})
	defer provider.Close()

	if provider.ID() != "openai" {
		t.Errorf("unexpected ID: %s", provider.ID())
	}
	if provider.Name() != "OpenAI" {
		t.Errorf("unexpected Name: %s", provider.Name())
	}
}

func TestProvider_RequestBody(t *testing.T) {
	var receivedBody map[string]interface{}

	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockOpenAIResponse()))
	})
	defer server.Close()

	provider, _ := NewOpenAI(&ai.ProviderConfig{
		APIKey:      "test-key",
		BaseURL:     server.URL,
		Temperature: 0.5,
		MaxTokens:   100,
	})
	defer provider.Close()

	provider.GenerateSQL(context.Background(), &ai.GenerateRequest{
		Prompt: "Get user with id 1",
	})

	// Check request body was constructed correctly
	if receivedBody["model"] != "gpt-4o" {
		t.Errorf("unexpected model: %v", receivedBody["model"])
	}
	if receivedBody["temperature"] != 0.5 {
		t.Errorf("unexpected temperature: %v", receivedBody["temperature"])
	}
	if receivedBody["max_tokens"] != float64(100) {
		t.Errorf("unexpected max_tokens: %v", receivedBody["max_tokens"])
	}
}

func TestExtractSQL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SELECT * FROM users", "SELECT * FROM users"},
		{"```sql\nSELECT * FROM users\n```", "SELECT * FROM users"},
		{"```\nSELECT * FROM users\n```", "SELECT * FROM users"},
		{"  SELECT * FROM users  ", "SELECT * FROM users"},
	}

	for _, tt := range tests {
		result := extractSQL(tt.input)
		if result != tt.expected {
			t.Errorf("extractSQL(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
