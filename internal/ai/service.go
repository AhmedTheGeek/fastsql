package ai

import (
	"context"
	"fmt"
	"sync"

	"github.com/AhmedTheGeek/fastsql/internal/schema"
)

// Service manages AI providers and routes requests.
type Service struct {
	mu              sync.RWMutex
	providers       map[string]Provider
	activeProvider  string
	config          *Config
	schemaCache     map[string]*schema.Schema
	schemaCacheMu   sync.RWMutex
}

// Config holds the AI service configuration.
type Config struct {
	// Enabled controls whether AI features are active.
	Enabled bool `toml:"enabled"`
	// DefaultProvider is the provider to use by default.
	DefaultProvider string `toml:"default_provider"`
	// AutoInjectSchema automatically includes schema in prompts.
	AutoInjectSchema bool `toml:"auto_inject_schema"`
	// MaxSchemaTables limits tables included in context.
	MaxSchemaTables int `toml:"max_schema_tables"`
	// Providers holds per-provider configurations.
	Providers map[string]*ProviderConfig `toml:"providers"`
}

// NewService creates a new AI service.
func NewService(config *Config) *Service {
	if config == nil {
		config = &Config{
			Enabled:          false,
			AutoInjectSchema: true,
			MaxSchemaTables:  50,
			Providers:        make(map[string]*ProviderConfig),
		}
	}
	return &Service{
		providers:   make(map[string]Provider),
		config:      config,
		schemaCache: make(map[string]*schema.Schema),
	}
}

// RegisterProvider registers a provider with the service.
func (s *Service) RegisterProvider(id string, provider Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[id] = provider
	if s.activeProvider == "" {
		s.activeProvider = id
	}
}

// UnregisterProvider removes a provider from the service.
func (s *Service) UnregisterProvider(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	provider, ok := s.providers[id]
	if !ok {
		return fmt.Errorf("provider not found: %s", id)
	}
	
	if err := provider.Close(); err != nil {
		return fmt.Errorf("failed to close provider: %w", err)
	}
	
	delete(s.providers, id)
	
	if s.activeProvider == id {
		s.activeProvider = ""
		for k := range s.providers {
			s.activeProvider = k
			break
		}
	}
	
	return nil
}

// SetActiveProvider changes the active provider.
func (s *Service) SetActiveProvider(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, ok := s.providers[id]; !ok {
		return fmt.Errorf("provider not found: %s", id)
	}
	s.activeProvider = id
	return nil
}

// GetActiveProvider returns the currently active provider.
func (s *Service) GetActiveProvider() (Provider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.activeProvider == "" {
		return nil, fmt.Errorf("no active provider")
	}
	
	provider, ok := s.providers[s.activeProvider]
	if !ok {
		return nil, fmt.Errorf("active provider not found: %s", s.activeProvider)
	}
	
	return provider, nil
}

// GetActiveProviderID returns the ID of the active provider.
func (s *Service) GetActiveProviderID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeProvider
}

// ListProviders returns all registered provider IDs.
func (s *Service) ListProviders() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	ids := make([]string, 0, len(s.providers))
	for id := range s.providers {
		ids = append(ids, id)
	}
	return ids
}

// GetProvider returns a specific provider by ID.
func (s *Service) GetProvider(id string) (Provider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	provider, ok := s.providers[id]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", id)
	}
	return provider, nil
}

// GenerateSQL generates SQL using the active provider.
func (s *Service) GenerateSQL(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	provider, err := s.GetActiveProvider()
	if err != nil {
		return nil, err
	}
	
	// Auto-inject schema if enabled and schema is provided
	if s.config.AutoInjectSchema && req.Schema != nil {
		// Schema is already in the request, will be formatted by BuildPrompt
	}
	
	return provider.GenerateSQL(ctx, req)
}

// StreamSQL streams SQL generation using the active provider.
func (s *Service) StreamSQL(ctx context.Context, req *GenerateRequest) (<-chan StreamChunk, error) {
	provider, err := s.GetActiveProvider()
	if err != nil {
		ch := make(chan StreamChunk, 1)
		ch <- StreamChunk{Error: err, Done: true}
		close(ch)
		return ch, nil
	}
	
	if !provider.SupportsStreaming() {
		// Fall back to non-streaming
		ch := make(chan StreamChunk, 1)
		go func() {
			defer close(ch)
			resp, err := provider.GenerateSQL(ctx, req)
			if err != nil {
				ch <- StreamChunk{Error: err, Done: true}
				return
			}
			ch <- StreamChunk{Text: resp.SQL, Done: true, TokensUsed: resp.TokensUsed}
		}()
		return ch, nil
	}
	
	return provider.StreamSQL(ctx, req)
}

// CacheSchema caches a schema for a connection.
func (s *Service) CacheSchema(connID string, sch *schema.Schema) {
	s.schemaCacheMu.Lock()
	defer s.schemaCacheMu.Unlock()
	s.schemaCache[connID] = sch
}

// GetCachedSchema retrieves a cached schema.
func (s *Service) GetCachedSchema(connID string) *schema.Schema {
	s.schemaCacheMu.RLock()
	defer s.schemaCacheMu.RUnlock()
	return s.schemaCache[connID]
}

// InvalidateSchemaCache removes a schema from cache.
func (s *Service) InvalidateSchemaCache(connID string) {
	s.schemaCacheMu.Lock()
	defer s.schemaCacheMu.Unlock()
	delete(s.schemaCache, connID)
}

// IsEnabled returns whether AI features are enabled.
func (s *Service) IsEnabled() bool {
	return s.config.Enabled
}

// Close shuts down the service and all providers.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	var lastErr error
	for id, provider := range s.providers {
		if err := provider.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close provider %s: %w", id, err)
		}
	}
	s.providers = make(map[string]Provider)
	s.activeProvider = ""
	
	return lastErr
}
