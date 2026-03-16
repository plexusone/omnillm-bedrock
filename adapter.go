// Package bedrock provides AWS Bedrock provider adapter for the omnillm unified interface.
// This is an external provider for github.com/plexusone/omnillm.
package bedrock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/plexusone/omnillm/provider"
)

// Provider represents the Bedrock provider adapter
type Provider struct {
	client BedrockClient
}

// NewProvider creates a new Bedrock provider adapter.
// Use this with omnillm.ClientConfig.CustomProvider to integrate Bedrock.
//
// Example:
//
//	bedrockProvider, err := bedrock.NewProvider("us-east-1")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	client, err := omnillm.NewClient(omnillm.ClientConfig{
//	    CustomProvider: bedrockProvider,
//	})
func NewProvider(region string) (provider.Provider, error) {
	client, err := New(region)
	if err != nil {
		return nil, err
	}
	return &Provider{client: client}, nil
}

// NewProviderWithClient creates a Provider with a custom BedrockClient.
// This is useful for testing with mock clients.
func NewProviderWithClient(client BedrockClient) *Provider {
	return &Provider{client: client}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return p.client.Name()
}

// CreateChatCompletion creates a chat completion using Bedrock's Converse API.
func (p *Provider) CreateChatCompletion(ctx context.Context, req *provider.ChatCompletionRequest) (*provider.ChatCompletionResponse, error) {
	input, err := p.buildConverseInput(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build converse input: %w", err)
	}

	output, err := p.client.Converse(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock converse failed: %w", err)
	}

	return p.convertConverseOutput(output, req.Model)
}

// CreateChatCompletionStream creates a streaming chat completion using Bedrock's ConverseStream API.
func (p *Provider) CreateChatCompletionStream(ctx context.Context, req *provider.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	input, err := p.buildConverseStreamInput(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build converse stream input: %w", err)
	}

	output, err := p.client.ConverseStream(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock converse stream failed: %w", err)
	}

	return newConverseStream(output, req.Model), nil
}

// Close closes the provider
func (p *Provider) Close() error {
	return p.client.Close()
}

// buildConverseInput converts a provider request to Bedrock ConverseInput.
func (p *Provider) buildConverseInput(req *provider.ChatCompletionRequest) (*bedrockruntime.ConverseInput, error) {
	systemBlocks, messages, err := p.convertMessages(req.Messages)
	if err != nil {
		return nil, err
	}

	input := &bedrockruntime.ConverseInput{
		ModelId:  &req.Model,
		Messages: messages,
	}

	if len(systemBlocks) > 0 {
		input.System = systemBlocks
	}

	if cfg := p.buildInferenceConfig(req); cfg != nil {
		input.InferenceConfig = cfg
	}

	if toolCfg := p.buildToolConfig(req); toolCfg != nil {
		input.ToolConfig = toolCfg
	}

	return input, nil
}

// buildConverseStreamInput converts a provider request to Bedrock ConverseStreamInput.
func (p *Provider) buildConverseStreamInput(req *provider.ChatCompletionRequest) (*bedrockruntime.ConverseStreamInput, error) {
	systemBlocks, messages, err := p.convertMessages(req.Messages)
	if err != nil {
		return nil, err
	}

	input := &bedrockruntime.ConverseStreamInput{
		ModelId:  &req.Model,
		Messages: messages,
	}

	if len(systemBlocks) > 0 {
		input.System = systemBlocks
	}

	if cfg := p.buildInferenceConfig(req); cfg != nil {
		input.InferenceConfig = cfg
	}

	if toolCfg := p.buildToolConfig(req); toolCfg != nil {
		input.ToolConfig = toolCfg
	}

	return input, nil
}

// convertMessages separates system messages and converts the rest to Bedrock format.
func (p *Provider) convertMessages(messages []provider.Message) ([]types.SystemContentBlock, []types.Message, error) {
	var systemBlocks []types.SystemContentBlock
	var bedrockMessages []types.Message

	for _, msg := range messages {
		switch msg.Role {
		case provider.RoleSystem:
			systemBlocks = append(systemBlocks, &types.SystemContentBlockMemberText{
				Value: msg.Content,
			})

		case provider.RoleUser:
			bedrockMessages = append(bedrockMessages, types.Message{
				Role: types.ConversationRoleUser,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{Value: msg.Content},
				},
			})

		case provider.RoleAssistant:
			content := p.convertAssistantMessage(msg)
			bedrockMessages = append(bedrockMessages, types.Message{
				Role:    types.ConversationRoleAssistant,
				Content: content,
			})

		case provider.RoleTool:
			// Tool results are sent as user messages with ToolResultBlock
			toolResult := p.convertToolResultMessage(msg)
			bedrockMessages = append(bedrockMessages, types.Message{
				Role:    types.ConversationRoleUser,
				Content: []types.ContentBlock{toolResult},
			})

		default:
			return nil, nil, fmt.Errorf("unsupported message role: %s", msg.Role)
		}
	}

	return systemBlocks, bedrockMessages, nil
}

// convertAssistantMessage converts an assistant message, handling both text and tool calls.
func (p *Provider) convertAssistantMessage(msg provider.Message) []types.ContentBlock {
	var content []types.ContentBlock

	// Add text content if present
	if msg.Content != "" {
		content = append(content, &types.ContentBlockMemberText{Value: msg.Content})
	}

	// Add tool use blocks for any tool calls
	for _, tc := range msg.ToolCalls {
		content = append(content, &types.ContentBlockMemberToolUse{
			Value: types.ToolUseBlock{
				ToolUseId: &tc.ID,
				Name:      &tc.Function.Name,
				Input:     jsonToLazyDocument(tc.Function.Arguments),
			},
		})
	}

	// Ensure we have at least one content block
	if len(content) == 0 {
		content = append(content, &types.ContentBlockMemberText{Value: ""})
	}

	return content
}

// convertToolResultMessage converts a tool result message to Bedrock format.
func (p *Provider) convertToolResultMessage(msg provider.Message) *types.ContentBlockMemberToolResult {
	var toolUseID string
	if msg.ToolCallID != nil {
		toolUseID = *msg.ToolCallID
	}

	return &types.ContentBlockMemberToolResult{
		Value: types.ToolResultBlock{
			ToolUseId: &toolUseID,
			Content: []types.ToolResultContentBlock{
				&types.ToolResultContentBlockMemberText{Value: msg.Content},
			},
		},
	}
}

// buildInferenceConfig creates InferenceConfiguration from request parameters.
func (p *Provider) buildInferenceConfig(req *provider.ChatCompletionRequest) *types.InferenceConfiguration {
	var cfg types.InferenceConfiguration
	hasConfig := false

	if req.MaxTokens != nil {
		v := safeIntToInt32(*req.MaxTokens)
		cfg.MaxTokens = &v
		hasConfig = true
	}

	if req.Temperature != nil {
		v := float32(*req.Temperature)
		cfg.Temperature = &v
		hasConfig = true
	}

	if req.TopP != nil {
		v := float32(*req.TopP)
		cfg.TopP = &v
		hasConfig = true
	}

	if len(req.Stop) > 0 {
		cfg.StopSequences = req.Stop
		hasConfig = true
	}

	if !hasConfig {
		return nil
	}
	return &cfg
}

// buildToolConfig creates ToolConfiguration from request tools.
func (p *Provider) buildToolConfig(req *provider.ChatCompletionRequest) *types.ToolConfiguration {
	if len(req.Tools) == 0 {
		return nil
	}

	var tools []types.Tool
	for _, t := range req.Tools {
		if t.Type != "function" {
			continue
		}

		spec := types.ToolSpecification{
			Name:        &t.Function.Name,
			Description: &t.Function.Description,
			InputSchema: &types.ToolInputSchemaMemberJson{
				Value: anyToLazyDocument(t.Function.Parameters),
			},
		}

		tools = append(tools, &types.ToolMemberToolSpec{Value: spec})
	}

	if len(tools) == 0 {
		return nil
	}

	cfg := &types.ToolConfiguration{
		Tools: tools,
	}

	// Handle tool choice
	if req.ToolChoice != nil {
		cfg.ToolChoice = p.convertToolChoice(req.ToolChoice)
	}

	return cfg
}

// convertToolChoice converts provider tool choice to Bedrock format.
func (p *Provider) convertToolChoice(choice any) types.ToolChoice {
	switch v := choice.(type) {
	case string:
		switch v {
		case "auto":
			return &types.ToolChoiceMemberAuto{Value: types.AutoToolChoice{}}
		case "required":
			return &types.ToolChoiceMemberAny{Value: types.AnyToolChoice{}}
		case "none":
			return nil
		}
	case map[string]any:
		if v["type"] == "function" {
			if fn, ok := v["function"].(map[string]any); ok {
				if name, ok := fn["name"].(string); ok {
					return &types.ToolChoiceMemberTool{
						Value: types.SpecificToolChoice{Name: &name},
					}
				}
			}
		}
	}
	return nil
}

// convertConverseOutput converts Bedrock output to provider response format.
func (p *Provider) convertConverseOutput(output *bedrockruntime.ConverseOutput, model string) (*provider.ChatCompletionResponse, error) {
	// Extract message content from output
	msgOutput, ok := output.Output.(*types.ConverseOutputMemberMessage)
	if !ok {
		return nil, fmt.Errorf("unexpected output type: %T", output.Output)
	}

	// Extract text content and tool calls from message
	var content string
	var toolCalls []provider.ToolCall

	for _, block := range msgOutput.Value.Content {
		switch b := block.(type) {
		case *types.ContentBlockMemberText:
			content = b.Value
		case *types.ContentBlockMemberToolUse:
			toolCalls = append(toolCalls, provider.ToolCall{
				ID:   derefString(b.Value.ToolUseId),
				Type: "function",
				Function: provider.ToolFunction{
					Name:      derefString(b.Value.Name),
					Arguments: documentToJSON(b.Value.Input),
				},
			})
		}
	}

	// Map stop reason
	finishReason := mapStopReason(output.StopReason)

	// Build response message
	msg := provider.Message{
		Role:    provider.RoleAssistant,
		Content: content,
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}

	// Build response
	resp := &provider.ChatCompletionResponse{
		ID:      generateID(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []provider.ChatCompletionChoice{
			{
				Index:        0,
				Message:      msg,
				FinishReason: &finishReason,
			},
		},
	}

	// Map token usage
	if output.Usage != nil {
		resp.Usage = provider.Usage{
			PromptTokens:     int(derefInt32(output.Usage.InputTokens)),
			CompletionTokens: int(derefInt32(output.Usage.OutputTokens)),
			TotalTokens:      int(derefInt32(output.Usage.TotalTokens)),
		}
	}

	return resp, nil
}

// mapStopReason converts Bedrock StopReason to OpenAI-style finish reason.
func mapStopReason(reason types.StopReason) string {
	switch reason {
	case types.StopReasonEndTurn:
		return "stop"
	case types.StopReasonMaxTokens:
		return "length"
	case types.StopReasonStopSequence:
		return "stop"
	case types.StopReasonToolUse:
		return "tool_calls"
	case types.StopReasonContentFiltered:
		return "content_filter"
	default:
		return string(reason)
	}
}

// generateID creates a random ID for the response.
func generateID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "chatcmpl-bedrock"
	}
	return "chatcmpl-" + hex.EncodeToString(b)
}

// derefInt32 safely dereferences an int32 pointer.
func derefInt32(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}

// safeIntToInt32 converts int to int32 with overflow protection.
func safeIntToInt32(n int) int32 {
	const maxInt32 = 1<<31 - 1
	if n > maxInt32 {
		return maxInt32
	}
	if n < -maxInt32-1 {
		return -maxInt32 - 1
	}
	return int32(n)
}

// derefString safely dereferences a string pointer.
func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// documentToJSON converts a Bedrock document to a JSON string.
func documentToJSON(doc document.Interface) string {
	if doc == nil {
		return "{}"
	}
	var result any
	if err := doc.UnmarshalSmithyDocument(&result); err != nil {
		return "{}"
	}
	b, err := json.Marshal(result)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// jsonToLazyDocument converts a JSON string to a Bedrock lazy document.
func jsonToLazyDocument(jsonStr string) document.Interface {
	if jsonStr == "" {
		return document.NewLazyDocument(map[string]any{})
	}
	var result any
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return document.NewLazyDocument(map[string]any{})
	}
	return document.NewLazyDocument(result)
}

// anyToLazyDocument converts any value to a Bedrock lazy document.
func anyToLazyDocument(v any) document.Interface {
	if v == nil {
		return document.NewLazyDocument(map[string]any{})
	}
	return document.NewLazyDocument(v)
}
