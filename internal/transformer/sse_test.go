package transformer

import (
	"strings"
	"testing"
)

func TestSSEState_ConvertChunk_Text(t *testing.T) {
	state := NewSSEState("gemini-3.8-flash", nil)

	googleChunk := []byte(`{
		"response": {
			"candidates": [
				{
					"content": {
						"role": "model",
						"parts": [
							{"text": "Hello world!"}
						]
					},
					"finishReason": "STOP"
				}
			],
			"modelVersion": "3.8",
			"responseId": "resp-123"
		}
	}`)

	out, isDone, err := state.ConvertChunk(googleChunk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !isDone {
		t.Errorf("expected isDone to be true on STOP")
	}

	outStr := string(out)
	if !strings.Contains(outStr, `"content":"Hello world!"`) {
		t.Errorf("output does not contain content: %s", outStr)
	}
	if !strings.Contains(outStr, `"finish_reason":"stop"`) {
		t.Errorf("output does not contain finish_reason stop: %s", outStr)
	}
}

func TestSSEState_ConvertChunk_ThinkingAndTools(t *testing.T) {
	toolMap := map[string]string{
		"run_bash_ide": "run_bash",
	}
	state := NewSSEState("claude-sonnet-4-6", toolMap)

	googleChunk := []byte(`{
		"response": {
			"candidates": [
				{
					"content": {
						"role": "model",
						"parts": [
							{"thought": true, "text": "Thinking about the command..."},
							{
								"functionCall": {
									"name": "run_bash_ide",
									"args": {"cmd": "ls -l"}
								},
								"thoughtSignature": "valid-sig-123"
							}
						]
					},
					"finishReason": "STOP"
				}
			]
		}
	}`)

	out, isDone, err := state.ConvertChunk(googleChunk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !isDone {
		t.Errorf("expected isDone true")
	}

	outStr := string(out)
	if !strings.Contains(outStr, `"reasoning_content":"Thinking about the command..."`) {
		t.Errorf("expected reasoning content, got: %s", outStr)
	}
	// Tool name should be restored to original "run_bash"
	if !strings.Contains(outStr, `"name":"run_bash"`) {
		t.Errorf("expected original un-cloaked tool name 'run_bash', got: %s", outStr)
	}
}
