package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/surtr85/agproxy/internal/config"
)

func TestDashboardEndpoints(t *testing.T) {
	store := &config.Store{}
	srv := NewServer(store, "127.0.0.1", 8080, "", "")

	// 1. Test HTML Dashboard mount
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/dashboard", nil)
	srv.handleDashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "agproxy // Mission Control") {
		t.Errorf("expected dashboard HTML title, got: %s", body[:100])
	}

	// 2. Test JSON Dashboard API
	recAPI := httptest.NewRecorder()
	reqAPI := httptest.NewRequest("GET", "/api/dashboard", nil)
	srv.handleAPIDashboard(recAPI, reqAPI)

	if recAPI.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for API, got %d", recAPI.Code)
	}
	apiBody := recAPI.Body.String()
	if !strings.Contains(apiBody, `"version":"0.1.0"`) {
		t.Errorf("expected json version, got: %s", apiBody)
	}
}
