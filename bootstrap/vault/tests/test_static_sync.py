"""Regression checks for operator-initiated secret transfers. No credentials or network."""
import base64
from contextlib import nullcontext
import importlib.util
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import unittest
from unittest.mock import MagicMock, patch

SOURCE = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(SOURCE))
from kv_sync import sync_fields


def load(name):
    spec = importlib.util.spec_from_file_location(name, SOURCE / (name + ".py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def response(code, value=b"", error=b""):
    return subprocess.CompletedProcess([], code, stdout=value, stderr=error)


class StaticSync(unittest.TestCase):
    @patch.dict(os.environ, {"VAULT_ADDR": "http://example.invalid"})
    @patch("subprocess.run")
    def test_merge_preserves_concurrent_edit(self, run):
        record = {"data": {"metadata": {"version": 7}, "data": {"unrelated": "keep"}}}
        run.side_effect = [response(0, json.dumps(record).encode()), response(2)]
        with self.assertRaisesRegex(ValueError, "version"):
            sync_fields("secret/demo", {"field": "dummy"})
        call = run.call_args
        self.assertIn("-cas=7", call.args[0])
        self.assertNotIn("dummy", str(call.args[0]))
        self.assertEqual(json.loads(call.kwargs["input"]), {"field": "dummy", "unrelated": "keep"})

    @patch.dict(os.environ, {"VAULT_ADDR": "http://example.invalid"})
    @patch("subprocess.run")
    def test_denied_read_never_writes(self, run):
        run.return_value = response(2, error=b"permission denied")
        with self.assertRaisesRegex(ValueError, "read failed"):
            sync_fields("secret/demo", {"field": "dummy"})
        self.assertEqual(run.call_count, 1)

    def test_pat_from_one_environment_feeds_both_consumers(self):
        mod = load("sync-github-read")
        auth = base64.b64encode(b"yu-min3:dummy-read").decode()
        child = MagicMock()
        child.poll.return_value = None
        headers = {"X-OAuth-Scopes": "read:packages"}
        with (
            patch.object(sys, "argv", ["sync-github-read.py", "--check"]),
            patch.dict(os.environ, {"KENSAN_OP_ENVIRONMENT": "test-id", "READ_GHCR_PAT": "wrong"}),
            patch.object(mod.shutil, "which", return_value="/bin/example"),
            patch.object(mod, "request", side_effect=[({"login": "yu-min3"}, headers),
                         ({"token": "test"}, {}), ({"token": "test"}, {})]),
            patch.object(mod, "capture", side_effect=["dummy-read", "{}"]) as capture,
            patch.object(subprocess, "Popen", return_value=child),
            patch.object(socket, "socket") as sock,
            patch.object(socket, "create_connection", return_value=nullcontext()),
            patch.object(mod, "sync_fields", side_effect=[True, False]) as publish,
        ):
            sock.return_value.__enter__.return_value.getsockname.return_value = ("127.0.0.1", 1000)
            self.assertEqual(mod.main(), 1)
            source = capture.call_args_list[0]
            self.assertIn("test-id", source.args[0])
            self.assertNotIn("READ_GHCR_PAT", source.kwargs["env"])
            self.assertNotIn("WRITE_GHCR_PAT", str(source.args[0]))
            first, second = publish.call_args_list
            self.assertEqual(first.args[0], "secret/ghcr/pull-token")
            self.assertEqual(json.loads(first.args[1]["dockerconfigjson"])["auths"]["ghcr.io"]["auth"], auth)
            self.assertEqual(second.args, ("secret/backstage/github", {"GITHUB_TOKEN": "dummy-read"}, True))
            self.assertEqual(capture.call_count, 2)  # No ESO updates during --check.
