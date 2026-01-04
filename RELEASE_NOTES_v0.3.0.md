# Release Notes v0.3.0

**Release Date:** January 4, 2026

## Overview

This release completes the core functionality of omnillm-bedrock with streaming support, tool calling, and simplified authentication via Bedrock API keys. The module is now fully functional for production use.

## Highlights

- **Streaming Support** - Real-time token streaming via Bedrock's ConverseStream API
- **Tool Calling** - Full function/tool calling support with streaming tool deltas
- **Bedrock API Keys** - Simplified authentication with `AWS_BEARER_TOKEN_BEDROCK`
- **Comprehensive Testing** - 18 unit tests + 8 integration tests with real AWS calls

## Installation

```bash
go get github.com/agentplexus/omnillm-bedrock@v0.3.0
```

## New Features

### Streaming Support

Stream responses token-by-token for responsive UIs:

```go
stream, err := provider.CreateChatCompletionStream(ctx, &provider.ChatCompletionRequest{
    Model: "anthropic.claude-3-haiku-20240307-v1:0",
    Messages: []provider.Message{
        {Role: provider.RoleUser, Content: "Tell me a story"},
    },
})
defer stream.Close()

for {
    chunk, err := stream.Recv()
    if err == io.EOF {
        break
    }
    fmt.Print(chunk.Choices[0].Delta.Content)
}
```

### Tool Calling

Define tools and let the model decide when to use them:

```go
resp, err := provider.CreateChatCompletion(ctx, &provider.ChatCompletionRequest{
    Model: "anthropic.claude-3-haiku-20240307-v1:0",
    Messages: []provider.Message{
        {Role: provider.RoleUser, Content: "What's the weather in Seattle?"},
    },
    Tools: []provider.Tool{{
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
    }},
    ToolChoice: "auto",
})
```

Tool calling also works with streaming for real-time tool use detection.

### Bedrock API Key Authentication

The simplest way to authenticate - generate a key in the AWS Console:

```bash
export AWS_BEARER_TOKEN_BEDROCK="your-bedrock-api-key"
```

No IAM credentials needed when using API keys. See [AWS Bedrock API Keys](https://docs.aws.amazon.com/bedrock/latest/userguide/api-keys.html) for details.

## Authentication Priority

Credentials are resolved in this order:

| Priority | Source | Environment Variable |
|----------|--------|---------------------|
| 1 | Bedrock API Key | `AWS_BEARER_TOKEN_BEDROCK` |
| 2 | IAM Credentials | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` |
| 3+ | Standard AWS SDK chain | Shared credentials, instance roles, etc. |

## Testing

### Unit Tests (18 tests)

```bash
go test -v ./...
```

### Integration Tests (8 tests)

```bash
# Using Bedrock API key
export AWS_BEARER_TOKEN_BEDROCK="your-key"
go test -tags=integration -v ./...

# Or using IAM credentials
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
go test -tags=integration -v ./...
```

## Migration from v0.2.0

No breaking changes. Existing code continues to work without modification.

**Optional improvements:**

1. Switch to Bedrock API key for simpler auth:
   ```bash
   export AWS_BEARER_TOKEN_BEDROCK="your-key"
   ```

2. Use streaming for better UX:
   ```go
   // Before: blocking call
   resp, err := provider.CreateChatCompletion(ctx, req)

   // After: streaming
   stream, err := provider.CreateChatCompletionStream(ctx, req)
   ```

## What's Next

Planned for future releases:

- Custom AWS credentials provider option
- VPC endpoint support
- Multi-modal inputs (images, documents)
- Retry configuration

## Contributors

- Bearer token authentication implementation
- Streaming and tool calling support
- Integration test suite
- Documentation improvements
