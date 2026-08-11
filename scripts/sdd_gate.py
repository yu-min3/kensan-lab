#!/usr/bin/env python3
"""Deterministic SDD classification and artifact gate checks."""

from __future__ import annotations

import argparse
import re
from pathlib import Path


PLACEHOLDERS = ("[NEEDS CLARIFICATION", "NNN-<", "YYYY-MM-DD", "<title>")


def section(text: str, heading: str) -> str:
    match = re.search(
        rf"^{re.escape(heading)}\s*$\n(.*?)(?=^##\s|\Z)", text, re.MULTILINE | re.DOTALL
    )
    return match.group(1) if match else ""


def classify(*, irreversible: bool = False, stateful: bool = False,
             secrets_method: bool = False, new_component: bool = False,
             new_namespace: bool = False, public_host: bool = False,
             behavior_change: bool = False, specific_acceptance: bool = False) -> str:
    if any((irreversible, stateful, secrets_method, new_component, new_namespace, public_host)):
        return "full"
    if behavior_change and specific_acceptance:
        return "lite"
    return "none"


def validate(spec_dir: Path, stage: str) -> list[str]:
    errors: list[str] = []
    spec = spec_dir / "spec.md"
    if not spec.is_file():
        return ["spec.md is required"]
    text = spec.read_text()
    mode_match = re.search(r"^mode:\s*(full|lite)\s*$", text, re.MULTILINE)
    if not mode_match:
        errors.append("spec.md must declare mode: full or lite")
        return errors
    mode = mode_match.group(1)
    files_to_check = [spec]
    if not re.search(r"^\s*- \[[ xX]\]\s+\S", section(text, "## Acceptance criteria"), re.MULTILINE):
        errors.append("spec.md must contain at least one acceptance criterion")

    pre_review = spec_dir / "reviews" / "codex-pre-impl.md"
    impl_review = spec_dir / "reviews" / "codex-impl.md"
    if stage in ("implement", "handoff") and not pre_review.is_file():
        errors.append("pre-implementation review is required")
    if mode == "full" and stage in ("implement", "handoff"):
        for name in ("plan.md", "tasks.md"):
            if not (spec_dir / name).is_file():
                errors.append(f"{name} is required for full mode")
            else:
                files_to_check.append(spec_dir / name)
        plan = spec_dir / "plan.md"
        tasks = spec_dir / "tasks.md"
        if plan.is_file():
            plan_text = plan.read_text()
            for heading in ("## Resources", "## Affected paths", "## Merge behavior and rollback"):
                if heading not in plan_text:
                    errors.append(f"plan.md is missing required section: {heading}")
        if tasks.is_file():
            tasks_text = tasks.read_text()
            for heading in ("## Implementation", "## Verification"):
                if heading not in tasks_text:
                    errors.append(f"tasks.md is missing required section: {heading}")
            if not re.search(
                r"^\s*- \[[ xX]\]\s+\S", section(tasks_text, "## Implementation"), re.MULTILINE
            ):
                errors.append("tasks.md must contain at least one task")
    if mode == "lite" and (spec_dir / "tasks.md").exists():
        errors.append("tasks.md must not be required for lite mode")
    if stage == "handoff" and not impl_review.is_file():
        errors.append("implementation review is required before handoff")

    for review in (pre_review, impl_review):
        if review.is_file():
            files_to_check.append(review)
            review_text = review.read_text()
            if len(review_text.strip()) < 80:
                errors.append(f"{review.name} is empty or incomplete")
            reviewer_match = re.search(r"^- Reviewer:\s*(.+)$", review_text, re.MULTILINE)
            result_match = re.search(r"^- Result:\s*(pass|findings|unavailable)\s*$", review_text, re.MULTILINE)
            if not reviewer_match or not reviewer_match.group(1).strip():
                errors.append(f"{review.name} lacks reviewer identity")
            if not result_match:
                errors.append(f"{review.name} lacks a valid result")
            if re.search(r"^- Result:\s*unavailable\s*$", review_text, re.MULTILINE) and not re.search(
                r"^- Fallback approval:\s*Yu approved\b", review_text, re.MULTILINE
            ):
                errors.append(f"{review.name} lacks Yu-approved human fallback")
            if re.search(r"^- Result:\s*unavailable\s*$", review_text, re.MULTILINE):
                if not re.search(r"^- Human fallback reviewer:\s*(?!N/A\s*$)\S.+$", review_text, re.MULTILINE):
                    errors.append(f"{review.name} lacks human fallback reviewer identity")
                if not re.search(r"^- Human fallback result:\s*(pass|findings)\s*$", review_text, re.MULTILINE):
                    errors.append(f"{review.name} lacks completed human fallback result")
            if re.search(r"\|\s*P0\s*\|.*\|\s*Hold\s*\|", review_text):
                errors.append(f"{review.name} has unresolved P0")
            for line in review_text.splitlines():
                cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
                if cells and cells[0] in ("P0", "P1", "P2"):
                    if len(cells) < 4 or cells[2] not in ("Adopt", "Reject", "Hold"):
                        errors.append(f"{review.name} has an unadjudicated finding")
    for path in files_to_check:
        content = path.read_text()
        for marker in PLACEHOLDERS:
            if marker in content:
                errors.append(f"{path.name} contains unresolved marker: {marker}")
        if re.search(r"<[^>\n]+>", content):
            errors.append(f"{path.name} contains unresolved angle-bracket placeholder")
    return errors


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)
    classify_parser = sub.add_parser("classify")
    for flag in ("irreversible", "stateful", "secrets-method", "new-component",
                 "new-namespace", "public-host", "behavior-change", "specific-acceptance"):
        classify_parser.add_argument(f"--{flag}", action="store_true")
    validate_parser = sub.add_parser("validate")
    validate_parser.add_argument("spec_dir", type=Path)
    validate_parser.add_argument("--stage", choices=("implement", "handoff"), required=True)
    args = parser.parse_args()
    if args.command == "classify":
        options = vars(args).copy()
        options.pop("command")
        print(classify(**options))
        return 0
    errors = validate(args.spec_dir, args.stage)
    for error in errors:
        print(error)
    return 1 if errors else 0


if __name__ == "__main__":
    raise SystemExit(main())
