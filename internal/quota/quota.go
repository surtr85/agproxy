package quota

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/surtr85/agproxy/internal/auth"
	"github.com/surtr85/agproxy/internal/config"
	"github.com/surtr85/agproxy/internal/upstream"
)

type FetchAvailableModelsResponse struct {
	Models map[string]ModelQuotaItem `json:"models"`
}

type ModelQuotaItem struct {
	QuotaInfo *struct {
		RemainingFraction float64 `json:"remainingFraction"`
		ResetTime         string  `json:"resetTime"`
	} `json:"quotaInfo"`
	DisplayName string `json:"displayName"`
	IsInternal  bool   `json:"isInternal"`
}

type RetrieveUserQuotaSummaryResponse struct {
	Groups []struct {
		DisplayName string `json:"displayName"`
		Buckets     []struct {
			BucketID          string  `json:"bucketId"`
			DisplayName       string  `json:"displayName"`
			Window            string  `json:"window"`
			RemainingFraction float64 `json:"remainingFraction"`
			ResetTime         string  `json:"resetTime"`
			Disabled          bool    `json:"disabled"`
		} `json:"buckets"`
	} `json:"groups"`
}

type QuotaReport struct {
	Email         string
	ProjectID     string
	TierID        string
	Models        map[string]float64 // model -> remaining %
	WeeklyGemini  float64
	SessionGemini float64
	WeeklyClaude  float64
	SessionClaude float64
}

func FetchAccountQuota(acc *config.Account, store *config.Store) (*QuotaReport, error) {
	token, err := auth.EnsureValidToken(acc, store)
	if err != nil {
		return nil, err
	}

	report := &QuotaReport{
		Email:         acc.Email,
		ProjectID:     acc.ProjectID,
		TierID:        acc.TierID,
		Models:        make(map[string]float64),
		WeeklyGemini:  -1,
		SessionGemini: -1,
		WeeklyClaude:  -1,
		SessionClaude: -1,
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// 1. Fetch available models quota
	modelsURL := fmt.Sprintf("%s/v1internal:fetchAvailableModels", upstream.BaseURL)
	reqBody, _ := json.Marshal(map[string]string{"project": acc.ProjectID})
	req, err := http.NewRequest("POST", modelsURL, bytes.NewReader(reqBody))
	if err == nil {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("User-Agent", upstream.UserAgent)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Client-Name", "antigravity")
		req.Header.Set("X-Client-Version", "2.11.0")

		if resp, err := client.Do(req); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var data FetchAvailableModelsResponse
				if err := json.NewDecoder(resp.Body).Decode(&data); err == nil {
					for k, v := range data.Models {
						if v.QuotaInfo != nil && !v.IsInternal {
							report.Models[k] = v.QuotaInfo.RemainingFraction * 100.0
						}
					}
				}
			}
		}
	}

	// 2. Fetch weekly / session quota summary
	summaryURL := fmt.Sprintf("%s/v1internal:retrieveUserQuotaSummary", upstream.BaseURL)
	req2, err := http.NewRequest("POST", summaryURL, bytes.NewReader(reqBody))
	if err == nil {
		req2.Header.Set("Authorization", "Bearer "+token)
		req2.Header.Set("User-Agent", upstream.UserAgent)
		req2.Header.Set("Content-Type", "application/json")

		if resp, err := client.Do(req2); err == nil {
			defer resp.Body.Close()
			bodyBytes, _ := io.ReadAll(resp.Body)
			var data RetrieveUserQuotaSummaryResponse
			if err := json.Unmarshal(bodyBytes, &data); err == nil {
				for _, g := range data.Groups {
					isGemini := bytes.Contains([]byte(g.DisplayName), []byte("Gemini"))
					isClaude := bytes.Contains([]byte(g.DisplayName), []byte("Claude")) || bytes.Contains([]byte(g.DisplayName), []byte("GPT"))

					for _, b := range g.Buckets {
						rem := b.RemainingFraction * 100.0
						if b.Disabled {
							rem = 0
						}
						w := b.Window
						if w == "weekly" || bytes.Contains([]byte(b.DisplayName), []byte("Weekly")) || bytes.Contains([]byte(g.DisplayName), []byte("Weekly")) {
							if isGemini {
								report.WeeklyGemini = rem
							} else if isClaude {
								report.WeeklyClaude = rem
							}
						} else {
							if isGemini {
								report.SessionGemini = rem
							} else if isClaude {
								report.SessionClaude = rem
							}
						}
					}
				}
			}
		}
	}

	return report, nil
}

func PrintQuotaTable(reports []*QuotaReport) {
	fmt.Println()
	fmt.Println("==========================================================================================")
	fmt.Printf("%-28s | %-12s | %-16s | %-16s\n", "ACCOUNT", "TIER", "GEMINI (5h / Wk)", "CLAUDE (5h / Wk)")
	fmt.Println("------------------------------------------------------------------------------------------")

	for _, r := range reports {
		geminiStr := fmtQuotaPair(r.SessionGemini, r.WeeklyGemini)
		claudeStr := fmtQuotaPair(r.SessionClaude, r.WeeklyClaude)

		tier := r.TierID
		if len(tier) > 12 {
			tier = tier[:12]
		}
		fmt.Printf("%-28s | %-12s | %-16s | %-16s\n", truncate(r.Email, 28), tier, geminiStr, claudeStr)
	}
	fmt.Println("==========================================================================================")

	// Detail per-model breakdown
	for _, r := range reports {
		if len(r.Models) > 0 {
			fmt.Printf("\nDetailed Model Quota for [%s]:\n", r.Email)
			var keys []string
			for k := range r.Models {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				val := r.Models[k]
				bar := renderProgressBar(val)
				fmt.Printf("  • %-26s %s %5.1f%%\n", k, bar, val)
			}
		}
	}
	fmt.Println()
}

func fmtQuotaPair(session, weekly float64) string {
	sStr := "N/A"
	wStr := "N/A"
	if session >= 0 {
		sStr = fmt.Sprintf("%.0f%%", session)
	}
	if weekly >= 0 {
		wStr = fmt.Sprintf("%.0f%%", weekly)
	}
	return fmt.Sprintf("%s / %s", sStr, wStr)
}

func renderProgressBar(pct float64) string {
	width := 15
	filled := int((pct / 100.0) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	bar := "["
	for i := 0; i < filled; i++ {
		bar += "#"
	}
	for i := filled; i < width; i++ {
		bar += "-"
	}
	bar += "]"
	return bar
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
