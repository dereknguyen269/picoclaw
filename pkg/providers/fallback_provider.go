package providers

import (
	"context"
	"fmt"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// FallbackEntry pairs a provider with the model name to use for it.
type FallbackEntry struct {
	Provider LLMProvider
	Model    string
}

// FallbackProvider tries the primary provider first, then falls back
// to alternatives in order if the primary returns an error.
type FallbackProvider struct {
	primary   LLMProvider
	model     string
	fallbacks []FallbackEntry
}

func NewFallbackProvider(primary LLMProvider, model string, fallbacks []FallbackEntry) *FallbackProvider {
	return &FallbackProvider{
		primary:   primary,
		model:     model,
		fallbacks: fallbacks,
	}
}

func (p *FallbackProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, model string, options map[string]interface{}) (*LLMResponse, error) {
	// Try primary
	resp, err := p.primary.Chat(ctx, messages, tools, model, options)
	if err == nil {
		return resp, nil
	}

	logger.WarnCF("provider", fmt.Sprintf("Primary provider failed: %v, trying fallbacks", err),
		map[string]interface{}{"model": model})

	// Try fallbacks in order
	var lastErr error
	for i, fb := range p.fallbacks {
		fbModel := fb.Model
		if fbModel == "" {
			fbModel = model
		}

		logger.InfoCF("provider", fmt.Sprintf("Trying fallback #%d", i+1),
			map[string]interface{}{"model": fbModel})

		resp, lastErr = fb.Provider.Chat(ctx, messages, tools, fbModel, options)
		if lastErr == nil {
			logger.InfoCF("provider", fmt.Sprintf("Fallback #%d succeeded", i+1),
				map[string]interface{}{"model": fbModel})
			return resp, nil
		}

		logger.WarnCF("provider", fmt.Sprintf("Fallback #%d failed: %v", i+1, lastErr),
			map[string]interface{}{"model": fbModel})
	}

	// All failed — return the last error
	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
	}
	return nil, err
}

func (p *FallbackProvider) GetDefaultModel() string {
	return p.model
}
