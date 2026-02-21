---
description: Core workflow rules - auto-testing and auto-documentation
globs:
---

# Workflow Rules

## Auto-Testing (指示がなくても必ず実行)

コードを変更したら、対応するテストを自動で実行すること:

- **Go コード変更時**: `cd backend && make test` を実行。失敗したら修正してから完了を報告する。
- **フロントエンド変更時**: `cd frontend && npm run build` で型チェックを実行（TSエラーがないことを確認）。
- **新しい関数/メソッド追加時**: 既存のテストファイルがあればテストも追加する。テストファイルがない場合は確認を取る。

テストが失敗した場合は、修正 → 再テスト → 成功するまでループすること。

## Auto-Documentation (指示がなくても必ず実行)

構造的な変更をしたら、対応する ARCHITECTURE.md を更新すること:

- **新しいサービス/エンドポイント追加** → `backend/ARCHITECTURE.md` のサービス一覧・API Reference 更新
- **新しいコンポーネント/ストア/ページ追加** → `frontend/src/ARCHITECTURE.md` の該当セクション更新
- **DB スキーマ変更** → `backend/ARCHITECTURE.md` のデータベースセクション更新
- **新しいルート追加** → `frontend/src/ARCHITECTURE.md` のルーティングセクション更新

軽微な修正（バグフィックス、スタイル調整、既存ロジックの小変更）では更新不要。

## Plan モードでの必須事項

Plan モードで計画を作成する際、以下を必ず最終ステップとして含めること。これらが欠けている計画は不完全とみなす。

- [ ] 変更した Go コードのテスト実行 (`cd backend && make test`) — 失敗時は修正ループ
- [ ] 変更したフロントエンドのビルド確認 (`cd frontend && npm run build`) — TS エラーがないこと
- [ ] 影響を受ける ARCHITECTURE.md の更新箇所を特定し、更新ステップを含める

計画レビュー時のセルフチェック: 「この計画にテストとドキュメント更新のステップがあるか？」を確認してから ExitPlanMode すること。

## Git Branch Rule

- `main` ブランチでは直接コード変更しない。feature ブランチを作成する。
- ブランチ命名: `feature/<name>`, `fix/<name>`, `refactor/<name>`
