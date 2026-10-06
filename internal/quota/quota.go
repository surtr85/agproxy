package quota

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/surtr85/agproxy/internal/auth"
	"github.com/surtr85/agproxy/internal/config"
	"github.com/surtr85/agproxy/internal/upstream"
)

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

type SingleQuotaItem struct {
	Name       string
	Used       int
	Total      int
	Percentage int
	ResetIn    string
	ResetTime  time.Time
}

type QuotaReport struct {
	Email     string
	ProjectID string
	TierID    string
	Items     []SingleQuotaItem
}

func FetchAccountQuota(acc *config.Account, store *config.Store) (*QuotaReport, error) {
	token, err := auth.EnsureValidToken(acc, store)
	if err != nil {
		return nil, err
	}

	report := &QuotaReport{
		Email:     acc.Email,
		ProjectID: acc.ProjectID,
		TierID:    acc.TierID,
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Fetch fresh tier info if needed
	if projectID, tier, err := auth.LoadProjectAndTier(token); err == nil && tier != "" {
		report.TierID = tier
		if projectID != "" {
			report.ProjectID = projectID
		}
	}

	// Retrieve User Quota Summary
	summaryURL := fmt.Sprintf("%s/v1internal:retrieveUserQuotaSummary", upstream.BaseURL)
	reqBody, _ := json.Marshal(map[string]string{"project": acc.ProjectID})
	req, err := http.NewRequest("POST", summaryURL, bytes.NewReader(reqBody))
	if err != nil {
		return report, nil
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", upstream.UserAgent)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return report, nil
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var data RetrieveUserQuotaSummaryResponse
	_ = json.Unmarshal(bodyBytes, &data)

	// Extract 5h window fractions for Gemini & 3P (Claude/GPT)
	var geminiRemaining = 1.0
	var geminiResetStr = "5h 0m"
	var geminiResetTime time.Time

	var claudeRemaining = 1.0
	var claudeResetStr = "5h 0m"
	var claudeResetTime time.Time

	now := time.Now()

	for _, g := range data.Groups {
		isGemini := bytes.Contains([]byte(g.DisplayName), []byte("Gemini"))
		is3P := bytes.Contains([]byte(g.DisplayName), []byte("Claude")) || bytes.Contains([]byte(g.DisplayName), []byte("GPT"))

		for _, b := range g.Buckets {
			if b.Window == "5h" || (!bytes.Contains([]byte(b.DisplayName), []byte("Weekly")) && !bytes.Contains([]byte(b.BucketID), []byte("weekly"))) {
				rem := b.RemainingFraction
				if b.Disabled {
					rem = 0
				}
				resetDiff := ""
				var t time.Time
				if b.ResetTime != "" {
					if parsed, err := time.Parse(time.RFC3339Nano, b.ResetTime); err == nil {
						t = parsed
						if parsed.After(now) {
							resetDiff = formatTimeRemaining(parsed.Sub(now))
						} else {
							resetDiff = "now"
						}
					}
				}

				if isGemini {
					geminiRemaining = rem
					geminiResetTime = t
					if resetDiff != "" {
						geminiResetStr = resetDiff
					}
				} else if is3P {
					claudeRemaining = rem
					claudeResetTime = t
					if resetDiff != "" {
						claudeResetStr = resetDiff
					}
				}
			}
		}
	}

	// Calculate 1,000 unit budget
	// Percentage = remainingFraction * 100
	// Used = round((1.0 - remainingFraction) * 1000)
	geminiPct := int(math.Round(geminiRemaining * 100))
	geminiUsed := int(math.Round((1.0 - geminiRemaining) * 1000))
	if geminiUsed < 0 {
		geminiUsed = 0
	}
	if geminiUsed > 1000 {
		geminiUsed = 1000
	}

	claudePct := int(math.Round(claudeRemaining * 100))
	claudeUsed := int(math.Round((1.0 - claudeRemaining) * 1000))
	if claudeUsed < 0 {
		claudeUsed = 0
	}
	if claudeUsed > 1000 {
		claudeUsed = 1000
	}

	// 4 Quota items matching the UI architecture:
	// 1. Gemini (Flash / Pro)
	// 2. Claude (Sonnet / Opus)
	// 3. GPT-OSS 120B (Medium)
	// 4. Gemini 3.1 Flash Image (shares Gemini bucket)
	report.Items = []SingleQuotaItem{
		{
			Name:       "Gemini (Flash / Pro)",
			Used:       geminiUsed,
			Total:      1000,
			Percentage: geminiPct,
			ResetIn:    geminiResetStr,
			ResetTime:  geminiResetTime,
		},
		{
			Name:       "Claude (Sonnet / Opus)",
			Used:       claudeUsed,
			Total:      1000,
			Percentage: claudePct,
			ResetIn:    claudeResetStr,
			ResetTime:  claudeResetTime,
		},
		{
			Name:       "GPT-OSS 120B (Medium)",
			Used:       claudeUsed,
			Total:      1000,
			Percentage: claudePct,
			ResetIn:    claudeResetStr,
			ResetTime:  claudeResetTime,
		},
		{
			Name:       "Gemini 3.1 Flash Image",
			Used:       geminiUsed,
			Total:      1000,
			Percentage: geminiPct,
			ResetIn:    geminiResetStr,
			ResetTime:  geminiResetTime,
		},
	}

	return report, nil
}

func formatTimeRemaining(d time.Duration) string {
	if d <= 0 {
		return "now"
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("in %dh %dm", hours, mins)
	}
	return fmt.Sprintf("in %dm", mins)
}

// PrintQuotaTable prints the Antigravity card layout matching the 4 quotas UI
func PrintQuotaTable(reports []*QuotaReport) {
	fmt.Println()
	for _, r := range reports {
		fmt.Println("┌─────────────────────────────────────────────────────────────────────────────┐")
		fmt.Printf("│ ✦ Antigravity  %-43s [%s]\n", r.Email, r.TierID)
		fmt.Printf("│   %d quotas\n", len(r.Items))
		fmt.Println("├─────────────────────────────────────────────────────────────────────────────┤")

		for _, item := range r.Items {
			bar := renderBar(item.Percentage, 24)
			colorDot := "●"
			if item.Percentage < 60 {
				colorDot = "🟡"
			} else {
				colorDot = "🟢"
			}

			fmt.Printf("│ %s %-23s %4d / %-5d %s  %3d%%  %-8s │\n",
				colorDot,
				item.Name,
				item.Used,
				item.Total,
				bar,
				item.Percentage,
				item.ResetIn,
			)
		}
		fmt.Println("└─────────────────────────────────────────────────────────────────────────────┘")
		fmt.Println()
	}
}

func renderBar(pct int, width int) string {
	filled := int((float64(pct) / 100.0) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	bar := ""
	for i := 0; i < filled; i++ {
		bar += "━"
	}
	for i := filled; i < width; i++ {
		bar += "─"
	}
	return bar
}
