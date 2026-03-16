package bedrock

import (
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/plexusone/omnillm/provider"
)

// mockEventStream simulates a Bedrock event stream for testing.
// It implements the eventStream interface.
type mockEventStream struct {
	events chan types.ConverseStreamOutput
	err    error
}

func newMockEventStream(events []types.ConverseStreamOutput) *mockEventStream {
	ch := make(chan types.ConverseStreamOutput, len(events))
	for _, e := range events {
		ch <- e
	}
	close(ch)
	return &mockEventStream{events: ch}
}

func (m *mockEventStream) Events() <-chan types.ConverseStreamOutput {
	return m.events
}

func (m *mockEventStream) Close() error {
	return nil
}

func (m *mockEventStream) Err() error {
	return m.err
}

func TestConverseStream_TextDelta(t *testing.T) {
	// Create mock events
	events := []types.ConverseStreamOutput{
		&types.ConverseStreamOutputMemberContentBlockDelta{
			Value: types.ContentBlockDeltaEvent{
				ContentBlockIndex: ptr(int32(0)),
				Delta:             &types.ContentBlockDeltaMemberText{Value: "Hello"},
			},
		},
		&types.ConverseStreamOutputMemberContentBlockDelta{
			Value: types.ContentBlockDeltaEvent{
				ContentBlockIndex: ptr(int32(0)),
				Delta:             &types.ContentBlockDeltaMemberText{Value: " world!"},
			},
		},
		&types.ConverseStreamOutputMemberMessageStop{
			Value: types.MessageStopEvent{
				StopReason: types.StopReasonEndTurn,
			},
		},
	}

	mockStream := newMockEventStream(events)

	// Create stream wrapper using the test constructor
	stream := newConverseStreamWithEventStream(mockStream, "test-model", "test-id")

	// Read chunks
	var chunks []*provider.ChatCompletionChunk
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv() error = %v", err)
		}
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 3 {
		t.Fatalf("Got %d chunks, want 3", len(chunks))
	}

	// First chunk: "Hello"
	if chunks[0].Choices[0].Delta.Content != "Hello" {
		t.Errorf("Chunk[0] content = %q, want Hello", chunks[0].Choices[0].Delta.Content)
	}

	// Second chunk: " world!"
	if chunks[1].Choices[0].Delta.Content != " world!" {
		t.Errorf("Chunk[1] content = %q, want ' world!'", chunks[1].Choices[0].Delta.Content)
	}

	// Third chunk: finish reason
	if chunks[2].Choices[0].FinishReason == nil || *chunks[2].Choices[0].FinishReason != "stop" {
		t.Errorf("Chunk[2] finish reason = %v, want 'stop'", chunks[2].Choices[0].FinishReason)
	}
}

func TestConverseStream_ToolUse(t *testing.T) {
	toolID := "tool_123"
	toolName := "get_weather"

	events := []types.ConverseStreamOutput{
		// Tool use start
		&types.ConverseStreamOutputMemberContentBlockStart{
			Value: types.ContentBlockStartEvent{
				ContentBlockIndex: ptr(int32(0)),
				Start: &types.ContentBlockStartMemberToolUse{
					Value: types.ToolUseBlockStart{
						ToolUseId: &toolID,
						Name:      &toolName,
					},
				},
			},
		},
		// Tool use delta (arguments)
		&types.ConverseStreamOutputMemberContentBlockDelta{
			Value: types.ContentBlockDeltaEvent{
				ContentBlockIndex: ptr(int32(0)),
				Delta: &types.ContentBlockDeltaMemberToolUse{
					Value: types.ToolUseBlockDelta{
						Input: ptr(`{"location":"Seattle"}`),
					},
				},
			},
		},
		// Content block stop
		&types.ConverseStreamOutputMemberContentBlockStop{
			Value: types.ContentBlockStopEvent{
				ContentBlockIndex: ptr(int32(0)),
			},
		},
		// Message stop
		&types.ConverseStreamOutputMemberMessageStop{
			Value: types.MessageStopEvent{
				StopReason: types.StopReasonToolUse,
			},
		},
	}

	mockStream := newMockEventStream(events)
	stream := newConverseStreamWithEventStream(mockStream, "test-model", "test-id")

	var chunks []*provider.ChatCompletionChunk
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv() error = %v", err)
		}
		chunks = append(chunks, chunk)
	}

	// Should have: tool start, tool delta, message stop (content block stop returns nil)
	if len(chunks) != 3 {
		t.Fatalf("Got %d chunks, want 3", len(chunks))
	}

	// First chunk: tool call start with ID and name
	if len(chunks[0].Choices[0].Delta.ToolCalls) != 1 {
		t.Fatalf("Chunk[0] tool calls = %d, want 1", len(chunks[0].Choices[0].Delta.ToolCalls))
	}
	tc := chunks[0].Choices[0].Delta.ToolCalls[0]
	if tc.ID != toolID {
		t.Errorf("ToolCall ID = %q, want %q", tc.ID, toolID)
	}
	if tc.Function.Name != toolName {
		t.Errorf("ToolCall Name = %q, want %q", tc.Function.Name, toolName)
	}

	// Last chunk: finish reason
	if chunks[2].Choices[0].FinishReason == nil || *chunks[2].Choices[0].FinishReason != "tool_calls" {
		t.Errorf("Final chunk finish reason = %v, want tool_calls", chunks[2].Choices[0].FinishReason)
	}
}

func TestConverseStream_Metadata(t *testing.T) {
	inputTokens := int32(10)
	outputTokens := int32(20)
	totalTokens := int32(30)

	events := []types.ConverseStreamOutput{
		&types.ConverseStreamOutputMemberContentBlockDelta{
			Value: types.ContentBlockDeltaEvent{
				ContentBlockIndex: ptr(int32(0)),
				Delta:             &types.ContentBlockDeltaMemberText{Value: "Hi"},
			},
		},
		&types.ConverseStreamOutputMemberMessageStop{
			Value: types.MessageStopEvent{
				StopReason: types.StopReasonEndTurn,
			},
		},
		&types.ConverseStreamOutputMemberMetadata{
			Value: types.ConverseStreamMetadataEvent{
				Usage: &types.TokenUsage{
					InputTokens:  &inputTokens,
					OutputTokens: &outputTokens,
					TotalTokens:  &totalTokens,
				},
			},
		},
	}

	mockStream := newMockEventStream(events)
	stream := newConverseStreamWithEventStream(mockStream, "test-model", "test-id")

	var chunks []*provider.ChatCompletionChunk
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv() error = %v", err)
		}
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 3 {
		t.Fatalf("Got %d chunks, want 3", len(chunks))
	}

	// Last chunk should have usage
	if chunks[2].Usage == nil {
		t.Fatal("Usage is nil")
	}
	if chunks[2].Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", chunks[2].Usage.PromptTokens)
	}
	if chunks[2].Usage.CompletionTokens != 20 {
		t.Errorf("CompletionTokens = %d, want 20", chunks[2].Usage.CompletionTokens)
	}
}
