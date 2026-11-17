import importlib.util
from pathlib import Path
import re
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("paths", ROOT / "scripts/check-explore-ci-paths.py")
paths = importlib.util.module_from_spec(spec)
spec.loader.exec_module(paths)


def matches(pattern, name):
    # Generated filters use literals plus GitHub's *, ** and ? wildcards.
    regex = re.escape(pattern).replace(r"\*\*", ".*").replace(r"\*", "[^/]*").replace(r"\?", "[^/]")
    return re.fullmatch(regex, name) is not None


class ExplorePathsTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.filters = paths.dependencies(ROOT)

    def selected(self, name):
        result = False
        for pattern in self.filters:
            if matches(pattern.lstrip("!"), name):
                result = not pattern.startswith("!")
        return result

    def test_shared_runtime_and_build_inputs(self):
        for name in ["charts/app-base/templates/deployment.yaml",
                     "kubernetes/network/istio/istiod/values.yaml",
                     "kubernetes/auth/keycloak/keycloak-service.yaml",
                     "kubernetes/policy/kyverno-policies/exceptions/new.yaml",
                     "kubernetes/argocd/projects/new.yaml",
                     "backstage/yarn.lock", "backstage/catalog/explore/demo.yaml",
                     "backstage/templates/fastapi-template/skeleton/docs/index.md",
                     "scripts/explore-ready-pod.py", "environments/kind/new-file.yaml"]:
            with self.subTest(name=name):
                self.assertTrue(self.selected(name))

    def test_production_only_changes_are_excluded(self):
        for name in ["kubernetes/apps/app-kensan/values.yaml",
                     "kubernetes/storage/longhorn/values.yaml",
                     "kubernetes/secrets/vault/values.yaml",
                     "charts/unrelated/templates/deployment.yaml",
                     "bootstrap/keycloak/setup.sh", "apps/kensan/main.go",
                     "docs/getting-started/try-it-with-kind.md"]:
            with self.subTest(name=name):
                self.assertFalse(self.selected(name))

    def test_new_files_in_shared_components_are_covered(self):
        for name in ["kubernetes/backstage/new-resource.yaml",
                     "kubernetes/auth/keycloak/new-resource.yaml",
                     "kubernetes/network/istio/new-component/new-values.yaml",
                     "kubernetes/observability/grafana/resources/new-dashboard.yaml",
                     "kubernetes/secrets/cert-manager/new-issuer.yaml"]:
            with self.subTest(name=name):
                self.assertTrue(self.selected(name))

    def test_production_files_within_shared_components_are_intentionally_covered(self):
        self.assertTrue(self.selected("kubernetes/backstage/backstage-deployment.yaml"))
        self.assertTrue(self.selected("kubernetes/network/istio/gateway-prod.yaml"))

    def test_comparison_only_and_non_runtime_scripts_are_excluded(self):
        for name in ["kubernetes/argocd/applications/apps/app-konro/app.yaml",
                     "kubernetes/argocd/applications/network/istiod/app.yaml",
                     "kubernetes/argocd/applications/gitops/argocd/app.yaml",
                     "kubernetes/argocd/applications/observability/applicationset.yaml",
                     "scripts/explore-screenshots.py", "scripts/explore-unused-check.sh"]:
            with self.subTest(name=name):
                self.assertFalse(self.selected(name))

    def test_markdown_exclusions_do_not_hide_template_documents(self):
        self.assertFalse(self.selected("charts/app-base/README.md"))
        self.assertFalse(self.selected("environments/kind/README.md"))
        self.assertTrue(self.selected("backstage/templates/new/README.md"))

    def test_new_chart_reference_changes_required_filters(self):
        app = 'kind: Application\nmetadata: {name: new}\nspec:\n  source:\n    path: charts/new-shared\n'
        with patch.object(paths.subprocess, "check_output", return_value=app):
            self.assertIn("charts/new-shared/**", paths.dependencies(ROOT))

    def test_missing_shared_filter_fails_static_check(self):
        text = (ROOT / ".github/workflows/explore-ci.yml").read_text()
        text = text.replace('      - "charts/app-base/**"\n', "")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            workflow = root / ".github/workflows/explore-ci.yml"
            workflow.parent.mkdir(parents=True)
            workflow.write_text(text)
            with patch.object(paths, "ROOT", root), patch.object(paths, "dependencies", return_value=self.filters):
                with patch("sys.argv", ["check-explore-ci-paths.py"]):
                    with self.assertRaises(SystemExit):
                        paths.main()


if __name__ == "__main__":
    unittest.main()
