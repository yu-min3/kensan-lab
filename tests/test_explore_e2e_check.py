"""Verify overlap and failure propagation without starting Kubernetes."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


class ConcurrentChecksTest(unittest.TestCase):
    def run_checks(self, refresh_exit=0, golden_exit=0, refresh_output="200"):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            scripts = root / "scripts"
            scripts.mkdir()
            shutil.copyfile(
                Path(__file__).parents[1] / "scripts/explore-e2e-check.sh",
                scripts / "explore-e2e-check.sh",
            )
            for name, other, status, output in [
                ("login", "golden-path", refresh_exit, refresh_output),
                ("golden-path", "login", golden_exit, "built"),
            ]:
                helper = scripts / f"explore-{name}-check.sh"
                helper.write_text(
                    '#!/usr/bin/env bash\nset -eu\n'
                    f'touch "$FIXTURE/{name}"\n'
                    # Neither helper can finish until the other has started.
                    f'for i in {{1..100}}; do\n'
                    f'  if [ -f "$FIXTURE/{other}" ]; then\n'
                    f'    echo "{output}"; exit {status}\n'
                    '  fi\n  sleep 0.01\ndone\nexit 99\n'
                )
                helper.chmod(0o755)
            return subprocess.run(
                ["bash", str(scripts / "explore-e2e-check.sh")],
                env={**os.environ, "FIXTURE": str(root)},
                capture_output=True, text=True, timeout=10,
            )

    def test_both_checks_overlap_and_pass(self):
        result = self.run_checks()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("both passed", result.stdout)

    def test_refresh_failure_fails_job(self):
        self.assertNotEqual(self.run_checks(refresh_exit=1).returncode, 0)

    def test_golden_path_failure_fails_job(self):
        self.assertNotEqual(self.run_checks(golden_exit=1).returncode, 0)

    def test_wrong_status_fails_job(self):
        self.assertNotEqual(self.run_checks(refresh_output="302").returncode, 0)


if __name__ == "__main__":
    unittest.main()
