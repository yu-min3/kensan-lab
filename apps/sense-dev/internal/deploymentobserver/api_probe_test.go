package deploymentobserver

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type apiFixture struct {
	k        KubeCLI
	p        Plan
	pod      PodState
	config   string
	mutation string
	routes   []string
	proxy    int
}

func newAPIFixture(t *testing.T) *apiFixture {
	t.Helper()
	f := &apiFixture{}
	f.pod = PodState{Name: "canary-abcdef-abcde", Namespace: namespace, UID: "uid-1", AppLabel: "canary", Ready: true, PodIP: netip.MustParseAddr("10.42.0.8"), Image: image + "@sha256:" + strings.Repeat("a", 64), ImageID: "containerd://sha256:" + strings.Repeat("b", 64)}
	f.p = Plan{SchemaVersion: 1, Namespace: namespace, Application: application, Repository: repository, Image: image, ProbeIP: f.pod.PodIP, ProbeTransport: "kubernetes-api", ProbePort: 8000, ProbePath: "/api/release", ExpectedRelease: "v2", probePod: &f.pod}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test private API"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("10.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.routes = append(f.routes, r.URL.RequestURI())
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer private-test-token" {
			t.Error("request is not credentialed GET")
		}
		switch r.URL.Path {
		case "/api/v1/namespaces/app-canary/pods/canary-abcdef-abcde":
			pod := f.pod
			switch f.mutation {
			case "changed UID":
				pod.UID = "uid-2"
			case "changed IP":
				pod.PodIP = netip.MustParseAddr("10.42.0.9")
			case "changed namespace":
				pod.Namespace = "other"
			case "changed image":
				pod.Image = image + ":tag"
			case "changed runtime":
				pod.ImageID = "containerd://other"
			case "changed name":
				pod.Name = "other"
			case "changed label":
				pod.AppLabel = "other"
			case "unready":
				pod.Ready = false
			}
			json.NewEncoder(w).Encode(map[string]any{"metadata": map[string]any{"name": pod.Name, "namespace": pod.Namespace, "uid": pod.UID, "labels": map[string]string{"app.kubernetes.io/name": pod.AppLabel}}, "spec": map[string]any{"containers": []any{map[string]string{"image": pod.Image}}}, "status": map[string]any{"phase": "Running", "podIP": pod.PodIP.String(), "containerStatuses": []any{map[string]any{"ready": pod.Ready, "imageID": pod.ImageID}}}})
		case "/api/v1/namespaces/app-canary/pods/canary-abcdef-abcde:8000/proxy/api/release":
			f.proxy++
			switch f.mutation {
			case "redirect":
				w.Header().Set("Location", "https://evil.example/api/release")
				w.WriteHeader(302)
			case "same API redirect":
				w.Header().Set("Location", "/other-route")
				w.WriteHeader(307)
			case "wrong release":
				w.Write([]byte(`{"release":"v1"}`))
			case "oversize":
				w.Write([]byte(strings.Repeat("x", 5000)))
			case "unknown field":
				w.Write([]byte(`{"release":"v2","token":"secret"}`))
			default:
				w.Write([]byte(`{"release":"v2"}`))
			}
		default:
			t.Errorf("unexpected fixed route %s", r.URL)
			w.WriteHeader(404)
		}
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	server.StartTLS()
	t.Cleanup(server.Close)
	f.config = fmt.Sprintf("apiVersion: v1\nkind: Config\ncurrent-context: observer\nclusters:\n- name: private\n  cluster:\n    server: https://10.0.0.1:6443\n    certificate-authority-data: %s\nusers:\n- name: observer\n  user:\n    token: private-test-token\ncontexts:\n- name: observer\n  context:\n    cluster: private\n    user: observer\n    namespace: app-canary\n", base64.StdEncoding.EncodeToString(ca))
	path := filepath.Join(t.TempDir(), "observer.yaml")
	if err := os.WriteFile(path, []byte(f.config), 0600); err != nil {
		t.Fatal(err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	f.k = KubeCLI{Binary: "/nonexistent-no-fallback-kubectl", Kubeconfig: path, apiDial: func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "10.0.0.1:6443" {
			t.Error("dial target changed")
		}
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}}
	return f
}
func TestAPIProxyUsesResolvedHostPodAndFixedGET(t *testing.T) {
	f := newAPIFixture(t)
	got, err := (HostSources{Kube: f.k}).Probe(context.Background(), f.p)
	if err != nil || got.ObservedRelease != "v2" || f.proxy != 1 || len(f.routes) != 2 {
		t.Fatalf("fixed proxy %+v %v %v", got, err, f.routes)
	}
	for _, mutation := range []string{"changed UID", "changed IP", "changed namespace", "changed image", "changed runtime", "changed name", "changed label", "unready", "redirect", "same API redirect", "wrong release", "oversize", "unknown field"} {
		t.Run(mutation, func(t *testing.T) {
			f := newAPIFixture(t)
			f.mutation = mutation
			if _, err := f.k.APIProbe(context.Background(), f.p); err == nil {
				t.Fatal("unverified API proxy observation accepted")
			}
			if len(f.routes) > 2 {
				t.Fatal("redirect or fallback requested another route")
			}
		})
	}
}
func TestAPIProxyRejectsPlanTargetsAndUnresolvedPods(t *testing.T) {
	for _, mutate := range []func(*Plan){
		func(p *Plan) { p.probePod = nil }, func(p *Plan) { p.probePod.Name = "../secret" }, func(p *Plan) { p.probePod.Namespace = "default" }, func(p *Plan) { p.ProbeIP = netip.Addr{}; p.ProbeCIDR = netip.MustParsePrefix("10.42.0.0/16") }, func(p *Plan) { p.ProbePath = "/health" }, func(p *Plan) { p.ProbePort = 8080 }, func(p *Plan) { p.ProbeTransport = "fallback" },
	} {
		f := newAPIFixture(t)
		mutate(&f.p)
		if _, err := f.k.APIProbe(context.Background(), f.p); err == nil || len(f.routes) > 0 {
			t.Fatal("invalid proxy target made a request")
		}
	}
	f := newAPIFixture(t)
	if _, err := HTTPProbe(context.Background(), f.p); err == nil {
		t.Fatal("API mode fell back to direct")
	}
	for _, field := range []string{"probe_pod", "pod_name", "url"} {
		body := `{"schema_version":1,"namespace":"app-canary","application":"app-canary","repository":"yu-min3/kensan-lab","image":"` + image + `","probe_ip":"10.42.0.8","probe_transport":"kubernetes-api","probe_port":8000,"probe_path":"/api/release","expected_release":"v2","` + field + `":"evil"}`
		path := filepath.Join(t.TempDir(), "plan.json")
		os.WriteFile(path, []byte(body), 0600)
		if _, err := LoadPlan(path); err == nil {
			t.Fatal("serialized Pod/URL accepted")
		}
	}
}
func TestRestrictedAPIKubeconfigRejectsAmbientPermissions(t *testing.T) {
	mutations := map[string]func(string) string{
		"public server": func(s string) string { return strings.Replace(s, "10.0.0.1", "8.8.8.8", 1) }, "hostname": func(s string) string { return strings.Replace(s, "10.0.0.1", "evil.example", 1) }, "HTTP": func(s string) string { return strings.Replace(s, "https://", "http://", 1) }, "server route": func(s string) string { return strings.Replace(s, ":6443", ":6443/evil", 1) }, "server userinfo": func(s string) string { return strings.Replace(s, "https://", "https://user@", 1) },
		"exec plugin": func(s string) string {
			return strings.Replace(s, "    token:", "    exec: {command: evil}\n    token:", 1)
		}, "auth provider": func(s string) string {
			return strings.Replace(s, "    token:", "    auth-provider: {name: evil}\n    token:", 1)
		}, "TLS server override": func(s string) string {
			return strings.Replace(s, "    server:", "    tls-server-name: evil.example\n    server:", 1)
		},
		"proxy URL": func(s string) string {
			return strings.Replace(s, "    server:", "    proxy-url: https://evil.example\n    server:", 1)
		}, "insecure": func(s string) string {
			return strings.Replace(s, "    server:", "    insecure-skip-tls-verify: true\n    server:", 1)
		}, "client key": func(s string) string {
			return strings.Replace(s, "    token:", "    client-key: /root/admin-key\n    token:", 1)
		}, "token file": func(s string) string {
			return strings.Replace(s, "    token:", "    tokenFile: /root/admin-token\n    token:", 1)
		},
		"duplicate": func(s string) string { return s + "current-context: other\n" }, "multidoc": func(s string) string { return s + "---\nkind: Config\n" }, "unknown context": func(s string) string {
			return strings.Replace(s, "current-context: observer", "current-context: admin", 1)
		}, "empty CA": func(s string) string {
			lines := strings.Split(s, "\n")
			for i, line := range lines {
				if strings.Contains(line, "certificate-authority-data:") {
					lines[i] = "    certificate-authority-data: ''"
				}
			}
			return strings.Join(lines, "\n")
		},
	}
	for name, mutation := range mutations {
		t.Run(name, func(t *testing.T) {
			f := newAPIFixture(t)
			os.WriteFile(f.k.Kubeconfig, []byte(mutation(f.config)), 0600)
			if _, err := f.k.privateAPI(); err == nil {
				t.Fatal("unsafe kubeconfig accepted")
			}
			if _, err := f.k.APIProbe(context.Background(), f.p); err == nil || len(f.routes) > 0 {
				t.Fatal("unsafe credential made a request")
			}
		})
	}
	f := newAPIFixture(t)
	os.Chmod(f.k.Kubeconfig, 0644)
	if _, err := f.k.privateAPI(); err == nil {
		t.Fatal("non-private kubeconfig accepted")
	}
}

func TestAPIProxyRejectsUntrustedTLSAndMissingCredential(t *testing.T) {
	f := newAPIFixture(t)
	other := newAPIFixture(t)
	getCA := func(config string) string {
		for _, line := range strings.Split(config, "\n") {
			if strings.Contains(line, "certificate-authority-data:") {
				return line
			}
		}
		return ""
	}
	body := strings.Replace(f.config, getCA(f.config), getCA(other.config), 1)
	if err := os.WriteFile(f.k.Kubeconfig, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.k.APIProbe(context.Background(), f.p); err == nil || len(f.routes) != 0 {
		t.Fatal("untrusted API TLS made an authenticated request")
	}
	if _, err := (KubeCLI{}).APIProbe(context.Background(), f.p); err == nil {
		t.Fatal("missing dedicated credential accepted")
	}
}
