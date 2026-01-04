# Roadmap

This document outlines the remaining work to make omnillm-bedrock a fully functional AWS Bedrock provider.

## Current Status

The module is **fully functional** for text chat completions and tool calling using Bedrock's Converse API. Streaming is supported.

## Phase 1: Core Implementation - COMPLETE

- [x] Implement Bedrock Converse API integration in `bedrock.go`
- [x] Implement `Provider.CreateChatCompletion` in `adapter.go`
  - Convert `provider.ChatCompletionRequest` to `ConverseInput`
  - Handle system/user/assistant message roles
  - Map inference parameters (MaxTokens, Temperature, TopP, Stop)
  - Convert `ConverseOutput` to `provider.ChatCompletionResponse`

## Phase 2: Streaming Support - COMPLETE

- [x] Implement `Provider.CreateChatCompletionStream` in `adapter.go`
  - Use Bedrock's `ConverseStream` API
  - Return a `provider.ChatCompletionStream` implementation
- [x] Create `stream.go` with streaming event handling
  - Handle text deltas, tool use deltas, message stop, and metadata events
  - Proper token usage reporting

## Phase 3: Tool Calling - COMPLETE

- [x] Tool/function calling support
  - Map `provider.Tool` to Bedrock `ToolConfiguration`
  - Handle tool choice (auto, required, specific tool)
  - Parse tool use responses from model
  - Handle `provider.RoleTool` messages for tool results
  - Streaming tool call support

## Phase 4: Model Support

- [x] Bedrock Converse API provides unified interface across models
  - Works with Claude, Titan, Llama, Cohere, and other models
  - No model-specific request formatting needed

## Phase 5: Testing - COMPLETE

- [x] Unit tests (18 tests passing)
  - Mock Bedrock client for isolated testing
  - Message conversion (system/user/assistant/tool)
  - Inference config mapping
  - Tool config and tool choice
  - Response parsing and stop reasons
  - Streaming text and tool deltas
  - Helper functions

- [x] Integration tests (8 tests, requires AWS credentials)
  - Basic completion, system messages, multi-turn conversations
  - Inference config (temperature, max tokens)
  - Streaming text responses
  - Tool calling (request and response)
  - Streaming tool calls
  - Run with: `go test -tags=integration -v ./...`

## Phase 6: Additional Features

- [ ] Configuration options
  - Custom AWS credentials provider
  - Endpoint override for VPC endpoints
  - Retry configuration

- [ ] Image/document support
  - Handle multi-modal inputs via ContentBlock

## Non-Goals

- Supporting Bedrock Knowledge Bases (out of scope for chat completions)
- Bedrock Agents integration
- Fine-tuned model management
