package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	http  *http.Client
	token string
}

func New(token string) *Client {
	return &Client{http: &http.Client{Timeout: 10 * time.Second}, token: token}
}

func NewWithHTTPClient(client *http.Client, token string) *Client {
	return &Client{http: client, token: token}
}

func (c *Client) Stars(ctx context.Context, repo string) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repo, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, fmt.Errorf("github returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var body struct {
		Stars float64 `json:"stargazers_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}
	return body.Stars, nil
}
