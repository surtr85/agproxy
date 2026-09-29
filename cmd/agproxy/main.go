package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/surtr85/agproxy/internal/auth"
	"github.com/surtr85/agproxy/internal/config"
	"github.com/surtr85/agproxy/internal/quota"
	"github.com/surtr85/agproxy/internal/server"
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
  accounts    Manage saved Google accounts (list, use, remove)
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
