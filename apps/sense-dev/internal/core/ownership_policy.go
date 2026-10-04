package core

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Ownership is controller policy, never supplied by an agent or task. Expanding
// the canary scope requires a policy revision and human review.
func appOwnedPath(path string) (values bool, allowed bool) {
	if strings.HasPrefix(path, "apps/canary/") {
		return false, true
	}
	return path == "kubernetes/apps/app-canary/values.yaml", path == "kubernetes/apps/app-canary/values.yaml"
}

func scanMatchesAuthorTeam(scan ReleaseScan, author Agent, task Task) bool {
	team := scan.Team
	if team == "" { // Old controller scans only covered Platform changes.
		team = Platform
	}
	return (team == App || team == Platform) && team == author.Team && team == task.Team
}

func appCommitOwnership(ctx context.Context, repo, commit string, parents []string, paths []string, indirect bool) []string {
	var findings []string
	if len(parents) != 2 || indirect {
		findings = append(findings, "App ownership denies merge, root, symlink or submodule change")
	}
	for _, path := range paths {
		values, owned := appOwnedPath(path)
		if !owned || sensitivePath(path) {
			findings = append(findings, "App ownership denies path: "+path)
			continue
		}
		if !values {
			continue
		}
		// Values must already exist: provisioning/removal belongs to Platform.
		if len(parents) != 2 {
			continue
		}
		before, errBefore := gitEvidence(ctx, repo, "show", parents[1]+":"+path)
		after, errAfter := gitEvidence(ctx, repo, "show", commit+":"+path)
		if errBefore != nil || errAfter != nil || appValuesChange(before, after) != nil {
			findings = append(findings, "App ownership denies values keys or content: "+path)
		}
	}
	return findings
}

// Decode YAML structurally; line matching would miss nested keys, removals,
// flow syntax, duplicates and anchors. Fail closed on ambiguous YAML.
func strictValues(body []byte) (map[string]any, error) {
	d := yaml.NewDecoder(bytes.NewReader(body))
	var node yaml.Node
	if err := d.Decode(&node); err != nil || len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("values must be one mapping")
	}
	var trailing yaml.Node
	if err := d.Decode(&trailing); err != io.EOF {
		return nil, errors.New("values must be a single YAML document")
	}
	var validate func(*yaml.Node) error
	validate = func(n *yaml.Node) error {
		if n.Kind == yaml.AliasNode || n.Anchor != "" {
			return errors.New("values aliases and anchors are not supported")
		}
		if n.Kind == yaml.MappingNode {
			seen := map[string]bool{}
			for i := 0; i < len(n.Content); i += 2 {
				key := n.Content[i]
				if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Value == "<<" || seen[key.Value] {
					return errors.New("values keys must be unique strings")
				}
				seen[key.Value] = true
			}
		}
		for _, c := range n.Content {
			if err := validate(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := validate(&node); err != nil {
		return nil, err
	}
	var value map[string]any
	if err := node.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func appValuesChange(before, after []byte) error {
	a, err := strictValues(before)
	if err != nil {
		return err
	}
	b, err := strictValues(after)
	if err != nil {
		return err
	}
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	for key := range keys {
		old, oldOK := a[key]
		next, nextOK := b[key]
		if oldOK == nextOK && reflect.DeepEqual(old, next) {
			continue
		}
		switch key {
		case "image":
			if !nextOK || validAppImage(next) != nil || oldOK && validAppImage(old) != nil {
				return errors.New("invalid image change")
			}
		case "replicas":
			if !nextOK {
				return errors.New("replicas cannot be removed")
			}
			n, ok := next.(int)
			if !ok || n < 0 || n > 10 {
				return errors.New("replicas outside delegated range")
			}
		case "env":
			if oldOK && validAppEnv(old) != nil || nextOK && validAppEnv(next) != nil {
				return errors.New("env change outside non-secret allowlist")
			}
		default:
			return errors.New("values key outside App ownership")
		}
	}
	return nil
}

var imageValuePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/:@-]*$`)

func validAppImage(value any) error {
	image, ok := value.(map[string]any)
	if !ok || len(image) == 0 {
		return errors.New("image must be a mapping")
	}
	for key, value := range image {
		s, ok := value.(string)
		if !ok || !imageValuePattern.MatchString(s) || len(s) > 512 {
			return errors.New("invalid image field")
		}
		switch key {
		case "repository", "tag":
		case "pullPolicy":
			if s != "Always" && s != "IfNotPresent" && s != "Never" {
				return errors.New("invalid pull policy")
			}
		default:
			return errors.New("unknown image key")
		}
	}
	if image["repository"] == nil || image["tag"] == nil {
		return errors.New("image repository and tag required")
	}
	return nil
}

func validAppEnv(value any) error {
	env, ok := value.([]any)
	if !ok {
		return errors.New("env must be a list")
	}
	seen := map[string]bool{}
	for _, entry := range env {
		m, ok := entry.(map[string]any)
		if !ok || len(m) != 2 {
			return errors.New("env must contain only name and value")
		}
		name, nameOK := m["name"].(string)
		value, valueOK := m["value"].(string)
		if !nameOK || !valueOK || seen[name] || len(value) > 4000 || credentialPattern.MatchString(value) {
			return errors.New("invalid or sensitive env")
		}
		switch name {
		case "APP_NAME", "APP_THEME", "APP_MESSAGE", "LOG_LEVEL":
		default:
			return errors.New("env name outside fixed non-secret allowlist")
		}
		seen[name] = true
	}
	return nil
}
