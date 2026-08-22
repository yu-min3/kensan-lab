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
| ID | Criterion | State | Evidence or defer reason |
|---|---|---|---|
| AC-1 | observable result | {ac_state} | {evidence} |
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
    def make_spec(self, mode: str, ac_state: str = "pending", evidence: str = "—") -> Path:
        root = Path(tempfile.mkdtemp())
        (root / "reviews").mkdir()
        (root / "spec.md").write_text(
            SPEC.format(mode=mode, ac_state=ac_state, evidence=evidence)
        )
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
        root = self.make_spec("lite", "verified", "fixture test passed")
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
        self.assertTrue(any("acceptance criteria state table" in e for e in validate(root, "implement")))

    def test_old_acceptance_checkbox_is_not_a_state_source(self):
        root = self.make_spec("lite")
        (root / "spec.md").write_text(
            "---\nmode: lite\n---\n# Spec: Old\n## Acceptance criteria\n- [x] result\n"
        )
        self.assertTrue(any("state table" in e for e in validate(root, "implement")))

    def test_implement_allows_pending_acceptance(self):
        root = self.make_spec("lite")
        self.assertEqual(validate(root, "implement"), [])

    def test_handoff_blocks_pending_or_failed_acceptance(self):
        for state in ("pending", "failed"):
            with self.subTest(state=state):
                root = self.make_spec("lite", state, "not complete")
                (root / "reviews" / "codex-impl.md").write_text(REVIEW)
                self.assertTrue(any(f"is not complete: {state}" in e for e in validate(root, "handoff")))

    def test_handoff_requires_evidence_or_defer_reason(self):
        for state in ("verified", "deferred"):
            with self.subTest(state=state):
                root = self.make_spec("lite", state, "—")
                (root / "reviews" / "codex-impl.md").write_text(REVIEW)
                self.assertTrue(any("lacks evidence or a defer reason" in e for e in validate(root, "handoff")))

    def test_handoff_accepts_verified_and_deferred_with_evidence(self):
        for state, evidence in (
            ("verified", "test passed"),
            ("deferred", "Reason: cluster unavailable; Next: run in Explore CI"),
        ):
            with self.subTest(state=state):
                root = self.make_spec("lite", state, evidence)
                (root / "reviews" / "codex-impl.md").write_text(REVIEW)
                self.assertEqual(validate(root, "handoff"), [])

    def test_deferred_requires_reason_and_next_verification(self):
        for evidence in ("later", "Reason: unavailable", "Next: run in CI"):
            with self.subTest(evidence=evidence):
                root = self.make_spec("lite", "deferred", evidence)
                (root / "reviews" / "codex-impl.md").write_text(REVIEW)
                self.assertTrue(any("requires Reason and Next" in e for e in validate(root, "handoff")))

    def test_malformed_acceptance_row_cannot_hide_a_pending_criterion(self):
        root = self.make_spec("lite", "verified", "test passed")
        with (root / "spec.md").open("a") as handle:
            handle.write("| AC-2 | hidden pending | pending |\n")
        (root / "reviews" / "codex-impl.md").write_text(REVIEW)
        self.assertTrue(any("malformed acceptance criterion row" in e for e in validate(root, "handoff")))

    def test_acceptance_ids_are_unique(self):
        root = self.make_spec("lite")
        with (root / "spec.md").open("a") as handle:
            handle.write("| AC-1 | duplicate | pending | — |\n")
        self.assertTrue(any("IDs must be unique" in e for e in validate(root, "implement")))

    def test_acceptance_state_is_known(self):
        root = self.make_spec("lite", "done", "looks good")
        self.assertTrue(any("invalid state" in e for e in validate(root, "implement")))

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
