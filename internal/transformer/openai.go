package transformer

import (
	"encoding/json"
	"strings"

	"github.com/surtr85/agproxy/internal/stealth"
	"github.com/surtr85/agproxy/internal/upstream"
)

// OpenAI Chat Completion Data Models
type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"` // string or []any for multimodal
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	Name       string           `json:"name,omitempty"`
}

type OpenAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type OpenAITool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description,omitempty"`
		Parameters  map[string]any `json:"parameters,omitempty"`
	} `json:"function"`
}

type OpenAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Tools       []OpenAITool    `json:"tools,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

// TransformResult holds the Antigravity payload and the tool name translation map
type TransformResult struct {
	Payload     *upstream.AntigravityRequestWrapper
	ToolNameMap map[string]string // suffixed -> original
}

func OpenAIToAntigravity(req *OpenAIChatRequest, projectID string) (*TransformResult, error) {
	modelInfo := upstream.ResolveModel(req.Model)

	firstPrompt := ""
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			firstPrompt = extractText(msg.Content)
			break
		}
	}
	sessionID := stealth.DeriveSessionID(firstPrompt)
	toolNameMap := make(map[string]string)

	var contents []upstream.AntigravityContent
	var systemParts []upstream.AntigravityPart

	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			txt := extractText(msg.Content)
			sanitized := stealth.SanitizePromptText(txt)
			if sanitized != "" {
				systemParts = append(systemParts, upstream.AntigravityPart{Text: sanitized})
			}

		case "user":
			parts := extractMessageParts(msg.Content)
			if len(parts) > 0 {
				contents = append(contents, upstream.AntigravityContent{
					Role:  "user",
					Parts: parts,
				})
			}

		case "assistant":
			var parts []upstream.AntigravityPart
			txt := extractText(msg.Content)
			if txt != "" {
				parts = append(parts, upstream.AntigravityPart{Text: txt})
			}

			// Add tool calls
			firstCall := true
			for _, tc := range msg.ToolCalls {
				var args map[string]any
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
				if args == nil {
					args = make(map[string]any)
				}

				name := tc.Function.Name
				suffixedName := name
				if !stealth.NativeToolsSet[name] {
					suffixedName = name + stealth.ToolSuffix
					toolNameMap[suffixedName] = name
				}

				sig := ""
				if firstCall {
					sig = stealth.GetThoughtSignature(tc.ID)
					firstCall = false
				}

				parts = append(parts, upstream.AntigravityPart{
					FunctionCall: &upstream.AntigravityFnCall{
						Name: suffixedName,
						Args: args,
					},
					ThoughtSignature: sig,
				})
			}

			if len(parts) > 0 {
				contents = append(contents, upstream.AntigravityContent{
					Role:  "model",
					Parts: parts,
				})
			}

		case "tool":
			// Tool response
			name := msg.Name
			suffixedName := name
			if !stealth.NativeToolsSet[name] && name != "" {
				suffixedName = name + stealth.ToolSuffix
				toolNameMap[suffixedName] = name
			}

			txt := extractText(msg.Content)
			var respMap map[string]any
			if err := json.Unmarshal([]byte(txt), &respMap); err != nil {
				respMap = map[string]any{"result": txt}
			}

			contents = append(contents, upstream.AntigravityContent{
				Role: "user",
				Parts: []upstream.AntigravityPart{
					{
						FunctionResponse: &upstream.AntigravityFnResp{
							Name:     suffixedName,
							Response: respMap,
						},
					},
				},
			})
		}
	}

	// Cloak tools and inject decoys
	var functionDecls []upstream.AntigravityFunctionDecl
	seenDecls := make(map[string]bool)

	for _, tool := range req.Tools {
		if tool.Type == "function" {
			name := tool.Function.Name
			suffixedName := name
			if !stealth.NativeToolsSet[name] {
				suffixedName = name + stealth.ToolSuffix
				toolNameMap[suffixedName] = name
			}

			if !seenDecls[suffixedName] {
				seenDecls[suffixedName] = true
				functionDecls = append(functionDecls, upstream.AntigravityFunctionDecl{
					Name:        suffixedName,
					Description: tool.Function.Description,
					Parameters:  stealth.CleanJSONSchema(tool.Function.Parameters),
				})
			}
		}
	}

	// Add Antigravity native decoy tools if custom tools are defined
	if len(functionDecls) > 0 {
		for _, decoy := range stealth.NativeDecoyTools {
			if !seenDecls[decoy] {
				seenDecls[decoy] = true
				functionDecls = append(functionDecls, upstream.AntigravityFunctionDecl{
					Name:        decoy,
					Description: "Antigravity IDE internal system tool.",
					Parameters:  stealth.CleanJSONSchema(nil),
				})
			}
		}
	}

	genConfig := map[string]any{
		"maxOutputTokens": 64000,
	}
	if req.Temperature != nil {
		genConfig["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		genConfig["topP"] = *req.TopP
	}
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		if *req.MaxTokens > 64000 {
			genConfig["maxOutputTokens"] = 64000
		} else {
			genConfig["maxOutputTokens"] = *req.MaxTokens
		}
	}

	agReq := upstream.AntigravityRequest{
		Contents:         contents,
		GenerationConfig: genConfig,
		SessionID:        sessionID,
	}

	if len(functionDecls) > 0 {
		agReq.Tools = []upstream.AntigravityTool{
			{FunctionDeclarations: functionDecls},
		}
		agReq.ToolConfig = map[string]any{
			"functionCallingConfig": map[string]any{
				"mode": "VALIDATED",
			},
		}
	}

	if len(systemParts) > 0 {
		agReq.SystemInstruction = &upstream.AntigravityContent{
			Parts: systemParts,
		}
	}

	step := len(contents)
	requestID := stealth.BuildIDERequestID(sessionID, modelInfo.UpstreamModel, step)

	// Note: We intentionally omit "requestType": "agent" on chat path to prevent Google 429
	wrapper := &upstream.AntigravityRequestWrapper{
		Project:   projectID,
		Model:     modelInfo.UpstreamModel,
		UserAgent: "antigravity",
		RequestID: requestID,
		Request:   agReq,
	}

	return &TransformResult{
		Payload:     wrapper,
		ToolNameMap: toolNameMap,
	}, nil
}

func extractText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var sb strings.Builder
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					sb.WriteString(t)
				}
			}
		}
		return sb.String()
	default:
		return ""
	}
}

func extractMessageParts(content any) []upstream.AntigravityPart {
	switch v := content.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []upstream.AntigravityPart{{Text: v}}
	case []any:
		var parts []upstream.AntigravityPart
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			partType, _ := m["type"].(string)
			switch partType {
			case "text":
				if txt, ok := m["text"].(string); ok && txt != "" {
					parts = append(parts, upstream.AntigravityPart{Text: txt})
				}
			case "image_url":
				if imgMap, ok := m["image_url"].(map[string]any); ok {
					if urlStr, ok := imgMap["url"].(string); ok {
						if strings.HasPrefix(urlStr, "data:") {
							// data:image/png;base64,...
							partsSplit := strings.SplitN(urlStr, ";base64,", 2)
							if len(partsSplit) == 2 {
								mimeType := strings.TrimPrefix(partsSplit[0], "data:")
								parts = append(parts, upstream.AntigravityPart{
									InlineData: &upstream.AntigravityBlob{
										MimeType: mimeType,
										Data:     partsSplit[1],
									},
								})
							}
						}
					}
				}
			}
		}
		return parts
	default:
		return nil
	}
}
