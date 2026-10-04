package publisherbridge

import (
	"context"
	"errors"
	"regexp"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/publisher"
)

var evidenceDigestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func imageEvidenceMatches(i core.PublishIntent, e publisher.ImageEvidence) bool {
	spec := i.ImageRelease
	return spec != nil && e.Repository == "yu-min3/kensan-lab" && e.Image == core.CanaryImageRepository && e.SourceSHA == spec.SourceSHA && e.SourceAppTreeSHA == spec.SourceAppTreeSHA && e.ImageTag == spec.ImageTag && e.DispatchID == spec.DispatchID && e.WorkflowSHA == spec.WorkflowSHA && e.WorkflowSHA256 == spec.WorkflowSHA256 && e.WorkflowRunID > 0 && e.Visibility == "private" && evidenceDigestPattern.MatchString(e.Digest)
}
func (c Client) ImageEvidence(ctx context.Context, i core.PublishIntent) (publisher.ImageEvidence, error) {
	r, err := c.call(ctx, "image_evidence", i)
	if err != nil {
		return publisher.ImageEvidence{}, err
	}
	if r.ImageEvidence == nil || !imageEvidenceMatches(i, *r.ImageEvidence) {
		return publisher.ImageEvidence{}, errors.New("image evidence differs from fixed release inputs")
	}
	return *r.ImageEvidence, nil
}
