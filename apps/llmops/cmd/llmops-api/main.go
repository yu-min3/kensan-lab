// llmops-api runs the shared service with operator-owned configuration files.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/api"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/identity"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/jobs"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/policy"
	runtimeclient "github.com/yu-min3/kensan-lab/apps/llmops/internal/runtime"
)

type storageConfig struct {
	App             string `json:"app"`
	Bucket          string `json:"bucket"`
	Region          string `json:"region"`
	Endpoint        string `json:"endpoint"`
	CAFile          string `json:"ca_file"`
	CredentialsFile string `json:"credentials_file"`
}
type runtimeConfig struct {
	App              string `json:"app"`
	Role             string `json:"role"`
	TargetID         string `json:"target_id"`
	ClientID         string `json:"client_id"`
	BaseURL          string `json:"base_url"`
	TokenURL         string `json:"token_url"`
	ClientSecretFile string `json:"client_secret_file"`
	CAFile           string `json:"ca_file"`
}
type config struct {
	Listen         string                    `json:"listen"`
	Issuer         string                    `json:"issuer"`
	Audience       string                    `json:"audience"`
	JWKSURL        string                    `json:"jwks_url"`
	CAFile         string                    `json:"ca_file"`
	MappingFile    string                    `json:"mapping_file"`
	StateDirectory string                    `json:"state_directory"`
	CursorKeyFile  string                    `json:"cursor_key_file"`
	TLSCertFile    string                    `json:"tls_cert_file"`
	TLSKeyFile     string                    `json:"tls_key_file"`
	Stores         []storageConfig           `json:"stores"`
	Runtimes       []runtimeConfig           `json:"runtimes"`
	DailyLimits    map[string]map[string]int `json:"daily_limits"`
}

func readConfig(path string) (config, error) {
	f, e := os.Open(path)
	if e != nil {
		return config{}, errors.New("configuration unavailable")
	}
	defer f.Close()
	var c config
	d := json.NewDecoder(io.LimitReader(f, 1<<20))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || d.Decode(new(any)) != io.EOF {
		return config{}, errors.New("invalid configuration")
	}
	return c, nil
}
func privateFile(path string) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, errors.New("private credential file unavailable")
	}
	defer f.Close()
	fi, e := f.Stat()
	if e != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 {
		return nil, errors.New("private credential file required")
	}
	raw, e := io.ReadAll(io.LimitReader(f, 65537))
	if e != nil || len(raw) == 0 || len(raw) > 65536 {
		return nil, errors.New("invalid credential file")
	}
	return raw, nil
}

type credentialsFile string

func (p credentialsFile) Retrieve(context.Context) (aws.Credentials, error) {
	raw, e := privateFile(string(p))
	if e != nil {
		return aws.Credentials{}, e
	}
	var v struct {
		Access  string `json:"access_key"`
		Secret  string `json:"secret_key"`
		Session string `json:"session_token"`
	}
	if json.Unmarshal(raw, &v) != nil || v.Access == "" || v.Secret == "" {
		return aws.Credentials{}, errors.New("invalid object credentials")
	}
	return aws.Credentials{AccessKeyID: v.Access, SecretAccessKey: v.Secret, SessionToken: v.Session, Source: "app-scoped credential file", CanExpire: true, Expires: time.Now().Add(time.Minute)}, nil
}
func server(ctx context.Context, c config) (*http.Server, *jobs.Store, error) {
	host, _, e := net.SplitHostPort(c.Listen)
	if e != nil {
		return nil, nil, errors.New("invalid listen address")
	}
	ip := net.ParseIP(host)
	if c.TLSCertFile == "" || c.TLSKeyFile == "" {
		if ip == nil || !ip.IsLoopback() || c.TLSCertFile != "" || c.TLSKeyFile != "" {
			return nil, nil, errors.New("non-loopback API requires TLS")
		}
	} else {
		if _, e := privateFile(c.TLSKeyFile); e != nil {
			return nil, nil, e
		}
	}
	mapping, e := os.ReadFile(c.MappingFile)
	if e != nil {
		return nil, nil, errors.New("client mapping unavailable")
	}
	cursor, e := privateFile(c.CursorKeyFile)
	if e != nil {
		return nil, nil, e
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if c.CAFile != "" {
		pem, e := os.ReadFile(c.CAFile)
		if e != nil {
			return nil, nil, errors.New("CA unavailable")
		}
		roots, e := x509.SystemCertPool()
		if e != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, nil, errors.New("invalid CA")
		}
		transport.TLSClientConfig.RootCAs = roots
	}
	verifier, e := identity.New(identity.Config{Issuer: c.Issuer, Audience: c.Audience, JWKSURL: c.JWKSURL, MappingJSON: mapping, HTTPClient: &http.Client{Timeout: 5 * time.Second, Transport: transport}})
	if e != nil {
		return nil, nil, e
	}
	stores := map[string]*objects.Store{}
	for _, sc := range c.Stores {
		if _, e := credentialsFile(sc.CredentialsFile).Retrieve(ctx); e != nil {
			return nil, nil, e
		}
		if _, ok := stores[sc.App]; ok {
			return nil, nil, errors.New("duplicate app store")
		}
		backend, e := objects.NewS3(objects.S3Config{App: sc.App, Bucket: sc.Bucket, Region: sc.Region, Endpoint: sc.Endpoint, CAFile: sc.CAFile, Credentials: credentialsFile(sc.CredentialsFile)})
		if e != nil {
			return nil, nil, e
		}
		store, e := objects.NewStore(sc.App, backend)
		if e != nil {
			return nil, nil, e
		}
		stores[sc.App] = store
	}
	ledger, e := jobs.Open(c.StateDirectory)
	if e != nil {
		return nil, nil, e
	}
	success := false
	defer func() {
		if !success {
			ledger.Close()
		}
	}()
	resolver := &api.StorageResolver{Stores: stores, Jobs: ledger}
	engine, e := policy.New(policy.Config{Resolver: resolver, CursorKey: cursor})
	if e != nil {
		return nil, nil, e
	}
	captures, e := api.NewCaptureFeedback(api.CaptureFeedbackConfig{Auth: verifier, Policy: engine, Stores: stores})
	if e != nil {
		return nil, nil, e
	}
	routes := map[string]api.RuntimeRoute{}
	for _, rc := range c.Runtimes {
		rc := rc
		if _, e := runtimeclient.New(runtimeclient.Config{Owner: jobs.Owner{App: rc.App, Role: rc.Role, Subject: "configuration-probe"}, TargetID: rc.TargetID, BaseURL: rc.BaseURL, TokenURL: rc.TokenURL, ClientID: rc.ClientID, ClientSecretFile: rc.ClientSecretFile, CAFile: rc.CAFile}); e != nil {
			return nil, nil, e
		}
		if _, ok := routes[rc.ClientID]; ok {
			return nil, nil, errors.New("duplicate runtime client route")
		}
		routes[rc.ClientID] = api.RuntimeRoute{AppID: rc.App, Role: rc.Role, TargetID: rc.TargetID, Factory: func(o jobs.Owner) (api.RuntimeGateway, error) {
			return runtimeclient.New(runtimeclient.Config{Owner: o, TargetID: rc.TargetID, BaseURL: rc.BaseURL, TokenURL: rc.TokenURL, ClientID: rc.ClientID, ClientSecretFile: rc.ClientSecretFile, CAFile: rc.CAFile})
		}}
	}
	tasks, e := api.NewJobs(api.JobConfig{Authenticate: captures.Authenticated, Policy: engine, Store: ledger, Objects: stores, Routes: routes, DailyLimits: c.DailyLimits})
	if e != nil {
		return nil, nil, e
	}
	resolver.Next = tasks
	mux := http.NewServeMux()
	captures.Register(mux)
	tasks.Register(mux)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	go tasks.Run(ctx)
	success = true
	return &http.Server{Addr: c.Listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 45 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}}, ledger, nil
}
func main() {
	path := flag.String("config", os.Getenv("LLMOPS_CONFIG_FILE"), "operator configuration JSON file")
	flag.Parse()
	if *path == "" {
		log.Fatal("configuration file required")
	}
	c, e := readConfig(*path)
	if e != nil {
		log.Fatal(e)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv, ledger, e := server(ctx, c)
	if e != nil {
		log.Fatal(e)
	}
	defer ledger.Close()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	if c.TLSCertFile != "" {
		e = srv.ListenAndServeTLS(c.TLSCertFile, c.TLSKeyFile)
	} else {
		e = srv.ListenAndServe()
	}
	if e != nil && !errors.Is(e, http.ErrServerClosed) {
		log.Fatal("API listener failed")
	}
}
