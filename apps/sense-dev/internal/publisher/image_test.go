package publisher

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

type imageFixture struct {
	g        GitHub
	intent   core.PublishIntent
	posts    int
	run      bool
	mutation string
	digest   string
	objects  map[string][]byte
}

func newImageFixture(t *testing.T, registryMutation ...string) *imageFixture {
	t.Helper()
	f := &imageFixture{}
	content := []byte("trusted fixed workflow")
	sum := sha256.Sum256(content)
	spec := &core.ImageReleaseSpec{SourceSHA: strings.Repeat("a", 40), SourceAppTreeSHA: strings.Repeat("b", 40), WorkflowSHA: strings.Repeat("c", 40), WorkflowSHA256: hex.EncodeToString(sum[:]), WorkflowRef: "refs/tags/sense-image-v1", WorkflowPath: core.CanaryImageWorkflowPath, DispatchID: strings.Repeat("d", 32)}
	spec.ImageTag = "sense-" + spec.SourceSHA + "-" + spec.DispatchID
	f.objects = map[string][]byte{}
	hashObject := func(path string, v any) string {
		body, _ := json.Marshal(v)
		sum := sha256.Sum256(body)
		digest := "sha256:" + hex.EncodeToString(sum[:])
		f.objects[path+digest] = body
		return digest
	}
	children := []any{}
	for _, arch := range []string{"amd64", "arm64"} {
		mutation := ""
		if len(registryMutation) > 0 {
			mutation = registryMutation[0]
		}
		if mutation == "missing-arm64" && arch == "arm64" {
			continue
		}
		source, workflow := spec.SourceSHA, spec.WorkflowSHA
		if mutation == "wrong-source-label" {
			source = spec.WorkflowSHA
		}
		if mutation == "wrong-workflow-label" {
			workflow = spec.SourceSHA
		}
		config := hashObject("blobs/", map[string]any{"os": "linux", "architecture": arch, "config": map[string]any{"Labels": map[string]string{"org.opencontainers.image.source": "https://github.com/" + repository, "org.opencontainers.image.revision": source, "dev.kensan.workflow-sha": workflow}}})
		child := hashObject("manifests/", map[string]any{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.manifest.v1+json", "config": map[string]string{"mediaType": "application/vnd.oci.image.config.v1+json", "digest": config}})
		children = append(children, map[string]any{"mediaType": "application/vnd.oci.image.manifest.v1+json", "digest": child, "platform": map[string]string{"os": "linux", "architecture": arch}})
	}
	f.digest = hashObject("manifests/", map[string]any{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.index.v1+json", "manifests": children})
	f.objects["manifests/"+spec.ImageTag] = f.objects["manifests/"+f.digest]
	f.intent = core.PublishIntent{Operation: "image_publish", Repository: repository, Ref: "refs/heads/sense-dev/app", HeadSHA: spec.SourceSHA, TargetEnvironment: "private-canary", PolicyVersion: core.ReleasePolicyVersion, ExpiresAt: time.Now().Add(time.Hour), ImageRelease: spec}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" || strings.HasPrefix(r.URL.Path, "/user/packages/") {
			if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer package-token" {
				t.Error("package API credential or method differs")
			}
		} else if r.URL.Path != "/token" && !strings.HasPrefix(r.URL.Path, "/v2/") && r.Header.Get("Authorization") != "Bearer token" {
			t.Error("repository API credential differs")
		}
		if strings.HasPrefix(r.URL.Path, "/users/") {
			t.Error("public-only package API used")
		}
		respond := func(v any) {
			if err := json.NewEncoder(w).Encode(v); err != nil {
				t.Error(err)
			}
		}
		path := r.URL.Path
		switch {
		case path == "/user":
			w.Header().Set("X-OAuth-Scopes", "read:packages")
			login := "yu-min3"
			if f.mutation == "package-owner" {
				login = "other"
			}
			if f.mutation == "package-scope" {
				w.Header().Set("X-OAuth-Scopes", "repo, read:packages")
			}
			respond(map[string]string{"login": login})
		case path == "/token":
			user, pass, ok := r.BasicAuth()
			if !ok || user != "yu-min3" || pass != "package-token" {
				t.Error("registry credential mismatch")
			}
			if f.mutation == "registry-auth" {
				w.WriteHeader(403)
				return
			}
			respond(map[string]string{"token": "registry-token"})
		case strings.HasPrefix(path, "/v2/yu-min3/kensan-lab/canary/"):
			if r.Header.Get("Authorization") != "Bearer registry-token" {
				t.Error("registry token missing")
			}
			key := strings.TrimPrefix(path, "/v2/yu-min3/kensan-lab/canary/")
			if f.mutation == "registry-redirect-untrusted" && strings.HasPrefix(key, "blobs/") {
				w.Header().Set("Location", "https://evil.example/blob")
				w.WriteHeader(307)
				return
			}
			body, ok := f.objects[key]
			if !ok {
				w.WriteHeader(404)
				return
			}
			if f.mutation == "registry-tag" && key == "manifests/"+spec.ImageTag || f.mutation == "registry-child" && strings.HasPrefix(key, "manifests/sha256:") && key != "manifests/"+f.digest || f.mutation == "registry-config" && strings.HasPrefix(key, "blobs/") {
				body = append([]byte(" "), body...)
			}
			w.Write(body)
		case strings.Contains(path, "/git/ref/tags/"):
			sha := spec.WorkflowSHA
			if f.mutation == "workflow-moved" {
				sha = spec.SourceSHA
			}
			respond(map[string]any{"object": map[string]string{"type": "commit", "sha": sha}})
		case strings.Contains(path, "/contents/"):
			data := content
			if f.mutation == "workflow-hash" {
				data = []byte("other")
			}
			respond(map[string]string{"type": "file", "encoding": "base64", "content": base64.StdEncoding.EncodeToString(data)})
		case strings.Contains(path, "/git/ref/heads/"):
			sha := spec.SourceSHA
			if f.mutation == "branch-moved" {
				sha = spec.WorkflowSHA
			}
			respond(map[string]any{"object": map[string]string{"sha": sha}})
		case strings.Contains(path, "/git/commits/"):
			respond(map[string]any{"sha": spec.SourceSHA, "tree": map[string]string{"sha": strings.Repeat("e", 40)}})
		case strings.Contains(path, "/git/trees/"):
			name, next := "apps", strings.Repeat("f", 40)
			if strings.HasSuffix(path, strings.Repeat("f", 40)) {
				name, next = "canary", spec.SourceAppTreeSHA
			}
			if f.mutation == "tree-moved" {
				next = strings.Repeat("9", 40)
			}
			respond(map[string]any{"sha": strings.TrimPrefix(path, "/repos/"+repository+"/git/trees/"), "truncated": false, "tree": []any{map[string]string{"path": name, "type": "tree", "mode": "040000", "sha": next}}})
		case strings.Contains(path, "/packages/") && strings.HasSuffix(path, "/versions"):
			versions := []any{}
			if f.mutation == "tag-reused" {
				versions = append(versions, map[string]any{"metadata": map[string]any{"container": map[string]any{"tags": []string{spec.ImageTag}}}})
			}
			respond(versions)
		case strings.Contains(path, "/packages/"):
			if f.mutation == "package-missing" {
				w.WriteHeader(404)
				return
			}
			visibility := "private"
			if f.mutation == "package-public" {
				visibility = "public"
			}
			respond(map[string]any{"visibility": visibility, "repository": map[string]string{"full_name": repository}})
		case strings.HasSuffix(path, "/dispatches"):
			f.posts++
			var body struct {
				Ref    string
				Inputs map[string]string
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Ref != "sense-image-v1" || len(body.Inputs) != 5 || body.Inputs["source_sha"] != spec.SourceSHA || body.Inputs["source_app_tree_sha"] != spec.SourceAppTreeSHA || body.Inputs["expected_workflow_sha"] != spec.WorkflowSHA || body.Inputs["image_tag"] != spec.ImageTag || body.Inputs["dispatch_id"] != spec.DispatchID {
				t.Error("dispatch differs from fixed spec")
			}
			f.run = true
			if f.mutation == "dispatch-unknown" {
				w.WriteHeader(502)
				return
			}
			w.WriteHeader(204)
		case strings.HasSuffix(path, "/workflows/canary-image.yml/runs"):
			runs := []any{}
			if f.run {
				run := map[string]any{"id": int64(42), "head_sha": spec.WorkflowSHA, "display_title": "sense-image-" + spec.DispatchID, "event": "workflow_dispatch", "status": "completed", "conclusion": "success", "path": spec.WorkflowPath, "run_attempt": 1, "repository": map[string]string{"full_name": repository}, "head_repository": map[string]string{"full_name": repository}}
				if f.mutation == "wrong-run-sha" {
					run["head_sha"] = spec.SourceSHA
				}
				if f.mutation == "rerun" {
					run["run_attempt"] = 2
				}
				if f.mutation == "pending" {
					run["status"] = "queued"
				}
				if f.mutation == "failed" {
					run["conclusion"] = "failure"
				}
				runs = append(runs, run)
				if f.mutation == "duplicate-run" {
					runs = append(runs, run)
				}
			}
			respond(map[string]any{"total_count": len(runs), "workflow_runs": runs})
		case strings.HasSuffix(path, "/runs/42/artifacts"):
			artifact := map[string]any{"id": 43, "name": "canary-image-" + spec.DispatchID, "expired": f.mutation == "artifact-expired", "workflow_run": map[string]any{"id": 42, "head_sha": spec.WorkflowSHA}}
			artifacts := []any{artifact}
			if f.mutation == "duplicate-artifact" {
				artifacts = append(artifacts, artifact)
			}
			respond(map[string]any{"total_count": len(artifacts), "artifacts": artifacts})
		case strings.HasSuffix(path, "/artifacts/43/zip"):
			if f.mutation == "redirect-untrusted" {
				w.Header().Set("Location", "https://evil.example/zip")
				w.WriteHeader(302)
				return
			}
			record := map[string]string{"repository": repository, "image": core.CanaryImageRepository, "source_sha": spec.SourceSHA, "source_app_tree_sha": spec.SourceAppTreeSHA, "tag": spec.ImageTag, "digest": f.digest, "visibility": "private", "dispatch_id": spec.DispatchID, "workflow_sha": spec.WorkflowSHA}
			if strings.HasPrefix(f.mutation, "provenance-") {
				record[strings.TrimPrefix(f.mutation, "provenance-")] = "wrong"
			}
			var archive bytes.Buffer
			zw := zip.NewWriter(&archive)
			name := "canary-image.json"
			if f.mutation == "archive-path" {
				name = "../canary-image.json"
			}
			file, _ := zw.Create(name)
			data, _ := json.Marshal(record)
			if f.mutation == "duplicate-json" {
				data = append([]byte(`{"repository":"other",`), data[1:]...)
			}
			if f.mutation == "trailing-json" {
				data = append(data, []byte(` {}`)...)
			}
			file.Write(data)
			if f.mutation == "archive-extra" {
				extra, _ := zw.Create("extra")
				extra.Write([]byte("no"))
			}
			zw.Close()
			w.Write(archive.Bytes())
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	token := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(token, []byte("token"), 0600); err != nil {
		t.Fatal(err)
	}
	packageToken := filepath.Join(t.TempDir(), "package-token")
	if err := os.WriteFile(packageToken, []byte("package-token"), 0600); err != nil {
		t.Fatal(err)
	}
	f.g = GitHub{TokenFile: token, PackageTokenFile: packageToken, API: server.URL, Client: server.Client(), RegistryAPI: server.URL}
	return f
}
func TestImageDispatchAndEvidence(t *testing.T) {
	f := newImageFixture(t)
	ctx := context.Background()
	if _, exists, err := f.g.Inspect(ctx, f.intent); err != nil || exists {
		t.Fatalf("absent dispatch: %v", err)
	}
	id, err := f.g.Execute(ctx, f.intent)
	if err != nil || id != f.intent.ImageRelease.DispatchID || f.posts != 1 {
		t.Fatalf("dispatch: %s %v", id, err)
	}
	if id, exists, err := f.g.Inspect(ctx, f.intent); err != nil || !exists || id != f.intent.ImageRelease.DispatchID {
		t.Fatalf("inspect: %s %v", id, err)
	}
	evidence, err := f.g.ImageEvidence(ctx, f.intent)
	if err != nil || evidence.SourceSHA != f.intent.HeadSHA || evidence.WorkflowSHA == evidence.SourceSHA || evidence.WorkflowRunID != 42 || evidence.WorkflowSHA256 != f.intent.ImageRelease.WorkflowSHA256 {
		t.Fatalf("evidence: %+v %v", evidence, err)
	}
	if _, err := f.g.Execute(ctx, f.intent); err == nil || f.posts != 1 {
		t.Fatal("accepted dispatch resent")
	}
	// Expired authorization can still be inspected and its evidence retrieved.
	f.intent.ExpiresAt = time.Now().Add(-time.Hour)
	if _, _, err := f.g.Inspect(ctx, f.intent); err != nil {
		t.Fatal(err)
	}
	if _, err := f.g.ImageEvidence(ctx, f.intent); err != nil {
		t.Fatal(err)
	}
}
func TestImageDispatchRejectsUnsafeInputs(t *testing.T) {
	for _, mutation := range []string{"workflow-moved", "workflow-hash", "branch-moved", "tree-moved", "package-missing", "package-public", "tag-reused", "expired"} {
		t.Run(mutation, func(t *testing.T) {
			f := newImageFixture(t)
			f.mutation = mutation
			if mutation == "expired" {
				f.intent.ExpiresAt = time.Now().Add(-time.Minute)
			}
			if _, err := f.g.Execute(context.Background(), f.intent); err == nil || f.posts != 0 {
				t.Fatal("unsafe dispatch executed")
			}
		})
	}
}
func TestImageEvidenceRejectsUntrustedProof(t *testing.T) {
	for _, mutation := range []string{"workflow-moved", "workflow-hash", "tree-moved", "package-missing", "package-public", "wrong-run-sha", "rerun", "duplicate-run", "pending", "failed", "artifact-expired", "duplicate-artifact", "redirect-untrusted", "archive-path", "archive-extra", "duplicate-json", "trailing-json", "provenance-repository", "provenance-image", "provenance-source_sha", "provenance-source_app_tree_sha", "provenance-tag", "provenance-digest", "provenance-visibility", "provenance-dispatch_id", "provenance-workflow_sha", "registry-auth", "registry-tag", "registry-child", "registry-config", "registry-redirect-untrusted"} {
		t.Run(mutation, func(t *testing.T) {
			f := newImageFixture(t)
			f.run = true
			f.mutation = mutation
			if _, err := f.g.ImageEvidence(context.Background(), f.intent); err == nil {
				t.Fatal("untrusted proof accepted")
			}
		})
	}
}
func TestImageUnknownDispatchIsReconciledWithoutResend(t *testing.T) {
	f := newImageFixture(t)
	f.mutation = "dispatch-unknown"
	if _, err := f.g.Execute(context.Background(), f.intent); err == nil {
		t.Fatal("unknown result accepted")
	}
	if id, exists, err := f.g.Inspect(context.Background(), f.intent); err != nil || !exists || id != f.intent.ImageRelease.DispatchID {
		t.Fatalf("unknown reconciliation %s %v", id, err)
	}
	if _, err := f.g.Execute(context.Background(), f.intent); err == nil || f.posts != 1 {
		t.Fatal("unknown dispatch resent")
	}
}

func TestImageRegistryRejectsValidlyHashedWrongSource(t *testing.T) {
	for _, mutation := range []string{"missing-arm64", "wrong-source-label", "wrong-workflow-label"} {
		t.Run(mutation, func(t *testing.T) {
			f := newImageFixture(t, mutation)
			f.run = true
			if _, err := f.g.ImageEvidence(context.Background(), f.intent); err == nil {
				t.Fatal("untrusted registry proof admitted")
			}
		})
	}
}
