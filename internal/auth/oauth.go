package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/surtr85/agproxy/internal/config"
)

var (
	cidMasked  = []byte{107, 106, 109, 107, 106, 106, 108, 106, 108, 106, 111, 99, 107, 119, 46, 55, 50, 41, 41, 51, 52, 104, 50, 104, 107, 54, 57, 40, 63, 104, 105, 111, 44, 46, 53, 54, 53, 48, 50, 110, 61, 110, 106, 105, 63, 42, 116, 59, 42, 42, 41, 116, 61, 53, 53, 61, 54, 63, 47, 41, 63, 40, 57, 53, 52, 46, 63, 52, 46, 116, 57, 53, 55}
	csecMasked = []byte{29, 21, 25, 9, 10, 2, 119, 17, 111, 98, 28, 13, 8, 110, 98, 108, 22, 62, 22, 16, 107, 55, 22, 24, 98, 41, 2, 25, 110, 32, 108, 43, 30, 27, 60}
)

func unmask(b []byte, key byte) string {
	res := make([]byte, len(b))
	for i, v := range b {
		res[i] = v ^ key
	}
	return string(res)
}

// GetClientID returns the OAuth Client ID from env or the default Antigravity client ID
func GetClientID() string {
	if v := os.Getenv("AGPROXY_CLIENT_ID"); v != "" {
		return v
	}
	return unmask(cidMasked, 0x5A)
}

// GetClientSecret returns the OAuth Client Secret from env or the default Antigravity client secret
func GetClientSecret() string {
	if v := os.Getenv("AGPROXY_CLIENT_SECRET"); v != "" {
		return v
	}
	return unmask(csecMasked, 0x5A)
}

const (
	AuthURL      = "https://accounts.google.com/o/oauth2/v2/auth"
	TokenURL     = "https://oauth2.googleapis.com/token"
	UserInfoURL  = "https://www.googleapis.com/oauth2/v1/userinfo"
	UserAgent    = "antigravity/ide/2.11.0 darwin/arm64"
)

var Scopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
	"https://www.googleapis.com/auth/cclog",
	"https://www.googleapis.com/auth/experimentsandconfigs",
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

type UserInfo struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func GenerateState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func BuildAuthURL(redirectURI, state string) string {
	params := url.Values{}
	params.Set("client_id", GetClientID())
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", strings.Join(Scopes, " "))
	params.Set("state", state)
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")

	return fmt.Sprintf("%s?%s", AuthURL, params.Encode())
}

func ExchangeCode(code, redirectURI string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", GetClientID())
	data.Set("client_secret", GetClientSecret())
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest("POST", TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

func RefreshToken(refreshToken string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", GetClientID())
	data.Set("client_secret", GetClientSecret())
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequest("POST", TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

func FetchUserInfo(accessToken string) (*UserInfo, error) {
	req, err := http.NewRequest("GET", UserInfoURL+"?alt=json", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", UserAgent)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("user info failed (%d): %s", resp.StatusCode, string(body))
	}

	var info UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &info, nil
}

func EnsureValidToken(acc *config.Account, store *config.Store) (string, error) {
	// If expires within 2 minutes or expired, refresh it
	if time.Until(acc.ExpiresAt) < 2*time.Minute {
		if acc.RefreshToken == "" {
			return acc.AccessToken, fmt.Errorf("no refresh token available for %s", acc.Email)
		}
		newTokens, err := RefreshToken(acc.RefreshToken)
		if err != nil {
			return acc.AccessToken, fmt.Errorf("failed to refresh token for %s: %w", acc.Email, err)
		}

		acc.AccessToken = newTokens.AccessToken
		if newTokens.RefreshToken != "" {
			acc.RefreshToken = newTokens.RefreshToken
		}
		acc.ExpiresAt = time.Now().Add(time.Duration(newTokens.ExpiresIn) * time.Second)
		_ = store.AddOrUpdateAccount(acc)
	}

	return acc.AccessToken, nil
}

// StartOAuthServer starts a temporary local server to handle OAuth redirect
func StartOAuthServer(port int, state string) (string, error) {
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return "", err
	}

	server := &http.Server{}
	http.HandleFunc("/oauth2callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
			errChan <- fmt.Errorf("invalid OAuth state mismatch")
			return
		}
		code := q.Get("code")
		if code == "" {
			errMsg := q.Get("error")
			http.Error(w, "Authentication error: "+errMsg, http.StatusBadRequest)
			errChan <- fmt.Errorf("oauth error: %s", errMsg)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`
			<!DOCTYPE html>
			<html>
			<head><title>Authentication Successful</title></head>
			<body style="font-family: sans-serif; text-align: center; padding: 50px; background: #0f172a; color: #f8fafc;">
				<h1 style="color: #38bdf8;">✓ Google Antigravity Authorization Successful!</h1>
				<p>You can close this window now and return to your terminal.</p>
			</body>
			</html>
		`))

		codeChan <- code
	})

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case code := <-codeChan:
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		return code, nil
	case err := <-errChan:
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		return "", err
	case <-time.After(5 * time.Minute):
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		return "", fmt.Errorf("oauth callback timed out after 5 minutes")
	}
}
