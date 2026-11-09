// Package catalog executes a fixed, embedded DuckDB bridge with scoped identity.
package catalog

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/yu-min3/kensan-lab/apps/llmops/internal/worker"
)

//go:embed bridge.py sql/konro.sql
var sources embed.FS
var ErrCatalog = errors.New("catalog operation failed")
var identifier = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

type Row struct {
	RecordID         string          `json:"record_id"`
	AppID            string          `json:"app_id"`
	CaptureID        string          `json:"capture_id"`
	RawObjectID      string          `json:"raw_object_id"`
	RawSHA256        string          `json:"raw_sha256"`
	SourceURI        string          `json:"source_uri"`
	SourceGroup      string          `json:"source_group"`
	SchemaVersion    string          `json:"schema_version"`
	TransformVersion string          `json:"transform_version"`
	AdapterSHA256    string          `json:"adapter_sha256"`
	PreparedText     string          `json:"prepared_text"`
	TextBytes        int             `json:"text_bytes"`
	TextTruncated    bool            `json:"text_truncated"`
	Content          json.RawMessage `json:"content"`
	OutputSchema     json.RawMessage `json:"output_schema"`
	Warnings         []string        `json:"warnings"`
}
type Record struct {
	RecordID  string `json:"record_id"`
	CaptureID string `json:"capture_id"`
	RawSHA256 string `json:"raw_sha256"`
	Title     string `json:"title"`
}
type Result struct {
	SnapshotID string   `json:"snapshot_id"`
	Records    []Record `json:"records"`
}

func Validate(c *worker.CatalogConfig, app, role string) error {
	if c == nil || c.Name != app || !identifier.MatchString(c.Namespace) || !identifier.MatchString(c.Table) || c.ClientID != "catalog-"+app+"-"+role || !filepath.IsAbs(c.PythonBinary) || !filepath.IsAbs(c.CAFile) || filepath.Base(c.ClientSecretFile) != c.ClientID+".secret" || !filepath.IsAbs(c.ClientSecretFile) {
		return worker.ErrConfig
	}
	u, e := url.Parse(c.URI)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return worker.ErrConfig
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && (u.Scheme != "http" || ip == nil || !ip.IsLoopback()) {
		return worker.ErrConfig
	}
	u, e = url.Parse(c.TokenURL)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return worker.ErrConfig
	}
	return nil
}

type limited struct {
	buf      bytes.Buffer
	max      int
	cancel   context.CancelFunc
	overflow bool
}

func (w *limited) Write(b []byte) (int, error) {
	if w.buf.Len()+len(b) > w.max {
		w.overflow = true
		w.cancel()
		return 0, ErrCatalog
	}
	return w.buf.Write(b)
}
func Commit(ctx context.Context, r *worker.Runner, c *worker.CatalogConfig, rows []Row) (Result, error) {
	var result Result
	if r.Role() != "transform" || len(rows) == 0 || len(rows) > 100 || Validate(c, r.App(), r.Role()) != nil {
		return result, worker.ErrConfig
	}
	info, e := os.Lstat(c.ClientSecretFile)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 16384 {
		return result, worker.ErrConfig
	}
	secret, e := os.ReadFile(c.ClientSecretFile)
	if e != nil || strings.TrimSpace(string(secret)) == "" {
		return result, worker.ErrConfig
	}
	for _, row := range rows {
		if row.AppID != r.App() {
			return result, worker.ErrRequest
		}
	}
	ddl, _ := sources.ReadFile("sql/konro.sql")
	script, _ := sources.ReadFile("bridge.py")
	input, e := json.Marshal(map[string]any{"operation": "curate", "catalog": c, "client_secret": strings.TrimSpace(string(secret)), "ddl": string(ddl), "rows": rows})
	if e != nil || len(input) > 32<<20 {
		return result, worker.ErrRequest
	}
	e = r.WithWorkspace(func(dir string) error {
		file := filepath.Join(dir, "catalog.py")
		if e := os.WriteFile(file, script, 0600); e != nil {
			return ErrCatalog
		}
		child, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		stdout := &limited{max: 1 << 20, cancel: cancel}
		stderr := &limited{max: 4096, cancel: cancel}
		cmd := exec.CommandContext(child, c.PythonBinary, file)
		cmd.Stdin = bytes.NewReader(input)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		cmd.Dir = dir
		cmd.Env = []string{"LANG=C.UTF-8", "HOME=" + dir, "TMPDIR=" + dir, "XDG_CACHE_HOME=" + dir, "PYTHONDONTWRITEBYTECODE=1"}
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error {
			if cmd.Process == nil {
				return os.ErrProcessDone
			}
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		cmd.WaitDelay = 200 * time.Millisecond
		if cmd.Run() != nil || stdout.overflow || stderr.overflow {
			var diagnostic struct {
				Type  string `json:"type"`
				Stage string `json:"stage"`
			}
			if !stderr.overflow && json.Unmarshal(stderr.buf.Bytes(), &diagnostic) == nil && regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,80}$`).MatchString(diagnostic.Type) && identifier.MatchString(diagnostic.Stage) {
				return fmt.Errorf("%w (%s/%s)", ErrCatalog, diagnostic.Type, diagnostic.Stage)
			}
			return ErrCatalog
		}
		decoder := json.NewDecoder(bytes.NewReader(stdout.buf.Bytes()))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&result) != nil || decoder.Decode(new(any)) != io.EOF || result.SnapshotID == "" || len(result.Records) != len(rows) {
			return ErrCatalog
		}
		return nil
	})
	return result, e
}
