package core

import (
	"bytes"
	"context"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

var authorizationPattern = regexp.MustCompile(`(?im)(kind:\s*(?:Role|ClusterRole|RoleBinding|ClusterRoleBinding|AuthorizationPolicy)|gatewayOAuth2|securityContext|podSecurityContext|allowPrivilegeEscalation|runAsNonRoot|hostNetwork|hostPID|hostIPC|serviceAccount|automountServiceAccountToken)`)
var secretPolicyPattern = regexp.MustCompile(`(?im)(kind:\s*(?:Secret|SealedSecret|ExternalSecret)|secretKeyRef|secretRef|vault|kubeconfig|permissions:|id-token:|secrets\.)`)
var publicationPolicyPattern = regexp.MustCompile(`(?im)(httproute|hostnames:|ingress:|kind:\s*(?:HTTPRoute|Gateway|Ingress|NetworkPolicy)|cloudflare|cloudflared|trycloudflare|ngrok|tailscale\s+funnel|type:\s*(?:NodePort|LoadBalancer))`)
var destructionPattern = regexp.MustCompile(`(?im)(Prune|reclaimPolicy|kind:\s*(?:Namespace|PersistentVolumeClaim|PersistentVolume)|storageClass)`)

// Classification is independent of the gate model's wording. Reasons contain
// fixed labels and paths only, so credential contents cannot enter cards/logs.
func classifyReleaseCommit(paths []string, patch []byte, indirect bool) (map[string]bool, []string, bool) {
	categories := map[string]bool{}
	var reasons []string
	add := func(category, reason string) { categories[category] = true; reasons = append(reasons, reason) }
	for _, path := range paths {
		p := strings.ToLower(path)
		switch {
		case strings.HasPrefix(p, "apps/sense-dev/"):
			add("control", "controller or release control change: "+path)
		case strings.HasPrefix(p, "kubernetes/auth/") || strings.HasPrefix(p, "bootstrap/keycloak/") || strings.HasPrefix(p, "kubernetes/policy/"):
			add("auth", "authentication or authorization policy path: "+path)
		case strings.HasPrefix(p, "kubernetes/secrets/") || sensitivePath(path):
			add("secret", "secret or credential policy path: "+path)
		case strings.HasPrefix(p, "kubernetes/network/") || strings.HasPrefix(p, "network/"):
			add("publication", "network or publication policy path: "+path)
		case highRiskPath(path):
			add("control", "deployment or control policy path: "+path)
		}
	}
	changed := changedPatchLines(patch)
	if authorizationPattern.Match(changed) {
		add("auth", "authentication, permission or pod security settings changed")
	}
	if secretPolicyPattern.Match(changed) {
		add("secret", "secret references or CI credential permissions changed")
	}
	if publicationPolicyPattern.Match(changed) || publicRoutePattern.Match(changed) {
		add("publication", "publication or network exposure settings changed")
	}
	if destructionPattern.Match(changed) {
		add("data_destruction", "resource lifecycle, storage or Prune settings changed")
	}
	if indirect {
		add("control", "symlink or submodule change requires explicit review")
	}
	return categories, reasons, credentialPattern.Match(patch) || rawSecretAssignmentPattern.Match(changed)
}

func changedPatchLines(patch []byte) []byte {
	var out bytes.Buffer
	for _, line := range bytes.Split(patch, []byte{'\n'}) {
		if len(line) > 0 && (line[0] == '+' || line[0] == '-') && !bytes.HasPrefix(line, []byte("+++")) && !bytes.HasPrefix(line, []byte("---")) {
			out.Write(line[1:])
			out.WriteByte('\n')
		}
	}
	return out.Bytes()
}

// ClassifyPlatformProposal is advisory for feedback triage. Its classification
// never substitutes for the scan and approval of the later committed change.
func ClassifyPlatformProposal(paths []string, description string) (categories, reasons []string) {
	set, details, _ := classifyReleaseCommit(paths, []byte("+"+strings.ReplaceAll(description, "\n", "\n+")), false)
	text := strings.ToLower(description)
	for category, words := range map[string][]string{
		"auth":             {"auth", "rbac", "keycloak", "認証", "認可", "権限"},
		"secret":           {"secret", "credential", "token", "vault", "kubeconfig", "秘密"},
		"publication":      {"route", "host", "gateway", "tunnel", "cloudflare", "public", "公開"},
		"control":          {"release gate", "publisher", "ownership", "sense-dev", "統制", "許可範囲"},
		"data_destruction": {"prune", "reclaim", "delete namespace", "delete pvc", "delete data", "namespace削除", "pvc削除", "データ削除", "破壊"},
	} {
		for _, word := range words {
			if strings.Contains(text, word) {
				set[category] = true
				details = append(details, "proposal mentions "+category)
				break
			}
		}
	}
	return sortedSet(set), details
}

func candidateClassification(scan ReleaseScan, candidate ReleaseCandidate) ReleaseScan {
	categories, reasons := ClassifyPlatformProposal(nil, candidate.Impact+"\n"+candidate.PullRequestSummary)
	cs, rs := map[string]bool{}, map[string]bool{}
	for _, c := range append(scan.HumanCategories, categories...) {
		cs[c] = true
	}
	for _, r := range append(scan.HumanReasons, reasons...) {
		rs[r] = true
	}
	scan.HumanCategories, scan.HumanReasons = sortedSet(cs), sortedSet(rs)
	if scan.Status == "candidate" && len(scan.HumanCategories) > 0 {
		scan.Status = "needs_human"
	}
	return scan
}

func classifyAppBaseValues(ctx context.Context, repo, commit string, parents, paths []string) (map[string]bool, []string, bool) {
	set := map[string]bool{}
	var reasons []string
	denied := false
	for _, path := range paths {
		if !strings.HasPrefix(path, "kubernetes/apps/") || !strings.HasSuffix(path, "/values.yaml") {
			continue
		}
		if len(parents) != 2 {
			set["control"] = true
			reasons = append(reasons, "values commit topology requires review: "+path)
			continue
		}
		before, oldErr := gitEvidence(ctx, repo, "show", parents[1]+":"+path)
		after, newErr := gitEvidence(ctx, repo, "show", commit+":"+path)
		if oldErr != nil || newErr != nil {
			set["control"] = true
			reasons = append(reasons, "values provisioning or removal requires review: "+path)
			if newErr == nil {
				value, err := strictValues(after)
				if err != nil {
					denied = true
				} else {
					for _, key := range []string{"httproute", "ingress", "route"} {
						if route, ok := value[key].(map[string]any); ok && route["enabled"] == true {
							set["publication"] = true
							denied = true
						}
					}
				}
			}
			continue
		}
		a, ae := strictValues(before)
		b, be := strictValues(after)
		if ae != nil || be != nil {
			set["control"] = true
			reasons = append(reasons, "ambiguous values require review: "+path)
			denied = true
			continue
		}
		keys := map[string]bool{}
		for key := range a {
			keys[key] = true
		}
		for key := range b {
			keys[key] = true
		}
		for key := range keys {
			if reflect.DeepEqual(a[key], b[key]) {
				continue
			}
			category := ""
			switch key {
			case "httproute", "ingress", "route", "host":
				category = "publication"
				if route, ok := b[key].(map[string]any); ok && route["enabled"] == true {
					denied = true
				}
			case "auth", "securityContext", "podSecurityContext", "serviceAccount":
				category = "auth"
			case "ghcrPullSecret":
				category = "secret"
			case "pvc":
				category = "data_destruction"
			case "env":
				for _, value := range []any{a[key], b[key]} {
					if entries, ok := value.([]any); ok {
						for _, entry := range entries {
							if env, ok := entry.(map[string]any); ok {
								if env["valueFrom"] != nil {
									category = "secret"
								}
								name, _ := env["name"].(string)
								name = strings.ToLower(name)
								if env["value"] != nil && (strings.Contains(name, "password") || strings.Contains(name, "token") || strings.Contains(name, "secret") || strings.Contains(name, "api_key")) {
									category = "secret"
									denied = true
								}
							}
						}
					}
				}
			case "image", "replicas", "nameOverride", "ports", "resources", "nodeSelector", "affinity", "probes", "otel", "reloader":
			default:
				category = "control"
			}
			if category != "" {
				set[category] = true
				reasons = append(reasons, "values key "+key+" changed: "+path)
			}
		}
	}
	return set, reasons, denied
}

func sortedSet(set map[string]bool) []string {
	var values []string
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}
