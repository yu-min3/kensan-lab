// Package worker runs trusted, pinned domain adapters with no inherited secrets.
// It is a process boundary, not an operating-system sandbox.
package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/yu-min3/kensan-lab/apps/llmops/contracts"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
)

var (
	ErrConfig    = errors.New("invalid worker configuration")
	ErrRequest   = errors.New("invalid adapter request")
	ErrExecution = errors.New("adapter execution failed")
	ErrLimit     = errors.New("adapter output limit exceeded")
	ErrProtocol  = errors.New("invalid adapter response")
	ErrTimeout   = errors.New("adapter deadline exceeded")
)
var name = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

type Adapter struct {
	Binary              string `json:"binary"`
	SHA256              string `json:"sha256"`
	DomainSchemaVersion string `json:"domain_schema_version"`
}
type Storage struct {
	Endpoint       string `json:"endpoint"`
	Bucket         string `json:"bucket"`
	Region         string `json:"region"`
	CredentialFile string `json:"credential_file"`
	CAFile         string `json:"ca_file,omitempty"`
}
type CatalogConfig struct {
	URI              string `json:"uri"`
	Name             string `json:"name"`
	Namespace        string `json:"namespace"`
	Table            string `json:"table"`
	TokenURL         string `json:"token_url"`
	ClientID         string `json:"client_id"`
	ClientSecretFile string `json:"client_secret_file"`
	CAFile           string `json:"ca_file"`
	PythonBinary     string `json:"python_binary"`
}

type TableLocation struct {
	Namespace string `json:"namespace"`
	Table     string `json:"table"`
}

type Config struct {
	App            string             `json:"app"`
	Role           string             `json:"role"`
	TempRoot       string             `json:"temp_root"`
	TimeoutMillis  int                `json:"timeout_millis"`
	MaxInputBytes  int                `json:"max_input_bytes"`
	MaxOutputBytes int                `json:"max_output_bytes"`
	Adapters       map[string]Adapter `json:"adapters"`
	Storage        *Storage           `json:"storage,omitempty"`
	Catalog        *CatalogConfig     `json:"catalog,omitempty"`
	DatasetSource  *TableLocation     `json:"dataset_source,omitempty"`
}
type Request struct {
	ProtocolVersion     string          `json:"protocol_version"`
	RequestID           string          `json:"request_id"`
	Operation           string          `json:"operation"`
	DomainSchemaVersion string          `json:"domain_schema_version"`
	Payload             json.RawMessage `json:"payload"`
}
type AdapterError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Response struct {
	ProtocolVersion string          `json:"protocol_version"`
	RequestID       string          `json:"request_id"`
	Result          json.RawMessage `json:"result,omitempty"`
	Error           *AdapterError   `json:"error,omitempty"`
}
type Runner struct{ cfg Config }

func DecodeConfig(data []byte) (Config, error) {
	var c Config
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || d.Decode(new(any)) != io.EOF {
		return c, ErrConfig
	}
	return c, nil
}
func New(c Config) (*Runner, error) {
	if !name.MatchString(c.App) || (c.Role != "transform" && c.Role != "eval") || !filepath.IsAbs(c.TempRoot) || len(c.Adapters) == 0 {
		return nil, ErrConfig
	}
	if c.TimeoutMillis == 0 {
		c.TimeoutMillis = 30000
	}
	if c.MaxInputBytes == 0 {
		c.MaxInputBytes = 8 << 20
	}
	if c.MaxOutputBytes == 0 {
		c.MaxOutputBytes = 8 << 20
	}
	if c.TimeoutMillis < 1 || c.TimeoutMillis > 300000 || c.MaxInputBytes < 1 || c.MaxInputBytes > 32<<20 || c.MaxOutputBytes < 1 || c.MaxOutputBytes > 32<<20 {
		return nil, ErrConfig
	}
	copied := make(map[string]Adapter, len(c.Adapters))
	for id, a := range c.Adapters {
		if !name.MatchString(id) || !filepath.IsAbs(a.Binary) || a.DomainSchemaVersion == "" || len(a.SHA256) != 64 {
			return nil, ErrConfig
		}
		if _, e := hex.DecodeString(a.SHA256); e != nil {
			return nil, ErrConfig
		}
		copied[id] = a
	}
	c.Adapters = copied
	if c.Storage != nil {
		s := *c.Storage
		if filepath.Base(s.CredentialFile) != c.Role+"-"+c.App+".json" || !filepath.IsAbs(s.CredentialFile) {
			return nil, ErrConfig
		}
		if _, e := readCredentials(s.CredentialFile); e != nil {
			return nil, e
		}
		c.Storage = &s
	}
	for _, dir := range []string{c.TempRoot, filepath.Join(c.TempRoot, c.App), filepath.Join(c.TempRoot, c.App, c.Role)} {
		if e := privateDir(dir); e != nil {
			return nil, e
		}
	}
	canonical, e := filepath.EvalSymlinks(c.TempRoot)
	if e != nil {
		return nil, ErrConfig
	}
	c.TempRoot = canonical
	return &Runner{c}, nil
}
func privateDir(path string) error {
	if e := os.Mkdir(path, 0700); e != nil && !os.IsExist(e) {
		return ErrConfig
	}
	info, e := os.Lstat(path)
	if e != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return ErrConfig
	}
	return nil
}
func (r *Runner) App() string  { return r.cfg.App }
func (r *Runner) Role() string { return r.cfg.Role }
func readCredentials(path string) (aws.Credentials, error) {
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 16384 {
		return aws.Credentials{}, ErrConfig
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return aws.Credentials{}, ErrConfig
	}
	var c struct {
		Access string `json:"access_key"`
		Secret string `json:"secret_key"`
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || d.Decode(new(any)) != io.EOF || len(c.Access) < 16 || len(c.Secret) < 24 {
		return aws.Credentials{}, ErrConfig
	}
	return aws.Credentials{AccessKeyID: c.Access, SecretAccessKey: c.Secret, Source: "worker-role-file"}, nil
}
func (r *Runner) Objects() (*objects.S3, error) {
	s := r.cfg.Storage
	if s == nil {
		return nil, ErrConfig
	}
	credential, e := readCredentials(s.CredentialFile)
	if e != nil {
		return nil, e
	}
	return objects.NewS3(objects.S3Config{App: r.cfg.App, Bucket: s.Bucket, Region: s.Region, Endpoint: s.Endpoint, CAFile: s.CAFile, Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) { return credential, nil })})
}

// WithWorkspace provides a private, disposable directory to trusted worker code.
func (r *Runner) WithWorkspace(fn func(string) error) error {
	dir, e := os.MkdirTemp(filepath.Join(r.cfg.TempRoot, r.cfg.App, r.cfg.Role), "attempt-")
	if e != nil {
		return ErrExecution
	}
	defer os.RemoveAll(dir)
	return fn(dir)
}

type bounded struct {
	data     bytes.Buffer
	max      int
	cancel   context.CancelFunc
	exceeded bool
}

func (w *bounded) Write(p []byte) (int, error) {
	if w.data.Len()+len(p) > w.max {
		w.exceeded = true
		w.cancel()
		return 0, ErrLimit
	}
	return w.data.Write(p)
}
func validate(schema string, b []byte) bool {
	var v any
	return json.Unmarshal(b, &v) == nil && contracts.Validate(schema, v) == nil
}
func (r *Runner) Call(ctx context.Context, id string, request Request) (Response, error) {
	var result Response
	a, ok := r.cfg.Adapters[id]
	if !ok {
		return result, ErrRequest
	}
	input, e := json.Marshal(request)
	if e != nil || len(input) > r.cfg.MaxInputBytes || !validate("adapter-request", input) || request.DomainSchemaVersion != a.DomainSchemaVersion {
		return result, ErrRequest
	}
	// Hash the exact executable before every call; reject symlink substitution.
	info, e := os.Lstat(a.Binary)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return result, ErrExecution
	}
	if info.Mode().Perm()&0022 != 0 || info.Size() > 128<<20 {
		return result, ErrExecution
	}
	file, e := os.Open(a.Binary)
	if e != nil {
		return result, ErrExecution
	}
	hash := sha256.New()
	_, e = io.Copy(hash, file)
	closeErr := file.Close()
	if e != nil || closeErr != nil || hex.EncodeToString(hash.Sum(nil)) != a.SHA256 {
		return result, ErrExecution
	}
	e = r.WithWorkspace(func(dir string) error {
		child, cancel := context.WithTimeout(ctx, time.Duration(r.cfg.TimeoutMillis)*time.Millisecond)
		defer cancel()
		stdout := &bounded{max: r.cfg.MaxOutputBytes, cancel: cancel}
		stderr := &bounded{max: 4096, cancel: cancel}
		cmd := exec.CommandContext(child, a.Binary)
		cmd.Dir = dir
		cmd.Stdin = bytes.NewReader(input)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		cmd.Env = []string{"LANG=C.UTF-8", "HOME=" + dir, "TMPDIR=" + dir, "XDG_CACHE_HOME=" + dir}
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error {
			if cmd.Process == nil {
				return os.ErrProcessDone
			}
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		cmd.WaitDelay = 200 * time.Millisecond
		runErr := cmd.Run()
		if stdout.exceeded || stderr.exceeded {
			return ErrLimit
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if child.Err() != nil {
			return ErrTimeout
		}
		if runErr != nil {
			return ErrExecution
		}
		data := stdout.data.Bytes()
		if !validate("adapter-response", data) {
			return ErrProtocol
		}
		if json.Unmarshal(data, &result) != nil || result.RequestID != request.RequestID {
			return ErrProtocol
		}
		return nil
	})
	return result, e
}

// WithLock enforces the V0 single-host writer boundary across CLI processes.
// All writers for an app/role must share the operator-configured TempRoot.
func (r *Runner) WithLock(id string, fn func() error) error {
	if !name.MatchString(id) {
		return ErrConfig
	}
	file, e := os.OpenFile(filepath.Join(r.cfg.TempRoot, r.cfg.App, r.cfg.Role, id+".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return ErrExecution
	}
	defer file.Close()
	if syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		return errors.New("worker writer is busy; retry later")
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return fn()
}
