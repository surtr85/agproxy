package server

import (
	"bufio"
	"bytes"
	"context"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/surtr85/agproxy/internal/auth"
	"github.com/surtr85/agproxy/internal/config"
	"github.com/surtr85/agproxy/internal/quota"
	"github.com/surtr85/agproxy/internal/stealth"
	"github.com/surtr85/agproxy/internal/transformer"
	"github.com/surtr85/agproxy/internal/upstream"
)

//go:embed web/dashboard.html
var dashboardHTML []byte

type Server struct {
	store           *config.Store
	upstreamClient  *upstream.Client
	port            int
	host            string
	apiKey          string
	semMu           sync.Mutex
	semaphores      map[string]chan struct{}
	lastRequestTime map[string]time.Time
}

func NewServer(store *config.Store, host string, port int, proxyURL string, apiKey string) *Server {
	return &Server{
		store:           store,
		upstreamClient:  upstream.NewClient(proxyURL),
		host:            host,
		port:            port,
		apiKey:          strings.TrimSpace(apiKey),
		semaphores:      make(map[string]chan struct{}),
		lastRequestTime: make(map[string]time.Time),
	}
}

func (s *Server) validateAuth(r *http.Request) bool {
	if s.apiKey == "" {
		return true // open / local mode
	}
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(parts[1]), []byte(s.apiKey)) == 1
}

func (s *Server) acquireAccountSlot(email string) func() {
	s.semMu.Lock()
	ch, ok := s.semaphores[email]
	if !ok {
		ch = make(chan struct{}, 2) // max 2 concurrent requests per Google account to prevent bot flagging
		s.semaphores[email] = ch
	}
	last := s.lastRequestTime[email]
	elapsed := time.Since(last)
	minInterval := 150 * time.Millisecond // minimum spacing to prevent bursting
	if elapsed < minInterval {
		time.Sleep(minInterval - elapsed)
	}
	s.lastRequestTime[email] = time.Now()
	s.semMu.Unlock()

	ch <- struct{}{}
	return func() {
		<-ch
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/dashboard", s.handleDashboard)
	mux.HandleFunc("/api/dashboard", s.handleAPIDashboard)
	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("/v1/images/generations", s.handleImageGenerations)
	mux.HandleFunc("/v1/images/", s.handleGetImage)
	mux.HandleFunc("/v1/models", s.handleModels)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/", s.handleRoot)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	log.Printf("[agproxy] Listening on http://%s", addr)
	log.Printf("[agproxy] Web Mission Control: http://%s/dashboard", addr)
	log.Printf("[agproxy] Ready for OpenAI-compatible clients (Cursor, Cline, Zed, Claude Code)")
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(dashboardHTML)
}

func (s *Server) handleAPIDashboard(w http.ResponseWriter, r *http.Request) {
	accounts := s.store.GetAllAccounts()
	active := s.store.GetActiveAccount()
	activeEmail := ""
	if active != nil {
		activeEmail = active.Email
	}

	type AccountDTO struct {
		Email   string             `json:"email"`
		Tier    string             `json:"tier"`
		Quotas  []quota.SingleQuotaItem `json:"quotas"`
		Blocked bool               `json:"blocked"`
	}

	var dtoList []AccountDTO
	for _, acc := range accounts {
		dto := AccountDTO{
			Email:   acc.Email,
			Tier:    acc.TierID,
			Blocked: !acc.BlockedUntil.IsZero() && acc.BlockedUntil.After(time.Now()),
		}

		if rep, err := quota.FetchAccountQuota(acc, s.store); err == nil && rep != nil {
			if rep.TierID != "" {
				dto.Tier = rep.TierID
			}
			dto.Quotas = rep.Items
		}
		dtoList = append(dtoList, dto)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"version":      "0.1.0",
		"active_email": activeEmail,
		"accounts":     dtoList,
		"timestamp":    time.Now().Unix(),
	})
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"service":        "agproxy",
		"status":         "running",
		"version":        "0.1.0",
		"active_account": s.store.GetActiveAccount() != nil,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK\n"))
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if !s.validateAuth(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Invalid or missing API key",
				"type":    "invalid_request_error",
				"code":    "invalid_api_key",
			},
		})
		return
	}

	type ModelEntry struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	type ModelsResponse struct {
		Object string       `json:"object"`
		Data   []ModelEntry `json:"data"`
	}

	var data []ModelEntry
	now := time.Now().Unix()

	for k := range upstream.ModelRegistry {
		data = append(data, ModelEntry{
			ID:      k,
			Object:  "model",
			Created: now,
			OwnedBy: "google-antigravity",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ModelsResponse{
		Object: "list",
		Data:   data,
	})
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.validateAuth(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Invalid or missing API key",
				"type":    "invalid_request_error",
				"code":    "invalid_api_key",
			},
		})
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var chatReq transformer.OpenAIChatRequest
	if err := json.Unmarshal(bodyBytes, &chatReq); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Retry loop for multi-account auto-rotation upon 429
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		acc := s.store.GetActiveAccount()
		if acc == nil {
			http.Error(w, "No active account configured. Run 'agproxy login' first.", http.StatusUnauthorized)
			return
		}

		token, err := auth.EnsureValidToken(acc, s.store)
		if err != nil {
			log.Printf("[agproxy] Token error for %s: %v", acc.Email, err)
			s.store.RotateNextAccount(acc.Email, 5*time.Minute)
			continue
		}

		transformRes, err := transformer.OpenAIToAntigravity(&chatReq, acc.ProjectID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Transformation error: %v", err), http.StatusInternalServerError)
			return
		}

		// Acquire rate limiter slot (max 2 concurrent, anti-burst pacing)
		release := s.acquireAccountSlot(acc.Email)

		// Traffic audit logging for stealth inspection
		log.Printf("[AUDIT:INCOMING] Client Model: %s, Tools: %d, Messages: %d, Stream: %v", chatReq.Model, len(chatReq.Tools), len(chatReq.Messages), chatReq.Stream)
		if transformRes.Payload.Request.SystemInstruction != nil && len(transformRes.Payload.Request.SystemInstruction.Parts) > 0 {
			log.Printf("[AUDIT:SYSTEM_PROMPT] Sanitized: %q", transformRes.Payload.Request.SystemInstruction.Parts[0].Text)
		}
		if len(transformRes.Payload.Request.Tools) > 0 {
			var declNames []string
			for _, d := range transformRes.Payload.Request.Tools[0].FunctionDeclarations {
				declNames = append(declNames, d.Name)
			}
			log.Printf("[AUDIT:OUTGOING_TOOLS] Cloaked/Decoy Tools: %v", declNames)
		}
		log.Printf("[AUDIT:OUTGOING_ENVELOPE] UpstreamModel: %s, Project: %s, RequestID: %s", transformRes.Payload.Model, transformRes.Payload.Project, transformRes.Payload.RequestID)

		if chatReq.Stream {
			streamBody, statusCode, headers, err := s.upstreamClient.StreamGenerateContent(r.Context(), token, transformRes.Payload)
			if err != nil {
				release()
				log.Printf("[agproxy] Upstream error: %v", err)
				http.Error(w, fmt.Sprintf("Upstream error: %v", err), http.StatusBadGateway)
				return
			}

			if statusCode == http.StatusTooManyRequests {
				errResp, _ := io.ReadAll(streamBody)
				_ = streamBody.Close()
				release()

				retryHeader := headers.Get("Retry-After")
				delay := stealth.ParseRetryDelay(retryHeader, string(errResp))
				if delay <= 0 {
					delay = 15 * time.Minute
				}

				log.Printf("[agproxy] 429 Rate limited on %s (Cooldown: %v). Rotating to next account...", acc.Email, delay)
				s.store.RotateNextAccount(acc.Email, delay)
				continue
			}

			if statusCode != http.StatusOK {
				errResp, _ := io.ReadAll(streamBody)
				_ = streamBody.Close()
				release()
				log.Printf("[agproxy] Google error (%d): %s", statusCode, string(errResp))
				http.Error(w, fmt.Sprintf("Google API error: %s", string(errResp)), statusCode)
				return
			}

			// Success with active account
			s.store.MarkSuccess(acc.Email)

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Accel-Buffering", "no")

			flusher, ok := w.(http.Flusher)
			if !ok {
				_ = streamBody.Close()
				release()
				http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
				return
			}

			s.pipeSSEStream(streamBody, w, flusher, chatReq.Model, transformRes.ToolNameMap)
			release()
			return
		} else {
			// Non-streaming request
			respBytes, statusCode, headers, err := s.upstreamClient.GenerateContent(r.Context(), token, transformRes.Payload)
			release()
			if err != nil {
				http.Error(w, fmt.Sprintf("Upstream error: %v", err), http.StatusBadGateway)
				return
			}

			if statusCode == http.StatusTooManyRequests {
				retryHeader := headers.Get("Retry-After")
				delay := stealth.ParseRetryDelay(retryHeader, string(respBytes))
				if delay <= 0 {
					delay = 15 * time.Minute
				}

				log.Printf("[agproxy] 429 Rate limited on %s (Cooldown: %v). Rotating to next account...", acc.Email, delay)
				s.store.RotateNextAccount(acc.Email, delay)
				continue
			}

			if statusCode != http.StatusOK {
				http.Error(w, fmt.Sprintf("Google API error: %s", string(respBytes)), statusCode)
				return
			}

			s.store.MarkSuccess(acc.Email)
			s.respondNonStreaming(w, respBytes, chatReq.Model, transformRes.ToolNameMap)
			return
		}
	}

	http.Error(w, "All accounts are currently rate-limited or exhausted.", http.StatusTooManyRequests)
}

func (s *Server) pipeSSEStream(upstreamStream io.ReadCloser, w http.ResponseWriter, flusher http.Flusher, model string, toolNameMap map[string]string) {
	defer upstreamStream.Close()

	sseState := transformer.NewSSEState(model, toolNameMap)
	reader := bufio.NewReader(upstreamStream)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("[agproxy] Stream read error: %v", err)
			}
			break
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// Google SSE lines start with "data: "
		if bytes.HasPrefix(line, []byte("data: ")) {
			payload := bytes.TrimPrefix(line, []byte("data: "))
			converted, isDone, err := sseState.ConvertChunk(payload)
			if err != nil {
				continue
			}
			if len(converted) > 0 {
				_, _ = w.Write(converted)
				flusher.Flush()
			}
			if isDone {
				break
			}
		}
	}

	// Emit finish DONE
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}

func (s *Server) respondNonStreaming(w http.ResponseWriter, googleRespBytes []byte, model string, toolNameMap map[string]string) {
	var gChunk upstream.GoogleStreamChunk
	if err := json.Unmarshal(googleRespBytes, &gChunk); err != nil {
		http.Error(w, "Failed to parse Google response", http.StatusInternalServerError)
		return
	}

	content := ""
	var toolCalls []transformer.OpenAIToolCallItem
	toolIndex := 0

	if len(gChunk.Response.Candidates) > 0 {
		cand := gChunk.Response.Candidates[0]
		for _, part := range cand.Content.Parts {
			if part.Text != "" && !part.Thought {
				content += part.Text
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
					content += fmt.Sprintf("\n\n![Generated Image](file://%s)\n\n", filePath)
				}
			}
			if part.FunctionCall != nil {
				callID := fmt.Sprintf("call_%d_%d", time.Now().UnixMilli(), toolIndex)
				origName := part.FunctionCall.Name
				if orig, ok := toolNameMap[origName]; ok {
					origName = orig
				}
				argsBytes, _ := json.Marshal(part.FunctionCall.Args)

				item := transformer.OpenAIToolCallItem{
					Index: toolIndex,
					ID:    callID,
					Type:  "function",
				}
				item.Function.Name = origName
				item.Function.Arguments = string(argsBytes)
				toolCalls = append(toolCalls, item)
				toolIndex++
			}
		}
	}

	type NonStreamMessage struct {
		Role      string                           `json:"role"`
		Content   string                           `json:"content"`
		ToolCalls []transformer.OpenAIToolCallItem `json:"tool_calls,omitempty"`
	}

	type NonStreamChoice struct {
		Index        int              `json:"index"`
		Message      NonStreamMessage `json:"message"`
		FinishReason string           `json:"finish_reason"`
	}

	type NonStreamResponse struct {
		ID      string            `json:"id"`
		Object  string            `json:"object"`
		Created int64             `json:"created"`
		Model   string            `json:"model"`
		Choices []NonStreamChoice `json:"choices"`
		Usage   struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	resp := NonStreamResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []NonStreamChoice{
			{
				Index: 0,
				Message: NonStreamMessage{
					Role:      "assistant",
					Content:   content,
					ToolCalls: toolCalls,
				},
				FinishReason: finishReason,
			},
		},
	}
	resp.Usage.PromptTokens = gChunk.Response.UsageMetadata.PromptTokenCount
	resp.Usage.CompletionTokens = gChunk.Response.UsageMetadata.CandidatesTokenCount
	resp.Usage.TotalTokens = gChunk.Response.UsageMetadata.TotalTokenCount

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleGetImage(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/v1/images/")
	filename = filepath.Base(filename)
	if filename == "" || filename == "." {
		http.NotFound(w, r)
		return
	}

	homeDir, _ := os.UserHomeDir()
	filePath := filepath.Join(homeDir, ".cache", "agproxy", "images", filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	if strings.HasSuffix(filename, ".png") {
		w.Header().Set("Content-Type", "image/png")
	} else {
		w.Header().Set("Content-Type", "image/jpeg")
	}
	http.ServeFile(w, r, filePath)
}

func (s *Server) handleImageGenerations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.validateAuth(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Invalid or missing API key",
				"type":    "invalid_request_error",
				"code":    "invalid_api_key",
			},
		})
		return
	}

	type ImageGenReq struct {
		Prompt         string `json:"prompt"`
		Model          string `json:"model"`
		N              int    `json:"n"`
		Size           string `json:"size"`
		ResponseFormat string `json:"response_format"`
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var req ImageGenReq
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Prompt) == "" {
		http.Error(w, "Prompt is required", http.StatusBadRequest)
		return
	}

	modelID := req.Model
	if modelID == "" {
		modelID = "gemini-3.1-flash-image"
	}
	modelInfo := upstream.ResolveModel(modelID)
	upstreamModel := modelInfo.UpstreamModel

	log.Printf("[IMAGE:START] Generating image with upstream model: %s (prompt: %q)", upstreamModel, req.Prompt)

	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		acc := s.store.GetActiveAccount()
		if acc == nil {
			http.Error(w, "No active Google Antigravity account available", http.StatusServiceUnavailable)
			return
		}

		token, err := auth.EnsureValidToken(acc, s.store)
		if err != nil {
			log.Printf("[IMAGE:AUTH_ERROR] Account %s token refresh error: %v", acc.Email, err)
			s.store.RotateNextAccount(acc.Email, 5*time.Minute)
			continue
		}

		release := s.acquireAccountSlot(acc.Email)

		projectID := acc.ProjectID
		if projectID == "" {
			projectID = "aicode-consumers"
		}

		payload := &upstream.AntigravityRequestWrapper{
			Project:   projectID,
			Model:     upstreamModel,
			UserAgent: upstream.UserAgent,
			RequestID: fmt.Sprintf("agent/img/%d", time.Now().UnixNano()),
			Request: upstream.AntigravityRequest{
				Contents: []upstream.AntigravityContent{
					{
						Role: "user",
						Parts: []upstream.AntigravityPart{
							{Text: req.Prompt},
						},
					},
				},
				SessionID: fmt.Sprintf("img-sess-%d", time.Now().UnixNano()),
			},
		}

		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		respBytes, statusCode, headers, err := s.upstreamClient.GenerateContent(ctx, token, payload)
		cancel()
		release()

		if err != nil {
			log.Printf("[IMAGE:ERROR] Attempt %d error: %v", attempt+1, err)
			continue
		}

		if statusCode == http.StatusTooManyRequests {
			retryHeader := headers.Get("Retry-After")
			delay := stealth.ParseRetryDelay(retryHeader, string(respBytes))
			if delay <= 0 {
				delay = 30 * time.Second
			}
			s.store.RotateNextAccount(acc.Email, delay)
			continue
		}

		if statusCode != http.StatusOK {
			log.Printf("[IMAGE:UPSTREAM_STATUS] Status %d: %s", statusCode, string(respBytes))
			continue
		}

		// Success!
		s.store.MarkSuccess(acc.Email)

		var gChunk upstream.GoogleStreamChunk
		if err := json.Unmarshal(respBytes, &gChunk); err != nil {
			http.Error(w, "Failed to parse Google response", http.StatusInternalServerError)
			return
		}

		var b64Data string
		mimeType := "image/jpeg"

		if len(gChunk.Response.Candidates) > 0 {
			for _, part := range gChunk.Response.Candidates[0].Content.Parts {
				if part.InlineData != nil && part.InlineData.Data != "" {
					b64Data = part.InlineData.Data
					if part.InlineData.MimeType != "" {
						mimeType = part.InlineData.MimeType
					}
					break
				}
			}
		}

		if b64Data == "" {
			http.Error(w, "No image was returned by upstream model", http.StatusBadGateway)
			return
		}

		homeDir, _ := os.UserHomeDir()
		cacheDir := filepath.Join(homeDir, ".cache", "agproxy", "images")
		_ = os.MkdirAll(cacheDir, 0755)
		ext := ".jpeg"
		if strings.Contains(mimeType, "png") {
			ext = ".png"
		}
		imgID := fmt.Sprintf("img_%d", time.Now().UnixNano())
		fileName := imgID + ext
		filePath := filepath.Join(cacheDir, fileName)

		if rawBytes, err := base64.StdEncoding.DecodeString(b64Data); err == nil {
			_ = os.WriteFile(filePath, rawBytes, 0644)
			log.Printf("[IMAGE:SAVED] Generated image saved to %s (%d bytes)", filePath, len(rawBytes))
		}

		imageURL := fmt.Sprintf("http://%s:%d/v1/images/%s", s.host, s.port, fileName)

		type ImageItem struct {
			B64JSON       string `json:"b64_json,omitempty"`
			URL           string `json:"url,omitempty"`
			RevisedPrompt string `json:"revised_prompt,omitempty"`
		}

		type ImageResponse struct {
			Created int64       `json:"created"`
			Data    []ImageItem `json:"data"`
		}

		item := ImageItem{
			RevisedPrompt: req.Prompt,
		}
		if req.ResponseFormat == "b64_json" {
			item.B64JSON = b64Data
		} else if req.ResponseFormat == "url" {
			item.URL = imageURL
		} else {
			// Include both
			item.B64JSON = b64Data
			item.URL = imageURL
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ImageResponse{
			Created: time.Now().Unix(),
			Data:    []ImageItem{item},
		})
		return
	}

	http.Error(w, "Failed to generate image across all available accounts", http.StatusServiceUnavailable)
}
