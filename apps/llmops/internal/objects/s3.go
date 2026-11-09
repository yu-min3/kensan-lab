package objects

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type S3Config struct {
	App, Bucket, Region, Endpoint, CAFile string
	Credentials                           aws.CredentialsProvider
	Timeout                               time.Duration
}
type S3 struct {
	app, bucket string
	client      *s3.Client
}

func NewS3(cfg S3Config) (*S3, error) {
	u, e := url.Parse(cfg.Endpoint)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" {
		return nil, ErrInvalid
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && (u.Scheme != "http" || ip == nil || !ip.IsLoopback()) {
		return nil, ErrInvalid
	}
	if !appID(cfg.App) || cfg.Bucket == "" || strings.ContainsAny(cfg.Bucket, "/\\: ") || cfg.Region == "" || cfg.Credentials == nil {
		return nil, ErrInvalid
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if timeout < time.Millisecond || timeout > time.Minute {
		return nil, ErrInvalid
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.CAFile != "" {
		pem, e := os.ReadFile(cfg.CAFile)
		if e != nil {
			return nil, ErrInvalid
		}
		roots, e := x509.SystemCertPool()
		if e != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, ErrInvalid
		}
		transport.TLSClientConfig.RootCAs = roots
	}
	httpClient := &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("S3 redirect refused") }}
	c := s3.NewFromConfig(aws.Config{Region: cfg.Region, Credentials: aws.NewCredentialsCache(cfg.Credentials), HTTPClient: httpClient, RetryMaxAttempts: 3, RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired, ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(strings.TrimRight(cfg.Endpoint, "/"))
		o.UsePathStyle = true
	})
	return &S3{app: cfg.App, bucket: cfg.Bucket, client: c}, nil
}
func (s *S3) AppID() string { return s.app }
func (s *S3) validKey(key string) bool {
	prefix := "apps/" + s.app + "/"
	if !strings.HasPrefix(key, prefix) || len(key) > 1024 {
		return false
	}
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
		for _, r := range part {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
				return false
			}
		}
	}
	return true
}
func classify(e error) error {
	if e == nil {
		return nil
	}
	var response *smithyhttp.ResponseError
	if errors.As(e, &response) {
		switch response.HTTPStatusCode() {
		case 404:
			return ErrNotFound
		case 412:
			return ErrExists
		}
	}
	var api smithy.APIError
	if errors.As(e, &api) {
		switch api.ErrorCode() {
		case "NoSuchKey", "NotFound":
			return ErrNotFound
		case "PreconditionFailed":
			return ErrExists
		}
	}
	// Never expose SDK errors, which may include endpoints or response text.
	return ErrUnavailable
}
func (s *S3) PutIfAbsent(ctx context.Context, key string, body []byte, mime string) error {
	if !s.validKey(key) || len(body) > int(MaxRawBytes) || mime == "" {
		return ErrInvalid
	}
	_, e := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Body: bytes.NewReader(body), ContentType: aws.String(mime), ContentLength: aws.Int64(int64(len(body))), IfNoneMatch: aws.String("*")})
	return classify(e)
}
func (s *S3) Get(ctx context.Context, key string, max int64) (Blob, error) {
	if !s.validKey(key) || max < 0 || max > MaxRawBytes {
		return Blob{}, ErrInvalid
	}
	r, e := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if e != nil {
		return Blob{}, classify(e)
	}
	defer r.Body.Close()
	if r.ContentLength != nil && *r.ContentLength > max {
		return Blob{}, ErrTooLarge
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, max+1))
	if e != nil {
		return Blob{}, ErrUnavailable
	}
	if int64(len(b)) > max {
		return Blob{}, ErrTooLarge
	}
	return Blob{Bytes: b, MIME: aws.ToString(r.ContentType)}, nil
}
