package main

import (
	"fmt"

	"github.com/sipeed/picoclaw/pkg/config"
)

func main() {
	// Test the current configuration
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Provider: "streamlake",
				Model:    "kat-coder-air-v1",
			},
		},
		Providers: config.ProvidersConfig{
			StreamLake: config.ProviderConfig{
				APIKey:  "fLnmpfSX_oG1j1me7KtsFgnhKEbqfWJl_2807_fdtUM",
				APIBase: "https://vanchin.streamlake.ai/api/gateway/v1/endpoints",
			},
		},
	}

	fmt.Printf("Before migration - HasProvidersConfig: %v\n", cfg.HasProvidersConfig())
	fmt.Printf("Before migration - ModelList length: %d\n", len(cfg.ModelList))

	result := config.ConvertProvidersToModelList(cfg)

	fmt.Printf("After migration - ModelList length: %d\n", len(result))
	for i, mc := range result {
		fmt.Printf("Model %d: Name=%s, Model=%s, APIKey=%s\n", i, mc.ModelName, mc.Model, mc.APIKey)
	}
}