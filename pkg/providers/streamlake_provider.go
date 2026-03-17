// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/providers/protocoltypes"
)

type StreamLakeProvider struct {
	apiKey     string
	apiBase    string
	proxy      string
	httpClient *http.Client
}

func NewStreamLakeProvider(apiKey, apiBase, proxy string) *StreamLakeProvider {
	return &StreamLakeProvider{
		apiKey: apiKey,
		apiBase: apiBase,
		proxy: proxy,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *StreamLakeProvider) Chat(
	ctx context.Context,
	messages []protocoltypes.Message,
	tools []ToolDefinition,
	model string,
	options map[string]any,
) (*LLMResponse, error) {
	if p.apiBase == "" {
		return nil, fmt.Errorf("API base not configured")
	}

	apiBase := strings.TrimRight(p.apiBase, "/")
	var urls []string
	if strings.Contains(apiBase, "/chat/completions") {
		urls = []string{apiBase}
	} else if strings.Contains(apiBase, "/endpoints") {
		urls = []string{apiBase + "/chat/completions"}
	} else {
		urls = []string{apiBase + "/chat/completions", apiBase + "/" + model + "/chat/completions"}
	}

	requestBody := map[string]any{
		"model":    model,
		"messages": serializeMessages(messages),
	}

	if len(tools) > 0 {
		requestBody["tools"] = tools
		requestBody["tool_choice"] = "auto"
	}

	if maxTokens, ok := options["max_tokens"].(int); ok {
		requestBody["max_tokens"] = maxTokens
	}

	if temperature, ok := options["temperature"].(float64); ok {
		requestBody["temperature"] = temperature
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	for _, url := range urls {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if p.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.apiKey)
			req.Header.Set("X-API-Key", p.apiKey)
		}

		resp, err := p.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to send request: %w", err)
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response: %w", readErr)
			continue
		}

		if resp.StatusCode >= 400 {
			bodyText := strings.TrimSpace(string(body))
			if bodyText == "" {
				bodyText = "{}"
			}
			lastErr = fmt.Errorf("API request failed:\n  URL: %s\n  Status: %d\n  Body: %s", url, resp.StatusCode, bodyText)
			continue
		}

		var apiResponse struct {
			Choices []struct {
				Message struct {
					Content          string                           `json:"content"`
					ReasoningContent string                           `json:"reasoning_content,omitempty"`
					Reasoning        string                           `json:"reasoning,omitempty"`
					ReasoningDetails []protocoltypes.ReasoningDetail `json:"reasoning_details,omitempty"`
					ToolCalls        []ToolCall                       `json:"tool_calls,omitempty"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage *protocoltypes.UsageInfo `json:"usage"`
		}

		if err := json.Unmarshal(body, &apiResponse); err != nil {
			return nil, fmt.Errorf("failed to decode response from %s: %w", url, err)
		}

		if len(apiResponse.Choices) == 0 {
			return nil, fmt.Errorf("no choices in response from %s", url)
		}

		choice := apiResponse.Choices[0]
		var toolCalls []ToolCall
		if len(choice.Message.ToolCalls) > 0 {
			toolCalls = choice.Message.ToolCalls
		} else {
			toolCalls = extractToolCallsFromResponse(apiResponse.Choices[0].Message.Content)
		}

		return &LLMResponse{
			Content:          choice.Message.Content,
			ReasoningContent: choice.Message.ReasoningContent,
			Reasoning:        choice.Message.Reasoning,
			ReasoningDetails: choice.Message.ReasoningDetails,
			ToolCalls:        toolCalls,
			FinishReason:     choice.FinishReason,
			Usage:            apiResponse.Usage,
		}, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("API request failed: no valid endpoint found")
}

func (p *StreamLakeProvider) GetDefaultModel() string {
	return ""
}

// extractToolCallsFromResponse is a helper function to extract tool calls from response content
// This is a simplified implementation - in practice, you might want to parse more carefully
func extractToolCallsFromResponse(content string) []ToolCall {
	// For now, return empty slice - StreamLake API should return tool_calls in the message field
	return nil
}

type openaiMessage struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
}

func serializeMessages(messages []protocoltypes.Message) []any {
	out := make([]any, 0, len(messages))
	for _, m := range messages {
		if len(m.Media) == 0 {
			out = append(out, openaiMessage{
				Role:             m.Role,
				Content:          m.Content,
				ReasoningContent: m.ReasoningContent,
				ToolCalls:        m.ToolCalls,
				ToolCallID:       m.ToolCallID,
			})
			continue
		}

		parts := make([]map[string]any, 0, 1+len(m.Media))
		if m.Content != "" {
			parts = append(parts, map[string]any{
				"type": "text",
				"text": m.Content,
			})
		}
		for _, mediaURL := range m.Media {
			parts = append(parts, map[string]any{
				"type": "image_url",
				"image_url": map[string]any{
					"url": mediaURL,
				},
			})
		}

		msg := map[string]any{
			"role":    m.Role,
			"content": parts,
		}
		if m.ToolCallID != "" {
			msg["tool_call_id"] = m.ToolCallID
		}
		if len(m.ToolCalls) > 0 {
			msg["tool_calls"] = m.ToolCalls
		}
		if m.ReasoningContent != "" {
			msg["reasoning_content"] = m.ReasoningContent
		}
		out = append(out, msg)
	}
	return out
}
