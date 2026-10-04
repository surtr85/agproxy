package upstream

type ModelInfo struct {
	ID            string `json:"id"`
	UpstreamModel string `json:"upstream_model"`
	DisplayName   string `json:"display_name"`
}

var ModelRegistry = map[string]ModelInfo{
	// Gemini 3.8 Family
	"gemini-3.8-flash": {
		ID:            "gemini-3.8-flash",
		UpstreamModel: "gemini-3.8-flash-medium",
		DisplayName:   "Gemini 3.8 Flash",
	},
	"gemini-3.8-flash-high": {
		ID:            "gemini-3.8-flash-high",
		UpstreamModel: "gemini-3.8-flash-high",
		DisplayName:   "Gemini 3.8 Flash (High)",
	},
	"gemini-3.8-flash-medium": {
		ID:            "gemini-3.8-flash-medium",
		UpstreamModel: "gemini-3.8-flash-medium",
		DisplayName:   "Gemini 3.8 Flash (Medium)",
	},
	"gemini-3.8-flash-low": {
		ID:            "gemini-3.8-flash-low",
		UpstreamModel: "gemini-3.8-flash-low",
		DisplayName:   "Gemini 3.8 Flash (Low)",
	},

	// Gemini 3.7 Family
	"gemini-3.7-flash": {
		ID:            "gemini-3.7-flash",
		UpstreamModel: "gemini-3.7-flash-medium",
		DisplayName:   "Gemini 3.7 Flash",
	},
	"gemini-3.7-flash-high": {
		ID:            "gemini-3.7-flash-high",
		UpstreamModel: "gemini-3.7-flash-high",
		DisplayName:   "Gemini 3.7 Flash (High)",
	},
	"gemini-3.7-flash-medium": {
		ID:            "gemini-3.7-flash-medium",
		UpstreamModel: "gemini-3.7-flash-medium",
		DisplayName:   "Gemini 3.7 Flash (Medium)",
	},
	"gemini-3.7-flash-low": {
		ID:            "gemini-3.7-flash-low",
		UpstreamModel: "gemini-3.7-flash-low",
		DisplayName:   "Gemini 3.7 Flash (Low)",
	},

	// Gemini 3.6 Family
	"gemini-3.6-flash": {
		ID:            "gemini-3.6-flash",
		UpstreamModel: "gemini-3.6-flash-medium",
		DisplayName:   "Gemini 3.6 Flash",
	},

	// Gemini Pro Family
	"gemini-3.1-pro": {
		ID:            "gemini-3.1-pro",
		UpstreamModel: "gemini-3.1-pro",
		DisplayName:   "Gemini 3.1 Pro",
	},

	// Laya Autonomous System-1 Router (Google Gemini Multi-Tier)
	"laya": {
		ID:            "laya",
		UpstreamModel: "laya",
		DisplayName:   "Laya System-1 Autonomous Router (Google Gemini Multi-Tier)",
	},

	// Claude Family via Antigravity
	"claude-sonnet-4-6": {
		ID:            "claude-sonnet-4-6",
		UpstreamModel: "claude-sonnet-4-6",
		DisplayName:   "Claude Sonnet 4.6 (Thinking)",
	},
	"claude-opus-4-6-thinking": {
		ID:            "claude-opus-4-6-thinking",
		UpstreamModel: "claude-opus-4-6-thinking",
		DisplayName:   "Claude Opus 4.6 (Thinking)",
	},

	// GPT OSS
	"gpt-oss-120b-medium": {
		ID:            "gpt-oss-120b-medium",
		UpstreamModel: "gpt-oss-120b-medium",
		DisplayName:   "GPT-OSS 120B (Medium)",
	},
}

func ResolveModel(requestedModel string) ModelInfo {
	if info, ok := ModelRegistry[requestedModel]; ok {
		return info
	}

	// Fallback alias mappings
	switch requestedModel {
	case "claude-3-7-sonnet", "claude-sonnet", "sonnet-4.6":
		return ModelRegistry["claude-sonnet-4-6"]
	case "claude-opus", "opus-4.6":
		return ModelRegistry["claude-opus-4-6-thinking"]
	case "research", "raw", "uncensored":
		return ModelRegistry["gemini-3.8-flash-raw"]
	default:
		// Default to gemini-3.8-flash
		return ModelInfo{
			ID:            requestedModel,
			UpstreamModel: requestedModel,
			DisplayName:   requestedModel,
		}
	}
}
