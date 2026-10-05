package deploymentobserver

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type restrictedKubeconfig struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Current    string `yaml:"current-context"`
	Clusters   []struct {
		Name    string `yaml:"name"`
		Cluster struct {
			Server string `yaml:"server"`
			CA     string `yaml:"certificate-authority-data"`
		} `yaml:"cluster"`
	} `yaml:"clusters"`
	Users []struct {
		Name string `yaml:"name"`
		User struct {
			Token string `yaml:"token"`
		} `yaml:"user"`
	} `yaml:"users"`
	Contexts []struct {
		Name    string `yaml:"name"`
		Context struct {
			Cluster   string `yaml:"cluster"`
			User      string `yaml:"user"`
			Namespace string `yaml:"namespace,omitempty"`
		} `yaml:"context"`
	} `yaml:"contexts"`
}

type privateAPI struct {
	server, token string
	client        *http.Client
}

func (k KubeCLI) privateAPI() (privateAPI, error) {
	fail := func() (privateAPI, error) {
		return privateAPI{}, errors.New("dedicated API observer kubeconfig outside restricted policy")
	}
	actual, err := filepath.EvalSymlinks(k.Kubeconfig)
	info, e := os.Lstat(k.Kubeconfig)
	if err != nil || e != nil || !filepath.IsAbs(k.Kubeconfig) || actual != k.Kubeconfig || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 64<<10 {
		return fail()
	}
	file, err := os.Open(k.Kubeconfig)
	if err != nil {
		return fail()
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return fail()
	}
	body, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	if err != nil || len(body) > 64<<10 {
		return fail()
	}
	dec := yaml.NewDecoder(bytes.NewReader(body))
	var root yaml.Node
	if dec.Decode(&root) != nil || dec.Decode(new(yaml.Node)) != io.EOF || !plainKubeYAML(&root) {
		return fail()
	}
	var config restrictedKubeconfig
	typed := yaml.NewDecoder(bytes.NewReader(body))
	typed.KnownFields(true)
	if typed.Decode(&config) != nil || config.APIVersion != "v1" || config.Kind != "Config" || len(config.Clusters) != 1 || len(config.Users) != 1 || len(config.Contexts) != 1 || config.Current == "" || config.Current != config.Contexts[0].Name || config.Contexts[0].Context.Cluster != config.Clusters[0].Name || config.Contexts[0].Context.User != config.Users[0].Name || config.Clusters[0].Name == "" || config.Users[0].Name == "" || config.Contexts[0].Context.Namespace != "" && config.Contexts[0].Context.Namespace != namespace {
		return fail()
	}
	server, err := url.Parse(config.Clusters[0].Cluster.Server)
	if err != nil || server.Scheme != "https" || server.User != nil || server.Path != "" || server.RawQuery != "" || server.Fragment != "" || server.Opaque != "" {
		return fail()
	}
	ip, err := netip.ParseAddr(server.Hostname())
	if err != nil || !ip.Is4() || !ip.IsPrivate() || server.Port() == "0" {
		return fail()
	}
	ca, err := base64.StdEncoding.DecodeString(config.Clusters[0].Cluster.CA)
	if err != nil || len(ca) > 32<<10 {
		return fail()
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return fail()
	}
	token := config.Users[0].User.Token
	if len(token) < 16 || strings.ContainsAny(token, " \t\r\n") {
		return fail()
	}
	transport := &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}, DisableKeepAlives: true}
	if k.apiDial != nil {
		transport.DialContext = k.apiDial
	}
	client := &http.Client{Timeout: 10 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("API observer redirect forbidden") }}
	return privateAPI{server: server.String(), token: token, client: client}, nil
}
func plainKubeYAML(n *yaml.Node) bool {
	if n == nil || n.Kind == yaml.AliasNode || n.Anchor != "" {
		return false
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			key := n.Content[i]
			if key.Kind != yaml.ScalarNode || seen[key.Value] {
				return false
			}
			seen[key.Value] = true
		}
	}
	for _, child := range n.Content {
		if !plainKubeYAML(child) {
			return false
		}
	}
	return true
}
func (a privateAPI) get(ctx context.Context, route string, limit int) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.server+route, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	response, err := a.client.Do(req)
	if err != nil {
		return nil, errors.New("fixed private API GET unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("fixed private API GET rejected")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, int64(limit)+1))
	if err != nil || len(body) > limit {
		return nil, errors.New("private API response exceeds bound")
	}
	return body, nil
}
func (api privateAPI) currentPod(ctx context.Context, name string) (PodState, error) {
	if !safePodName(name) {
		return PodState{}, errors.New("unsafe API Pod name")
	}
	body, err := api.get(ctx, "/api/v1/namespaces/"+namespace+"/pods/"+name, 1<<20)
	if err != nil {
		return PodState{}, err
	}
	// Reuse the same strict single-container namespace/IP/readiness parser.
	list := []byte(`{"items":[`)
	list = append(list, body...)
	list = append(list, ']', '}')
	pods, err := parsePods(list)
	if err != nil || len(pods) != 1 {
		return PodState{}, errors.New("selected API Pod unavailable")
	}
	if pods[0].Name != name {
		return PodState{}, errors.New("selected API Pod name changed")
	}
	return pods[0], nil
}
