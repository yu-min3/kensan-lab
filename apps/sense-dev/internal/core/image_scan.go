package core

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"
)

func (d *ReleaseDriver) enrichImageScan(task Task, scan *ReleaseScan) error {
	image := deploymentImageForTask(d.Store.Snapshot(), task)
	if image == nil {
		if task.ImageSourceTaskID != "" || d.Plan.ImageWorkflow != nil && task.Team == Platform && task.CheckoutSourceTaskID != "" {
			return errors.New("deployment lacks verified image lineage")
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	repo := filepath.Join(d.WorktreeRoot, task.ID)
	raw, err := gitEvidence(ctx, repo, "rev-parse", "--verify", task.HeadSHA+":apps/canary")
	if err != nil {
		return err
	}
	tree := strings.TrimSpace(string(raw))
	kind, err := gitEvidence(ctx, repo, "cat-file", "-t", tree)
	if err != nil || strings.TrimSpace(string(kind)) != "tree" || tree != image.SourceAppTreeSHA {
		return errors.New("GitOps candidate changes the built App tree")
	}
	raw, err = gitEvidence(ctx, repo, "show", task.HeadSHA+":kubernetes/apps/app-canary/values.yaml")
	if err != nil {
		return err
	}
	values, err := strictValues(raw)
	if err != nil {
		return err
	}
	mapping, ok := values["image"].(map[string]any)
	if !ok || mapping["repository"] != CanaryImageRepository || mapping["digest"] != image.Digest || mapping["tag"] != nil && mapping["tag"] != "" {
		return errors.New("GitOps candidate does not pin verified image digest")
	}
	scan.SourceAppTreeSHA = tree
	scan.ImageDigest = image.Digest
	if task.ImageSourceTaskID == "" {
		return nil
	}
	if _, err := gitEvidence(ctx, repo, "merge-base", "--is-ancestor", image.SourceSHA, task.HeadSHA); err != nil {
		return errors.New("deployment child does not descend from image source")
	}
	commits, err := gitEvidence(ctx, repo, "rev-list", "--reverse", image.SourceSHA+".."+task.HeadSHA)
	if err != nil {
		return err
	}
	sequence := strings.Fields(string(commits))
	if len(sequence) == 0 || len(sequence) > 100 {
		return errors.New("deployment child needs reviewed values commit")
	}
	for _, commit := range sequence {
		parents, err := gitEvidence(ctx, repo, "rev-list", "--parents", "-n", "1", commit)
		if err != nil {
			return err
		}
		p := strings.Fields(string(parents))
		if len(p) != 2 {
			return errors.New("deployment child cannot add merge commits")
		}
		raw, err := gitEvidence(ctx, repo, "diff-tree", "--no-renames", "--no-commit-id", "--raw", "-r", "-z", commit)
		if err != nil {
			return err
		}
		paths, indirect, err := parseChangedPaths(raw)
		if err != nil || indirect || len(paths) != 1 || paths[0] != "kubernetes/apps/app-canary/values.yaml" {
			return errors.New("deployment child may change only existing canary image values")
		}
		before, err := gitEvidence(ctx, repo, "show", p[1]+":"+paths[0])
		if err != nil {
			return err
		}
		after, err := gitEvidence(ctx, repo, "show", commit+":"+paths[0])
		if err != nil {
			return err
		}
		if !imageOnlyValuesChange(before, after) {
			return errors.New("deployment child changed non-image values")
		}
	}
	scan.ImageValuesOnly = true
	return nil
}
