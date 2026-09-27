package core

import (
	"strings"
	"testing"
)

func TestReleaseCandidateMustDescribeExactScannedOperation(t *testing.T) {
	scan := ReleaseScan{Operation: "pr_create", Repository: "yu-min3/kensan-lab", Ref: "refs/heads/feat/canary", HeadSHA: strings.Repeat("a", 40)}
	valid := ReleaseCandidate{SchemaVersion: 1, Operation: scan.Operation, Repository: scan.Repository, Ref: scan.Ref, HeadSHA: scan.HeadSHA, TargetEnvironment: "github", Impact: "private code branch", Rollback: "close PR"}
	if err := validateReleaseCandidate(valid, scan); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*ReleaseCandidate){
		func(c *ReleaseCandidate) { c.HeadSHA = strings.Repeat("b", 40) },
		func(c *ReleaseCandidate) { c.Operation = "merge" },
		func(c *ReleaseCandidate) { c.TargetEnvironment = "public" },
		func(c *ReleaseCandidate) { c.Impact = "" },
		func(c *ReleaseCandidate) { c.Rollback = "" },
	} {
		bad := valid
		mutate(&bad)
		if err := validateReleaseCandidate(bad, scan); err == nil {
			t.Fatalf("unsafe candidate accepted: %+v", bad)
		}
	}
}
