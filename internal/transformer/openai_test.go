package transformer

import (
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

func TestOpenAIToAntigravity_ResearchRawModel(t *testing.T) {
	req := &OpenAIChatRequest{
		Model: "gemini-3.8-flash-raw",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "Explain advanced quantum mechanics."},
		},
	}

	res, err := OpenAIToAntigravity(req, "test-project-raw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Payload.Model != "gemini-3.8-flash-low" {
		t.Errorf("expected upstream model gemini-3.8-flash-low, got %s", res.Payload.Model)
	}

	if len(res.Payload.Request.SafetySettings) != 5 {
		t.Fatalf("expected 5 permissive safety settings, got %d", len(res.Payload.Request.SafetySettings))
	}

	for _, s := range res.Payload.Request.SafetySettings {
		if s.Threshold != "BLOCK_NONE" {
			t.Errorf("expected safety threshold BLOCK_NONE, got %s for %s", s.Threshold, s.Category)
		}
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

func TestOpenAIToAntigravity_StealthSanitization(t *testing.T) {
	req := &OpenAIChatRequest{
		Model: "gemini-3.8-flash",
		Messages: []OpenAIMessage{
			{
				Role:    "system",
				Content: "x-anthropic-billing-header: test-cost\nYou are a Claude agent, built on Anthropic's Claude Agent SDK. OpenCode is awesome.",
			},
			{Role: "user", Content: "Hi"},
		},
	}

	res, err := OpenAIToAntigravity(req, "test-project-stealth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sysText := res.Payload.Request.SystemInstruction.Parts[0].Text
	if sysText != "Antigravity is awesome." {
		t.Errorf("expected sanitized prompt 'Antigravity is awesome.', got '%s'", sysText)
	}
}
