// Package bedrock provides AWS Bedrock API client implementation.
// This is an external provider for github.com/plexusone/omnillm.
package bedrock

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/smithy-go/auth/bearer"
)

// BedrockClient defines the interface for Bedrock API operations.
// This interface is implemented by Client and can be mocked for testing.
type BedrockClient interface {
	Name() string
	Converse(ctx context.Context, input *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error)
	ConverseStream(ctx context.Context, input *bedrockruntime.ConverseStreamInput) (*bedrockruntime.ConverseStreamOutput, error)
	Close() error
}

// Client implements BedrockClient using the AWS SDK.
type Client struct {
	client *bedrockruntime.Client
	region string
}

// Verify Client implements BedrockClient.
var _ BedrockClient = (*Client)(nil)

// BearerTokenEnvVar is the environment variable for Bedrock API key authentication.
const BearerTokenEnvVar = "AWS_BEARER_TOKEN_BEDROCK" //nolint:gosec // env var name, not a credential

// New creates a new Bedrock client.
// Authentication is resolved in this order:
//  1. Bearer token from AWS_BEARER_TOKEN_BEDROCK environment variable
//  2. Standard AWS credential chain (env vars, shared credentials, instance metadata, etc.)
func New(region string) (*Client, error) {
	if region == "" {
		region = "us-east-1"
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Check for bearer token (Bedrock API key)
	var clientOpts []func(*bedrockruntime.Options)
	if token := os.Getenv(BearerTokenEnvVar); token != "" {
		clientOpts = append(clientOpts, func(o *bedrockruntime.Options) {
			o.BearerAuthTokenProvider = bearer.StaticTokenProvider{
				Token: bearer.Token{Value: token},
			}
		})
	}

	return &Client{
		client: bedrockruntime.NewFromConfig(cfg, clientOpts...),
		region: region,
	}, nil
}

// Name returns the provider name
func (c *Client) Name() string {
	return "bedrock"
}

// Converse sends a conversation request to Bedrock using the Converse API.
func (c *Client) Converse(ctx context.Context, input *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
	return c.client.Converse(ctx, input)
}

// ConverseStream sends a streaming conversation request to Bedrock.
func (c *Client) ConverseStream(ctx context.Context, input *bedrockruntime.ConverseStreamInput) (*bedrockruntime.ConverseStreamOutput, error) {
	return c.client.ConverseStream(ctx, input)
}

// Close closes the client
func (c *Client) Close() error {
	return nil
}
