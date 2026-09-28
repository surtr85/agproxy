package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	LoadCodeAssistURL = "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"
	OnboardUserURL    = "https://cloudcode-pa.googleapis.com/v1internal:onboardUser"
)

type ClientMetadata struct {
	IDEType    int `json:"ideType"`
	Platform   int `json:"platform"`
	PluginType int `json:"pluginType"`
}

var DefaultMetadata = ClientMetadata{
	IDEType:    9, // Antigravity
	Platform:   3, // Linux AMD64
	PluginType: 2, // Gemini
}

type LoadCodeAssistRequest struct {
	Metadata ClientMetadata `json:"metadata"`
}

type AllowedTier struct {
	ID        string `json:"id"`
	IsDefault bool   `json:"isDefault"`
}

type LoadCodeAssistResponse struct {
	CloudAICompanionProject any           `json:"cloudaicompanionProject"`
	AllowedTiers            []AllowedTier `json:"allowedTiers"`
	CurrentTier             struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"currentTier"`
}

type OnboardUserRequest struct {
	TierID   string         `json:"tierId"`
	Metadata ClientMetadata `json:"metadata"`
}

type OnboardUserResponse struct {
	Done     bool `json:"done"`
	Response any  `json:"response"`
}

func LoadProjectAndTier(accessToken string) (string, string, error) {
	reqBody, _ := json.Marshal(LoadCodeAssistRequest{Metadata: DefaultMetadata})
	req, err := http.NewRequest("POST", LoadCodeAssistURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("loadCodeAssist error (%d): %s", resp.StatusCode, string(body))
	}

	var data LoadCodeAssistResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return "", "", err
	}

	projectID := ""
	switch v := data.CloudAICompanionProject.(type) {
	case string:
		projectID = v
	case map[string]any:
		if id, ok := v["id"].(string); ok {
			projectID = id
		}
	}

	tierID := "legacy-tier"
	for _, tier := range data.AllowedTiers {
		if tier.IsDefault && tier.ID != "" {
			tierID = tier.ID
			break
		}
	}
	if data.CurrentTier.ID != "" {
		tierID = data.CurrentTier.ID
	}

	return projectID, tierID, nil
}

func OnboardUser(accessToken, tierID string) error {
	if tierID == "" {
		tierID = "legacy-tier"
	}

	client := &http.Client{Timeout: 10 * time.Second}

	for i := 0; i < 10; i++ {
		reqBody, _ := json.Marshal(OnboardUserRequest{
			TierID:   tierID,
			Metadata: DefaultMetadata,
		})

		req, err := http.NewRequest("POST", OnboardUserURL, bytes.NewReader(reqBody))
		if err != nil {
			return err
		}

		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", UserAgent)

		resp, err := client.Do(req)
		if err == nil {
			var result OnboardUserResponse
			if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
				_ = resp.Body.Close()
				if result.Done {
					return nil
				}
			} else {
				_ = resp.Body.Close()
			}
		}
		time.Sleep(3 * time.Second)
	}

	return nil
}
