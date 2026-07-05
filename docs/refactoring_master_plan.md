# kensan-lab リファクタリング マスタープラン

> **Status**: 承認済み・実行中 / 作成: 2026-07-03（origin/main f0a31dd + 実クラスタ確認済み）
> Phase 4 の legacy 処遇は **(a) tag + 削除で決定**（Yu、2026-07-03）。tag `kensan-legacy-final` 作成・push 済み
>
> ⚠️ `docs/` は MkDocs で GitHub Pages（+ llms.txt）として **public 公開される**。
> 本ドキュメントを公開したくない場合は commit 前に `mkdocs.yml` の `exclude_docs` へ追加するか、置き場所を変えること。

---

## 1. 目的

- **リポジトリの認知負荷を下げる**: 追跡ファイル 1,192 のうち約 630（53%）が廃止済みの `apps/kensan-legacy`。「今生きているもの」と「死んでいるもの」の区別に毎回コストがかかっている
- **public リポジトリとしての見通しを良くする**: kensan-lab は homelab リファレンスアーキテクチャとして公開する方針。デモ残骸・stale docs・死んだ Makefile は公開品質を下げる
- **cutover の残渣を確実に片付ける**: 旧 kensan は PR #394（2026-07-02）+ 手動 teardown で**クラスタからは撤去済み**。ただし Vault engine の zombie Application ×2（OutOfSync/Missing で稼働中）、git 上の instance values、実質空の `app-prod` ns、廃止対象と衝突する stale ブランチ/PR が残っている
- **セーフティネットを先に張る**: 現状 CI は docs build のみ。Go テスト・manifest 検証・chart render 検証を先に自動化し、以降のフェーズの回帰を機械的に検出できる状態を作る

## 2. 調査サマリ

### 2.1 プロジェクト構造と主要ディレクトリの役割

| ディレクトリ | 追跡ファイル数 | 役割 | 状態 |
|---|---|---|---|
| `apps/kensan-legacy/` | ~630 | 旧 kensan（React + Go microservices ×6 + Python AI + Iceberg lakehouse） | **廃止済み**。PR #394 でデプロイ撤去、ソースは「参照用アーカイブ」として残置中 |
| `apps/kensan/` | ~73 | 現行 kensan（Go 単一サービス + React SPA、ファイル SSoT） | active。backend 8,700 LOC |
| `kubernetes/` | 209 | GitOps 管理のプラットフォーム全体（argocd / network / observability / auth / secrets / storage / policy / apps / namespaces / kube-system） | active。Pattern A/B 規約あり（`kubernetes/README.md`） |
| `backstage/` | 109 | 開発者ポータル（app/ = Backstage 本体、manifests/ = デプロイ定義） | active。ただし `app/examples/` はデフォルト scaffold のデモデータ |
| `docs/` | 92 | ADR ×16、アーキテクチャ、runbook、design system。MkDocs で public 公開 | active。一部 deprecated / stale |
| `charts/app-base/` | 11 | platform 提供の汎用アプリ chart（app-kensan が最初の消費者） | active |
| `bootstrap/` | 13 | Vault（Terraform）+ Keycloak の初期化 | active |
| `packages/design-tokens/` | 4 | Whetstone design system の tokens（css / json） | active |
| `.claude/` | 20 | エージェント規約（rules ×8）+ スキル ×11 | active |
| ルート `Makefile` | 1 | **存在しない `test-app/` を参照する死んだ Makefile** | 死物 |

### 2.2 依存関係の流れ

```
Git push (main)
  └→ Argo CD platform-root-app（App of Apps、applications/ を recurse）
       ├→ Application CR + ApplicationSet ×3
       │    ├ Pattern A: upstream chart + kubernetes/<cat>/<comp>/values.yaml + resources/
       │    ├ Pattern B: raw YAML フラット
       │    ├ observability: ApplicationSet + 各 config.json（chart 版数の SoT）
       │    └ vault-database / transit-engine: 自作 chart + shared/ + platform-values/（ApplicationSet）
       └→ Application "app-kensan"（現行）: charts/app-base + kubernetes/apps/app-kensan（multi-source）→ ns app-kensan

アプリ image: apps/* → docker buildx (multi-arch) → GHCR → Application CR が参照
secrets: Vault (dynamic/static/transit) + ESO、bootstrap 系のみ SealedSecret
```

**cutover の現在地（2026-07-03 実クラスタ確認）**:

- 旧 `kensan` Application: **削除済み**（PR #394）。ns `kensan`・pods・PVC も存在しない（手動 teardown 完了済み）
- **zombie ×2**: `vault-db-kensan-dagster` / `vault-transit-kensan-users` Application が **OutOfSync / Missing** で残存。ApplicationSet が git 上の instance values（下記）から生成し続けているが、destination（旧 lakehouse / 旧 user service）が消えたため宙に浮いている
  - `kubernetes/secrets/vault-database-engine/platform-values/vault-database/kensan-dagster.yaml`
  - `kubernetes/secrets/vault-transit-engine/platform-values/vault-transit/kensan-users.yaml`
- ns `app-prod`（旧 shared landing zone）: 中身は `ghcr-pull-secret` ExternalSecret + default SA のみの**実質空**。ADR-006 の「空になり次第削除候補」の条件を満たしている

### 2.3 主要モジュールの責務

- **`apps/kensan/backend`**: `internal/workspace`（scan/frontmatter/write、インデックスレス設計）、`internal/tasks`・`goals`・`projects`（ドメイン）、`internal/api`（HTTP + SPA 配信）、`internal/telemetry`。責務分割は健全
- **`apps/kensan/frontend`**: ページ ×10 + コンポーネント。**テストゼロ**。`EditorLabPage`（`/editor-lab`）+ Milkdown / TipTap の 2 エディタ実装が併存（実験の残骸）
- **`charts/app-base`**: deployment / service / httproute(+oauth2) / pvc / ghcr ExternalSecret / serviceaccount。小さく健全
- **`backstage/app`**: packages/app + packages/backend + templates（fastapi-template）。`examples/` は Backstage デフォルト生成のデモカタログ（e-commerce 等、24 ファイル）

## 3. 現状の問題点

### 3.1 肥大化している箇所

| 箇所 | 規模 | 内容 |
|---|---|---|
| `apps/kensan-legacy/` | 630 ファイル / 82,000 LOC | リポジトリの過半。廃止済みなのに全 grep・全解析のノイズ源。46 のテストファイルも保守されない |
| ローカル/リモートブランチ | local 180 / remote 98 | マージ済み・放棄ブランチの堆積 |
| disk 上の untracked 残骸 | 計 ~230MB+ | `.venv-docs/` 150MB、`temp/` 16MB（**本番 DB backup SQL・raw secret を含む**）、`bootstrap/vault/.terraform/` 42MB、`site/` 5.9MB、`apps/kensan/{kensan-ai,lakehouse,docs,temp}`（.venv 等の抜け殻） |

### 3.2 重複している処理・コンテンツ

- **kensan 二重系（ソースのみ残存）**: frontend / backend / AI / lakehouse のソースが新旧 2 系統。クラスタ上の並走は解消済みだが、git 上は過半を旧系が占めたまま
- **エディタ 2 実装**: `MilkdownEditor.tsx` と `TipTapEditor.tsx` + 実験ページ `EditorLabPage`
- **Backstage デモデータ**: `backstage/app/examples/`（架空の e-commerce ドメイン・チーム）と実カタログ `backstage/app/catalog/` が併存
- **ロゴ・ブランドアセット**: `docs/assets/logos/`、`docs/design/brand/`、`apps/kensan-legacy/docs/design/` に分散

### 3.3 責務が曖昧な箇所

- **`kubernetes/namespaces/`**: 「shared ns の bootstrap」だが実体は `app-prod` のみで、その app-prod は実質空。ns lifecycle の Application 群（`argocd/applications/namespaces/`）との二層構造も初見に分かりにくい
- **`backstage/manifests/`**: 他の全コンポーネントは `kubernetes/<category>/` 配下なのに backstage だけトップレベルに deploy 定義がある（Pattern A/B の外）
- **ルート `Makefile`**: `test-app/` という存在しないディレクトリの image build 用。リポジトリの顔なのに完全に死んでいる
- **docs の鮮度混在**: `docs/concepts/kustomize-guidelines.md` は明示 DEPRECATED、`docs/operations/*-dev-cleanup.md` は廃止済み dev 環境の掃除記録、ルート README の Repository Structure は存在しない `kubernetes/environments/` を記載（実体は `namespaces/`）

### 3.4 技術的負債

| # | 負債 | 影響 |
|---|---|---|
| D1 | CI が `docs.yml`（MkDocs build）のみ。`.yamllint` / `.yamlfmt` / `playwright.config.ts` は存在するが CI で実行されない | 回帰検出が全て手動・実機頼み |
| D2 | ルート Makefile が死んでいる（`test-app` 不在） | 初見の導線が壊れている |
| D3 | `local-path-provisioner` が legacy 扱いのまま残存（`storage/`、`applications/namespaces/local-path-storage`） | Longhorn 移行完了後の削除が未実施 |
| D4 | README / docs のドリフト（`environments/` 記述、deprecated docs、旧 kensan 前提の記述） | public リポジトリとしての信頼性低下 |
| D5 | `temp/` に本番 SQL backup・raw secret が平置き（gitignored だが disk 上に残存） | ローカル漏洩リスク・ノイズ |
| D6 | ブランチ 180 本 / stale PR（#256, #298, #352 は 5〜6 月から放置、#387, #388 は draft）。**`feat/polaris-postgres-persistence` は撤去済み legacy の永続化作業で、目的自体が消滅** | worktree 運用の妨げ。特に polaris ブランチは古い main 由来で legacy Application CR を含んだままであり、扱いを誤ると混乱の元 |
| D7 | vault engine instance values（kensan-dagster / kensan-users）が git に残り、**zombie Application ×2 が OutOfSync/Missing で常時表示** | Argo CD の health 一覧が常に汚れ、本物の異常を見落とす温床 |

### 3.5 テストが不足している箇所

| 対象 | 現状 | 必要性 |
|---|---|---|
| `apps/kensan/backend` | 8 test ファイル（workspace ×2, tasks ×2, goals, projects, api, reviews） | 比較的健全。write 系（`write_handlers.go`, `workspace/write.go`, `history`）のカバレッジ確認は必要 |
| `apps/kensan/frontend` | **ゼロ** | TaskBoard の DnD・エディタ保存など、データを壊しうる操作にテストなし |
| E2E（新 kensan） | なし（旧には 26 ファイルあった） | cutover 完了で唯一の UI 系になった。最低限の smoke E2E が欲しい |
| `charts/app-base` | render テストなし | 消費者が増える前に `helm template` の golden test を用意したい |
| `kubernetes/` manifest | 検証なし | kubeconform + yamllint を CI に。ApplicationSet / Application の重複・path 存在チェックも |
| 旧 kensan（46 test ファイル） | 廃止済み | **投資しない** |

### 3.6 影響範囲が大きく慎重に扱うべき箇所

過去の実事故（PR #340, #366, #377-379, #382-383 ほか）を踏まえた高リスク領域:

1. **`kubernetes/argocd/`（root-app / Application / ApplicationSet）**: recurse で全 CR を拾うため、rename・移動が prune → 再作成になる。**Application rename は destination ns ごと消した実績あり**。ApplicationSet の同名 takeover → cascade prune も実績あり
2. **stateful リソース**: `kensan-workspace` PVC（日記等の生活データ）、`syncthing-config` PVC（device 鍵）、Vault raft、Keycloak DB、Longhorn SC / RecurringJob。`Prune=false` は**リソース個別 annotation でのみ有効**（Application 側では子を守れない）
3. **`network/network-policy/`**: clusterwide default-deny。誤削除で全通信断。**ns 削除は per-ns NetworkPolicy を巻き添えにする**（app-prod 削除時に注意）
4. **Istio 関連**: sidecar injection と Postgres wire protocol の相性問題の実績。ns label / PeerAuthentication の変更は波及大
5. **stale ブランチの誤 merge**: `feat/polaris-postgres-persistence` は #394 以前の main 由来で、working tree に旧 `kensan` Application CR を含む。三方 merge では main 側の削除が保たれる見込みだが、**cherry-pick や手動取り込みで app.yaml が復活すると legacy の再デプロイが走る**。close する（推奨）か、少なくとも merge しないこと
6. **ApplicationSet からの instance 削除**: zombie 2 つの削除は「ApplicationSet が生成した Application の削除」になる。ApplicationSet の cascade 挙動（ownerRef）を理解した上で、instance values ファイル削除 → 生成 Application の消滅、という git 経由の正規ルートで行う（ApplicationSet 本体には触れない）

### 3.7 削除候補一覧

| 対象 | 種別 | 削除条件 |
|---|---|---|
| vault engine instance values（kensan-dagster / kensan-users） | git tracked | **即時可**（対象は撤去済み、zombie 解消）。Vault 側 role / transit key の失効も同時に。transit key は暗号化済みデータが残っていないこと確認後 |
| `kubernetes/namespaces/app-prod/` + `applications/namespaces/app-prod` | git tracked | 実質空を確認済み。`ghcr-pull-secret` ESO を参照する workload が無いこと最終確認後 |
| ルート `Makefile`（test-app 参照） | git tracked | 即時可（新 Makefile に書き換え or 削除） |
| `backstage/app/examples/`（24 ファイル） | git tracked | `app-config.yaml` の catalog locations から参照が無いこと確認後 |
| `docs/concepts/kustomize-guidelines.md` | git tracked | 即時可（Backstage template の Kustomize 使用有無を確認し、記述を移すか削除） |
| `docs/operations/*-dev-cleanup.md` 等の stale 運用記録 | git tracked | 履歴的価値があれば `docs/incidents/` 相当へ、なければ削除 |
| `apps/kensan-legacy/` 全体 | git tracked | **決定済み: tag `kensan-legacy-final` を打って削除**（Yu、2026-07-03）。Dagster / Iceberg / Agent SDK (ADK) の実装例として tag から参照する旨を ADR-017 に残す |
| `local-path-provisioner` 関連 | git tracked | 全 PVC の Longhorn 移行完了後 |
| `apps/kensan/{kensan-ai,lakehouse,docs,temp}` の抜け殻 | untracked | 即時可（`.venv` 等のみ。`temp/` 内 raw secret は要中身確認） |
| `temp/` の旧 backup SQL・raw secret・古い script 群 | untracked | 中身確認の上ローカル削除（backup は必要なら R2 等へ退避） |
| `.venv-docs/`, `site/` | untracked | 再生成可能。即時可 |
| マージ済み local/remote ブランチ + `feat/polaris-postgres-persistence` | git refs | merged 分は棚卸しで削除。polaris ブランチは目的消滅につき close 推奨（Yu 判断） |
| stale PR（#256 #298 #352 #387 #388） | GitHub | Yu 判断で close or 完遂 |

## 4. フェーズ分割

> 原則: **(1) セーフティネットを先に張る → (2) 無リスクの死物から消す → (3) クラスタに効く削除は 1 PR = 1 ステップで隔離する**。
> 全フェーズ共通で、作業は `~/kensan-lab.worktrees/<name>` の worktree + PR 経由。merge はレビュー完了 + Yu の指示待ち。

### Phase 0: セーフティネット整備（CI とベースライン）

- **目的**: 以降の全フェーズの回帰を機械検出できる状態を作る。既存動作は一切変えない
- **実施すること**:
  - GitHub Actions 追加: ① `go test ./...` + `go vet`（apps/kensan/backend）、② frontend `tsc --noEmit` + `vite build`、③ `helm template` render 検証（charts/app-base + vault-database/transit chart、代表 values で golden diff）、④ yamllint + kubeconform（`kubernetes/` 配下、CRD スキーマは skip リスト管理）
  - Argo CD Application の静的検証スクリプト: `applications/**/app.yaml` の `path` が実在するか、destination ns / project の整合
  - stateful リソースの `Prune=false` annotation 棚卸し（現状確認のみ、変更は別 PR）
  - ベースライン記録: 稼働中 Application 一覧・healthy 状態・PVC 一覧を `temp/` にスナップショット（zombie 2 件が既知の異常として記録される）
- **対象範囲**: `.github/workflows/` 新規のみ（既存ファイル変更なし）
- **リスク**: 低。CI が落ちても本番影響なし
- **ロールバック**: workflow ファイル削除のみ

### Phase 1: 無リスクの死物除去（クラスタに一切触れない）

- **目的**: 「消しても何も起きない」ものを先に消し、リポジトリの視界を確保する
- **実施すること**:
  - ルート `Makefile` を削除 or docs build / lint 用に書き換え
  - `backstage/app/examples/` 削除（事前に `app-config.yaml` / `catalog-info` の参照 grep）
  - ルート README の Repository Structure 修正（`environments/` → `namespaces/`、現状に同期）
  - `docs/concepts/kustomize-guidelines.md` の処置（削除 or Backstage template 節のみ残して縮約）
  - stale docs の棚卸し（`docs/operations/` の廃止済み dev 掃除記録など）
  - ローカル disk 残骸の掃除（untracked のみ）: `apps/kensan/{kensan-ai,lakehouse,docs,temp}`、`.venv-docs/`、`site/`、`temp/` の古い backup / raw secret（**中身を確認し、必要なものは退避してから**）
  - ブランチ / PR の棚卸し: merged ブランチ削除、未マージ分と stale PR は一覧を Yu に提示して判断を仰ぐ（`feat/polaris-postgres-persistence` の close 提案を含む）
- **対象範囲**: ルートファイル、`backstage/app/examples/`、`docs/` の一部、untracked ファイル、git refs。**`kubernetes/` と `apps/` の tracked ファイルには触れない**
- **リスク**: 低。全て git revert / 再生成で戻る。唯一の注意は temp/ の backup 削除（不可逆）→ 退避を先に
- **ロールバック**: git revert。untracked 削除分は事前退避で担保

### Phase 2: cutover 残渣の後片付け（クラスタに効く削除・小さく隔離）

- **目的**: PR #394 で完了した cutover の残り物を消し、Argo CD の一覧を「全 green」に戻す
- **実施すること（1 項目 = 1 PR）**:
  1. `vault-database-engine/platform-values/vault-database/kensan-dagster.yaml` 削除 → ApplicationSet 経由で zombie `vault-db-kensan-dagster` が消えることを確認 → Vault 側の database role / connection の失効確認
  2. `vault-transit-engine/platform-values/vault-transit/kensan-users.yaml` 削除 → 同様に zombie 解消 → **transit key で暗号化されたデータがどこにも残っていないことを確認してから** Vault 側 key を失効（復号不能化は不可逆）
  3. `kubernetes/namespaces/app-prod/` + `applications/namespaces/app-prod/app.yaml` 削除 → ns `app-prod` 削除。事前に ghcr-pull-secret の参照ゼロを確認、per-ns NetworkPolicy の巻き添え有無を確認
  4. legacy 残参照の掃除: `kubernetes/network/istio/gateway-prod.yaml`・`vault-transit-engine/README.md`・`argocd/applications/apps/README.md` 内の kensan-legacy 言及を現状に同期
- **対象範囲**: `kubernetes/secrets/*/platform-values/`、`kubernetes/namespaces/`、`kubernetes/argocd/applications/namespaces/`、docs コメント
- **リスク**: 中。ApplicationSet 経由の削除と ns 削除を含む。ただし対象は全て「すでに死んでいるもの」で、実データは無い（teardown 済み・app-prod は実質空）
- **ロールバック**: git revert + sync で宣言状態は戻る。Vault 側の transit key 失効だけは不可逆なので、確認を 2 段階（暗号化データ残存チェック → 猶予期間を置いて失効）にする

### Phase 3: docs / 規約の再編（公開品質の底上げ）

- **目的**: public リファレンスとしての docs を現状（cutover 完了後の世界）に同期させる
- **実施すること**:
  - README / docs の旧 kensan 説明を「歴史」セクションに縮約し、現行 kensan 中心の構成に
  - `docs/architecture/` 6 ドメインページの仕上げ（進行中の docs 施策と統合）
  - mkdocs nav と実ファイルの突き合わせ（nav 漏れ・リンク切れ）
  - ロゴ / ブランドアセットの配置を `docs/design/brand/` に一本化
- **対象範囲**: `docs/`、`README.md`、`mkdocs.yml`
- **リスク**: 低（docs のみ）。公開サイトに即反映される点だけ意識
- **ロールバック**: git revert

### Phase 4: `apps/kensan-legacy/` の削除（tag アーカイブ方式・決定済み）

- **目的**: リポジトリの 53% を占める廃止コードを削除しつつ、**Dagster / Iceberg lakehouse・Google ADK agent の実装例としての参照性を tag で恒久確保する**
- **決定**（Yu、2026-07-03）: PR #394 時点の「保持」を改め、tag + 削除を採用。tag `kensan-legacy-final`（annotated、参照方法をメッセージに記載）は origin へ push 済み
- **実施すること**:
  - `apps/kensan-legacy/` 全削除
  - **ADR-017 を追加**: 削除の経緯と、tag `kensan-legacy-final` から実装例（Dagster / Iceberg / ADK / Go microservices / OTel 計装）を参照する方法を明文化
  - README / docs / `.claude/rules/` / `docs/design/adoption-status.md` から legacy 参照を除去（歴史的記録・incident 記録は除く）
  - 旧 kensan 由来の GHCR image の棚卸し（削除 or 放置の判断は Yu）
- **リスク**: 低〜中。クラスタ参照はゼロ確認済み（Argo CD Application は app-kensan のみ）。復元は tag から機械的に可能
- **ロールバック**: `git revert`、または `git checkout kensan-legacy-final -- apps/kensan-legacy`

### Phase 5: 構造の一貫性向上

- **目的**: 「例外」を減らし、Pattern A/B 規約への収束度を上げる
- **実施すること**:
  - `backstage/manifests/` の置き場を検討（`kubernetes/` 配下へ寄せる or 現状維持を ADR 化）。**移動する場合は Application rename ではなく path 変更のみ**（rename = prune 事故）
  - `local-path-provisioner` の廃止完了（残 PVC の Longhorn 移行確認 → SC / provisioner / ns app 削除）
  - `kubernetes/namespaces/` の解消（Phase 2 で app-prod が消えれば README だけになる → ディレクトリごと整理）
  - ルート README の Repository Structure を最終形に更新
- **対象範囲**: `kubernetes/`、`backstage/manifests/`
- **リスク**: 中。Application の source path 変更は「削除→再作成」ではなく in-place 更新になることを PR ごとに確認。storage 変更は stateful 巻き添えに注意
- **ロールバック**: git revert + Argo CD sync

### Phase 6: 新 kensan の品質強化

- **目的**: 唯一のアプリとなった新 kensan の保守性を上げる
- **実施すること**:
  - エディタ実装の一本化（Milkdown / TipTap のどちらかに決めて他方 + `EditorLabPage` を削除。判断材料を整理して Yu が選定）
  - frontend テスト導入（Vitest + Testing Library。まず `lib/api.ts`・TaskBoard の移動ロジックなど壊すと痛い所から）
  - smoke E2E（Playwright。ダッシュボード表示・daily 作成・タスク移動の 3 シナリオ程度）
  - backend write 系のカバレッジ補強（`write_handlers`、`history`、`workspace/write` の異常系）
- **対象範囲**: `apps/kensan/` のみ
- **リスク**: 低〜中（アプリ内で閉じる）。データを触るテストは必ず tempdir で
- **ロールバック**: git revert

## 5. 着手順序

### 先に着手すべき箇所

1. **Phase 0 の CI**（全ての土台。他のどの作業とも競合しない）
2. **Phase 1 の死物除去 + ブランチ/PR 棚卸し**（リスクゼロで公開品質に直結。polaris ブランチの close 判断を早めに仰ぐことで誤 merge リスクも消える）
3. **Phase 2 の zombie 解消**（cutover の「完了宣言」に相当。Argo CD 一覧が green に戻り、以降の異常検知が効くようになる）

### 後回しにすべき箇所

- ~~**`apps/kensan-legacy` の処遇（Phase 4）**: Yu の意思決定待ち~~ → **決定済み（tag + 削除）**。クラスタ参照ゼロ確認済みのため git-only 作業として前倒し実行可
- **`backstage/manifests/` の移動（Phase 5）**: 動いているものの構造美化は最後。ADR だけ先に書くのは可
- **旧 kensan の test ファイルの手入れ**: 絶対にやらない（廃止対象への投資）
- **Vault / Keycloak / network-policy の「ついで整理」**: 本プランのスコープ外とし、必要が生じたら個別 ADR で

## 6. リスクが高い箇所(再掲・集約)

| 箇所 | リスク | 緩和策 |
|---|---|---|
| Application / ApplicationSet の rename・移動 | ns ごと prune、cascade prune の実績 | rename 禁止。path 変更は in-place になることを diff で確認 |
| ApplicationSet instance 削除（zombie 解消） | ownerRef cascade の誤操作 | ApplicationSet 本体に触れず、instance values ファイル削除の git 正規ルートのみ |
| Vault transit key の失効 | 暗号化データの復号不能化（不可逆） | データ残存チェック → 猶予期間 → 失効の 2 段階 |
| PVC（workspace / syncthing-config）・Vault・Keycloak DB | データ不可逆喪失 | 個別 `Prune=false` 維持 + 事前 backup + Longhorn snapshot |
| ns 削除（app-prod） | per-ns NetworkPolicy 巻き添え → default-deny で通信断 | ns 内リソースの完全棚卸し後に削除 |
| `temp/` の backup / raw secret 掃除 | 不可逆削除 | 削除前に必要分を R2 / Vault へ退避 |
| stale ブランチ（特に polaris）との衝突・誤 merge | 撤去済み legacy Application の復活 | 早期 close。merge しない。cherry-pick 経由の取り込みも禁止 |

## 7. 事前に追加すべきテスト（Phase 0 の詳細）

1. **Go**: `go test ./... && go vet ./...`（apps/kensan/backend）— 既存 8 test を CI 化するだけでまず価値がある
2. **Frontend**: `tsc --noEmit` + `vite build` — テストが無い現状でも型とビルドの破壊は検出できる
3. **Helm render**: `helm template` を代表 values で実行し、(a) エラーなし、(b) golden ファイルとの diff、を検証（app-base / vault-database / vault-transit の 3 chart）
4. **Manifest 検証**: yamllint（既存 `.yamllint` を使用）+ kubeconform（CRD は `-ignore-missing-schemas` か schema 登録）
5. **Argo CD 静的検証**: 全 `app.yaml` / `applicationset.yaml` の source path 実在チェック、Application 名の重複チェック（takeover 事故の再発防止）。**これが Phase 4 の legacy 削除時に「参照ゼロ」を機械保証する**
6. **（Phase 6 で）frontend unit + smoke E2E**: 上記 4. Phase 6 参照

## 8. レビュー方針

- **1 PR = 1 関心事**。フェーズをまたぐ PR は作らない。削除 PR は「なぜ安全か」（参照 grep 結果・Argo CD 影響の有無・実クラスタ確認結果）を本文に必ず書く（`.claude/rules/collaboration.md` 準拠: 背景・影響範囲・検証方法を記載）
- **レビュー観点は CLAUDE.md の P0/P1/P2 基準**を適用。特に本プランでは: 生 secret の混入（temp/ 掃除時）、`Prune=false` 漏れ、rendered manifest の commit、を重点確認
- **エージェントレビュー必須**: Claude `/code-review` + 必要に応じ Codex セカンドオピニオン。**merge はレビュー完了 + Yu の明示指示のみ**（独断マージ禁止）
- **Phase 2 の各ステップは PR レビューに加えて実機確認**（zombie Application の消滅・残置リソースの棚卸し結果）を PR コメントに記録してから次へ進む

## 9. ロールバック方針

- **git のみで完結する変更**（Phase 1, 3, 4, 6）: `git revert` が唯一の手順。revert 後に CI green を確認
- **Argo CD 管理リソースに効く変更**（Phase 2, 5）: revert + sync で宣言状態は戻る。ただし:
  - **Vault 側の操作（role 削除・transit key 失効）は git 管轄外** → 手順書に「戻し方」（role 再作成 / key の soft-delete 期間）を必ず併記
  - ns 削除の revert は ns 再作成になるが、per-ns NetworkPolicy・ESO の再 sync まで確認する
  - SealedSecret は controller の master key が生きていれば過去 commit から `kubectl apply` で復号・復元可能（災害時の切り札）
- **untracked 削除**（Phase 1）: revert 不能。退避（R2 / 別ディスク）を削除前チェックリストに入れる
- **各フェーズの completion 時に tag を打つ**（`refactor-phase-N-done`）。問題発覚時の bisect 起点にする

## 10. 実装を GPT-5.5 / Opus 4.7 に依頼する場合の作業単位

原則: **1 依頼 = 1 PR = 上記フェーズ内の 1 項目**。エージェントには毎回 (a) `CLAUDE.md` + 関連 `.claude/rules/*.md`、(b) 本ドキュメントの該当フェーズ、(c) 対象ファイル一覧、を渡す。worktree 作成（`~/kensan-lab.worktrees/<name>`）から PR 作成までを任せ、**merge は任せない**。

| # | 作業単位 | Phase | 難易度 | 渡すコンテキスト |
|---|---|---|---|---|
| W1 | CI: Go test + frontend build workflow | 0 | 低 | `apps/kensan/`、既存 `docs.yml` |
| W2 | CI: helm render + yamllint + kubeconform | 0 | 中 | `charts/`、`kubernetes/`、`.yamllint` |
| W3 | Argo CD 静的検証スクリプト + CI | 0 | 中 | `kubernetes/argocd/`、`kubernetes/README.md` |
| W4 | ルート Makefile 処置 + README structure 修正 | 1 | 低 | ルートファイルのみ |
| W5 | backstage examples 削除（参照 grep 付き） | 1 | 低 | `backstage/app/` |
| W6 | stale docs 棚卸し（kustomize-guidelines 含む） | 1, 3 | 低 | `docs/`、`mkdocs.yml` |
| W7 | ブランチ / PR 棚卸しレポート生成（削除・close はしない） | 1 | 低 | git refs、`gh pr list` |
| W8 | zombie 解消 ①: kensan-dagster instance 削除 | 2 | 中 | `kubernetes/secrets/vault-database-engine/`、本書 §4 Phase 2 |
| W9 | zombie 解消 ②: kensan-users instance 削除（transit key はデータ残存チェック付き） | 2 | 中 | `kubernetes/secrets/vault-transit-engine/`、`docs/secret-management/` |
| W10 | app-prod ns 撤去（参照ゼロ確認付き） | 2 | 中 | `kubernetes/namespaces/`、`kubernetes/argocd/applications/namespaces/` |
| W11 | legacy 残参照の docs / comment 掃除 | 2 | 低 | grep 結果 3 ファイル |
| W12 | docs 再編・ブランドアセット一本化 | 3 | 中 | `docs/`、`README.md` |
| W13 | `apps/kensan-legacy` 削除 + ADR-017 + 参照除去 | 4 | 中（機械的だが巨大） | 参照 grep 結果、tag `kensan-legacy-final` |
| W14 | backstage/manifests 配置の ADR 起案 | 5 | 低 | `docs/adr/`、`kubernetes/README.md` |
| W15 | local-path 廃止完了 | 5 | 中 | `kubernetes/storage/`、PVC 棚卸し |
| W16 | エディタ一本化の比較レポート → 実装 | 6 | 中 | `apps/kensan/frontend/` |
| W17 | frontend Vitest 導入 + 初期テスト | 6 | 中 | `apps/kensan/frontend/` |
| W18 | smoke E2E（Playwright） | 6 | 中 | `apps/kensan/`、`DEPLOY.md` |

**依頼時の注意（全作業共通のガードレール）**:

- `kubectl apply` は Verification Exception の範囲のみ。適用したら push か巻き戻しで必ず収束させる
- multi-arch image・`Prune=false`・rendered manifest 禁止などの P0/P1 は依頼文に明記して渡す
- Phase 2 系（W8-W10）は自律実行させず、実機確認のチェックポイントを Yu / メインセッションが挟む
- Vault 側の操作（role / key）は git 外なので、実行コマンドを `temp/*.sh` に書き出して Yu が実行する形にする

---

## Appendix: 調査時点のスナップショット（2026-07-03、origin/main f0a31dd）

- 追跡ファイル 1,192 / LOC: legacy 81,940、新 kensan 8,714（go/ts/py/css）
- cutover: PR #394（7/2）で旧 `kensan` Application 削除 + HTTPRoute 移管済み。ns `kensan`・pods・PVC は手動 teardown 済みでクラスタに存在しない（実機確認済み）
- 残存 zombie: `vault-db-kensan-dagster` / `vault-transit-kensan-users`（OutOfSync / Missing）
- ns `app-prod`: 実質空（ghcr-pull-secret ESO + default SA のみ）
- ブランチ: local 180 / remote 98。open PR: #395, #388(draft), #387(draft), #352, #298, #256。`feat/polaris-postgres-persistence` は目的消滅（legacy Polaris 永続化）
- CI: `.github/workflows/docs.yml` のみ
- テスト: backend 8 ファイル / frontend 0 / 新 E2E なし / legacy 46（廃止済み）
