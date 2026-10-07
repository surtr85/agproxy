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

func TestOpenAIToAntigravity_DeepSeekHarnessSanitization(t *testing.T) {
	rawDSHPrompt := `You are an AI agent powered by DeepSeek Harness.

You are a coding agent powered by the gemini-3.7-flash-medium model.

The DeepSeek Harness implementation checkout is at /home/amadeus/.local/share/dsh/. The checkout location and current working directory are separate values and may differ; never infer the working directory from this path. Use pwd to determine the current working directory. Use this checkout only to inspect or extend DSH itself.

You are interacting with the user through the DeepSeek Harness Web GUI at http://127.0.0.1:3080. When the user refers to "this page", "this GUI", or "this app" without naming another target, they mean this GUI. The browser provides no implicit DOM, route, or screenshot context. The client-plugin HMR receiver is active, but client-plugin changes reload without a refresh only while pnpm run dev:web is also running from this same checkout to rebuild their bundles; verify that watcher before promising automatic updates. Every other change — the apps/web shell and plain packages — requires rebuilding the affected Web artifacts and verifying this existing URL after a page refresh. Starting another server does not update this GUI. The apps/web Vite entry builds the shell but is not a standalone application because only dsh web injects window.__DSH_BOOT__. Do not start a replacement server unless the user asks; if one is needed, use a managed background job and verify its exact URL.

Your working directory is /home/amadeus/Projects.`

	rawUserPrompt := `pingCurrent runtime context. This snapshot supersedes earlier runtime-context snapshots.

Current DSH file policy: workspace-write. Any available operation enforced by the DSH file sandbox may modify files under the session workspace: "/home/amadeus/Projects". Some platform temporary areas may also be writable.

Approval policy: ask. Operations that require approval may ask through the configured answerers; without an available answerer, the request fails closed.[model changed: assistant turns above this point were generated by laya; the session continues with gemini-3.7-flash-medium]`

	req := &OpenAIChatRequest{
		Model: "gemini-3.8-flash",
		Messages: []OpenAIMessage{
			{Role: "system", Content: rawDSHPrompt},
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
					Description: "Managed $DSH_* variables expose current harness environment facts.",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"command": map[string]any{
								"type":        "string",
								"description": "Run command with DSH safety sandbox",
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
	if contains(sysText, "DeepSeek Harness") || contains(sysText, "dsh") || contains(sysText, "127.0.0.1:3080") {
		t.Errorf("System prompt still contains DeepSeek Harness leaks: %s", sysText)
	}
	if !contains(sysText, "Antigravity") {
		t.Errorf("Expected Antigravity persona in system prompt: %s", sysText)
	}

	userText := res.Payload.Request.Contents[0].Parts[0].Text
	if contains(userText, "Current DSH file policy") || contains(userText, "model changed") {
		t.Errorf("User prompt still contains DSH runtime leak: %s", userText)
	}

	// Tool name check - must NOT be bash_ide_ide
	toolDecl := res.Payload.Request.Tools[0].FunctionDeclarations[0]
	if toolDecl.Name != "bash_ide" {
		t.Errorf("Expected tool name 'bash_ide', got %q", toolDecl.Name)
	}
	if contains(toolDecl.Description, "$DSH_") {
		t.Errorf("Tool description still contains $DSH_: %s", toolDecl.Description)
	}

	// Check tool parameter description sanitization
	cmdProp := toolDecl.Parameters["properties"].(map[string]any)["command"].(map[string]any)
	if contains(cmdProp["description"].(string), "DSH") {
		t.Errorf("Tool property description still contains DSH: %s", cmdProp["description"])
	}
}

func TestOpenAIToAntigravity_EmptyPartFiltering(t *testing.T) {
	// Simulate user message that gets completely stripped, followed by a valid user message
	req := &OpenAIChatRequest{
		Model: "gemini-3.8-flash",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "[model changed: assistant turns above this point were generated by laya; the session continues with gemini-3.7-flash-medium]"},
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
		t.Errorf("expected tool response name 'custom_search_ide', got %q", toolRespPart.Name)
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
		t.Errorf("expected model gemini-3.8-flash-low for 'ping', got %s", res.Payload.Model)
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
