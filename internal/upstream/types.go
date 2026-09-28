package upstream

// Antigravity Wire Protocol Types

type AntigravityRequestWrapper struct {
	Project   string             `json:"project"`
	Model     string             `json:"model"`
	UserAgent string             `json:"userAgent"`
	RequestID string             `json:"requestId"`
	Request   AntigravityRequest `json:"request"`
}

type AntigravityRequest struct {
	Contents          []AntigravityContent `json:"contents"`
	SystemInstruction *AntigravityContent  `json:"systemInstruction,omitempty"`
	Tools             []AntigravityTool    `json:"tools,omitempty"`
	ToolConfig        any                  `json:"toolConfig,omitempty"`
	GenerationConfig  map[string]any       `json:"generationConfig,omitempty"`
	SessionID         string               `json:"sessionId"`
	SafetySettings    []SafetySetting      `json:"safetySettings,omitempty"`
}

type SafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type AntigravityContent struct {
	Role  string            `json:"role,omitempty"`
	Parts []AntigravityPart `json:"parts"`
}

type AntigravityPart struct {
	Text             string              `json:"text,omitempty"`
	Thought          bool                `json:"thought,omitempty"`
	ThoughtSignature string              `json:"thoughtSignature,omitempty"`
	FunctionCall     *AntigravityFnCall  `json:"functionCall,omitempty"`
	FunctionResponse *AntigravityFnResp  `json:"functionResponse,omitempty"`
}

type AntigravityFnCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type AntigravityFnResp struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type AntigravityTool struct {
	FunctionDeclarations []AntigravityFunctionDecl `json:"functionDeclarations"`
}

type AntigravityFunctionDecl struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type GoogleStreamChunk struct {
	Response struct {
		Candidates []struct {
			Content struct {
				Role  string `json:"role"`
				Parts []struct {
					Text             string             `json:"text"`
					Thought          bool               `json:"thought"`
					ThoughtSignature string             `json:"thoughtSignature"`
					FunctionCall     *AntigravityFnCall `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
		ModelVersion string `json:"modelVersion"`
		ResponseID   string `json:"responseId"`
	} `json:"response"`
}
