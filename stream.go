package bedrock

import (
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/plexusone/omnillm/provider"
)

// eventStream defines the interface for reading Bedrock stream events.
// This allows mocking in tests.
type eventStream interface {
	Events() <-chan types.ConverseStreamOutput
	Close() error
	Err() error
}

// converseStream implements provider.ChatCompletionStream for Bedrock's ConverseStream API.
type converseStream struct {
	output *bedrockruntime.ConverseStreamOutput
	stream eventStream
	model  string
	id     string
	index  int

	// Tool call tracking
	currentToolID   string
	currentToolName string
	toolInputParts  []string
}

// newConverseStream creates a new converseStream wrapper.
func newConverseStream(output *bedrockruntime.ConverseStreamOutput, model string) *converseStream {
	return &converseStream{
		output: output,
		stream: output.GetStream(),
		model:  model,
		id:     generateID(),
		index:  0,
	}
}

// newConverseStreamWithEventStream creates a converseStream with a custom event stream (for testing).
func newConverseStreamWithEventStream(stream eventStream, model, id string) *converseStream {
	return &converseStream{
		stream: stream,
		model:  model,
		id:     id,
	}
}

// Recv receives the next chunk from the stream.
func (s *converseStream) Recv() (*provider.ChatCompletionChunk, error) {
	for {
		event, ok := <-s.stream.Events()
		if !ok {
			// Channel closed - check for errors
			if err := s.stream.Err(); err != nil {
				return nil, err
			}
			return nil, io.EOF
		}

		chunk := s.processEvent(event)
		if chunk != nil {
			return chunk, nil
		}
		// Continue to next event if this one didn't produce a chunk
	}
}

// Close closes the stream.
func (s *converseStream) Close() error {
	return s.stream.Close()
}

// processEvent converts a Bedrock stream event to a ChatCompletionChunk.
// Returns nil if the event doesn't produce a chunk (e.g., message start).
func (s *converseStream) processEvent(event types.ConverseStreamOutput) *provider.ChatCompletionChunk {
	switch e := event.(type) {
	case *types.ConverseStreamOutputMemberContentBlockStart:
		return s.handleContentBlockStart(e.Value)

	case *types.ConverseStreamOutputMemberContentBlockDelta:
		return s.handleContentBlockDelta(e.Value)

	case *types.ConverseStreamOutputMemberContentBlockStop:
		return s.handleContentBlockStop(e.Value)

	case *types.ConverseStreamOutputMemberMessageStop:
		return s.handleMessageStop(e.Value)

	case *types.ConverseStreamOutputMemberMetadata:
		return s.handleMetadata(e.Value)

	default:
		// MessageStart - no chunk needed
		return nil
	}
}

// handleContentBlockStart processes the start of a content block (tool use).
func (s *converseStream) handleContentBlockStart(event types.ContentBlockStartEvent) *provider.ChatCompletionChunk {
	if toolStart, ok := event.Start.(*types.ContentBlockStartMemberToolUse); ok {
		// Start tracking a new tool call
		s.currentToolID = derefString(toolStart.Value.ToolUseId)
		s.currentToolName = derefString(toolStart.Value.Name)
		s.toolInputParts = nil

		// Emit initial tool call chunk
		return &provider.ChatCompletionChunk{
			ID:      s.id,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   s.model,
			Choices: []provider.ChatCompletionChoice{
				{
					Index: 0,
					Delta: &provider.Message{
						Role: provider.RoleAssistant,
						ToolCalls: []provider.ToolCall{
							{
								ID:   s.currentToolID,
								Type: "function",
								Function: provider.ToolFunction{
									Name: s.currentToolName,
								},
							},
						},
					},
				},
			},
		}
	}
	return nil
}

// handleContentBlockDelta processes text and tool use delta events.
func (s *converseStream) handleContentBlockDelta(event types.ContentBlockDeltaEvent) *provider.ChatCompletionChunk {
	switch delta := event.Delta.(type) {
	case *types.ContentBlockDeltaMemberText:
		s.index++
		return &provider.ChatCompletionChunk{
			ID:      s.id,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   s.model,
			Choices: []provider.ChatCompletionChoice{
				{
					Index: 0,
					Delta: &provider.Message{
						Role:    provider.RoleAssistant,
						Content: delta.Value,
					},
				},
			},
		}

	case *types.ContentBlockDeltaMemberToolUse:
		// Accumulate tool input for later
		if delta.Value.Input != nil {
			s.toolInputParts = append(s.toolInputParts, *delta.Value.Input)
		}
		// Emit argument delta chunk
		return &provider.ChatCompletionChunk{
			ID:      s.id,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   s.model,
			Choices: []provider.ChatCompletionChoice{
				{
					Index: 0,
					Delta: &provider.Message{
						Role: provider.RoleAssistant,
						ToolCalls: []provider.ToolCall{
							{
								Function: provider.ToolFunction{
									Arguments: derefString(delta.Value.Input),
								},
							},
						},
					},
				},
			},
		}
	}

	return nil
}

// handleContentBlockStop processes the end of a content block.
func (s *converseStream) handleContentBlockStop(_ types.ContentBlockStopEvent) *provider.ChatCompletionChunk {
	// Reset tool tracking state when content block ends
	if s.currentToolID != "" {
		s.currentToolID = ""
		s.currentToolName = ""
		s.toolInputParts = nil
	}
	return nil
}

// handleMessageStop processes the message stop event with finish reason.
func (s *converseStream) handleMessageStop(event types.MessageStopEvent) *provider.ChatCompletionChunk {
	finishReason := mapStopReason(event.StopReason)

	return &provider.ChatCompletionChunk{
		ID:      s.id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   s.model,
		Choices: []provider.ChatCompletionChoice{
			{
				Index:        0,
				Delta:        &provider.Message{},
				FinishReason: &finishReason,
			},
		},
	}
}

// handleMetadata processes the metadata event with usage information.
func (s *converseStream) handleMetadata(event types.ConverseStreamMetadataEvent) *provider.ChatCompletionChunk {
	if event.Usage == nil {
		return nil
	}

	return &provider.ChatCompletionChunk{
		ID:      s.id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   s.model,
		Choices: []provider.ChatCompletionChoice{},
		Usage: &provider.Usage{
			PromptTokens:     int(derefInt32(event.Usage.InputTokens)),
			CompletionTokens: int(derefInt32(event.Usage.OutputTokens)),
			TotalTokens:      int(derefInt32(event.Usage.TotalTokens)),
		},
	}
}
