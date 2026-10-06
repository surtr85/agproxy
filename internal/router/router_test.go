package router

import (
	"testing"
)

func TestRouteWithLayaContext_TrivialGreeting(t *testing.T) {
	// Fresh conversational ping
	model := RouteWithLayaContext("ping", 4, false)
	if model.ID != "gemini-3.8-flash-low" {
		t.Errorf("expected gemini-3.8-flash-low, got %s", model.ID)
	}

	// Follow-up ping in a large coding session with tools
	modelWithTools := RouteWithLayaContext("ok", 5000, true)
	if modelWithTools.ID == "gemini-3.8-flash-low" {
		t.Errorf("expected not to downgrade to flash-low in tool-assisted session, got %s", modelWithTools.ID)
	}
}
