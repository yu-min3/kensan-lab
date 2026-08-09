import importlib.util
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).parents[1] / "scripts" / "check-github-render.py"
SPEC = importlib.util.spec_from_file_location("check_github_render", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class DualRenderedTest(unittest.TestCase):
    def test_discovers_single_block_and_suffixed_includes(self):
        with tempfile.TemporaryDirectory() as directory:
            repo = Path(directory)
            docs = repo / "docs"
            docs.mkdir()
            for name in ("single.md", "block.md", "section.md"):
                (repo / name).write_text(name, encoding="utf-8")
            (docs / "page.md").write_text(
                '\n'.join(
                    (
                        '-8<- "single.md"',
                        '--8<--',
                        'block.md',
                        '; ignored.md',
                        'section.md:section-name',
                        '--8<--',
                    )
                ),
                encoding="utf-8",
            )

            found = {path.name for path in MODULE.dual_rendered(repo)}

            self.assertEqual(found, {"single.md", "block.md", "section.md"})

    def test_expands_explicit_directory(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "nested").mkdir()
            (root / "one.md").write_text("one", encoding="utf-8")
            (root / "nested" / "two.md").write_text("two", encoding="utf-8")
            (root / "ignored.txt").write_text("ignored", encoding="utf-8")

            found = MODULE.markdown_files([root])

            self.assertEqual({path.name for path in found}, {"one.md", "two.md"})


if __name__ == "__main__":
    unittest.main()
