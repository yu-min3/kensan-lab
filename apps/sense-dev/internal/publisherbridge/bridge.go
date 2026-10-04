// Package publisherbridge keeps GitHub credentials in a separate host process.
// The controller remains the only ledger writer; model workers cannot reach the
// Unix socket or either authentication file.
package publisherbridge

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

type Client struct{ Socket, AuthFile string }
type request struct {
	Action string             `json:"action"`
	TaskID string             `json:"task_id,omitempty"`
	Intent core.PublishIntent `json:"intent"`
}
type response struct {
	ExternalID  string                  `json:"external_id"`
	Exists      bool                    `json:"exists"`
	Observation *core.DeploymentReceipt `json:"observation,omitempty"`
}

func readAuth(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !filepath.IsAbs(path) || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 4096 {
		return "", errors.New("publisher bridge auth must be a private regular file")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", errors.New("publisher bridge auth unavailable")
	}
	token := strings.TrimSpace(string(b))
	if len(token) < 32 || strings.ContainsAny(token, "\r\n") {
		return "", errors.New("publisher bridge auth must contain a strong single-line secret")
	}
	return token, nil
}
func validSocket(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("absolute publisher socket required")
	}
	parent := filepath.Dir(path)
	canonical, err := filepath.EvalSymlinks(parent)
	if err != nil || canonical != parent {
		return errors.New("publisher socket directory cannot contain symlinks")
	}
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		return errors.New("publisher socket directory must not be group or world writable")
	}
	return nil
}
func (c Client) call(ctx context.Context, action string, intent core.PublishIntent, taskID ...string) (response, error) {
	var result response
	if err := validSocket(c.Socket); err != nil {
		return result, err
	}
	info, err := os.Lstat(c.Socket)
	if err != nil || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm()&0007 != 0 {
		return result, errors.New("private publisher socket unavailable")
	}
	token, err := readAuth(c.AuthFile)
	if err != nil {
		return result, err
	}
	input := request{Action: action, Intent: intent}
	if len(taskID) > 0 {
		input.TaskID = taskID[0]
	}
	b, err := json.Marshal(input)
	if err != nil {
		return result, err
	}
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", c.Socket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 2 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("publisher redirect forbidden") }}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://publisher.local/v1/publish", bytes.NewReader(b))
	if err != nil {
		return result, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return result, errors.New("publisher bridge result unknown; inspect before retry")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return result, errors.New("publisher bridge rejected request; inspect before retry")
	}
	dec := json.NewDecoder(io.LimitReader(res.Body, 8192))
	dec.DisallowUnknownFields()
	if dec.Decode(&result) != nil {
		return result, errors.New("publisher bridge returned invalid result")
	}
	return result, nil
}
func (c Client) Inspect(ctx context.Context, i core.PublishIntent) (string, bool, error) {
	r, e := c.call(ctx, "inspect", i)
	return r.ExternalID, r.Exists, e
}
func (c Client) Execute(ctx context.Context, i core.PublishIntent) (string, error) {
	r, e := c.call(ctx, "execute", i)
	return r.ExternalID, e
}

type ObservationFunc func(context.Context, string, string) (core.DeploymentReceipt, error)

// Observe receives host evidence only through the authenticated publisher socket.
func (c Client) Observe(ctx context.Context, i core.PublishIntent, taskID string) (core.DeploymentReceipt, error) {
	r, err := c.call(ctx, "observe", i, taskID)
	if err != nil {
		return core.DeploymentReceipt{}, err
	}
	if r.Observation == nil {
		return core.DeploymentReceipt{}, errors.New("host observation unavailable")
	}
	return *r.Observation, nil
}

// Handler trusts only the host controller holding the dedicated bridge secret.
// It owns no Store and exposes no model-facing HTTP route.
func Handler(authFile string, transport core.PublishTransport) (http.Handler, error) {
	return HandlerWithObserver(authFile, transport, nil)
}

func HandlerWithObserver(authFile string, transport core.PublishTransport, observe ObservationFunc) (http.Handler, error) {
	auth, err := readAuth(authFile)
	if err != nil {
		return nil, err
	}
	if transport == nil {
		return nil, errors.New("publisher transport required")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reject := func(code int) { http.Error(w, "publisher request rejected", code) }
		if r.Method != http.MethodPost || r.URL.Path != "/v1/publish" {
			reject(http.StatusNotFound)
			return
		}
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), []byte(auth)) != 1 {
			reject(http.StatusUnauthorized)
			return
		}
		var input request
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		dec.DisallowUnknownFields()
		if dec.Decode(&input) != nil || dec.Decode(new(any)) != io.EOF {
			reject(http.StatusBadRequest)
			return
		}
		i := input.Intent
		if input.Action != "inspect" && input.Action != "execute" && input.Action != "observe" || i.ID == "" || i.DecisionID == "" || i.Repository != "yu-min3/kensan-lab" || i.PolicyVersion != core.ReleasePolicyVersion || !strings.HasPrefix(i.Ref, "refs/heads/") || i.HeadSHA == "" || (i.Operation != "branch_push" && i.Operation != "pr_create" && i.Operation != "merge" && i.Operation != "deploy") {
			reject(http.StatusForbidden)
			return
		}
		if (i.Operation == "merge" || i.Operation == "deploy") && i.TargetEnvironment != "private-canary" {
			reject(http.StatusForbidden)
			return
		}
		var result response
		var err error
		if input.Action == "observe" {
			if observe == nil || input.TaskID == "" || i.Status != "sent" || (i.Operation != "merge" && i.Operation != "deploy") || i.ExternalID == "" {
				reject(http.StatusForbidden)
				return
			}
			revision, exists, e := transport.Inspect(r.Context(), i)
			if e != nil || !exists || revision != i.ExternalID {
				reject(http.StatusBadGateway)
				return
			}
			receipt, e := observe(r.Context(), i.HeadSHA, revision)
			if e != nil {
				reject(http.StatusBadGateway)
				return
			}
			receipt.TaskID, receipt.DecisionID, receipt.IntentID = input.TaskID, i.DecisionID, i.ID
			result.Observation = &receipt
		} else if input.Action == "execute" {
			if i.ExpiresAt.IsZero() || !time.Now().Before(i.ExpiresAt) {
				reject(http.StatusForbidden)
				return
			}
			result.ExternalID, err = transport.Execute(r.Context(), i)
			result.Exists = err == nil && result.ExternalID != ""
		} else {
			result.ExternalID, result.Exists, err = transport.Inspect(r.Context(), i)
		}
		if err != nil || result.Exists && result.ExternalID == "" {
			reject(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}), nil
}

// Listen refuses existing paths. A stale socket after a crash needs explicit
// host reconciliation, avoiding deletion of another publisher's socket.
func Listen(path string, group int) (net.Listener, error) {
	if err := validSocket(path); err != nil {
		return nil, err
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return nil, errors.New("publisher socket already exists; reconcile its owner")
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	mode := os.FileMode(0600)
	if group >= 0 {
		if err = os.Chown(path, -1, group); err != nil {
			_ = listener.Close()
			return nil, err
		}
		mode = 0660
	}
	if err = os.Chmod(path, mode); err != nil {
		_ = listener.Close()
		return nil, err
	}
	return listener, nil
}
