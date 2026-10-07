package stealth

import (
	"testing"
)

func TestCleanJSONSchema_ArrayItems(t *testing.T) {
	// Case 1: Array without items should have fallback items added
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"includes": map[string]any{
				"type":        "array",
				"description": "Files to include",
			},
		},
	}

	cleaned := CleanJSONSchema(schema)
	props, ok := cleaned["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map, got %v", cleaned["properties"])
	}

	includes, ok := props["includes"].(map[string]any)
	if !ok {
		t.Fatalf("expected includes map, got %v", props["includes"])
	}

	items, ok := includes["items"].(map[string]any)
	if !ok {
		t.Fatalf("expected items map in includes, got %v", includes["items"])
	}

	if items["type"] != "string" {
		t.Errorf("expected items.type == 'string', got %v", items["type"])
	}
}

func TestCleanJSONSchema_ArrayWithEmptyItems(t *testing.T) {
	// Case 2: Array with empty items map
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tags": map[string]any{
				"type":  "array",
				"items": map[string]any{},
			},
		},
	}

	cleaned := CleanJSONSchema(schema)
	props := cleaned["properties"].(map[string]any)
	tags := props["tags"].(map[string]any)
	items := tags["items"].(map[string]any)

	if items["type"] != "string" {
		t.Errorf("expected items.type == 'string', got %v", items["type"])
	}
}

func TestCleanJSONSchema_NullableType(t *testing.T) {
	// Case 3: type list with null like ["string", "null"]
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type": []any{"string", "null"},
			},
		},
	}

	cleaned := CleanJSONSchema(schema)
	props := cleaned["properties"].(map[string]any)
	query := props["query"].(map[string]any)

	if query["type"] != "string" {
		t.Errorf("expected query.type == 'string', got %v", query["type"])
	}
}

func TestCleanJSONSchema_NestedArray(t *testing.T) {
	// Case 4: Nested array
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"matrix": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "array",
				},
			},
		},
	}

	cleaned := CleanJSONSchema(schema)
	props := cleaned["properties"].(map[string]any)
	matrix := props["matrix"].(map[string]any)
	items := matrix["items"].(map[string]any)
	innerItems := items["items"].(map[string]any)

	if items["type"] != "array" {
		t.Errorf("expected items.type == 'array', got %v", items["type"])
	}
	if innerItems["type"] != "string" {
		t.Errorf("expected innerItems.type == 'string', got %v", innerItems["type"])
	}
}
