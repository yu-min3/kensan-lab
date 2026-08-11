import tempfile
import unittest
from pathlib import Path

from scripts.sdd_gate import classify, validate


SPEC = """---
id: 001-fixture
mode: {mode}
status: draft
created: 2026-08-11
updated: 2026-08-11
---
# Spec: Fixture
## Acceptance criteria
- [ ] observable result
"""

REVIEW = """# Review
- Reviewer: Codex (read-only)
- Result: findings
- Fallback approval: N/A
- Human fallback reviewer: N/A
- Human fallback result: N/A

| Priority | Finding | Decision | Rationale / action |
|---|---|---|---|
| P1 | example | Adopt | fixed |
"""


class ClassifierTest(unittest.TestCase):
    def test_full_precedes_other_conditions(self):
        for flag in ("irreversible", "stateful", "secrets_method", "new_component",
                     "new_namespace", "public_host"):
            self.assertEqual(classify(**{flag: True}), "full")

    def test_lite_requires_behavior_and_specific_acceptance(self):
        self.assertEqual(classify(behavior_change=True, specific_acceptance=True), "lite")
        self.assertEqual(classify(behavior_change=True), "none")


class ArtifactGateTest(unittest.TestCase):
    def make_spec(self, mode: str) -> Path:
        root = Path(tempfile.mkdtemp())
        (root / "reviews").mkdir()
        (root / "spec.md").write_text(SPEC.format(mode=mode))
        (root / "reviews" / "codex-pre-impl.md").write_text(REVIEW)
        return root

    def test_full_requires_plan_and_tasks(self):
        root = self.make_spec("full")
        self.assertIn("plan.md is required for full mode", validate(root, "implement"))
        self.assertIn("tasks.md is required for full mode", validate(root, "implement"))

    def test_lite_runs_without_plan_or_tasks(self):
        root = self.make_spec("lite")
        self.assertEqual(validate(root, "implement"), [])

    def test_handoff_requires_implementation_review(self):
        root = self.make_spec("lite")
        self.assertIn("implementation review is required before handoff", validate(root, "handoff"))

    def test_unresolved_p0_blocks(self):
        root = self.make_spec("lite")
        (root / "reviews" / "codex-pre-impl.md").write_text(
            REVIEW + "| P0 | unsafe | Hold | unresolved |\n"
        )
        self.assertIn("codex-pre-impl.md has unresolved P0", validate(root, "implement"))

    def test_clarification_blocks(self):
        root = self.make_spec("lite")
        with (root / "spec.md").open("a") as handle:
            handle.write("[NEEDS CLARIFICATION: fixture]\n")
        self.assertTrue(any("unresolved marker" in e for e in validate(root, "implement")))

    def test_placeholder_in_full_plan_blocks(self):
        root = self.make_spec("full")
        (root / "plan.md").write_text("# Plan: <title>\n")
        (root / "tasks.md").write_text("# Tasks: Fixture\n")
        self.assertTrue(any("plan.md contains unresolved marker" in e for e in validate(root, "implement")))

    def test_empty_full_artifacts_block(self):
        root = self.make_spec("full")
        (root / "plan.md").write_text("")
        (root / "tasks.md").write_text("")
        errors = validate(root, "implement")
        self.assertTrue(any("plan.md is missing required section" in e for e in errors))
        self.assertTrue(any("tasks.md is missing required section" in e for e in errors))

    def test_unavailable_review_requires_yu_approval(self):
        root = self.make_spec("lite")
        (root / "reviews" / "codex-pre-impl.md").write_text(
            REVIEW + "- Result: unavailable\n- Fallback approval: N/A\n"
        )
        self.assertTrue(any("lacks Yu-approved" in e for e in validate(root, "implement")))

    def test_unavailable_review_requires_completed_human_review(self):
        root = self.make_spec("lite")
        (root / "reviews" / "codex-pre-impl.md").write_text(
            REVIEW
            + "- Result: unavailable\n- Fallback approval: Yu approved 2026-08-11\n"
            + "- Human fallback reviewer: N/A\n- Human fallback result: N/A\n"
        )
        errors = validate(root, "implement")
        self.assertTrue(any("reviewer identity" in e for e in errors))
        self.assertTrue(any("completed human fallback" in e for e in errors))

    def test_acceptance_criterion_is_required(self):
        root = self.make_spec("lite")
        (root / "spec.md").write_text("---\nmode: lite\n---\n# Spec: Empty\n")
        self.assertTrue(any("acceptance criterion" in e for e in validate(root, "implement")))

    def test_acceptance_checkbox_must_be_in_its_section(self):
        root = self.make_spec("lite")
        (root / "spec.md").write_text(
            "---\nmode: lite\n---\n# Spec: Empty\n## Acceptance criteria\nNone\n"
            "## Verification notes\n- [ ] unrelated\n"
        )
        self.assertTrue(any("acceptance criterion" in e for e in validate(root, "implement")))

    def test_review_metadata_is_required(self):
        root = self.make_spec("lite")
        (root / "reviews" / "codex-pre-impl.md").write_text("x" * 100)
        errors = validate(root, "implement")
        self.assertTrue(any("reviewer identity" in e for e in errors))
        self.assertTrue(any("valid result" in e for e in errors))

    def test_implementation_task_must_be_in_its_section(self):
        root = self.make_spec("full")
        (root / "plan.md").write_text(
            "# Plan: Fixture\n## Resources\nnone\n## Affected paths\nnone\n"
            "## Merge behavior and rollback\nrevert\n"
        )
        (root / "tasks.md").write_text(
            "# Tasks: Fixture\n## Implementation\nNone\n## Verification\n- [ ] verify\n"
        )
        self.assertTrue(any("at least one task" in e for e in validate(root, "implement")))

    def test_finding_requires_known_decision(self):
        root = self.make_spec("lite")
        (root / "reviews" / "codex-pre-impl.md").write_text(
            REVIEW + "| P2 | unclear | Maybe | later |\n"
        )
        self.assertTrue(any("unadjudicated" in e for e in validate(root, "implement")))

    def test_impl_skill_invokes_both_gate_stages(self):
        skill = Path(".claude/skills/sdd-impl/SKILL.md").read_text()
        self.assertIn("--stage implement", skill)
        self.assertIn("--stage handoff", skill)


if __name__ == "__main__":
    unittest.main()
