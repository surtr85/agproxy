package transformer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/surtr85/agproxy/internal/stealth"
	"github.com/surtr85/agproxy/internal/upstream"
)

type OpenAISSEDelta struct {
	Role             string               `json:"role,omitempty"`
	Content          string               `json:"content,omitempty"`
	ReasoningContent string               `json:"reasoning_content,omitempty"`
	ToolCalls        []OpenAIToolCallItem `json:"tool_calls,omitempty"`
}

type OpenAIToolCallItem struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAISSEChunk struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []struct {
		Index        int            `json:"index"`
		Delta        OpenAISSEDelta `json:"delta"`
		FinishReason *string        `json:"finish_reason"`
	} `json:"choices"`
	Usage *OpenAIUsage `json:"usage,omitempty"`
}

type SSEState struct {
	ResponseID       string
	Model            string
	Created          int64
	ToolNameMap      map[string]string
	ToolIndex        int
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

func NewSSEState(model string, toolNameMap map[string]string) *SSEState {
	return &SSEState{
		ResponseID:  fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Model:       model,
		Created:     time.Now().Unix(),
		ToolNameMap: toolNameMap,
		ToolIndex:   0,
	}
}

func (s *SSEState) ConvertChunk(googleChunkBytes []byte) ([]byte, bool, error) {
	var gChunk upstream.GoogleStreamChunk
	if err := json.Unmarshal(googleChunkBytes, &gChunk); err != nil {
		return nil, false, err
	}

	if len(gChunk.Response.Candidates) == 0 {
		return nil, false, nil
	}

	candidate := gChunk.Response.Candidates[0]
	delta := OpenAISSEDelta{}

	for _, part := range candidate.Content.Parts {
		if part.Thought {
			delta.ReasoningContent += part.Text
		} else if part.Text != "" {
			delta.Content += part.Text
		}

		if part.InlineData != nil && part.InlineData.Data != "" {
			homeDir, _ := os.UserHomeDir()
			cacheDir := filepath.Join(homeDir, ".cache", "agproxy", "images")
			_ = os.MkdirAll(cacheDir, 0755)
			ext := ".jpeg"
			if strings.Contains(part.InlineData.MimeType, "png") {
				ext = ".png"
			}
			imgID := fmt.Sprintf("img_%d", time.Now().UnixNano())
			filePath := filepath.Join(cacheDir, imgID+ext)
			if raw, err := base64.StdEncoding.DecodeString(part.InlineData.Data); err == nil {
				_ = os.WriteFile(filePath, raw, 0644)
				delta.Content += fmt.Sprintf("\n\n![Generated Image](file://%s)\n\n", filePath)
			}
		}

		if part.FunctionCall != nil {
			callID := fmt.Sprintf("call_%d_%d", time.Now().UnixMilli(), s.ToolIndex)
			if part.ThoughtSignature != "" {
				stealth.StoreThoughtSignature(callID, part.ThoughtSignature)
			}

			origName := part.FunctionCall.Name
			if orig, ok := s.ToolNameMap[origName]; ok {
				origName = orig
			}
			stealth.StoreToolCallName(callID, part.FunctionCall.Name)

			argsBytes, _ := json.Marshal(part.FunctionCall.Args)

			toolItem := OpenAIToolCallItem{
				Index: s.ToolIndex,
				ID:    callID,
				Type:  "function",
			}
			toolItem.Function.Name = origName
			toolItem.Function.Arguments = string(argsBytes)

			delta.ToolCalls = append(delta.ToolCalls, toolItem)
			s.ToolIndex++
		}
	}

	var finishReason *string
	isDone := false
	if candidate.FinishReason != "" && candidate.FinishReason != "UNSPECIFIED" {
		isDone = true
		r := "stop"
		switch candidate.FinishReason {
		case "STOP":
			r = "stop"
		case "MAX_TOKENS":
			r = "length"
		case "SAFETY":
			r = "content_filter"
		}
		if len(delta.ToolCalls) > 0 {
			r = "tool_calls"
		}
		finishReason = &r
	}

	if gChunk.Response.UsageMetadata.TotalTokenCount > 0 {
		s.PromptTokens = gChunk.Response.UsageMetadata.PromptTokenCount
		s.CompletionTokens = gChunk.Response.UsageMetadata.CandidatesTokenCount
		s.TotalTokens = gChunk.Response.UsageMetadata.TotalTokenCount
	}

	if delta.Content == "" && delta.ReasoningContent == "" && len(delta.ToolCalls) == 0 && finishReason == nil {
		return nil, isDone, nil
	}

	chunk := OpenAISSEChunk{
		ID:      s.ResponseID,
		Object:  "chat.completion.chunk",
		Created: s.Created,
		Model:   s.Model,
	}

	choice := struct {
		Index        int            `json:"index"`
		Delta        OpenAISSEDelta `json:"delta"`
		FinishReason *string        `json:"finish_reason"`
	}{
		Index:        0,
		Delta:        delta,
		FinishReason: finishReason,
	}

	chunk.Choices = append(chunk.Choices, choice)

	if isDone && s.TotalTokens > 0 {
		chunk.Usage = &OpenAIUsage{
			PromptTokens:     s.PromptTokens,
			CompletionTokens: s.CompletionTokens,
			TotalTokens:      s.TotalTokens,
		}
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		return nil, isDone, err
	}

	return []byte(fmt.Sprintf("data: %s\n\n", string(data))), isDone, nil
}
