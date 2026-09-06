import importlib.util
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).parents[1] / "scripts" / "check-llms-coverage.py"
SPEC = importlib.util.spec_from_file_location("check_llms_coverage", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


CONFIG = """\
site_name: test
docs_dir: docs

exclude_docs: |
  bootstrapping/scripts/

theme:
  name: material
  features:
    - navigation.tabs

plugins:
  - search
  - llmstxt:
      sections:
        Overview:
          - index.md
        Architecture:
          - architecture/*.md

nav:
  - Home: index.md
"""


def _repo(files):
    directory = tempfile.TemporaryDirectory()
    repo = Path(directory.name)
    (repo / "mkdocs.yml").write_text(CONFIG, encoding="utf-8")
    for name in files:
        path = repo / "docs" / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text("# page\n", encoding="utf-8")
    return directory, repo


class CoverageTest(unittest.TestCase):
    def test_passes_when_every_page_matches_a_section(self):
        directory, repo = _repo(["index.md", "architecture/network.md"])
        with directory:
            self.assertEqual(MODULE.main([str(repo / "mkdocs.yml")]), 0)

    def test_glob_reaches_into_subdirectories(self):
        # architecture/*.md has to cover architecture/network/index.md, which is
        # the shape every domain overview takes since the nav restructure.
        directory, repo = _repo(["index.md", "architecture/network/index.md"])
        with directory:
            self.assertEqual(MODULE.main([str(repo / "mkdocs.yml")]), 0)

    def test_fails_on_a_page_no_section_matches(self):
        directory, repo = _repo(["index.md", "newthing/foo.md"])
        with directory:
            self.assertEqual(MODULE.main([str(repo / "mkdocs.yml")]), 1)

    def test_honours_exclude_docs(self):
        # Excluded paths never reach the site, so they must not be demanded of
        # llms.txt either.
        directory, repo = _repo(["index.md", "bootstrapping/scripts/setup.md"])
        with directory:
            self.assertEqual(MODULE.main([str(repo / "mkdocs.yml")]), 0)

    def test_fails_when_a_page_is_in_two_sections(self):
        directory, repo = _repo(["index.md"])
        with directory:
            config = (repo / "mkdocs.yml").read_text(encoding="utf-8")
            config = config.replace(
                "        Architecture:\n          - architecture/*.md",
                "        Architecture:\n          - index.md",
            )
            (repo / "mkdocs.yml").write_text(config, encoding="utf-8")
            self.assertEqual(MODULE.main([str(repo / "mkdocs.yml")]), 1)

    def test_no_llmstxt_plugin_is_not_a_failure(self):
        directory, repo = _repo(["index.md"])
        with directory:
            config = (repo / "mkdocs.yml").read_text(encoding="utf-8")
            start = config.index("  - llmstxt:")
            end = config.index("nav:")
            (repo / "mkdocs.yml").write_text(config[:start] + config[end:], encoding="utf-8")
            self.assertEqual(MODULE.main([str(repo / "mkdocs.yml")]), 0)


if __name__ == "__main__":
    unittest.main()
