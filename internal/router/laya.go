package router

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/surtr85/agproxy/internal/upstream"
)

type LayaDecision struct {
	Answers map[string]struct {
		Choice string  `json:"choice"`
		Score  float64 `json:"score"`
		Noul   float64 `json:"noul"`
	} `json:"answers"`
}

// RouteWithLaya queries Laya System-1 on port 8089 to dynamically select the optimal Google model
func RouteWithLaya(prompt string) upstream.ModelInfo {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return upstream.ModelRegistry["gemini-3.8-flash-low"]
	}

	// 1. Prepare fast request to Laya System-1 on port 8089
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	payload, _ := json.Marshal(map[string]any{
		"state": trimmed,
		"questions": map[string]any{
			"domain": map[string]any{
				"type":         "choice",
				"instructions": "What domain does request belong to?",
				"criteria": map[string]string{
					"code":          "programming, software engineering, debugging, architecture, scripts",
					"math_or_logic": "mathematics, logic proofs, complex multi-step calculation",
					"writing":       "creative writing, essays, emails",
					"chitchat":      "casual conversation, greetings, small talk, jokes",
					"other":         "general knowledge, simple lookups, definitions",
				},
			},
			"difficulty": map[string]any{
				"type":         "score",
				"instructions": "How hard is request for a language model?",
				"criteria": []string{
					"trivial: a lookup or one-liner",
					"easy: short answer, no reasoning",
					"moderate: several steps",
					"hard: long multi-step reasoning or specialist knowledge",
				},
			},
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", "http://127.0.0.1:8089/v1/systemone", bytes.NewReader(payload))
	if err != nil {
		log.Printf("[LAYA:FALLBACK] Failed to create Laya request: %v", err)
		return upstream.ModelRegistry["gemini-3.8-flash-medium"]
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 200 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[LAYA:FALLBACK] Laya service unreachable (%v), falling back to gemini-3.8-flash-medium", err)
		return upstream.ModelRegistry["gemini-3.8-flash-medium"]
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[LAYA:FALLBACK] Laya returned HTTP %d, falling back to gemini-3.8-flash-medium", resp.StatusCode)
		return upstream.ModelRegistry["gemini-3.8-flash-medium"]
	}

	var decision LayaDecision
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		log.Printf("[LAYA:FALLBACK] Failed to decode Laya response: %v", err)
		return upstream.ModelRegistry["gemini-3.8-flash-medium"]
	}

	domain := decision.Answers["domain"].Choice
	diff := decision.Answers["difficulty"].Score

	// Google-Only Model Routing Matrix
	var selectedModel string
	switch {
	// Deep Math, Logic, or Critical Hard Complexity -> Gemini 3.1 Pro
	case diff >= 2.5 || domain == "math_or_logic":
		selectedModel = "gemini-3.1-pro"

	// High Complexity Coding or Deep Multi-step Architecture -> Gemini 3.8 Flash (High effort)
	case (domain == "code" && diff >= 1.6) || diff >= 2.0:
		selectedModel = "gemini-3.8-flash-high"

	// Conversational, Greeting, Trivial Lookup -> Gemini 3.8 Flash (Low effort)
	case diff < 0.9 && (domain == "chitchat" || domain == "factual_lookup"):
		selectedModel = "gemini-3.8-flash-low"

	// Balanced Standard Tasks -> Gemini 3.8 Flash (Medium effort)
	default:
		selectedModel = "gemini-3.8-flash-medium"
	}

	log.Printf("[LAYA:ROUTER] System-1 Decision: domain=%s, difficulty=%.2f => Selected Google Model: %s", domain, diff, selectedModel)
	return upstream.ModelRegistry[selectedModel]
}
