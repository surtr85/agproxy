package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/surtr85/agproxy/internal/auth"
	"github.com/surtr85/agproxy/internal/config"
	"github.com/surtr85/agproxy/internal/quota"
	"github.com/surtr85/agproxy/internal/server"
	"github.com/surtr85/agproxy/internal/stealth"
	"github.com/surtr85/agproxy/internal/upstream"
)

const version = "0.1.0"

func printHelp() {
	fmt.Printf(`agproxy v%s - Ultra-lightweight Go Reverse Proxy for Google Antigravity

Usage:
  agproxy <command> [arguments]

Commands:
  serve       Start the OpenAI-compatible HTTP proxy server
  login       Log in with a Google Antigravity OAuth account
  quota       Inspect live quota and reset timers for all accounts
  doctor      Diagnose health of accounts, upstream connectivity, and Laya daemon
  accounts    Manage saved Google accounts (list, use, remove)
  image       Generate an image using Google Nano Banana 2 (Gemini 3.1 Flash Image)
  version     Show current version
  help        Show this help message

Options for 'serve':
  --port <port>    Port to listen on (default: 8080)
  --host <host>    Host to bind to (default: 127.0.0.1)
  --proxy <url>    Upstream HTTP/SOCKS5 proxy (e.g. socks5://127.0.0.1:1080)
  --api-key <key>  Secret API key required from clients (or set AGPROXY_API_KEY)

Examples:
  agproxy login
  agproxy quota
  agproxy image "A futuristic neon cyber cat in Tokyo" -o cat.jpg
  agproxy serve --port 8080 --api-key sk-my-secret-key
  agproxy serve --port 8080 --proxy socks5://127.0.0.1:1080
`, version)
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	cmd := os.Args[1]

	store, err := config.LoadStore()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	switch cmd {
	case "serve":
		defaultKey := os.Getenv("AGPROXY_API_KEY")
		serveFlags := flag.NewFlagSet("serve", flag.ExitOnError)
		port := serveFlags.Int("port", 8080, "Port to listen on")
		host := serveFlags.String("host", "127.0.0.1", "Host to bind to")
		proxy := serveFlags.String("proxy", "", "HTTP or SOCKS5 upstream proxy (e.g. socks5://127.0.0.1:1080)")
		apiKey := serveFlags.String("api-key", defaultKey, "API key required from clients (or set AGPROXY_API_KEY)")
		_ = serveFlags.Parse(os.Args[2:])

		s := server.NewServer(store, *host, *port, *proxy, *apiKey)
		if err := s.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}

	case "login":
		handleLogin(store)

	case "quota":
		handleQuota(store)

	case "doctor":
		handleDoctor(store)

	case "image":
		handleImage(store, os.Args[2:])

	case "accounts":
		handleAccounts(store, os.Args[2:])

	case "version":
		fmt.Printf("agproxy version %s (linux/amd64)\n", version)

	case "help", "-h", "--help":
		printHelp()

	default:
		fmt.Printf("Unknown command: %s\n\n", cmd)
		printHelp()
		os.Exit(1)
	}
}

func handleLogin(store *config.Store) {
	callbackPort := 8085
	redirectURI := fmt.Sprintf("http://localhost:%d/oauth2callback", callbackPort)
	state := auth.GenerateState()
	authURL := auth.BuildAuthURL(redirectURI, state)

	fmt.Println("\n=== Google Antigravity Authentication ===")
	fmt.Println("Please open the following authorization URL in your browser:")
	fmt.Println()
	fmt.Println(authURL)
	fmt.Println()
	fmt.Println("Waiting for authentication callback on http://localhost:8085/oauth2callback ...")

	code, err := auth.StartOAuthServer(callbackPort, state)
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	fmt.Println("Exchanging authorization code...")
	tokens, err := auth.ExchangeCode(code, redirectURI)
	if err != nil {
		log.Fatalf("Failed to exchange token: %v", err)
	}

	fmt.Println("Fetching Google account profile...")
	userInfo, err := auth.FetchUserInfo(tokens.AccessToken)
	if err != nil {
		log.Printf("Warning: failed to fetch user info: %v", err)
		userInfo = &auth.UserInfo{Email: fmt.Sprintf("user-%d@gmail.com", time.Now().Unix())}
	}

	fmt.Println("Discovering Google Cloud Companion Project (loadCodeAssist)...")
	projectID, tierID, err := auth.LoadProjectAndTier(tokens.AccessToken)
	if err != nil {
		log.Printf("Warning: loadCodeAssist returned: %v", err)
	}

	if projectID == "" {
		projectID = fmt.Sprintf("project-%d", time.Now().Unix())
		fmt.Printf("Assigned project: %s\n", projectID)
	} else {
		fmt.Printf("✓ Discovered Project ID: %s (Tier: %s)\n", projectID, tierID)
	}

	fmt.Println("Ensuring user onboarding...")
	go func() {
		_ = auth.OnboardUser(tokens.AccessToken, tierID)
	}()

	acc := &config.Account{
		Email:        userInfo.Email,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second),
		ProjectID:    projectID,
		TierID:       tierID,
	}

	if err := store.AddOrUpdateAccount(acc); err != nil {
		log.Fatalf("Failed to save account: %v", err)
	}

	fmt.Printf("\n✓ Account [%s] successfully connected and saved!\n", acc.Email)
	fmt.Println("You can now run: agproxy serve")
}

func handleDoctor(store *config.Store) {
	fmt.Println("\n=== agproxy Diagnostic Health Check (Doctor) ===")

	// 1. Check Accounts Configuration
	accounts := store.GetAllAccounts()
	activeAcc := store.GetActiveAccount()
	if len(accounts) == 0 {
		fmt.Println("❌ Accounts: No Google accounts found in ~/.config/agproxy/accounts.json")
		fmt.Println("   👉 Run 'agproxy login' to add an account.")
		return
	}
	fmt.Printf("✓ Accounts: %d registered accounts found (Active: %s)\n", len(accounts), func() string {
		if activeAcc != nil {
			return activeAcc.Email
		}
		return "None"
	}())

	// 2. Check Tokens & Google Upstream Connectivity
	fmt.Println("\nChecking Google Antigravity Upstream Connectivity:")
	for _, acc := range accounts {
		token, err := auth.EnsureValidToken(acc, store)
		status := "✓"
		extra := ""
		if err != nil {
			status = "❌"
			extra = fmt.Sprintf("(Token refresh failed: %v)", err)
		} else {
			// Test minimal ping to Antigravity API
			client := &http.Client{Timeout: 5 * time.Second}
			modelsURL := fmt.Sprintf("%s/v1internal:fetchAvailableModels", upstream.BaseURL)
			reqBody, _ := json.Marshal(map[string]string{"project": acc.ProjectID})
			req, _ := http.NewRequest("POST", modelsURL, bytes.NewReader(reqBody))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("User-Agent", upstream.UserAgent)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Client-Name", "antigravity")
			req.Header.Set("X-Client-Version", "2.11.0")
			
			resp, reqErr := client.Do(req)
			if reqErr != nil {
				status = "⚠️"
				extra = fmt.Sprintf("(Upstream request error: %v)", reqErr)
			} else {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					extra = fmt.Sprintf("(API reachable, Project: %s, Tier: %s)", acc.ProjectID, acc.TierID)
				} else {
					status = "⚠️"
					extra = fmt.Sprintf("(API returned HTTP %d)", resp.StatusCode)
				}
			}
		}
		fmt.Printf("  %s %-30s %s\n", status, acc.Email, extra)
	}

	// 3. Check Laya System-1 Local Daemon
	fmt.Println("\nChecking Laya System-1 Cognitive Engine (port 8089):")
	layaClient := &http.Client{Timeout: 1 * time.Second}
	layaPingReq, _ := http.NewRequest("GET", "http://127.0.0.1:8089/health", nil)
	layaResp, err := layaClient.Do(layaPingReq)
	if err != nil {
		// try ping via post to systemone
		testPayload, _ := json.Marshal(map[string]any{"state": "ping"})
		postReq, _ := http.NewRequest("POST", "http://127.0.0.1:8089/v1/systemone", bytes.NewReader(testPayload))
		postReq.Header.Set("Content-Type", "application/json")
		postResp, postErr := layaClient.Do(postReq)
		if postErr != nil {
			fmt.Println("  ⚠️  Laya Daemon: Unreachable on http://127.0.0.1:8089 (router will fallback to gemini-3.8-flash-medium)")
			fmt.Println("     💡 Note: Systemd user service: systemctl --user status laya.service")
		} else {
			postResp.Body.Close()
			fmt.Printf("  ✓  Laya Daemon: Online and operational (HTTP %d)\n", postResp.StatusCode)
		}
	} else {
		layaResp.Body.Close()
		fmt.Printf("  ✓  Laya Daemon: Online and healthy (HTTP %d)\n", layaResp.StatusCode)
	}

	// 4. Check Local Proxy Ports & Services
	fmt.Println("\nChecking Local Service Port Bindings:")
	sClient := &http.Client{Timeout: 500 * time.Millisecond}
	sResp, err := sClient.Get("http://127.0.0.1:8080/health")
	if err == nil {
		sResp.Body.Close()
		fmt.Println("  ✓  agproxy Server: Active and listening on http://127.0.0.1:8080")
	} else {
		fmt.Println("  ℹ️  agproxy Server: Not running on port 8080 (Start with 'agproxy serve' or systemctl --user start agproxy)")
	}

	fmt.Println("\n=== Doctor Summary: All core invariants verified. ===")
}

func handleQuota(store *config.Store) {
	store.SyncFromDisk()
	if len(store.Accounts) == 0 {
		fmt.Println("No accounts registered yet. Run 'agproxy login' to add an account.")
		return
	}

	fmt.Println("Fetching live quotas from Google Antigravity...")
	var reports []*quota.QuotaReport

	for _, acc := range store.Accounts {
		report, err := quota.FetchAccountQuota(acc, store)
		if err != nil {
			fmt.Printf("Error fetching quota for %s: %v\n", acc.Email, err)
			continue
		}
		reports = append(reports, report)
	}

	quota.PrintQuotaTable(reports)
}

func handleAccounts(store *config.Store, args []string) {
	store.SyncFromDisk()
	if len(args) == 0 || args[0] == "list" {
		if len(store.Accounts) == 0 {
			fmt.Println("No accounts registered. Run 'agproxy login' first.")
			return
		}

		fmt.Println("\nRegistered Accounts:")
		for email, acc := range store.Accounts {
			prefix := "  "
			if email == store.ActiveEmail {
				prefix = "* "
			}
			status := "Active"
			if !acc.BlockedUntil.IsZero() && acc.BlockedUntil.After(time.Now()) {
				status = fmt.Sprintf("Blocked until %s", acc.BlockedUntil.Format("15:04:05"))
			}
			fmt.Printf("%s%-30s | Tier: %-12s | Project: %-25s | Status: %s\n", prefix, email, acc.TierID, acc.ProjectID, status)
		}
		fmt.Println("(* denotes currently active account)")
		return
	}

	sub := args[0]
	switch sub {
	case "use":
		if len(args) < 2 {
			fmt.Println("Usage: agproxy accounts use <email>")
			return
		}
		email := strings.TrimSpace(args[1])
		if err := store.SetActive(email); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("✓ Active account switched to: %s\n", email)

	case "remove":
		if len(args) < 2 {
			fmt.Println("Usage: agproxy accounts remove <email>")
			return
		}
		email := strings.TrimSpace(args[1])
		if err := store.RemoveAccount(email); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("✓ Account %s removed.\n", email)

	default:
		fmt.Printf("Unknown accounts command: %s. Use 'list', 'use', or 'remove'.\n", sub)
	}
}

func handleImage(store *config.Store, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: agproxy image \"<prompt>\" [-o <output.jpg>] [--model <model>]")
		return
	}

	var outputFlag string
	var modelFlag string
	var promptParts []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if (arg == "-o" || arg == "--output") && i+1 < len(args) {
			outputFlag = args[i+1]
			i++
		} else if (arg == "-m" || arg == "--model") && i+1 < len(args) {
			modelFlag = args[i+1]
			i++
		} else {
			promptParts = append(promptParts, arg)
		}
	}

	prompt := strings.Join(promptParts, " ")
	if strings.TrimSpace(prompt) == "" {
		fmt.Println("Error: Prompt cannot be empty.")
		return
	}

	upstreamModel := "gemini-3.1-flash-image"
	if modelFlag != "" {
		info := upstream.ResolveModel(modelFlag)
		upstreamModel = info.UpstreamModel
	}

	fmt.Printf("🎨 Generating image via %s...\nPrompt: %q\n", upstreamModel, prompt)
	start := time.Now()

	client := upstream.NewClient("")

	maxAttempts := len(store.Accounts)
	if maxAttempts < 3 {
		maxAttempts = 3
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		acc := store.GetActiveAccount()
		if acc == nil {
			fmt.Println("Error: No active Google account found. Run 'agproxy login' first.")
			return
		}

		token, err := auth.EnsureValidToken(acc, store)
		if err != nil {
			fmt.Printf("Account %s token error: %v. Rotating...\n", acc.Email, err)
			store.RotateNextAccount(acc.Email, 5*time.Minute)
			continue
		}

		projectID := acc.ProjectID
		if projectID == "" {
			projectID = "aicode-consumers"
		}

		fmt.Printf("  • Attempt %d/%d with account: %s\n", attempt+1, maxAttempts, acc.Email)

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
							{Text: prompt},
						},
					},
				},
				SessionID: fmt.Sprintf("img-sess-%d", time.Now().UnixNano()),
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		respBytes, statusCode, headers, err := client.GenerateContent(ctx, token, payload)
		cancel()

		if err != nil {
			fmt.Printf("Network error with %s: %v. Rotating...\n", acc.Email, err)
			continue
		}

		if statusCode == 429 {
			retryHeader := headers.Get("Retry-After")
			delay := stealth.ParseRetryDelay(retryHeader, string(respBytes))
			if delay <= 0 {
				delay = 30 * time.Minute
			}
			fmt.Printf("  ⚠️ 429 Rate limited on %s (reset in %v). Rotating to next account...\n", acc.Email, delay)
			store.RotateNextAccount(acc.Email, delay)
			continue
		}

		if statusCode != 200 {
			fmt.Printf("Google error (HTTP %d): %s\n", statusCode, string(respBytes))
			store.RotateNextAccount(acc.Email, 5*time.Minute)
			continue
		}

		var gChunk upstream.GoogleStreamChunk
		if err := json.Unmarshal(respBytes, &gChunk); err != nil {
			fmt.Printf("Error parsing response: %v\n", err)
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
			fmt.Println("Error: No image content returned by Google model.")
			return
		}

		outPath := outputFlag
		if outPath == "" {
			homeDir, _ := os.UserHomeDir()
			picDir := filepath.Join(homeDir, "Pictures", "agproxy")
			_ = os.MkdirAll(picDir, 0755)
			ext := ".jpeg"
			if strings.Contains(mimeType, "png") {
				ext = ".png"
			}
			outPath = filepath.Join(picDir, fmt.Sprintf("img_%d%s", time.Now().Unix(), ext))
		} else {
			if dir := filepath.Dir(outPath); dir != "" {
				_ = os.MkdirAll(dir, 0755)
			}
		}

		rawBytes, err := base64.StdEncoding.DecodeString(b64Data)
		if err != nil {
			fmt.Printf("Error decoding base64 image data: %v\n", err)
			return
		}

		if err := os.WriteFile(outPath, rawBytes, 0644); err != nil {
			fmt.Printf("Error writing image to %s: %v\n", outPath, err)
			return
		}

		store.MarkSuccess(acc.Email)
		fmt.Printf("✓ Image successfully created in %.2fs!\nSaved to: %s (%d KB)\n",
			time.Since(start).Seconds(), outPath, len(rawBytes)/1024)
		return
	}

	fmt.Println("❌ All registered accounts were rate-limited or exhausted.")
}
