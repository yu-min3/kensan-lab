package deploymentobserver

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// appTreeAt reads actual Git objects from GitHub. Commit ancestry is checked
// by the publisher; this binds the canary build input across A, B and C.
func appTreeAt(ctx context.Context, token, commitSHA string) (string, error) {
	if !shaPattern.MatchString(commitSHA) {
		return "", errors.New("Git tree revision is not a full SHA")
	}
	var commit struct{ Tree struct{ SHA string } }
	if err := githubGET(ctx, token, "/repos/"+repository+"/git/commits/"+commitSHA, &commit); err != nil {
		return "", err
	}
	if !shaPattern.MatchString(commit.Tree.SHA) {
		return "", errors.New("Git commit has no tree")
	}
	apps, err := childTree(ctx, token, commit.Tree.SHA, "apps")
	if err != nil {
		return "", err
	}
	return childTree(ctx, token, apps, "canary")
}

func childTree(ctx context.Context, token, parentSHA, name string) (string, error) {
	if !shaPattern.MatchString(parentSHA) {
		return "", errors.New("Git parent tree is invalid")
	}
	var tree struct {
		Truncated bool
		Tree      []struct{ Path, Mode, Type, SHA string }
	}
	if err := githubGET(ctx, token, "/repos/"+repository+"/git/trees/"+parentSHA, &tree); err != nil {
		return "", err
	}
	if tree.Truncated {
		return "", errors.New("Git tree response was truncated")
	}
	var match string
	for _, entry := range tree.Tree {
		if entry.Path == name && entry.Type == "tree" && entry.Mode == "040000" && shaPattern.MatchString(entry.SHA) {
			if match != "" {
				return "", errors.New("duplicate Git tree path")
			}
			match = entry.SHA
		}
	}
	if match == "" {
		return "", fmt.Errorf("required %s tree absent", name)
	}
	return match, nil
}

func imageValuesAt(ctx context.Context, token, mergeSHA, digest string) error {
	if !shaPattern.MatchString(mergeSHA) || !digestPattern.MatchString(digest) {
		return errors.New("values revision or digest invalid")
	}
	var content struct {
		Type, Encoding, Content, SHA string
		Size                         int
	}
	path := "/repos/" + repository + "/contents/kubernetes/apps/app-canary/values.yaml?ref=" + mergeSHA
	if err := githubGET(ctx, token, path, &content); err != nil {
		return err
	}
	if content.Type != "file" || content.Encoding != "base64" || content.Size <= 0 || content.Size > 32<<10 || !shaPattern.MatchString(content.SHA) {
		return errors.New("canary values file is unavailable")
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(content.Content, "\n", ""))
	if err != nil || len(decoded) != content.Size {
		return errors.New("canary values content invalid")
	}
	return validateImageValues(decoded, digest)
}

func workflowHashAt(ctx context.Context, token, workflowSHA, expectedDigest string) error {
	if !shaPattern.MatchString(workflowSHA) || len(expectedDigest) != 64 || !digestPattern.MatchString("sha256:"+expectedDigest) {
		return errors.New("trusted workflow identity invalid")
	}
	var content struct {
		Type, Encoding, Content, SHA string
		Size                         int
	}
	path := "/repos/" + repository + "/contents/.github/workflows/canary-image.yml?ref=" + workflowSHA
	if err := githubGET(ctx, token, path, &content); err != nil {
		return err
	}
	if content.Type != "file" || content.Encoding != "base64" || content.Size <= 0 || content.Size > 64<<10 || !shaPattern.MatchString(content.SHA) {
		return errors.New("trusted workflow content unavailable")
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(content.Content, "\n", ""))
	if err != nil || len(decoded) != content.Size {
		return errors.New("trusted workflow content invalid")
	}
	sum := sha256.Sum256(decoded)
	if hex.EncodeToString(sum[:]) != expectedDigest {
		return errors.New("trusted workflow content differs from Gate hash")
	}
	return nil
}

func validateImageValues(data []byte, digest string) error {
	if !digestPattern.MatchString(digest) {
		return errors.New("proven image digest invalid")
	}
	var values struct {
		Image struct{ Repository, Tag, Digest string }
	}
	if err := yaml.Unmarshal(data, &values); err != nil {
		return errors.New("canary values YAML invalid")
	}
	if values.Image.Repository != image || values.Image.Tag != "" || values.Image.Digest != digest {
		return errors.New("deployed canary values do not pin the proven digest")
	}
	return nil
}
