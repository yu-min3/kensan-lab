package core

import (
	"errors"
	"regexp"
	"strings"
)

const CanaryImageRepository = "ghcr.io/yu-min3/kensan-lab/canary"
const CanaryImageWorkflowPath = ".github/workflows/canary-image.yml"

var dispatchIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
var githubCommitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var workflowRefPattern = regexp.MustCompile(`^refs/tags/[A-Za-z0-9][A-Za-z0-9._/-]*$`)
var imageDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var imageTagPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)

// ValidateImageReleaseSpec fixes all image workflow inputs. The trusted
// operator plan supplies WorkflowRef/SHA/hash; workers cannot select them.
// SourceAppTreeSHA is independently taken from the controller Git scan.
func ValidateImageReleaseSpec(spec ImageReleaseSpec) error {
	if !githubCommitPattern.MatchString(spec.SourceSHA) || !githubCommitPattern.MatchString(spec.SourceAppTreeSHA) || !githubCommitPattern.MatchString(spec.WorkflowSHA) || !fullDigest(spec.WorkflowSHA256) || spec.WorkflowPath != CanaryImageWorkflowPath || !workflowRefPattern.MatchString(spec.WorkflowRef) || strings.Contains(spec.WorkflowRef, "..") || strings.Contains(spec.WorkflowRef, "//") || strings.HasSuffix(spec.WorkflowRef, "/") || strings.HasSuffix(spec.WorkflowRef, ".lock") || !dispatchIDPattern.MatchString(spec.DispatchID) || spec.ImageTag != "sense-"+spec.SourceSHA+"-"+spec.DispatchID || !imageTagPattern.MatchString(spec.ImageTag) {
		return errors.New("image release source, workflow, tag or dispatch identity is invalid")
	}
	return nil
}

func cloneImageRelease(spec *ImageReleaseSpec) *ImageReleaseSpec {
	if spec == nil {
		return nil
	}
	copy := *spec
	return &copy
}

func imageReleasesEqual(a, b *ImageReleaseSpec) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func validateCandidateImage(candidate ReleaseCandidate, scan ReleaseScan) error {
	if candidate.Operation != "image_publish" {
		if candidate.ImageRelease != nil {
			return errors.New("image release spec supplied for another operation")
		}
		return nil
	}
	if candidate.ImageRelease == nil || candidate.TargetEnvironment != "private-canary" || candidate.HeadSHA != candidate.ImageRelease.SourceSHA || scan.Team != App || scan.SourceAppTreeSHA != candidate.ImageRelease.SourceAppTreeSHA || !fullSHA(scan.SourceAppTreeSHA) {
		return errors.New("image publish must bind private App source and scanned tree")
	}
	return ValidateImageReleaseSpec(*candidate.ImageRelease)
}
