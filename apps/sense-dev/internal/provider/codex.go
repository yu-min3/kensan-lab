package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

var (
	ErrAuthRequired = errors.New("subscription authentication required")
	ErrQuotaWait    = errors.New("subscription quota wait")
	ErrModelChanged = errors.New("provider rerouted requested model")
)

type Request struct {
	Model           string
	Workdir         string
	Prompt          string
	ExistingSession string
	OnSession       func(string) error // persist before starting the turn
}

type Result struct {
	SessionID string
	Text      string
	Status    string
}

type Codex struct {
	Binary   string
	AuthHome string
	MaxUsage float64
}

func (c Codex) Run(ctx context.Context, req Request) (Result, error) {
	if c.Binary == "" || c.AuthHome == "" || req.Model == "" || req.Workdir == "" || req.Prompt == "" || req.OnSession == nil {
		return Result{}, errors.New("Codex binary, auth home, model, workdir, prompt and session callback required")
	}
	if c.MaxUsage <= 0 || c.MaxUsage > 100 {
		return Result{}, errors.New("Codex quota threshold required")
	}
	cmd := exec.CommandContext(ctx, c.Binary, "app-server")
	cmd.Dir = req.Workdir
	// No API key can reach the child. CODEX_HOME must be a dedicated directory
	// authenticated by the user with the official ChatGPT flow.
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "CODEX_HOME=" + c.AuthHome}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return Result{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return Result{}, err
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()
	return runCodexProtocol(ctx, stdin, stdout, req, c.MaxUsage)
}

type rpcMessage struct {
	ID     *int            `json:"id"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
	Params json.RawMessage `json:"params"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func runCodexProtocol(ctx context.Context, input io.Writer, output io.Reader, req Request, maxUsage float64) (Result, error) {
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	stream := make(chan rpcMessage, 64)
	readErr := make(chan error, 1)
	buffered := make([]rpcMessage, 0, 8)
	go func() {
		defer close(stream)
		for scanner.Scan() {
			var msg rpcMessage
			if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
				readErr <- fmt.Errorf("invalid app-server JSON: %w", err)
				return
			}
			select {
			case stream <- msg:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			readErr <- err
		} else {
			readErr <- io.EOF
		}
	}()
	send := func(id int, method string, params any) error {
		payload := map[string]any{"method": method, "params": params}
		if id >= 0 {
			payload["id"] = id
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		_, err = input.Write(append(b, '\n'))
		return err
	}
	wait := func(id int) (rpcMessage, error) {
		for {
			select {
			case <-ctx.Done():
				return rpcMessage{}, ctx.Err()
			case msg, ok := <-stream:
				if !ok {
					return rpcMessage{}, <-readErr
				}
				if msg.Error != nil && msg.ID != nil && *msg.ID == id {
					return rpcMessage{}, fmt.Errorf("app-server request %d failed: %s", id, msg.Error.Message)
				}
				if msg.Method != "" && msg.ID != nil {
					return rpcMessage{}, errors.New("unexpected app-server request requires human handling")
				}
				if msg.ID != nil && *msg.ID == id {
					return msg, nil
				}
				if msg.Method == "model/rerouted" {
					return rpcMessage{}, ErrModelChanged
				}
				if msg.Method == "item/completed" || msg.Method == "turn/completed" {
					buffered = append(buffered, msg)
				}
			}
		}
	}
	if err := send(1, "initialize", map[string]any{"clientInfo": map[string]string{"name": "kensan_sense_dev", "title": "Kensan Sense Dev", "version": "0.1.0"}}); err != nil {
		return Result{}, err
	}
	if _, err := wait(1); err != nil {
		return Result{}, err
	}
	if err := send(-1, "initialized", map[string]any{}); err != nil {
		return Result{}, err
	}
	if err := send(2, "account/read", map[string]any{"refreshToken": false}); err != nil {
		return Result{}, err
	}
	account, err := wait(2)
	if err != nil {
		return Result{}, err
	}
	var auth struct {
		Account *struct {
			Type string `json:"type"`
		} `json:"account"`
	}
	if err := json.Unmarshal(account.Result, &auth); err != nil || auth.Account == nil || auth.Account.Type != "chatgpt" {
		return Result{}, ErrAuthRequired
	}
	if err := send(3, "account/rateLimits/read", map[string]any{}); err != nil {
		return Result{}, err
	}
	limits, err := wait(3)
	if err != nil {
		return Result{}, err
	}
	type window struct {
		UsedPercent float64 `json:"usedPercent"`
	}
	type bucket struct {
		Primary              *window `json:"primary"`
		Secondary            *window `json:"secondary"`
		RateLimitReachedType *string `json:"rateLimitReachedType"`
	}
	var usage struct {
		RateLimits          *bucket           `json:"rateLimits"`
		RateLimitsByLimitID map[string]bucket `json:"rateLimitsByLimitId"`
	}
	if err := json.Unmarshal(limits.Result, &usage); err != nil || usage.RateLimits == nil || usage.RateLimits.Primary == nil {
		return Result{}, ErrQuotaWait
	}
	checkBucket := func(b bucket) bool {
		return b.Primary == nil || b.Primary.UsedPercent >= maxUsage || b.Secondary != nil && b.Secondary.UsedPercent >= maxUsage || b.RateLimitReachedType != nil
	}
	if checkBucket(*usage.RateLimits) {
		return Result{}, ErrQuotaWait
	}
	for _, b := range usage.RateLimitsByLimitID {
		if checkBucket(b) {
			return Result{}, ErrQuotaWait
		}
	}
	var threadID string
	if req.ExistingSession != "" {
		threadID = req.ExistingSession
		if err := send(4, "thread/resume", map[string]any{"threadId": threadID, "model": req.Model, "cwd": req.Workdir, "approvalPolicy": "never", "sandbox": "workspaceWrite"}); err != nil {
			return Result{}, err
		}
	} else {
		if err := send(4, "thread/start", map[string]any{"model": req.Model, "cwd": req.Workdir, "approvalPolicy": "never", "sandbox": "workspaceWrite", "serviceName": "kensan_sense_dev"}); err != nil {
			return Result{}, err
		}
	}
	thread, err := wait(4)
	if err != nil {
		return Result{}, err
	}
	var threadResult struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(thread.Result, &threadResult); err != nil || threadResult.Thread.ID == "" {
		return Result{}, errors.New("app-server returned no thread ID")
	}
	if threadID != "" && threadID != threadResult.Thread.ID {
		return Result{}, errors.New("resumed wrong Codex thread")
	}
	threadID = threadResult.Thread.ID
	if err := req.OnSession(threadID); err != nil {
		return Result{}, err
	}
	if err := send(5, "turn/start", map[string]any{"threadId": threadID, "input": []map[string]string{{"type": "text", "text": req.Prompt}}, "cwd": req.Workdir, "model": req.Model, "approvalPolicy": "never", "sandboxPolicy": map[string]any{"type": "workspaceWrite", "writableRoots": []string{req.Workdir}, "networkAccess": false}}); err != nil {
		return Result{}, err
	}
	if _, err := wait(5); err != nil {
		return Result{}, err
	}
	result := Result{SessionID: threadID}
	for {
		var msg rpcMessage
		if len(buffered) > 0 {
			msg, buffered = buffered[0], buffered[1:]
		} else {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case next, ok := <-stream:
				if !ok {
					return result, <-readErr
				}
				msg = next
			}
		}
		if msg.Method != "" && msg.ID != nil {
			return result, errors.New("app-server requested an unhandled privileged action")
		}
		if msg.Method == "model/rerouted" {
			return result, ErrModelChanged
		}
		if msg.Method == "item/completed" {
			var item struct {
				Item struct {
					Type  string `json:"type"`
					Text  string `json:"text"`
					Phase string `json:"phase"`
				} `json:"item"`
			}
			if json.Unmarshal(msg.Params, &item) == nil && item.Item.Type == "agentMessage" && (item.Item.Phase == "final_answer" || item.Item.Phase == "") {
				result.Text = item.Item.Text
			}
		}
		if msg.Method == "turn/completed" {
			var done struct {
				Turn struct {
					Status string `json:"status"`
				} `json:"turn"`
			}
			if err := json.Unmarshal(msg.Params, &done); err != nil {
				return result, err
			}
			result.Status = done.Turn.Status
			if result.Status != "completed" {
				return result, fmt.Errorf("Codex turn ended: %s", result.Status)
			}
			return result, nil
		}
	}
}

func WithTimeout(parent context.Context, duration time.Duration) (context.Context, context.CancelFunc) {
	if duration <= 0 || duration > 45*time.Minute {
		duration = 20 * time.Minute
	}
	return context.WithTimeout(parent, duration)
}

func Classify(err error) string {
	if err == nil {
		return "success"
	}
	if errors.Is(err, ErrAuthRequired) {
		return "auth_required"
	}
	if errors.Is(err, ErrQuotaWait) {
		return "quota_wait"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "retry_wait"
	}
	if errors.Is(err, ErrModelChanged) {
		return "failed"
	}
	if strings.Contains(strings.ToLower(err.Error()), "rate limit") || strings.Contains(strings.ToLower(err.Error()), "quota") {
		return "quota_wait"
	}
	return "failed"
}
