package providers

import (
	"github.com/AhmedTheGeek/fastsql/internal/ai"
)

// OpenAI-compatible providers that use the same API format.
// These providers wrap the OpenAI provider with different defaults.

// KimiProvider implements the Provider interface for Moonshot Kimi.
type KimiProvider struct {
	*OpenAIProvider
}

// NewKimi creates a new Kimi provider.
func NewKimi(config *ai.ProviderConfig) (*KimiProvider, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.moonshot.cn/v1"
	}
	if config.Model == "" {
		config.Model = "moonshot-v1-8k"
	}
	
	base := newBaseProvider("kimi", "Kimi", config, config.BaseURL, true)
	base.config = config
	
	return &KimiProvider{
		OpenAIProvider: &OpenAIProvider{baseProvider: base},
	}, nil
}

func (p *KimiProvider) ID() string   { return "kimi" }
func (p *KimiProvider) Name() string { return "Kimi" }

// DeepSeekProvider implements the Provider interface for DeepSeek.
type DeepSeekProvider struct {
	*OpenAIProvider
}

// NewDeepSeek creates a new DeepSeek provider.
func NewDeepSeek(config *ai.ProviderConfig) (*DeepSeekProvider, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.deepseek.com/v1"
	}
	if config.Model == "" {
		config.Model = "deepseek-coder"
	}
	
	base := newBaseProvider("deepseek", "DeepSeek", config, config.BaseURL, true)
	base.config = config
	
	return &DeepSeekProvider{
		OpenAIProvider: &OpenAIProvider{baseProvider: base},
	}, nil
}

func (p *DeepSeekProvider) ID() string   { return "deepseek" }
func (p *DeepSeekProvider) Name() string { return "DeepSeek" }

// GroqProvider implements the Provider interface for Groq.
type GroqProvider struct {
	*OpenAIProvider
}

// NewGroq creates a new Groq provider.
func NewGroq(config *ai.ProviderConfig) (*GroqProvider, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.groq.com/openai/v1"
	}
	if config.Model == "" {
		config.Model = "llama-3.1-70b-versatile"
	}
	
	base := newBaseProvider("groq", "Groq", config, config.BaseURL, true)
	base.config = config
	
	return &GroqProvider{
		OpenAIProvider: &OpenAIProvider{baseProvider: base},
	}, nil
}

func (p *GroqProvider) ID() string   { return "groq" }
func (p *GroqProvider) Name() string { return "Groq" }

// OpenRouterProvider implements the Provider interface for OpenRouter.
type OpenRouterProvider struct {
	*OpenAIProvider
}

// NewOpenRouter creates a new OpenRouter provider.
func NewOpenRouter(config *ai.ProviderConfig) (*OpenRouterProvider, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://openrouter.ai/api/v1"
	}
	if config.Model == "" {
		config.Model = "anthropic/claude-3.5-sonnet"
	}
	
	base := newBaseProvider("openrouter", "OpenRouter", config, config.BaseURL, true)
	base.config = config
	
	provider := &OpenRouterProvider{
		OpenAIProvider: &OpenAIProvider{baseProvider: base},
	}
	
	return provider, nil
}

func (p *OpenRouterProvider) ID() string   { return "openrouter" }
func (p *OpenRouterProvider) Name() string { return "OpenRouter" }

// GenericProvider implements the Provider interface for any OpenAI-compatible endpoint.
type GenericProvider struct {
	*OpenAIProvider
}

// NewGeneric creates a new generic OpenAI-compatible provider.
func NewGeneric(config *ai.ProviderConfig) (*GenericProvider, error) {
	if config.Model == "" {
		config.Model = "default"
	}
	
	base := newBaseProvider("generic", "Generic", config, config.BaseURL, false)
	base.config = config
	
	return &GenericProvider{
		OpenAIProvider: &OpenAIProvider{baseProvider: base},
	}, nil
}

func (p *GenericProvider) ID() string   { return "generic" }
func (p *GenericProvider) Name() string { return "Generic" }

func (p *GenericProvider) ValidateConfig() error {
	if p.baseProvider.baseURL == "" {
		return &ai.ProviderError{
			Provider: "Generic",
			Message:  "base_url is required for generic provider",
		}
	}
	return nil
}
