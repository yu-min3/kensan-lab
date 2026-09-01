# Plan: kind demo PVC persistence dogfood

> Spec: [spec.md](./spec.md)

## Constitution check

- [x] `CLAUDE.md` — kind内のGitOps同期だけを変更し、本番へ直接applyしない。
- [x] `.claude/rules/gitops-workflow.md` — branch専用worktreeで変更し、remote操作とmergeを行わない。
- [x] `.claude/rules/collaboration.md` —設計、検証、rollbackをhandoffへ投影する。
- [x] `.claude/rules/kubernetes-cluster.md` — PVCはrepository標準の`longhorn` StorageClassを指定する。

## Layout

- Category: `environments/kind` + shared `charts/app-base`
- Pattern: Explore Application + in-repo Helm chart
- Component path: `environments/kind/values/demo-app.yaml`, `charts/app-base/`
- Application path: `environments/kind/applications/templates/demo-app.yaml`（変更なし）

## Resources

| Change | kind / name | Namespace | Prune behavior |
|---|---|---|---|
| Add | `PersistentVolumeClaim/demo-data` | `app-demo` | `Prune=false` |
| Change | `Deployment/demo` volume mount | `app-demo` | Deploymentは通常のprune |
| Delete | なし | — | — |

## Chart and version

- Repository / chart: in-repo `charts/app-base`
- `targetRevision`: Explore rootから渡されるbranch/SHA（既存）
- Existing layout exception: Exploreの既存values overlayを使い、新規raw workloadを増やさない。

## Policy and namespace

- Namespace labels / PSS: 既存`app-demo` namespaceを使い変更なし。
- Kyverno / PolicyException impact: 新規例外なし。既存restricted securityContextを維持する。
- Argo CD project / sync options / wave: 既存`app-project` / automated syncを維持。PVC resourceに`Prune=false`を付ける。

## Storage

- PVC / storageClass / capacity: `demo-data` / `longhorn` / `128Mi`
- Backup / restore: kindの使い捨て環境なので対象外。Pod再作成だけを検証する。
- `Prune=false`: `charts/app-base/templates/pvc.yaml`の生成PVCへ付与する。

## Network and authentication

- Gateway / host / HTTPRoute / AuthPolicy: 既存の`gateway-explore`とdemo hostを維持。
- Keycloak redirect URI: 不要。
- NetworkPolicy: 変更なし。

## Secrets

- Method / path / Reloader: 変更なし。

## Affected paths

- `charts/app-base/templates/pvc.yaml`
- `environments/kind/values/demo-app.yaml`
- `.github/workflows/explore-ci.yml`
- `specs/001-kind-pvc-dogfood/`

## Merge behavior and rollback

- Merge behavior: Explore同期時にdemo PVCが作成され、CIがmarkerの永続性を検査する。shared chartはPVC作成時だけPrune保護を追加する。
- Persistence oracle: podinfo 6.9.2に`/bin/sh`があることを実像確認済み。CIはlabelでPodを一意に解決し、markerと旧Pod UIDを記録、`/data`へwrite/read、Pod delete、新UIDのReady Podをtimeout付きで待機、同じPVC上のmarker完全一致、gateway経由HTTP 200をretry確認する。失敗時はPod/PVC/eventをdiagnosticsへ出す。
- Risk: podinfo image内のshellと`/data`書込可否にCIが依存し、Pod再作成待ちで時間が増える。image更新時はoracleも再確認する。
- Rollback: 変更commitをrevertする。CIではjob末尾にclusterを破棄する。ローカルでclusterを継続利用する場合、`Prune=false`の`demo-data`は残るため、所有者とデータ不要を人間が確認した後だけ明示削除する。
