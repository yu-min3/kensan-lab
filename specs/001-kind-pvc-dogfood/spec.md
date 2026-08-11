---
id: 001-kind-pvc-dogfood
mode: full
status: implemented
created: 2026-08-11
updated: 2026-08-12
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

- [ ] `helm template app-demo charts/app-base -f environments/kind/values/demo-app.yaml`が`longhorn` StorageClassのPVCと`/data` mountを生成する。
- [ ] 生成PVCに`argocd.argoproj.io/sync-options: Prune=false`が付く。
- [ ] Explore CIでapp-demo PVCが`Bound`になる。
- [ ] Explore CIで`/data`に書いたmarkerがapp-demo Pod再作成後も一致する。
- [ ] Pod再作成後、既存demo hostが期限内にgateway経由HTTP 200へ戻る。

## Open questions

- None
