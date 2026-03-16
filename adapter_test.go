package bedrock

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/plexusone/omnillm/provider"
)

// mockClient implements BedrockClient for testing.
type mockClient struct {
	name               string
	converseFunc       func(ctx context.Context, input *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error)
	converseStreamFunc func(ctx context.Context, input *bedrockruntime.ConverseStreamInput) (*bedrockruntime.ConverseStreamOutput, error)
}

func (m *mockClient) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock-bedrock"
}

func (m *mockClient) Converse(ctx context.Context, input *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
	if m.converseFunc != nil {
		return m.converseFunc(ctx, input)
	}
	return nil, errors.New("converseFunc not set")
}

func (m *mockClient) ConverseStream(ctx context.Context, input *bedrockruntime.ConverseStreamInput) (*bedrockruntime.ConverseStreamOutput, error) {
	if m.converseStreamFunc != nil {
		return m.converseStreamFunc(ctx, input)
	}
	return nil, errors.New("converseStreamFunc not set")
}

func (m *mockClient) Close() error {
	return nil
}

// Helper to create a pointer to a value.
func ptr[T any](v T) *T {
	return &v
}

func TestProviderName(t *testing.T) {
	mock := &mockClient{name: "test-bedrock"}
	p := NewProviderWithClient(mock)

	if got := p.Name(); got != "test-bedrock" {
		t.Errorf("Name() = %q, want %q", got, "test-bedrock")
	}
}

func TestCreateChatCompletion_SimpleMessage(t *testing.T) {
	inputTokens := int32(10)
	outputTokens := int32(20)
	totalTokens := int32(30)

	mock := &mockClient{
		converseFunc: func(ctx context.Context, input *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
			// Verify input
			if *input.ModelId != "anthropic.claude-3-haiku-20240307-v1:0" {
				t.Errorf("ModelId = %q, want claude-3-haiku", *input.ModelId)
			}
			if len(input.Messages) != 1 {
				t.Errorf("Messages length = %d, want 1", len(input.Messages))
			}
			if input.Messages[0].Role != types.ConversationRoleUser {
				t.Errorf("Message role = %v, want user", input.Messages[0].Role)
			}

			return &bedrockruntime.ConverseOutput{
				Output: &types.ConverseOutputMemberMessage{
					Value: types.Message{
						Role: types.ConversationRoleAssistant,
						Content: []types.ContentBlock{
							&types.ContentBlockMemberText{Value: "Hello! How can I help you?"},
						},
					},
				},
				StopReason: types.StopReasonEndTurn,
				Usage: &types.TokenUsage{
					InputTokens:  &inputTokens,
					OutputTokens: &outputTokens,
					TotalTokens:  &totalTokens,
				},
			}, nil
		},
	}

	p := NewProviderWithClient(mock)
	resp, err := p.CreateChatCompletion(context.Background(), &provider.ChatCompletionRequest{
		Model: "anthropic.claude-3-haiku-20240307-v1:0",
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "Hello!"},
		},
	})

	if err != nil {
		t.Fatalf("CreateChatCompletion() error = %v", err)
	}
	if resp.Object != "chat.completion" {
		t.Errorf("Object = %q, want chat.completion", resp.Object)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("Choices length = %d, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message.Role != provider.RoleAssistant {
		t.Errorf("Message role = %v, want assistant", resp.Choices[0].Message.Role)
	}
	if resp.Choices[0].Message.Content != "Hello! How can I help you?" {
		t.Errorf("Message content = %q, want greeting", resp.Choices[0].Message.Content)
	}
	if *resp.Choices[0].FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want stop", *resp.Choices[0].FinishReason)
	}
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 20 {
		t.Errorf("CompletionTokens = %d, want 20", resp.Usage.CompletionTokens)
	}
}

func TestCreateChatCompletion_WithSystemMessage(t *testing.T) {
	mock := &mockClient{
		converseFunc: func(ctx context.Context, input *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
			// Verify system message is extracted
			if len(input.System) != 1 {
				t.Errorf("System length = %d, want 1", len(input.System))
			}
			// Verify user message is in Messages
			if len(input.Messages) != 1 {
				t.Errorf("Messages length = %d, want 1", len(input.Messages))
			}

			return &bedrockruntime.ConverseOutput{
				Output: &types.ConverseOutputMemberMessage{
					Value: types.Message{
						Role: types.ConversationRoleAssistant,
						Content: []types.ContentBlock{
							&types.ContentBlockMemberText{Value: "I am a helpful assistant."},
						},
					},
				},
				StopReason: types.StopReasonEndTurn,
			}, nil
		},
	}

	p := NewProviderWithClient(mock)
	resp, err := p.CreateChatCompletion(context.Background(), &provider.ChatCompletionRequest{
		Model: "anthropic.claude-3-haiku-20240307-v1:0",
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: "You are a helpful assistant."},
			{Role: provider.RoleUser, Content: "Who are you?"},
		},
	})

	if err != nil {
		t.Fatalf("CreateChatCompletion() error = %v", err)
	}
	if resp.Choices[0].Message.Content != "I am a helpful assistant." {
		t.Errorf("Message content = %q", resp.Choices[0].Message.Content)
	}
}

func TestCreateChatCompletion_WithInferenceConfig(t *testing.T) {
	mock := &mockClient{
		converseFunc: func(ctx context.Context, input *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
			// Verify inference config
			if input.InferenceConfig == nil {
				t.Error("InferenceConfig is nil")
				return nil, errors.New("no inference config")
			}
			if input.InferenceConfig.MaxTokens == nil || *input.InferenceConfig.MaxTokens != 100 {
				t.Errorf("MaxTokens = %v, want 100", input.InferenceConfig.MaxTokens)
			}
			if input.InferenceConfig.Temperature == nil || *input.InferenceConfig.Temperature != 0.7 {
				t.Errorf("Temperature = %v, want 0.7", input.InferenceConfig.Temperature)
			}
			if input.InferenceConfig.TopP == nil || *input.InferenceConfig.TopP != 0.9 {
				t.Errorf("TopP = %v, want 0.9", input.InferenceConfig.TopP)
			}
			if len(input.InferenceConfig.StopSequences) != 1 || input.InferenceConfig.StopSequences[0] != "STOP" {
				t.Errorf("StopSequences = %v, want [STOP]", input.InferenceConfig.StopSequences)
			}

			return &bedrockruntime.ConverseOutput{
				Output: &types.ConverseOutputMemberMessage{
					Value: types.Message{
						Role:    types.ConversationRoleAssistant,
						Content: []types.ContentBlock{&types.ContentBlockMemberText{Value: "OK"}},
					},
				},
				StopReason: types.StopReasonEndTurn,
			}, nil
		},
	}

	p := NewProviderWithClient(mock)
	_, err := p.CreateChatCompletion(context.Background(), &provider.ChatCompletionRequest{
		Model:       "anthropic.claude-3-haiku-20240307-v1:0",
		Messages:    []provider.Message{{Role: provider.RoleUser, Content: "Hi"}},
		MaxTokens:   ptr(100),
		Temperature: ptr(0.7),
		TopP:        ptr(0.9),
		Stop:        []string{"STOP"},
	})

	if err != nil {
		t.Fatalf("CreateChatCompletion() error = %v", err)
	}
}

func TestCreateChatCompletion_Error(t *testing.T) {
	mock := &mockClient{
		converseFunc: func(ctx context.Context, input *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
			return nil, errors.New("bedrock error")
		},
	}

	p := NewProviderWithClient(mock)
	_, err := p.CreateChatCompletion(context.Background(), &provider.ChatCompletionRequest{
		Model:    "anthropic.claude-3-haiku-20240307-v1:0",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "Hi"}},
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestCreateChatCompletion_UnsupportedRole(t *testing.T) {
	mock := &mockClient{}
	p := NewProviderWithClient(mock)

	_, err := p.CreateChatCompletion(context.Background(), &provider.ChatCompletionRequest{
		Model:    "anthropic.claude-3-haiku-20240307-v1:0",
		Messages: []provider.Message{{Role: "invalid", Content: "Hi"}},
	})

	if err == nil {
		t.Error("Expected error for unsupported role, got nil")
	}
}

func TestMapStopReason(t *testing.T) {
	tests := []struct {
		input types.StopReason
		want  string
	}{
		{types.StopReasonEndTurn, "stop"},
		{types.StopReasonMaxTokens, "length"},
		{types.StopReasonStopSequence, "stop"},
		{types.StopReasonToolUse, "tool_calls"},
		{types.StopReasonContentFiltered, "content_filter"},
		{types.StopReason("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := mapStopReason(tt.input); got != tt.want {
				t.Errorf("mapStopReason(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSafeIntToInt32(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int32
	}{
		{"normal", 100, 100},
		{"zero", 0, 0},
		{"negative", -50, -50},
		{"max int32", 2147483647, 2147483647},
		{"overflow", 3000000000, 2147483647}, // Should cap at max int32
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safeIntToInt32(tt.input); got != tt.want {
				t.Errorf("safeIntToInt32(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestDerefString(t *testing.T) {
	s := "hello"
	if got := derefString(&s); got != "hello" {
		t.Errorf("derefString(&s) = %q, want hello", got)
	}
	if got := derefString(nil); got != "" {
		t.Errorf("derefString(nil) = %q, want empty", got)
	}
}

func TestDerefInt32(t *testing.T) {
	n := int32(42)
	if got := derefInt32(&n); got != 42 {
		t.Errorf("derefInt32(&n) = %d, want 42", got)
	}
	if got := derefInt32(nil); got != 0 {
		t.Errorf("derefInt32(nil) = %d, want 0", got)
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == id2 {
		t.Error("generateID() returned same ID twice")
	}
	if len(id1) < 10 {
		t.Errorf("generateID() = %q, too short", id1)
	}
	if id1[:9] != "chatcmpl-" {
		t.Errorf("generateID() = %q, should start with chatcmpl-", id1)
	}
}

func TestCreateChatCompletion_WithTools(t *testing.T) {
	mock := &mockClient{
		converseFunc: func(ctx context.Context, input *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
			// Verify tool config
			if input.ToolConfig == nil {
				t.Error("ToolConfig is nil")
				return nil, errors.New("no tool config")
			}
			if len(input.ToolConfig.Tools) != 1 {
				t.Errorf("Tools length = %d, want 1", len(input.ToolConfig.Tools))
			}

			// Return a tool use response
			toolID := "call_123"
			toolName := "get_weather"
			return &bedrockruntime.ConverseOutput{
				Output: &types.ConverseOutputMemberMessage{
					Value: types.Message{
						Role: types.ConversationRoleAssistant,
						Content: []types.ContentBlock{
							&types.ContentBlockMemberToolUse{
								Value: types.ToolUseBlock{
									ToolUseId: &toolID,
									Name:      &toolName,
									Input:     document.NewLazyDocument(map[string]any{"location": "Seattle"}),
								},
							},
						},
					},
				},
				StopReason: types.StopReasonToolUse,
			}, nil
		},
	}

	p := NewProviderWithClient(mock)
	resp, err := p.CreateChatCompletion(context.Background(), &provider.ChatCompletionRequest{
		Model:    "anthropic.claude-3-haiku-20240307-v1:0",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "What's the weather in Seattle?"}},
		Tools: []provider.Tool{
			{
				Type: "function",
				Function: provider.ToolSpec{
					Name:        "get_weather",
					Description: "Get current weather for a location",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{"type": "string"},
						},
						"required": []string{"location"},
					},
				},
			},
		},
	})

	if err != nil {
		t.Fatalf("CreateChatCompletion() error = %v", err)
	}
	if *resp.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %q, want tool_calls", *resp.Choices[0].FinishReason)
	}
	if len(resp.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("ToolCalls length = %d, want 1", len(resp.Choices[0].Message.ToolCalls))
	}
	tc := resp.Choices[0].Message.ToolCalls[0]
	if tc.ID != "call_123" {
		t.Errorf("ToolCall ID = %q, want call_123", tc.ID)
	}
	if tc.Function.Name != "get_weather" {
		t.Errorf("ToolCall Name = %q, want get_weather", tc.Function.Name)
	}
}

func TestCreateChatCompletion_WithToolResult(t *testing.T) {
	mock := &mockClient{
		converseFunc: func(ctx context.Context, input *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
			// Should have 2 messages: assistant with tool call, user with tool result
			if len(input.Messages) != 2 {
				t.Errorf("Messages length = %d, want 2", len(input.Messages))
			}

			// Second message should be user role (tool results go as user messages)
			if input.Messages[1].Role != types.ConversationRoleUser {
				t.Errorf("Tool result message role = %v, want user", input.Messages[1].Role)
			}

			return &bedrockruntime.ConverseOutput{
				Output: &types.ConverseOutputMemberMessage{
					Value: types.Message{
						Role: types.ConversationRoleAssistant,
						Content: []types.ContentBlock{
							&types.ContentBlockMemberText{Value: "The weather in Seattle is 65°F and cloudy."},
						},
					},
				},
				StopReason: types.StopReasonEndTurn,
			}, nil
		},
	}

	toolCallID := "call_123"
	p := NewProviderWithClient(mock)
	resp, err := p.CreateChatCompletion(context.Background(), &provider.ChatCompletionRequest{
		Model: "anthropic.claude-3-haiku-20240307-v1:0",
		Messages: []provider.Message{
			{
				Role:    provider.RoleAssistant,
				Content: "",
				ToolCalls: []provider.ToolCall{
					{
						ID:   "call_123",
						Type: "function",
						Function: provider.ToolFunction{
							Name:      "get_weather",
							Arguments: `{"location":"Seattle"}`,
						},
					},
				},
			},
			{
				Role:       provider.RoleTool,
				Content:    `{"temperature": 65, "condition": "cloudy"}`,
				ToolCallID: &toolCallID,
			},
		},
	})

	if err != nil {
		t.Fatalf("CreateChatCompletion() error = %v", err)
	}
	if resp.Choices[0].Message.Content != "The weather in Seattle is 65°F and cloudy." {
		t.Errorf("Message content = %q", resp.Choices[0].Message.Content)
	}
}

func TestBuildToolConfig_ToolChoice(t *testing.T) {
	p := &Provider{}

	tests := []struct {
		name       string
		toolChoice any
		wantNil    bool
	}{
		{"auto", "auto", false},
		{"required", "required", false},
		{"none", "none", true},
		{"nil", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.convertToolChoice(tt.toolChoice)
			if tt.wantNil && got != nil {
				t.Errorf("convertToolChoice(%v) = %v, want nil", tt.toolChoice, got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("convertToolChoice(%v) = nil, want non-nil", tt.toolChoice)
			}
		})
	}
}

func TestMultiTurnConversation(t *testing.T) {
	mock := &mockClient{
		converseFunc: func(ctx context.Context, input *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
			// Should have system block and 3 messages
			if len(input.System) != 1 {
				t.Errorf("System length = %d, want 1", len(input.System))
			}
			if len(input.Messages) != 3 {
				t.Errorf("Messages length = %d, want 3", len(input.Messages))
			}
			// Verify alternating roles
			expectedRoles := []types.ConversationRole{
				types.ConversationRoleUser,
				types.ConversationRoleAssistant,
				types.ConversationRoleUser,
			}
			for i, msg := range input.Messages {
				if msg.Role != expectedRoles[i] {
					t.Errorf("Message[%d] role = %v, want %v", i, msg.Role, expectedRoles[i])
				}
			}

			return &bedrockruntime.ConverseOutput{
				Output: &types.ConverseOutputMemberMessage{
					Value: types.Message{
						Role:    types.ConversationRoleAssistant,
						Content: []types.ContentBlock{&types.ContentBlockMemberText{Value: "Response"}},
					},
				},
				StopReason: types.StopReasonEndTurn,
			}, nil
		},
	}

	p := NewProviderWithClient(mock)
	_, err := p.CreateChatCompletion(context.Background(), &provider.ChatCompletionRequest{
		Model: "anthropic.claude-3-haiku-20240307-v1:0",
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: "You are helpful."},
			{Role: provider.RoleUser, Content: "Hello"},
			{Role: provider.RoleAssistant, Content: "Hi there!"},
			{Role: provider.RoleUser, Content: "How are you?"},
		},
	})

	if err != nil {
		t.Fatalf("CreateChatCompletion() error = %v", err)
	}
}
