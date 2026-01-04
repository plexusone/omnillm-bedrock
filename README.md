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

## AWS Authentication

Credentials are resolved in priority order:

| Priority | Source | Description | Use Case |
|----------|--------|-------------|----------|
| 1 | **Bedrock API Key** | `AWS_BEARER_TOKEN_BEDROCK` | Simplest setup, dev/prototyping |
| 2 | Environment variables | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` | CI/CD pipelines, containers |
| 3 | Shared credentials file | `~/.aws/credentials` with optional `AWS_PROFILE` | Local development |
| 4 | Shared config file | `~/.aws/config` (profiles, SSO, assume role) | Local development with SSO |
| 5 | EC2 instance metadata | IMDS (Instance Metadata Service) | EC2 instances |
| 6 | ECS container credentials | Task IAM role via `AWS_CONTAINER_CREDENTIALS_RELATIVE_URI` | ECS/Fargate tasks |
| 7 | EKS Pod Identity / IRSA | Web identity token via `AWS_WEB_IDENTITY_TOKEN_FILE` | Kubernetes workloads |

### Bedrock API Key (Recommended for Development)

The simplest authentication method. Generate a key in the [AWS Console](https://console.aws.amazon.com/bedrock/) under **API keys**:

```bash
export AWS_BEARER_TOKEN_BEDROCK="your-bedrock-api-key"
```

> **Note:** For production, use IAM credentials or instance roles for better security and auditability.

### IAM Credentials

Set environment variables:

```bash
export AWS_ACCESS_KEY_ID=AKIA...
export AWS_SECRET_ACCESS_KEY=...
```

Or use AWS profiles:

```bash
export AWS_PROFILE=my-profile
```

### Production (EKS with IRSA)

For Kubernetes workloads, use [IRSA](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html):

1. Create an IAM role with the required policy (see below)
2. Associate the role with a Kubernetes service account
3. The SDK automatically uses the web identity token

### Required IAM Permissions

The IAM user or role must have permissions to invoke Bedrock models:

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": [
      "bedrock:InvokeModel",
      "bedrock:InvokeModelWithResponseStream"
    ],
    "Resource": "arn:aws:bedrock:*::foundation-model/*"
  }]
}
```

### Bedrock Model Access

Serverless foundation models are automatically enabled on first invocation in AWS commercial regions. No manual setup required.

> **Note:** Some models (e.g., certain Anthropic models) may require accepting an EULA in the AWS Console under **Bedrock → Model access** before first use.

## Testing

### Unit Tests

```bash
go test -v ./...
```

### Integration Tests

Integration tests make real API calls to AWS Bedrock. Configure credentials and run with the `integration` build tag:

```bash
go test -tags=integration -v ./...
```

#### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `AWS_BEARER_TOKEN_BEDROCK` | Option 1 | - | Bedrock API key (simplest) |
| `AWS_ACCESS_KEY_ID` | Option 2 | - | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | Option 2 | - | AWS secret key |
| `AWS_SESSION_TOKEN` | No | - | Session token (for temporary credentials) |
| `BEDROCK_TEST_REGION` | No | `us-east-1` | AWS region for tests |
| `BEDROCK_TEST_MODEL` | No | `anthropic.claude-3-haiku-20240307-v1:0` | Model ID for tests |

Use **either** `AWS_BEARER_TOKEN_BEDROCK` (Option 1) **or** IAM credentials (Option 2).

#### CI/CD Example (GitHub Actions)

Using Bedrock API key (simpler):

```yaml
- name: Run integration tests
  env:
    AWS_BEARER_TOKEN_BEDROCK: ${{ secrets.AWS_BEARER_TOKEN_BEDROCK }}
    BEDROCK_TEST_REGION: us-east-1
  run: go test -tags=integration -v ./...
```

Using IAM credentials:

```yaml
- name: Run integration tests
  env:
    AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
    AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
    BEDROCK_TEST_REGION: us-east-1
  run: go test -tags=integration -v ./...
```

## Why a Separate Module?

The AWS SDK v2 pulls in 17+ transitive dependencies (credentials, STS, SSO, EC2 IMDS, etc.). By keeping Bedrock in a separate module, users who don't need AWS integration avoid downloading and compiling these dependencies.

## Features

- Chat completions (non-streaming and streaming)
- Tool/function calling support
- System prompts and multi-turn conversations
- Works with all Bedrock models via the Converse API

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
