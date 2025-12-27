# OmniLLM Provider for AWS Bedrock

[![Build Status][build-status-svg]][build-status-url]
[![Lint Status][lint-status-svg]][lint-status-url]
[![Go Report Card][goreport-svg]][goreport-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![License][license-svg]][license-url]

[OmniLLM](https://github.com/agentplexus/omnillm) Provider for AWS Bedrock is an external provider module that demonstrates how to create custom providers for OmniLLM without adding heavy dependencies to the core library.

## Installation

```bash
go get github.com/agentplexus/omnillm-bedrock
```

## Usage

```go
package main

import (
    "context"
    "log"

    "github.com/agentplexus/omnillm"
    "github.com/agentplexus/omnillm-bedrock"
    "github.com/agentplexus/omnillm/provider"
)

func main() {
    // Create the Bedrock provider
    bedrockProvider, err := bedrock.NewProvider("us-east-1")
    if err != nil {
        log.Fatal(err)
    }

    // Use it with omnillm via CustomProvider
    client, err := omnillm.NewClient(omnillm.ClientConfig{
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

To create a custom provider for omnillm:

1. Implement the `provider.Provider` interface from `github.com/agentplexus/omnillm/provider`
2. Use `omnillm.ClientConfig.CustomProvider` to inject your provider

See the source code of this module as a reference implementation.

 [build-status-svg]: https://github.com/agentplexus/omnillm-bedrock/actions/workflows/ci.yaml/badge.svg?branch=main
 [build-status-url]: https://github.com/agentplexus/omnillm-bedrock/actions/workflows/ci.yaml
 [lint-status-svg]: https://github.com/agentplexus/omnillm-bedrock/actions/workflows/lint.yaml/badge.svg?branch=main
 [lint-status-url]: https://github.com/agentplexus/omnillm-bedrock/actions/workflows/lint.yaml
 [goreport-svg]: https://goreportcard.com/badge/github.com/agentplexus/omnillm-bedrock
 [goreport-url]: https://goreportcard.com/report/github.com/agentplexus/omnillm-bedrock
 [docs-godoc-svg]: https://pkg.go.dev/badge/github.com/agentplexus/omnillm-bedrock
 [docs-godoc-url]: https://pkg.go.dev/github.com/agentplexus/omnillm-bedrock
 [license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
 [license-url]: https://github.com/agentplexus/omnillm-bedrock/blob/master/LICENSE
 [used-by-svg]: https://sourcegraph.com/github.com/agentplexus/omnillm-bedrock/-/badge.svg
 [used-by-url]: https://sourcegraph.com/github.com/agentplexus/omnillm-bedrock?badge
