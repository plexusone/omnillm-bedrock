//go:build integration

package bedrock

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/plexusone/omnillm/provider"
)

// Integration tests require AWS credentials configured via environment or AWS config file.
// Run with: go test -tags=integration -v ./...
//
// Required: AWS credentials with Bedrock access
// Optional: BEDROCK_TEST_REGION (defaults to us-east-1)
// Optional: BEDROCK_TEST_MODEL (defaults to anthropic.claude-3-haiku-20240307-v1:0)

func getTestRegion() string {
	if region := os.Getenv("BEDROCK_TEST_REGION"); region != "" {
		return region
	}
	return "us-east-1"
}

func getTestModel() string {
	if model := os.Getenv("BEDROCK_TEST_MODEL"); model != "" {
		return model
	}
	return "anthropic.claude-3-haiku-20240307-v1:0"
}

func setupProvider(t *testing.T) provider.Provider {
	t.Helper()

	p, err := NewProvider(getTestRegion())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Errorf("Failed to close provider: %v", err)
		}
	})

	return p
}

func TestIntegration_BasicCompletion(t *testing.T) {
	p := setupProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := p.CreateChatCompletion(ctx, &provider.ChatCompletionRequest{
		Model: getTestModel(),
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "Say 'hello' and nothing else."},
		},
		MaxTokens: ptr(50),
	})

	if err != nil {
		t.Fatalf("CreateChatCompletion() error = %v", err)
	}

	// Validate response structure
	if resp.ID == "" {
		t.Error("Response ID is empty")
	}
	if resp.Object != "chat.completion" {
		t.Errorf("Object = %q, want chat.completion", resp.Object)
	}
	if resp.Model != getTestModel() {
		t.Errorf("Model = %q, want %q", resp.Model, getTestModel())
	}
	if resp.Created == 0 {
		t.Error("Created timestamp is 0")
	}

	// Validate choices
	if len(resp.Choices) != 1 {
		t.Fatalf("Choices length = %d, want 1", len(resp.Choices))
	}
	choice := resp.Choices[0]
	if choice.Message.Role != provider.RoleAssistant {
		t.Errorf("Message role = %v, want assistant", choice.Message.Role)
	}
	if choice.Message.Content == "" {
		t.Error("Message content is empty")
	}
	if choice.FinishReason == nil {
		t.Error("FinishReason is nil")
	}

	// Validate usage
	if resp.Usage.PromptTokens == 0 {
		t.Error("PromptTokens is 0")
	}
	if resp.Usage.CompletionTokens == 0 {
		t.Error("CompletionTokens is 0")
	}

	t.Logf("Response: %q (finish: %s, tokens: %d prompt, %d completion)",
		choice.Message.Content, *choice.FinishReason,
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
}

func TestIntegration_SystemMessage(t *testing.T) {
	p := setupProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := p.CreateChatCompletion(ctx, &provider.ChatCompletionRequest{
		Model: getTestModel(),
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: "You are a pirate. Always respond like a pirate."},
			{Role: provider.RoleUser, Content: "Hello!"},
		},
		MaxTokens: ptr(100),
	})

	if err != nil {
		t.Fatalf("CreateChatCompletion() error = %v", err)
	}

	content := strings.ToLower(resp.Choices[0].Message.Content)
	// Check for pirate-like response (this is a heuristic check)
	pirateWords := []string{"ahoy", "arr", "matey", "ye", "avast", "aye"}
	hasPirateWord := false
	for _, word := range pirateWords {
		if strings.Contains(content, word) {
			hasPirateWord = true
			break
		}
	}

	t.Logf("Response: %q", resp.Choices[0].Message.Content)
	if !hasPirateWord {
		t.Log("Warning: Response may not reflect pirate persona (heuristic check)")
	}
}

func TestIntegration_MultiTurn(t *testing.T) {
	p := setupProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := p.CreateChatCompletion(ctx, &provider.ChatCompletionRequest{
		Model: getTestModel(),
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "My name is Alice."},
			{Role: provider.RoleAssistant, Content: "Hello Alice! Nice to meet you."},
			{Role: provider.RoleUser, Content: "What is my name?"},
		},
		MaxTokens: ptr(50),
	})

	if err != nil {
		t.Fatalf("CreateChatCompletion() error = %v", err)
	}

	content := strings.ToLower(resp.Choices[0].Message.Content)
	if !strings.Contains(content, "alice") {
		t.Errorf("Response should mention 'Alice', got: %q", resp.Choices[0].Message.Content)
	}

	t.Logf("Response: %q", resp.Choices[0].Message.Content)
}

func TestIntegration_InferenceConfig(t *testing.T) {
	p := setupProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	temp := 0.0 // Use deterministic output
	resp, err := p.CreateChatCompletion(ctx, &provider.ChatCompletionRequest{
		Model: getTestModel(),
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "What is 2+2? Answer with just the number."},
		},
		MaxTokens:   ptr(10),
		Temperature: &temp,
	})

	if err != nil {
		t.Fatalf("CreateChatCompletion() error = %v", err)
	}

	content := resp.Choices[0].Message.Content
	if !strings.Contains(content, "4") {
		t.Errorf("Expected response to contain '4', got: %q", content)
	}

	t.Logf("Response: %q", content)
}

func TestIntegration_Streaming(t *testing.T) {
	p := setupProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := p.CreateChatCompletionStream(ctx, &provider.ChatCompletionRequest{
		Model: getTestModel(),
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "Count from 1 to 5, each number on a new line."},
		},
		MaxTokens: ptr(100),
	})

	if err != nil {
		t.Fatalf("CreateChatCompletionStream() error = %v", err)
	}
	defer stream.Close()

	var chunks []*provider.ChatCompletionChunk
	var contentBuilder strings.Builder
	var finishReason string
	var usage *provider.Usage

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv() error = %v", err)
		}

		chunks = append(chunks, chunk)

		// Accumulate content
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
			contentBuilder.WriteString(chunk.Choices[0].Delta.Content)

			if chunk.Choices[0].FinishReason != nil {
				finishReason = *chunk.Choices[0].FinishReason
			}
		}

		// Capture usage
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
	}

	if len(chunks) < 2 {
		t.Errorf("Expected multiple chunks, got %d", len(chunks))
	}

	content := contentBuilder.String()
	if content == "" {
		t.Error("Accumulated content is empty")
	}

	if finishReason != "stop" {
		t.Errorf("FinishReason = %q, want stop", finishReason)
	}

	// Check for expected numbers
	for _, num := range []string{"1", "2", "3", "4", "5"} {
		if !strings.Contains(content, num) {
			t.Errorf("Response should contain %q, got: %q", num, content)
		}
	}

	t.Logf("Received %d chunks", len(chunks))
	t.Logf("Content: %q", content)
	if usage != nil {
		t.Logf("Usage: %d prompt, %d completion tokens", usage.PromptTokens, usage.CompletionTokens)
	}
}

func TestIntegration_ToolCalling(t *testing.T) {
	p := setupProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First request: ask about weather with tool available
	resp, err := p.CreateChatCompletion(ctx, &provider.ChatCompletionRequest{
		Model: getTestModel(),
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "What's the weather in Seattle?"},
		},
		Tools: []provider.Tool{
			{
				Type: "function",
				Function: provider.ToolSpec{
					Name:        "get_weather",
					Description: "Get the current weather for a location",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type":        "string",
								"description": "The city name",
							},
						},
						"required": []string{"location"},
					},
				},
			},
		},
		ToolChoice: "auto",
		MaxTokens:  ptr(200),
	})

	if err != nil {
		t.Fatalf("CreateChatCompletion() error = %v", err)
	}

	// Check for tool call
	if len(resp.Choices[0].Message.ToolCalls) == 0 {
		t.Skip("Model did not choose to use the tool - this can happen occasionally")
	}

	tc := resp.Choices[0].Message.ToolCalls[0]
	if tc.Type != "function" {
		t.Errorf("ToolCall type = %q, want function", tc.Type)
	}
	if tc.Function.Name != "get_weather" {
		t.Errorf("ToolCall name = %q, want get_weather", tc.Function.Name)
	}
	if tc.ID == "" {
		t.Error("ToolCall ID is empty")
	}
	if tc.Function.Arguments == "" {
		t.Error("ToolCall arguments is empty")
	}

	// Verify finish reason
	if resp.Choices[0].FinishReason == nil || *resp.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %v, want tool_calls", resp.Choices[0].FinishReason)
	}

	t.Logf("Tool call: %s(%s)", tc.Function.Name, tc.Function.Arguments)

	// Second request: send tool result
	toolCallID := tc.ID
	resp2, err := p.CreateChatCompletion(ctx, &provider.ChatCompletionRequest{
		Model: getTestModel(),
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "What's the weather in Seattle?"},
			{
				Role:      provider.RoleAssistant,
				ToolCalls: resp.Choices[0].Message.ToolCalls,
			},
			{
				Role:       provider.RoleTool,
				Content:    `{"temperature": 55, "condition": "rainy", "humidity": 85}`,
				ToolCallID: &toolCallID,
			},
		},
		Tools: []provider.Tool{
			{
				Type: "function",
				Function: provider.ToolSpec{
					Name:        "get_weather",
					Description: "Get the current weather for a location",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type":        "string",
								"description": "The city name",
							},
						},
						"required": []string{"location"},
					},
				},
			},
		},
		MaxTokens: ptr(200),
	})

	if err != nil {
		t.Fatalf("CreateChatCompletion() with tool result error = %v", err)
	}

	content := strings.ToLower(resp2.Choices[0].Message.Content)
	// Check that response mentions weather details
	weatherWords := []string{"55", "rainy", "rain", "weather", "temperature", "humid"}
	hasWeatherWord := false
	for _, word := range weatherWords {
		if strings.Contains(content, word) {
			hasWeatherWord = true
			break
		}
	}

	if !hasWeatherWord {
		t.Errorf("Response should mention weather details, got: %q", resp2.Choices[0].Message.Content)
	}

	t.Logf("Final response: %q", resp2.Choices[0].Message.Content)
}

func TestIntegration_StreamingToolCall(t *testing.T) {
	p := setupProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := p.CreateChatCompletionStream(ctx, &provider.ChatCompletionRequest{
		Model: getTestModel(),
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "What time is it in Tokyo?"},
		},
		Tools: []provider.Tool{
			{
				Type: "function",
				Function: provider.ToolSpec{
					Name:        "get_time",
					Description: "Get the current time for a timezone",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"timezone": map[string]any{
								"type":        "string",
								"description": "The timezone name (e.g., Asia/Tokyo)",
							},
						},
						"required": []string{"timezone"},
					},
				},
			},
		},
		ToolChoice: "required",
		MaxTokens:  ptr(200),
	})

	if err != nil {
		t.Fatalf("CreateChatCompletionStream() error = %v", err)
	}
	defer stream.Close()

	var chunks []*provider.ChatCompletionChunk
	var toolID, toolName string
	var argumentParts []string
	var finishReason string

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv() error = %v", err)
		}

		chunks = append(chunks, chunk)

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
			delta := chunk.Choices[0].Delta

			// Capture tool call info
			if len(delta.ToolCalls) > 0 {
				tc := delta.ToolCalls[0]
				if tc.ID != "" {
					toolID = tc.ID
				}
				if tc.Function.Name != "" {
					toolName = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					argumentParts = append(argumentParts, tc.Function.Arguments)
				}
			}

			if chunk.Choices[0].FinishReason != nil {
				finishReason = *chunk.Choices[0].FinishReason
			}
		}
	}

	if toolID == "" {
		t.Error("Did not receive tool ID in stream")
	}
	if toolName != "get_time" {
		t.Errorf("Tool name = %q, want get_time", toolName)
	}
	if finishReason != "tool_calls" {
		t.Errorf("FinishReason = %q, want tool_calls", finishReason)
	}

	arguments := strings.Join(argumentParts, "")
	if arguments == "" {
		t.Error("Did not receive tool arguments in stream")
	}

	t.Logf("Received %d chunks", len(chunks))
	t.Logf("Tool: %s (id=%s)", toolName, toolID)
	t.Logf("Arguments: %s", arguments)
}

func TestIntegration_MaxTokens(t *testing.T) {
	p := setupProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Request with very low max tokens to force length limit
	resp, err := p.CreateChatCompletion(ctx, &provider.ChatCompletionRequest{
		Model: getTestModel(),
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "Write a very long story about a dragon."},
		},
		MaxTokens: ptr(5),
	})

	if err != nil {
		t.Fatalf("CreateChatCompletion() error = %v", err)
	}

	if resp.Choices[0].FinishReason == nil {
		t.Error("FinishReason is nil")
	} else if *resp.Choices[0].FinishReason != "length" {
		t.Logf("FinishReason = %q (may be 'stop' if model completed quickly)", *resp.Choices[0].FinishReason)
	}

	t.Logf("Response (%d completion tokens): %q",
		resp.Usage.CompletionTokens, resp.Choices[0].Message.Content)
}
