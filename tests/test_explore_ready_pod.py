import copy
import importlib.util
import io
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location(
    "ready_pod", Path(__file__).resolve().parents[1] / "scripts/explore-ready-pod.py")
ready = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ready)


def pod(name="new", image="image:v2"):
    return {"metadata": {"name": name, "uid": f"uid-{name}",
                         "labels": {"app.kubernetes.io/name": "widget"}},
            "spec": {"containers": [{"name": "app", "image": image}]},
            "status": {"conditions": [{"type": "Ready", "status": "True"}]}}


class PodSelectionTest(unittest.TestCase):
    def test_terminating_old_pod_in_either_order(self):
        old = pod("old", "image:v1")
        old["metadata"]["deletionTimestamp"] = "2026-09-06T11:21:31Z"
        for items in ([old, pod()], [pod(), old]):
            self.assertEqual(ready.select_pod(items, "widget", "image:v2"), ("new", "uid-new"))

    def test_rejects_ineligible_pods(self):
        cases = [pod(image="image:v1")]
        for section, key, value in [
            ("metadata", "deletionTimestamp", "2026-09-06T11:21:31Z"),
            ("metadata", "labels", {"app.kubernetes.io/name": "other"}),
            ("metadata", "uid", ""),
            ("status", "conditions", [{"type": "Ready", "status": "False"}]),
            ("status", "conditions", []),
            ("spec", "containers", [{"name": "sidecar", "image": "image:v2"}]),
        ]:
            item = pod()
            item[section][key] = value
            cases.append(item)
        for item in cases:
            with self.subTest(item=item):
                self.assertIsNone(ready.select_pod([item], "widget", "image:v2"))

    def test_missing_or_ambiguous_pods_are_not_success(self):
        for items in ([], [pod(), pod("another")], [{}]):
            self.assertIsNone(ready.select_pod(items, "widget", "image:v2"))

    def test_waits_until_expected_pod_is_ready(self):
        pending = copy.deepcopy(pod())
        pending["status"]["conditions"][0]["status"] = "False"
        with patch.object(ready, "kubectl", side_effect=[{"items": [pending]}, {"items": [pod()]}]), \
                patch.object(ready.time, "sleep") as sleep:
            self.assertEqual(ready.wait_for_pod("app-widget", "widget", "image:v2", 10),
                             ("new", "uid-new"))
            sleep.assert_called_once()

    def test_timeout_reports_pod_and_deployment_state(self):
        output = io.StringIO()
        with patch.object(ready, "kubectl", side_effect=[
            {"items": [pod("old", "image:v1")]},
            {"metadata": {"generation": 2}, "status": {"observedGeneration": 2}},
        ]), patch.object(ready.sys, "stderr", output):
            with self.assertRaises(TimeoutError):
                ready.wait_for_pod("app-widget", "widget", "image:v2", 0)
        self.assertIn("uid-old", output.getvalue())
        self.assertIn("observedGeneration", output.getvalue())


if __name__ == "__main__":
    unittest.main()
