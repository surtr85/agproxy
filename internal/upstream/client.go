package upstream

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

const (
	BaseURL   = "https://daily-cloudcode-pa.googleapis.com"
	UserAgent = "antigravity/ide/2.11.0 darwin/arm64"
)

type Client struct {
	httpClient *http.Client
}

func NewClient(proxyURL string) *Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Minute, // long timeout for complex reasoning/large tasks
		},
	}
}

func (c *Client) StreamGenerateContent(ctx context.Context, accessToken string, payload *AntigravityRequestWrapper) (io.ReadCloser, int, http.Header, error) {
	reqURL := fmt.Sprintf("%s/v1internal:streamGenerateContent?alt=sse", BaseURL)

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}

	return resp.Body, resp.StatusCode, resp.Header, nil
}

func (c *Client) GenerateContent(ctx context.Context, accessToken string, payload *AntigravityRequestWrapper) ([]byte, int, http.Header, error) {
	reqURL := fmt.Sprintf("%s/v1internal:generateContent", BaseURL)

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, resp.Header, err
	}

	return data, resp.StatusCode, resp.Header, nil
}
