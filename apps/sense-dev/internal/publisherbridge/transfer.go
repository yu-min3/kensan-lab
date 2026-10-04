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
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/committransfer"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

const transferBodyLimit = committransfer.MaxBundleBytes*4/3 + (64 << 10)

type transferRequest struct {
	Intent core.PublishIntent     `json:"intent"`
	TaskID string                 `json:"task_id,omitempty"`
	Bundle *committransfer.Bundle `json:"bundle,omitempty"`
}
type transferResponse struct {
	SHA256 string                 `json:"sha256,omitempty"`
	Bundle *committransfer.Bundle `json:"bundle,omitempty"`
}
type Transfers struct {
	RepoPath string
	Export   func(context.Context, string, string, string) (committransfer.Bundle, error)
}

func (c Client) transfer(ctx context.Context, path string, input transferRequest) (transferResponse, error) {
	var result transferResponse
	if err := validSocket(c.Socket); err != nil {
		return result, err
	}
	info, err := os.Lstat(c.Socket)
	if err != nil || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm()&0007 != 0 {
		return result, errors.New("private publisher socket unavailable")
	}
	// Reuse normal socket validation before constructing the bounded transfer.
	token, err := readAuth(c.AuthFile)
	if err != nil {
		return result, err
	}
	body, err := json.Marshal(input)
	if err != nil || len(body) > transferBodyLimit {
		return result, errors.New("invalid commit transfer request")
	}
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", c.Socket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 2 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("commit transfer redirect forbidden") }}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://publisher.local"+path, bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return result, errors.New("commit transfer unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return result, errors.New("commit transfer rejected")
	}
	dec := json.NewDecoder(io.LimitReader(res.Body, transferBodyLimit+1))
	dec.DisallowUnknownFields()
	if dec.Decode(&result) != nil || dec.Decode(new(any)) != io.EOF {
		return result, errors.New("invalid commit transfer response")
	}
	return result, nil
}
func (c Client) ImportCommit(ctx context.Context, i core.PublishIntent, b committransfer.Bundle) error {
	if err := b.Validate(); err != nil {
		return err
	}
	r, err := c.transfer(ctx, "/v1/commits", transferRequest{Intent: i, Bundle: &b})
	if err != nil {
		return err
	}
	if r.SHA256 != b.SHA256 {
		return errors.New("commit transfer acknowledgement differs")
	}
	return nil
}
func (c Client) ExportMerged(ctx context.Context, i core.PublishIntent, taskID string) (committransfer.Bundle, error) {
	r, err := c.transfer(ctx, "/v1/merged", transferRequest{Intent: i, TaskID: taskID})
	if err != nil {
		return committransfer.Bundle{}, err
	}
	if r.Bundle == nil || r.Bundle.Validate() != nil || r.Bundle.TaskID != taskID || r.Bundle.BaseSHA != i.BaseSHA || r.Bundle.HeadSHA != i.ExternalID {
		return committransfer.Bundle{}, errors.New("merged commit binding differs")
	}
	return *r.Bundle, nil
}

func HandlerWithTransfers(authFile string, transport core.PublishTransport, observe ObservationFunc, transfers Transfers, imageEvidence ...ImageEvidenceFunc) (http.Handler, error) {
	var evidence ImageEvidenceFunc
	if len(imageEvidence) > 0 {
		evidence = imageEvidence[0]
	}
	base, err := HandlerWithImageEvidence(authFile, transport, observe, evidence)
	if err != nil {
		return nil, err
	}
	auth, err := readAuth(authFile)
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/commits" && r.URL.Path != "/v1/merged" {
			base.ServeHTTP(w, r)
			return
		}
		reject := func(code int) { http.Error(w, "commit transfer rejected", code) }
		if r.Method != http.MethodPost {
			reject(http.StatusMethodNotAllowed)
			return
		}
		got := r.Header.Get("Authorization")
		if subtle.ConstantTimeCompare([]byte(got), []byte("Bearer "+auth)) != 1 {
			reject(http.StatusUnauthorized)
			return
		}
		var input transferRequest
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, transferBodyLimit))
		dec.DisallowUnknownFields()
		if dec.Decode(&input) != nil || dec.Decode(new(any)) != io.EOF {
			reject(http.StatusBadRequest)
			return
		}
		i := input.Intent
		if i.ID == "" || i.DecisionID == "" || i.Repository != "yu-min3/kensan-lab" || i.PolicyVersion != core.ReleasePolicyVersion || i.TargetEnvironment != "private-canary" {
			reject(http.StatusForbidden)
			return
		}
		var result transferResponse
		if r.URL.Path == "/v1/commits" {
			b := input.Bundle
			if b == nil || b.Validate() != nil || i.Operation != "branch_push" || i.Ref != "refs/heads/sense-dev/"+b.TaskID || i.BaseSHA != b.BaseSHA || i.HeadSHA != b.HeadSHA || i.ExpiresAt.IsZero() || !time.Now().Before(i.ExpiresAt) || transfers.RepoPath == "" {
				reject(http.StatusForbidden)
				return
			}
			if err := committransfer.Import(r.Context(), transfers.RepoPath, *b); err != nil {
				reject(http.StatusBadGateway)
				return
			}
			result.SHA256 = b.SHA256
		} else {
			if input.Bundle != nil || input.TaskID == "" || i.Ref != "refs/heads/sense-dev/"+input.TaskID || i.Status != "sent" || (i.Operation != "merge" && i.Operation != "deploy") || i.ExternalID == "" || transfers.Export == nil {
				reject(http.StatusForbidden)
				return
			}
			revision, exists, err := transport.Inspect(r.Context(), i)
			if err != nil || !exists || revision != i.ExternalID {
				reject(http.StatusBadGateway)
				return
			}
			b, err := transfers.Export(r.Context(), input.TaskID, i.BaseSHA, revision)
			if err != nil || b.Validate() != nil || b.TaskID != input.TaskID || b.BaseSHA != i.BaseSHA || b.HeadSHA != revision {
				reject(http.StatusBadGateway)
				return
			}
			result.Bundle = &b
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}), nil
}
