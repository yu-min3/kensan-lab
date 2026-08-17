# AI Harness Phase 0 — implementation review report

## 結論

AI Harness Phase 0は、**Claude Codeを唯一の変更主体、Codexをread-onlyの独立reviewer、Yuを出荷gate**とする最小ループとして実装した。旧PR #388の1,258行を移植せず、現行の規約とCIを正として、SDD固有の状態・停止条件・独立レビューだけを再構成している。

Full SDD dogfoodではkind demoのPVC永続性を題材に、spec → plan → pre-implementation review → implementation → verification → diff review → handoffを一周した。独立レビューの最終結果はP0/P1/P2なしで、専用kind clusterでもPVC Bound、Pod UID交代、marker永続、Service HTTP 200まで確認した。

2026-08-17にlatest main（`1340773`）へrebaseし、競合なしで追従した。SDD gate・17 fixture・workflow YAML・diff checkは再通過した。Manifest runnerとdemo kubeconformはGitHub上のschema取得障害で完走できず、外部依存の再実行待ちである。また、`spec.md`のAcceptance checkboxが未更新でもhandoff gateを通過できる点は、Phase 0で残った実装上の穴である。

## レビュー対象

| 項目 | 値 |
|---|---|
| Branch | `feat/001-kind-pvc-dogfood` |
| Phase 0 commit | `a69580c feat(sdd): AIハーネスPhase 0追加` |
| Dogfood commit | `7d0242d test(explore): PVC永続性を検証` |
| Base | `1340773`（2026-08-17 rebase、mainとのbehind 0） |
| Diff | 26 files / +972 / -46（本レポート追加前） |
| Remote | 未公開。push / PR作成なし |
| Worktree | `/private/tmp/kensan-lab-sdd-dogfood` |

Claude Codeには、まず上記2 commitsを分けてレビューしてほしい。Phase 0の仕組みとdogfoodの業務変更を同じ観点で混ぜないためである。

## Acceptance criteriaとは何か

Acceptance criteria（受入基準）は、**何を観測できたら「要求を満たした」と判断するか**を実装前に固定したもの。このハーネスでは、エージェントが「たぶん完成」で停止しないためのoracleであり、テスト項目そのものより一段上の完了条件である。

今回のdogfoodでは、`specs/001-kind-pvc-dogfood/spec.md`に次の5項目を定義した。

| # | Acceptance criterion | 検証結果 |
|---|---|---|
| 1 | Helm renderが`longhorn` PVCと`/data` mountを生成する | local verified |
| 2 | 生成PVCに`Prune=false`が付く | local verified |
| 3 | app-demo PVCが`Bound`になる | isolated kind verified |
| 4 | `/data`のmarkerがPod再作成後も一致する | isolated kind verified |
| 5 | Pod再作成後にHTTP 200へ戻る | Service verified、GatewayはExplore CIへdefer |

問題は受入基準の内容ではなく、**結果の保存場所が二重になったこと**である。実行結果は`tasks.md`のAcceptance statusへ記録した一方、要求のSoTである`spec.md`のcheckboxは`[ ]`のままになった。それでも現行gateはcriterionが存在すれば通すため、`status: implemented`と未チェックcheckboxが共存できる。

したがって出荷前に判断すべきなのは「この5項目が必要か」ではなく、**完了状態をspecへ直接戻すか、criterion IDと別status表を機械照合するか**である。

## 対象範囲

### Phase 0に含めたもの

| 領域 | 実装 | 意図 |
|---|---|---|
| 適用判定 | Full / Lite / None classifier | 小変更へ一律にceremonyを課さない |
| 状態 | `specs/NNN-<slug>/` | chatやterminalではなくfileをSoTにする |
| Authoring | `/sdd-spec`, `/sdd-plan`, `/sdd-tasks`, `/sdd-impl` | 変更主体をClaude Codeへ一本化する |
| Independent review | `codex-pre-impl.md`, `codex-impl.md` | 実装前とdiff後に別視点で反証する |
| Deterministic gate | `scripts/sdd_gate.py` | 欠落artifact、未解決P0、未承認fallbackを機械的に止める |
| Verification | `scripts/validate-manifests.sh` | localとManifest CIで同じ検査定義を使う |
| Regression | `tests/test_sdd_gate.py`, `sdd-ci.yml` | Full/Liteとnegative caseをfixtureで固定する |
| Documentation | SDD guide、templates、doc-layout | 運用契約と作業状態の置き場を明確にする |

### 意図的に含めなかったもの

| 選択肢 | 判定 | 理由 |
|---|---|---|
| 旧PR #388をrebase / cherry-pick | 却下 | 既存CIとの重複、手書きHTML、現行規約との差が大きい |
| Codexへ実装権限を与える | 却下 | worktree・task・変更状態の所有者が二重になる |
| Codex findingの自動修正 | 却下 | 独立reviewerが変更主体へ変質する |
| 自動ready / merge | 却下 | 人間の不可逆gateを失う |
| loop C / drift検出 | Phase 2以降 | Phase 0の価値検証前に面を広げない |
| Full dogfoodで本番PVCを操作 | 却下 | 初回評価に本番データriskを混ぜない |

## 設計意図

### 1. 指示ではなくoracleを強くする

このハーネスが固定するのは、実装手順そのものではない。固定対象は次の4つである。

1. 何を満たせば完了か。
2. 誰が変更できるか。
3. 何を検証し、未検証をどう記録するか。
4. どの状態なら停止して人間へ返すか。

実装方法はClaude Codeに残し、`scripts/sdd_gate.py`は成果物の存在・未解決marker・review metadata・P0状態を検査する。これは「長い手順を守らせる」より「正しい出口以外を閉じる」設計である。

### 2. 状態の所有者を一人にする

```text
Yu intent
  -> Claude: classify + spec + plan
  -> Codex: read-only pre-implementation review
  -> Claude: adjudicate + implement + verify
  -> Codex: read-only diff review
  -> Claude: adjudicate + handoff
  -> Yu: ready / merge decision
```

Claude Codeだけがworktree、file、task checkbox、commitを変更する。Codexの出力は一度`temp/`へ受け、Claude Codeが内容を確認してreview artifactへ転記する。Codexが利用不能な場合は自動続行せず、Yuが承認したhuman fallbackのreviewerと結果を記録しなければgateを通さない。

### 3. CIとlocalの検証定義を一本化する

旧PRはyamllint、kubeconform、Helm render等のコマンドをskill本文へ複製していた。Phase 0では`.github/workflows/manifest-ci.yml`の実体を`scripts/validate-manifests.sh`へ抽出し、CIとlocalが同じentrypointを呼ぶ。skillは検証の責務だけを示し、コマンド定義を持たない。

## 実装内容

### Gate 0 — classification

`scripts/sdd_gate.py classify`と`/sdd-spec`が次の優先順を持つ。

1. irreversible / stateful / secrets method change → Full
2. new component / namespace / public host → Full
3. behavior changeかつ固有のacceptance criterionあり → Lite
4. それ以外 → None

Fullが他条件より必ず優先される。Liteは`plan.md`と`tasks.md`を要求しない。

### Artifact gate

`scripts/sdd_gate.py validate`はimplement / handoffの2段階を持つ。

| Check | Implement | Handoff |
|---|---:|---:|
| `spec.md` + mode + Acceptance criteria | 必須 | 必須 |
| Fullのplan / tasks必須section | 必須 | 必須 |
| pre-implementation review metadata | 必須 | 必須 |
| unresolved P0 / unknown decision | block | block |
| Codex unavailable時のYu承認 + human reviewer/result | block | block |
| implementation review | — | 必須 |

fixtureは17件あり、空artifact、section外checkbox、review metadata欠落、unknown decision、未承認fallback、未解決P0をnegative caseとして持つ。

### Full dogfood

dogfoodは`specs/001-kind-pvc-dogfood/`に監査記録を残した。

| Resource | Change | Merge後の効果 |
|---|---|---|
| `PersistentVolumeClaim/demo-data` | 新規、128Mi、`longhorn`、`Prune=false` | kindのstorage substitutionをworkloadから使う |
| `Deployment/demo` | `/data` mount追加 | Pod置換を跨ぐ永続性を検証できる |
| Explore CI | Bound → write → Pod delete → UID交代 → marker一致 → HTTP 200 | stateful pathをbehavior oracleで確認する |
| Delete | なし | 自動pruneによる削除は発生しない |

shared `app-base` chartのPVC templateにも`Prune=false`を追加した。ただし現在の本番consumerであるkensan / konroはいずれも`pvc.create: false`で、既存raw PVCを参照するため、この変更が本番PVCを新規生成・変更することはない。

## 検証結果

| Gate | Result | Evidence |
|---|---|---|
| SDD fixture | PASS | 17 tests |
| Manifest runner | PASS | 220 resources、invalid 0 |
| Argo CD static validator | PASS | 44 Applications |
| demo Helm render | PASS | PVC、`longhorn`、`/data` mount、`Prune=false` |
| kubeconform | PASS | 5 resources、invalid 0 |
| server dry-run | PASS | isolated kind cluster |
| PVC bind | PASS | `demo-data` / 128Mi / `longhorn` |
| Pod replacement | PASS | UID `c873277c-...` → `fb81d4b6-...` |
| marker persistence | PASS | `sdd-live-2026-08-12`完全一致 |
| workload HTTP | PASS | replacement後Service経由200 |
| Gateway route | 未実行 | 公開revisionが必要。Explore CIで確認 |
| independent diff review | PASS | 最終P0/P1/P2なし |

## レビューで実際に見つかった問題

独立reviewは帳票確認だけでなく、以下の実装問題を検出した。

| Priority | Finding | Resolution |
|---|---|---|
| P1 | GitHub runnerの`/usr/local/bin`へsudoなしinstall | checkout内binaryをPATHへ追加 |
| P1 | deterministic gateがskillから未接続 | implement / handoffで必須実行 |
| P1 | 空plan/tasks/reviewがgateを通る | section・metadata・内容検査を追加 |
| P1 | fallback承認文字列だけでhuman reviewを省略可能 | reviewer identity + result必須化 |
| P1 | `rollout status`がPod置換後Readyを保証しない | Ready + old UID除外 + cardinality=1を直接poll |
| P1 | helperの3列出力を2変数で受けUID比較が壊れる | 出力をname / UIDの2列へ限定 |

## 既知の問題・制約

### ✅ Rebase完了 — latest mainとの差

2026-08-17にmain `1340773`へ競合なしでrebaseし、behind 0になった。SDD handoff gate、17 fixture、workflow YAML、diff checkは成功した。Manifest runnerとdemo kubeconformは`raw.githubusercontent.com`からのschema取得が繰り返し失敗したため、ネットワーク回復後またはCIでの再実行が必要。

### 🟠 Acceptance checkboxの状態不整合

`spec.md`は`status: implemented`だが、`## Acceptance criteria`のcheckboxは`[ ]`のままである。検証結果は`tasks.md`へ記録されているものの、要求のSoT上は未完了に見える。さらに現行`sdd_gate.py`はcriterionの存在を検査するが、handoff時に全項目が`[x]`または理由付きdeferredであることを機械検査しない。

推奨は、Acceptance criteriaに状態を直接持たせるか、criterion IDと`tasks.md`のstatus tableを照合するvalidatorを追加することである。単に今回のcheckboxを手で`[x]`へ変えるだけでは再発防止にならない。

### 🟠 Gateway E2Eは出荷後確認

専用kind clusterはGateway stackを入れない最小構成だったため、Service HTTP 200までを確認した。Gateway host経由の200は、branchをremoteから参照できる状態にしてExplore CIを走らせる必要がある。

### 🟠 main worktreeへは統合していない

mainのworktreeに別作業の大規模な未コミット差分が存在したため、安全のためmergeしていない。remote操作もworkspace規約により行っていない。

### 🟡 live検証時のkubeconfig事故

最初の一時kind検証で、専用contextが登録されていないまま後続commandが既存Explore clusterへ向いた。空の`demo-data` PVCとlast-applied annotationが一時的に作られた。直後に既知の変更を削除し、Deploymentにvolume mountがないこと、Serviceが元のport 8000であることを確認した。その後は専用kubeconfigを明示し、検証clusterを削除した。

再発防止として、live test script化時は次をhard gateにする必要がある。

- `KUBECONFIG`を専用fileへ固定する。
- `kubectl config current-context`が期待値と完全一致しなければ終了する。
- setup commandを`;`で継続せず`set -euo pipefail`または`&&`で停止する。
- 作成resourceへtest run固有labelを付け、cleanup対象を機械的に限定する。

## Claude Codeに見てほしい観点

1. **指示過多**: 4 skillとtemplateが、oracleを強くする範囲を超えて手順消化を促していないか。
2. **gateの正しさ**: `sdd_gate.py`にfalse positive / false negative、Markdown parser依存の脆さがないか。
3. **権限境界**: Codex read-only、Claude owner、human gateがskillと実装で一致しているか。
4. **CI共通化**: `validate-manifests.sh`抽出で既存Manifest CIの意味を変えていないか。
5. **Lite成立性**: plan/tasksなしで本当に安全にhandoffへ到達できるか。
6. **dogfood CI**: Pod selection、timeout、UID交代、marker checkがflakyでないか。
7. **Prune=false**: shared chartへの常時annotationが適切か、valuesでopt-out/inすべきか。
8. **最新mainとの統合**: rebase後のmain変更により前提が変わっていないか。schema取得失敗分をCIで再確認できるか。

## Yuが決めること

1. Phase 0の運用モデルをこの形で採用するか。
2. Acceptance criterionの完了状態をgateで強制する修正を、出荷前の必須対応にするか。
3. Claude Code review後、latest mainへrebaseして通常の出荷フローへ渡すか。
4. Gateway E2E成功をPhase 1完了条件にするか。
