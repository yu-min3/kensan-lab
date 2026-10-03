package reportdelivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

// Slack sends one immutable outbox entry to a fixed channel. The token is
// loaded only by the controller sender process and is never logged.
type Slack struct {
	TokenFile string
	BaseURL   string
	Client    *http.Client
	Endpoint  string // test override
}

func (s Slack) Validate() error {
	if s.TokenFile == "" {
		return errors.New("Slack token file required")
	}
	info, err := os.Stat(s.TokenFile)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return errors.New("Slack token file must be private and regular")
	}
	u, err := url.Parse(s.BaseURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("HTTPS report link base required")
	}
	if s.Endpoint != "" && !strings.HasPrefix(s.Endpoint, "http://127.0.0.1:") && s.Endpoint != "https://slack.com/api/chat.postMessage" {
		return errors.New("untrusted Slack endpoint")
	}
	return nil
}

func (s Slack) Send(ctx context.Context, entry core.ReportOutboxEntry) (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	if entry.Destination == "" || entry.Status != "sending" {
		return "", errors.New("outbox entry is not ready to send")
	}
	tokenBytes, err := os.ReadFile(s.TokenFile)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return "", errors.New("empty Slack token")
	}
	base := strings.TrimRight(s.BaseURL, "/")
	lines := []string{"sense 日報 " + entry.Date, entry.Summary}
	for _, id := range entry.TaskIDs {
		lines = append(lines, "• <"+base+"/tasks/"+url.PathEscape(id)+"|案件 "+id+">")
	}
	for _, id := range entry.PendingIDs {
		lines = append(lines, "• <"+base+"/#"+url.PathEscape(id)+"|判断 "+id+">")
	}
	message := map[string]any{"channel": entry.Destination, "text": strings.Join(lines, "\n"), "unfurl_links": false, "unfurl_media": false}
	body, err := json.Marshal(message)
	if err != nil {
		return "", err
	}
	endpoint := s.Endpoint
	if endpoint == "" {
		endpoint = "https://slack.com/api/chat.postMessage"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Slack redirect rejected") }}
	}
	response, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Slack returned HTTP %d", response.StatusCode)
	}
	var result struct {
		OK    bool   `json:"ok"`
		TS    string `json:"ts"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return "", err
	}
	if !result.OK || result.TS == "" {
		return "", errors.New("Slack did not provide delivery receipt")
	}
	return result.TS, nil
}
