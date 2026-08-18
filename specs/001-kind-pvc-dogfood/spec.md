---
id: 001-kind-pvc-dogfood
mode: full
status: implemented
created: 2026-08-11
updated: 2026-08-17
---

# Spec: kind demo PVC persistence dogfood

## Problem

Exploreは`longhorn`という名前でkindのlocal-path provisionerを代用しているが、実際のworkloadがPVCを作成・mountし、Pod再作成後もデータを読めることを検証していない。AIハーネスPhase 0もFull SDDを実変更で通していない。

## Goals

- app-demoが`longhorn` StorageClassのPVCをmountする。
- Explore CIがPVCのbindとPod再作成を跨ぐデータ永続性を確認する。
- chartが生成するstateful resourceへArgo CDのprune保護契約を付ける。
- Full SDDのpre-review、実装、検証、diff review、handoffを一周する。

## Non-goals

- kindでLonghorn本体、replication、snapshot、backupを再現しない。
- `Prune=false`の実際のorphan動作を破壊的に試験しない（renderされたannotationを契約oracleとする）。
- 本番clusterのPVC、StorageClass、workloadを変更しない。
- mergeやremoteへのpushを行わない。

## Scope

- Namespace: `app-demo`（既存）
- Public host: `demo.127-0-0-1.sslip.io`（既存、変更なし）
- Entry point: `gateway-explore`（既存、変更なし）
- Secrets / authentication: 変更なし

## Acceptance criteria

| ID | Criterion | State | Evidence or defer reason |
|---|---|---|---|
| AC-1 | `helm template app-demo charts/app-base -f environments/kind/values/demo-app.yaml`が`longhorn` StorageClassのPVCと`/data` mountを生成する。 | verified | local Helm renderで確認。 |
| AC-2 | 生成PVCに`argocd.argoproj.io/sync-options: Prune=false`が付く。 | verified | local Helm renderで確認。 |
| AC-3 | Explore CIでapp-demo PVCが`Bound`になる。 | verified | isolated kindで`demo-data`（128Mi、`longhorn`）のBoundを確認。 |
| AC-4 | Explore CIで`/data`に書いたmarkerがapp-demo Pod再作成後も一致する。 | verified | isolated kindでPod UID交代後もmarker一致を確認。 |
| AC-5 | Pod再作成後、既存demo hostが期限内にgateway経由HTTP 200へ戻る。 | deferred | Reason: isolated kindにはGatewayを導入せず、Service HTTP 200までを確認したため。 Next: 公開revisionを参照するExplore CIでgateway routeを確認する。 |

状態はこの表を正とする。`pending` / `failed`はhandoff不可、`deferred`は`Reason: ...; Next: ...`を必須とする。

## Open questions

- None
