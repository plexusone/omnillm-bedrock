# fluxllm-bedrock

AWS Bedrock provider for [fluxllm](https://github.com/grokify/fluxllm).

This is an external provider module that demonstrates how to create custom providers for fluxllm without adding heavy dependencies to the core library.

## Installation

```bash
go get github.com/grokify/fluxllm-bedrock
```

## Usage

```go
package main

import (
    "context"
    "log"

    "github.com/grokify/fluxllm"
    "github.com/grokify/fluxllm-bedrock"
    "github.com/grokify/fluxllm/provider"
)

func main() {
    // Create the Bedrock provider
    bedrockProvider, err := bedrock.NewProvider("us-east-1")
    if err != nil {
        log.Fatal(err)
    }

    // Use it with fluxllm via CustomProvider
    client, err := fluxllm.NewClient(fluxllm.ClientConfig{
        CustomProvider: bedrockProvider,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Make requests as usual
    resp, err := client.CreateChatCompletion(context.Background(), &provider.ChatCompletionRequest{
        Model: "anthropic.claude-3-sonnet-20240229-v1:0",
        Messages: []provider.Message{
            {Role: provider.RoleUser, Content: "Hello!"},
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Println(resp.Choices[0].Message.Content)
}
```

## Why a Separate Module?

The AWS SDK v2 pulls in 17+ transitive dependencies (credentials, STS, SSO, EC2 IMDS, etc.). By keeping Bedrock in a separate module, users who don't need AWS integration avoid downloading and compiling these dependencies.

## Status

This provider is a demonstration/stub. Full implementation is pending.

## Creating Your Own External Provider

To create a custom provider for fluxllm:

1. Implement the `provider.Provider` interface from `github.com/grokify/fluxllm/provider`
2. Use `fluxllm.ClientConfig.CustomProvider` to inject your provider

See the source code of this module as a reference implementation.
