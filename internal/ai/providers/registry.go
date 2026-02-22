// Package providers implements AI providers for SQL generation.
package providers

import (
	"fmt"

	"github.com/AhmedTheGeek/fastsql/internal/ai"
)

// ProviderType identifies the type of AI provider.
type ProviderType string

const (
	TypeOpenAI     ProviderType = "openai"
	TypeAnthropic  ProviderType = "anthropic"
	TypeOllama     ProviderType = "ollama"
	TypeKimi       ProviderType = "kimi"
	TypeDeepSeek   ProviderType = "deepseek"
	TypeGoogle     ProviderType = "google"
	TypeGroq       ProviderType = "groq"
	TypeOpenRouter ProviderType = "openrouter"
	TypeGeneric    ProviderType = "generic"
)

// ProviderInfo describes a provider type.
type ProviderInfo struct {
	Type        ProviderType
	Name        string
	Description string
	RequiresKey bool
	DefaultURL  string
}

var providerInfos = map[ProviderType]ProviderInfo{
	TypeOpenAI: {
		Type:        TypeOpenAI,
		Name:        "OpenAI",
		Description: "OpenAI GPT models (GPT-4o, GPT-4, etc.)",
		RequiresKey: true,
		DefaultURL:  "https://api.openai.com/v1",
	},
	TypeAnthropic: {
		Type:        TypeAnthropic,
		Name:        "Anthropic",
		Description: "Claude models (Claude 3.5 Sonnet, Claude 3 Opus)",
		RequiresKey: true,
		DefaultURL:  "https://api.anthropic.com",
	},
	TypeOllama: {
		Type:        TypeOllama,
		Name:        "Ollama",
		Description: "Local models via Ollama (llama, codellama, mistral)",
		RequiresKey: false,
		DefaultURL:  "http://localhost:11434",
	},
	TypeKimi: {
		Type:        TypeKimi,
		Name:        "Kimi",
		Description: "Moonshot AI Kimi models",
		RequiresKey: true,
		DefaultURL:  "https://api.moonshot.cn/v1",
	},
	TypeDeepSeek: {
		Type:        TypeDeepSeek,
		Name:        "DeepSeek",
		Description: "DeepSeek Coder models",
		RequiresKey: true,
		DefaultURL:  "https://api.deepseek.com/v1",
	},
	TypeGoogle: {
		Type:        TypeGoogle,
		Name:        "Google",
		Description: "Google Gemini models",
		RequiresKey: true,
		DefaultURL:  "https://generativelanguage.googleapis.com/v1beta",
	},
	TypeGroq: {
		Type:        TypeGroq,
		Name:        "Groq",
		Description: "Groq fast inference",
		RequiresKey: true,
		DefaultURL:  "https://api.groq.com/openai/v1",
	},
	TypeOpenRouter: {
		Type:        TypeOpenRouter,
		Name:        "OpenRouter",
		Description: "OpenRouter multi-model proxy",
		RequiresKey: true,
		DefaultURL:  "https://openrouter.ai/api/v1",
	},
	TypeGeneric: {
		Type:        TypeGeneric,
		Name:        "Generic",
		Description: "Any OpenAI-compatible endpoint (LM Studio, vLLM, etc.)",
		RequiresKey: false,
		DefaultURL:  "",
	},
}

// NewProvider creates a provider of the given type.
func NewProvider(providerType ProviderType, config *ai.ProviderConfig) (ai.Provider, error) {
	switch providerType {
	case TypeOpenAI:
		return NewOpenAI(config)
	case TypeAnthropic:
		return NewAnthropic(config)
	case TypeOllama:
		return NewOllama(config)
	case TypeKimi:
		return NewKimi(config)
	case TypeDeepSeek:
		return NewDeepSeek(config)
	case TypeGoogle:
		return NewGoogle(config)
	case TypeGroq:
		return NewGroq(config)
	case TypeOpenRouter:
		return NewOpenRouter(config)
	case TypeGeneric:
		return NewGeneric(config)
	default:
		return nil, fmt.Errorf("unknown provider type: %s", providerType)
	}
}

// NewProviderFromConfig creates a provider from a config with type field.
func NewProviderFromConfig(config *ai.ProviderConfig) (ai.Provider, error) {
	return NewProvider(ProviderType(config.Type), config)
}

// ListProviderTypes returns all available provider types.
func ListProviderTypes() []ProviderType {
	return []ProviderType{
		TypeOpenAI,
		TypeAnthropic,
		TypeOllama,
		TypeKimi,
		TypeDeepSeek,
		TypeGoogle,
		TypeGroq,
		TypeOpenRouter,
		TypeGeneric,
	}
}

// ListProviderInfo returns info about all providers.
func ListProviderInfo() []ProviderInfo {
	types := ListProviderTypes()
	infos := make([]ProviderInfo, len(types))
	for i, t := range types {
		infos[i] = providerInfos[t]
	}
	return infos
}

// GetProviderInfo returns info about a specific provider type.
func GetProviderInfo(providerType ProviderType) (ProviderInfo, error) {
	info, ok := providerInfos[providerType]
	if !ok {
		return ProviderInfo{}, fmt.Errorf("unknown provider type: %s", providerType)
	}
	return info, nil
}
