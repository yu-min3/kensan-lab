---
name: helm-outdated
description: 全 Helm Application の chart バージョン鮮度を一括チェック — 各 app.yaml の targetRevision を upstream 最新と突き合わせ、更新候補を優先度付きでレポート（/helm-upgrade の前段）
argument-hint:
---

# Helm Chart Freshness Check

全 Application CR の pinned version（SoT は app.yaml の `targetRevision`）と upstream 最新の差分を一覧化する。**チェックのみで変更は行わない**。実際の更新は `/helm-upgrade <component> <ver>` で個別に行う。

## Steps

1. **Helm chart を使う Application CR を列挙**:
   ```bash
   grep -l "chart:" $(find kubernetes/argocd/applications -name app.yaml)
   ```
   各ファイルから `repoURL`（chart repo 側の source）・`chart`・`targetRevision` を抽出する。git 直 deploy の app（`chart:` なし）は対象外。

2. **upstream 最新バージョンを取得**（repo add 不要の方式）:
   ```bash
   # HTTP helm repo の場合
   helm show chart <chart> --repo <repoURL> 2>/dev/null | grep '^version:'
   # OCI の場合（repoURL が oci:// 始まり）
   helm show chart <repoURL>/<chart> 2>/dev/null | grep '^version:'
   ```
   - 件数が多いので Bash で全件ループさせる（1 件ずつ実行しない）
   - 取得失敗した chart は「確認不可」として残す（黙って落とさない）

3. **semver 比較してレポート**:

   | Component | Current | Latest | Gap | 備考 |
   |-----------|---------|--------|-----|------|
   | kyverno | 3.8.1 | x.y.z | major/minor/patch behind | |

   - **major 遅れ**: ⚠️ で強調。breaking changes の確認が必要
   - **minor / patch 遅れ**: 通常更新候補
   - 最新: ✅

4. **更新候補の優先順位付け**:
   - セキュリティ系コンポーネント（cert-manager, vault, external-secrets, sealed-secrets, oauth2-proxy, kyverno）の遅れを優先
   - CNI / service mesh（cilium, istio-*）は **クラスタ影響が大きいので慎重に**。istio-base / istiod / istio-cni は同時更新が前提な点を明記

5. **Summary**:
   - 更新推奨リストを優先度順に提示し、各行に `/helm-upgrade <component> <version>` を添える
   - major 更新は upstream の release notes / 移行ガイドの URL を添える（context7 が使える場合は breaking changes の要約も）

## Notes

- バージョン SoT は Application CR の `targetRevision`（`helm-multisource.md`）。values.yaml 側にバージョンを書く運用はない
- renovate / dependabot は導入していないため、本 skill が定期的な鮮度確認の代替。月次レビューのタイミングでの実行を想定
