// Package ai provides AI-powered SQL generation capabilities.
package ai

import (
	"context"

	"github.com/AhmedTheGeek/fastsql/internal/schema"
)

// Provider defines the interface for AI SQL generation providers.
type Provider interface {
	// ID returns the unique identifier for this provider.
	ID() string
	// Name returns the human-readable name for this provider.
	Name() string
	// GenerateSQL generates SQL from a natural language prompt.
	GenerateSQL(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
	// StreamSQL streams SQL generation (if supported).
	StreamSQL(ctx context.Context, req *GenerateRequest) (<-chan StreamChunk, error)
	// SupportsStreaming returns true if this provider supports streaming.
	SupportsStreaming() bool
	// ValidateConfig validates the provider configuration.
	ValidateConfig() error
	// Close releases any resources held by the provider.
	Close() error
}

// ProviderConfig holds configuration for an AI provider.
type ProviderConfig struct {
	// Type is the provider type (openai, anthropic, ollama, etc.)
	Type string `toml:"type"`
	// APIKey is the API key (if required).
	APIKey string `toml:"api_key"`
	// BaseURL is the API endpoint (for custom/self-hosted).
	BaseURL string `toml:"base_url"`
	// Model is the model name to use.
	Model string `toml:"model"`
	// Temperature controls randomness (0-1).
	Temperature float64 `toml:"temperature"`
	// MaxTokens limits the response length.
	MaxTokens int `toml:"max_tokens"`
	// Options holds provider-specific options.
	Options map[string]interface{} `toml:"options"`
}

// GenerateRequest holds the input for SQL generation.
type GenerateRequest struct {
	// Schema is the database schema for context.
	Schema *schema.Schema
	// Prompt is the natural language query.
	Prompt string
	// Dialect is the SQL dialect (postgres, mysql, sqlite).
	Dialect string
	// History is previous conversation messages (optional).
	History []Message
	// MaxTokens limits the response length (overrides config).
	MaxTokens int
	// Temperature controls randomness (overrides config).
	Temperature float64
}

// GenerateResponse holds the output from SQL generation.
type GenerateResponse struct {
	// SQL is the generated SQL query.
	SQL string
	// Explanation is an optional explanation of the query.
	Explanation string
	// TokensUsed is the total tokens consumed.
	TokensUsed int
}

// StreamChunk represents a streaming response chunk.
type StreamChunk struct {
	// Text is the chunk content.
	Text string
	// Done indicates if this is the final chunk.
	Done bool
	// Error holds any error that occurred.
	Error error
	// TokensUsed is populated on the final chunk.
	TokensUsed int
}

// Message represents a conversation message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ProviderError represents an error from an AI provider.
type ProviderError struct {
	Provider   string
	StatusCode int
	Message    string
	Err        error
}

func (e *ProviderError) Error() string {
	if e.Err != nil {
		return e.Provider + ": " + e.Message + ": " + e.Err.Error()
	}
	return e.Provider + ": " + e.Message
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}
