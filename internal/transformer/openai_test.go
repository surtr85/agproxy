package transformer

import (
	"strings"
	"testing"
)

func TestOpenAIToAntigravity_Basic(t *testing.T) {
	req := &OpenAIChatRequest{
		Model: "gemini-3.8-flash",
		Messages: []OpenAIMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello!"},
		},
	}

	res, err := OpenAIToAntigravity(req, "test-project-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Payload.Project != "test-project-123" {
		t.Errorf("expected project test-project-123, got %s", res.Payload.Project)
	}

	if res.Payload.Model != "gemini-3.8-flash-medium" {
		t.Errorf("expected model gemini-3.8-flash-medium, got %s", res.Payload.Model)
	}

	if len(res.Payload.Request.Contents) != 1 {
		t.Fatalf("expected 1 user content, got %d", len(res.Payload.Request.Contents))
	}

	if res.Payload.Request.Contents[0].Parts[0].Text != "Hello!" {
		t.Errorf("expected text Hello!, got %s", res.Payload.Request.Contents[0].Parts[0].Text)
	}
}

func TestOpenAIToAntigravity_ToolCloaking(t *testing.T) {
	req := &OpenAIChatRequest{
		Model: "gemini-3.8-flash",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "Calculate 2+2"},
		},
		Tools: []OpenAITool{
			{
				Type: "function",
				Function: struct {
					Name        string         `json:"name"`
					Description string         `json:"description,omitempty"`
					Parameters  map[string]any `json:"parameters,omitempty"`
				}{
					Name:        "custom_calculator",
					Description: "Performs math",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"expression": map[string]any{"type": "string"},
						},
						"$schema":              "http://json-schema.org/draft-07/schema#",
						"additionalProperties": false,
					},
				},
			},
		},
	}

	res, err := OpenAIToAntigravity(req, "test-project-tools")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Tool should have _ide suffix
	expectedSuffixed := "custom_calculator_ide"
	if res.ToolNameMap[expectedSuffixed] != "custom_calculator" {
		t.Errorf("expected tool map to map %s to custom_calculator, got %v", expectedSuffixed, res.ToolNameMap)
	}

	// Tools declarations should contain the suffixed tool and decoy tools
	decls := res.Payload.Request.Tools[0].FunctionDeclarations
	foundCustom := false
	foundDecoy := false
	for _, d := range decls {
		if d.Name == expectedSuffixed {
			foundCustom = true
			// Check cleaned schema
			if _, hasSchema := d.Parameters["$schema"]; hasSchema {
				t.Errorf("schema cleaner failed to strip $schema")
			}
			if _, hasAddProps := d.Parameters["additionalProperties"]; hasAddProps {
				t.Errorf("schema cleaner failed to strip additionalProperties")
			}
		}
		if d.Name == "run_command" || d.Name == "view_file" {
			foundDecoy = true
		}
	}

	if !foundCustom {
		t.Errorf("did not find suffixed custom tool in declarations")
	}
	if !foundDecoy {
		t.Errorf("did not find Antigravity decoy tools in declarations")
	}
}

func TestOpenAIToAntigravity_SystemInstructionSanitization(t *testing.T) {
	rawPrompt := "You are an expert coding assistant operating inside pi, a coding agent harness.\n\nYou are a coding agent powered by the laya model.\n\nYour working directory is /home/amadeus/Projects."

	rawUserPrompt := "Check files in /home/amadeus/Projects/nix-config/modules/home/ai/"

	req := &OpenAIChatRequest{
		Model: "gemini-3.8-flash",
		Messages: []OpenAIMessage{
			{Role: "system", Content: rawPrompt},
			{Role: "user", Content: rawUserPrompt},
		},
		Tools: []OpenAITool{
			{
				Type: "function",
				Function: struct {
					Name        string         `json:"name"`
					Description string         `json:"description,omitempty"`
					Parameters  map[string]any `json:"parameters,omitempty"`
				}{
					Name:        "bash_ide",
					Description: "Execute bash commands",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"command": map[string]any{
								"type":        "string",
								"description": "Command to run",
							},
						},
					},
				},
			},
		},
	}

	res, err := OpenAIToAntigravity(req, "aicode-consumers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sysText := res.Payload.Request.SystemInstruction.Parts[0].Text
	if !contains(sysText, "Antigravity") {
		t.Errorf("Expected Antigravity persona in system prompt: %s", sysText)
	}

	userText := res.Payload.Request.Contents[0].Parts[0].Text
	if userText != rawUserPrompt {
		t.Errorf("User prompt was modified! expected %q, got %q", rawUserPrompt, userText)
	}

	// Tool name check - must NOT be bash_ide_ide
	toolDecl := res.Payload.Request.Tools[0].FunctionDeclarations[0]
	if toolDecl.Name != "bash_ide" {
		t.Errorf("Expected tool name bash_ide, got %q", toolDecl.Name)
	}
}

func TestOpenAIToAntigravity_EmptyPartFiltering(t *testing.T) {
	// Simulate user message that gets completely stripped, followed by a valid user message
	req := &OpenAIChatRequest{
		Model: "gemini-3.8-flash",
		Messages: []OpenAIMessage{
			{Role: "user", Content: ""},
			{Role: "assistant", Content: ""},
			{Role: "user", Content: "Hello world!"},
		},
	}

	res, err := OpenAIToAntigravity(req, "aicode-consumers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// There should only be 1 content, the non-empty "Hello world!"
	if len(res.Payload.Request.Contents) != 1 {
		t.Fatalf("expected 1 content after dropping empty messages, got %d", len(res.Payload.Request.Contents))
	}

	for i, c := range res.Payload.Request.Contents {
		for j, p := range c.Parts {
			if p.Text == "" && p.InlineData == nil && p.FunctionCall == nil && p.FunctionResponse == nil {
				t.Fatalf("content[%d].parts[%d] has no initialized data field!", i, j)
			}
		}
	}
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func TestOpenAIToAntigravity_ToolResponseResolution(t *testing.T) {
	req := &OpenAIChatRequest{
		Model: "gemini-3.8-flash",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "Run search"},
			{
				Role: "assistant",
				ToolCalls: []OpenAIToolCall{
					{
						ID:   "call_12345",
						Type: "function",
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{
							Name:      "custom_search",
							Arguments: `{"q":"test"}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				ToolCallID: "call_12345",
				// Name is empty, like in pi client!
				Content: `{"status":"ok"}`,
			},
		},
	}

	res, err := OpenAIToAntigravity(req, "test-proj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Payload.Request.Contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(res.Payload.Request.Contents))
	}

	toolRespPart := res.Payload.Request.Contents[2].Parts[0].FunctionResponse
	if toolRespPart == nil {
		t.Fatalf("expected function response part")
	}

	if toolRespPart.Name != "custom_search_ide" {
		t.Errorf("expected tool response name custom_search_ide, got %q", toolRespPart.Name)
	}
}

func TestOpenAIToAntigravity_LayaRouting(t *testing.T) {
	req := &OpenAIChatRequest{
		Model: "laya",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "ping"},
		},
	}

	res, err := OpenAIToAntigravity(req, "test-proj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Payload.Model != "gemini-3.8-flash-low" {
		t.Errorf("expected model gemini-3.8-flash-low for ping, got %s", res.Payload.Model)
	}

	// Test prefixed model name
	reqPrefixed := &OpenAIChatRequest{
		Model: "agproxy/laya",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "ping"},
		},
	}

	resPrefixed, err := OpenAIToAntigravity(reqPrefixed, "test-proj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resPrefixed.Payload.Model != "gemini-3.8-flash-low" {
		t.Errorf("expected model gemini-3.8-flash-low for agproxy/laya, got %s", resPrefixed.Payload.Model)
	}
}
