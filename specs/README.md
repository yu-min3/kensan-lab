# Specs

`specs/` は SDD を適用する変更の作業状態と監査記録を保存する。恒久的な説明は [`docs/guides/sdd-workflow.md`](../docs/guides/sdd-workflow.md) を正とする。

```text
specs/NNN-<slug>/
├── spec.md
├── plan.md                 # Full のみ
├── tasks.md                # Full のみ
└── reviews/
    ├── codex-pre-impl.md
    └── codex-impl.md
```

- 要求・受入基準: `spec.md`
- 実装設計・影響範囲: `plan.md`
- 作業進捗・検証結果: `tasks.md`
- 独立レビューと Claude の裁定: `reviews/codex-*.md`

`NNN` は作成直前に再確認する。同じ番号またはslugが存在したら採番をやり直し、上書きしない。
