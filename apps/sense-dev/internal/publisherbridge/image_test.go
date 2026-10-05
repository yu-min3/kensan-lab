package publisherbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/publisher"
)

func validBridgeImageDeployment() *core.ImageDeploymentSpec {
	source, id := strings.Repeat("a", 40), strings.Repeat("b", 32)
	return &core.ImageDeploymentSpec{WorkflowRef: "refs/tags/trusted-image-v1", SourceSHA: source, SourceAppTreeSHA: strings.Repeat("c", 40), ImageTag: "sense-" + source + "-" + id, Digest: "sha256:" + strings.Repeat("d", 64), WorkflowSHA: strings.Repeat("e", 40), WorkflowSHA256: strings.Repeat("f", 64), DispatchID: id, WorkflowRunID: 42}
}
func imageBridgeIntent() core.PublishIntent {
	spec := validBridgeImageDeployment()
	i := bridgeIntent()
	i.Operation = "image_publish"
	i.Status = "sent"
	i.ExternalID = spec.DispatchID
	i.ExpiresAt = time.Now().Add(-time.Minute)
	i.ImageRelease = &core.ImageReleaseSpec{SourceSHA: spec.SourceSHA, SourceAppTreeSHA: spec.SourceAppTreeSHA, ImageTag: spec.ImageTag, WorkflowSHA: spec.WorkflowSHA, WorkflowSHA256: spec.WorkflowSHA256, WorkflowRef: "refs/tags/trusted-image-v1", WorkflowPath: core.CanaryImageWorkflowPath, DispatchID: spec.DispatchID}
	return i
}
func bridgeEvidence(i core.PublishIntent) publisher.ImageEvidence {
	s := i.ImageRelease
	return publisher.ImageEvidence{Repository: "yu-min3/kensan-lab", Image: core.CanaryImageRepository, SourceSHA: s.SourceSHA, SourceAppTreeSHA: s.SourceAppTreeSHA, ImageTag: s.ImageTag, Digest: "sha256:" + strings.Repeat("d", 64), Visibility: "private", DispatchID: s.DispatchID, WorkflowSHA: s.WorkflowSHA, WorkflowSHA256: s.WorkflowSHA256, WorkflowRunID: 42}
}
func TestImageEvidenceIsReadOnlyAndBoundToSentDispatchEvenAfterExpiry(t *testing.T) {
	transport := &fakeTransport{}
	calls := 0
	handler, err := HandlerWithImageEvidence(bridgeAuth(t), transport, nil, func(_ context.Context, i core.PublishIntent) (publisher.ImageEvidence, error) {
		calls++
		return bridgeEvidence(i), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*core.PublishIntent)
		want   int
	}{
		{"expired observation", nil, 200},
		{"pending dispatch", func(i *core.PublishIntent) { i.Status = "pending_reconcile" }, 403},
		{"unknown dispatch", func(i *core.PublishIntent) { i.Status = "unknown" }, 403},
		{"wrong dispatch", func(i *core.PublishIntent) { i.ExternalID = "other" }, 403},
		{"missing spec", func(i *core.PublishIntent) { i.ImageRelease = nil }, 403},
		{"wrong source", func(i *core.PublishIntent) { i.HeadSHA = strings.Repeat("b", 40) }, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			intent := imageBridgeIntent()
			if tc.mutate != nil {
				tc.mutate(&intent)
			}
			body, _ := json.Marshal(request{Action: "image_evidence", Intent: intent})
			r := httptest.NewRequest(http.MethodPost, "/v1/publish", bytes.NewReader(body))
			r.Header.Set("Authorization", "Bearer "+strings.Repeat("x", 40))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status %d", w.Code)
			}
		})
	}
	if calls != 1 || transport.calls.Load() != 0 {
		t.Fatal("read-only observation invoked execution")
	}
	wrong, err := HandlerWithImageEvidence(bridgeAuth(t), transport, nil, func(_ context.Context, i core.PublishIntent) (publisher.ImageEvidence, error) {
		e := bridgeEvidence(i)
		e.Visibility = "public"
		return e, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(request{Action: "image_evidence", Intent: imageBridgeIntent()})
	r := httptest.NewRequest(http.MethodPost, "/v1/publish", bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("x", 40))
	w := httptest.NewRecorder()
	wrong.ServeHTTP(w, r)
	if w.Code == 200 {
		t.Fatal("unbound public image evidence accepted")
	}
}

func TestBridgeDoesNotAcceptPackageCredentialConfiguration(t *testing.T) {
	calls := 0
	handler, err := HandlerWithImageEvidence(bridgeAuth(t), &fakeTransport{}, nil, func(_ context.Context, i core.PublishIntent) (publisher.ImageEvidence, error) {
		calls++
		return bridgeEvidence(i), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"token_file", "package_token_file", "observer_package_token_file"} {
		input := map[string]any{"action": "image_evidence", "intent": imageBridgeIntent(), field: "/worker/provided-token"}
		body, _ := json.Marshal(input)
		r := httptest.NewRequest(http.MethodPost, "/v1/publish", bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+strings.Repeat("x", 40))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("worker credential field accepted: %s", field)
		}
	}
	if calls != 0 {
		t.Fatal("credential injection reached host observation")
	}
}
