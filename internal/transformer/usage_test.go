package transformer

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSSEChunkUsageConversion(t *testing.T) {
	state := NewSSEState("gemini-3.8-flash", nil)

	googleChunk := `{
		"response": {
			"candidates": [
				{
					"content": {
						"parts": [
							{"text": "Hello world"}
						]
					},
					"finishReason": "STOP"
				}
			],
			"usageMetadata": {
				"promptTokenCount": 12,
				"candidatesTokenCount": 5,
				"totalTokenCount": 17
			}
		}
	}`

	data, isDone, err := state.ConvertChunk([]byte(googleChunk))
	if err != nil {
		t.Fatalf("ConvertChunk failed: %v", err)
	}
	if !isDone {
		t.Fatalf("expected isDone to be true")
	}

	raw := strings.TrimPrefix(string(data), "data: ")
	var chunk OpenAISSEChunk
	if err := json.Unmarshal([]byte(raw), &chunk); err != nil {
		t.Fatalf("failed to unmarshal SSE chunk: %v", err)
	}

	if chunk.Usage == nil {
		t.Fatalf("expected usage to be non-nil")
	}
	if chunk.Usage.TotalTokens != 17 {
		t.Errorf("expected 17 total tokens, got %d", chunk.Usage.TotalTokens)
	}
}
