package router

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
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

// Fast regex for trivial conversational greetings and pleasantries (< 1ms)
var trivialGreetingRegex = regexp.MustCompile(`(?i)^(?:hi|hello|hey|howdy|greetings|good\s+(?:morning|afternoon|evening)|how\s+are\s+you|what'?s\s+up|sup|thanks?|thank\s+you|ping|pong|test|سلام|درود|خوبی|چطوری|احوالت|صبح\s*بخیر|عصر\s*بخیر|شب\s*بخیر|مرسی|ممنون|دمت\s*گرم|قربانت|پینگ|تست)[\s!?.،]*$`)

// RouteWithLaya queries Laya System-1 on port 8089 to dynamically select the optimal Google model
func RouteWithLaya(prompt string) upstream.ModelInfo {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return upstream.ModelRegistry["gemini-3.8-flash-low"]
	}

	// Fast heuristic: Ultra-short greetings & small talk bypass network directly to Flash-Low
	if len(trimmed) < 80 && trivialGreetingRegex.MatchString(trimmed) {
		log.Printf("[LAYA:FAST_ROUTER] Trivial conversational greeting detected: %q => Selected Google Model: gemini-3.8-flash-low", trimmed)
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

	// Google-Only Model Routing Matrix calibrated to empirical Laya difficulty distributions:
	// - [0.0 - 1.35]: Trivial / Easy / Small talk / Basic one-liners
	// - [1.35 - 1.70]: Moderate standard tasks
	// - [1.70 - 2.05]: High complexity coding / Deep system logic
	// - [>= 2.05 or math_or_logic]: Hard logic, proofs, critical architecture
	var selectedModel string
	switch {
	// 1. Trivial chitchat, greetings, or very easy queries (< 1.30) -> Flash Low
	case domain == "chitchat" || diff < 1.30 || (domain == "other" && diff < 1.40):
		selectedModel = "gemini-3.8-flash-low"

	// 2. High-difficulty mathematical proofs, formal logic, or extreme complexity -> Gemini 3.1 Pro
	case domain == "math_or_logic" || diff >= 2.10:
		selectedModel = "gemini-3.1-pro"

	// 3. High-complexity coding, distributed systems, deep concurrency, architecture -> Gemini 3.8 Flash High
	case (domain == "code" && diff >= 1.65) || diff >= 1.85:
		selectedModel = "gemini-3.8-flash-high"

	// 4. Standard coding, writing, refactoring -> Gemini 3.8 Flash Medium
	default:
		selectedModel = "gemini-3.8-flash-medium"
	}

	log.Printf("[LAYA:ROUTER] System-1 Decision: domain=%s, difficulty=%.2f => Selected Google Model: %s", domain, diff, selectedModel)
	return upstream.ModelRegistry[selectedModel]
}
