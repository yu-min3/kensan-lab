# Policy Enforcement (Kyverno)

クラスタの Policy as Code の SoT。設計判断の経緯は [ADR-012](../adr/012-policy-enforcement-kyverno.md) を参照。

## 構成

| 要素 | 場所 | Argo CD app |
|---|---|---|
| kyverno ns | `kubernetes/policy/namespace.yaml` | `kyverno-namespace` (environments パターン、wave -4) |
| Engine (Helm) | `kubernetes/policy/kyverno/values.yaml` | `kyverno` (Pattern A、wave -2) |
| ポリシー本体 | `kubernetes/policy/kyverno-policies/*.yaml` | `kyverno-policies` (Pattern B、wave -1) |
| 例外 | `kubernetes/policy/kyverno-policies/exceptions/` | 同上 |

- engine とポリシーは **別 app** — ポリシー変更の PR が engine の Helm sync に触れない
- `PolicyException` は **kyverno ns のみ受理** (`features.policyExceptions.namespace=kyverno`)。例外の追加は必ず Git 経由

## ポリシー Inventory

| ポリシー | scope | mode | 内容 |
|---|---|---|---|
| `pss-baseline` | PSA `enforce=privileged` label の ns **以外** (label selector 除外 — ns 側の PSA 宣言が SoT) | **Audit** | PSS baseline (`validate.podSecurity`)。monitoring 等 PSA 未設定 ns の回収 |
| `disallow-latest-tag` | app tier | **Audit** | `:latest` / tag 省略禁止 (ADR-011 の教訓) |
| `require-requests` | app tier | **Audit** | cpu / memory requests 必須 |
| `require-ns-labels` | `app-*` ns (**app-prod は exclude** — env-shared landing zone のため単一 team/app を割当不能) | **Audit** | ADR-006 3-axis labels + `tier=application` 必須 |

「app tier」= `kensan-lab.platform/tier=application` label の ns ∪ ns 名 `app-*` / `kensan`。
label が SoT で、name match は label 欠落 ns が静かに scope 外へ落ちるのを防ぐ defense-in-depth
(label 欠落自体は require-ns-labels が検出する)。scope 定義は disallow-latest-tag /
require-requests の match block と本 doc の 3 ヶ所にあり、変更時は同期すること。

全ポリシー `webhookConfiguration.failurePolicy: Ignore` (admission controller replica 1 のため。ADR-012 §3)。

## PolicyException Inventory

| 例外 | 対象 | exempt する control | 理由 |
|---|---|---|---|
| `node-exporter-host-access` | monitoring / `*node-exporter*` | Host Namespaces / Host Ports / HostPath Volumes | node metrics 収集に host アクセスが必須 |

## 運用

### violation の確認

```bash
kubectl get cpol                                  # ClusterPolicy の Ready 状態
kubectl get polr -A                               # namespaced リソースの PolicyReport
kubectl get cpolr                                 # cluster-scoped リソース (Namespace 等) は
                                                  # ClusterPolicyReport — require-ns-labels はこちら
kubectl get polr -A -o wide | grep -v " 0 *$"     # FAIL を含む report に絞る
kubectl describe polr -n <ns> <name>              # violation の詳細
```

PolicyReport は background scan (1h 間隔、values.yaml で明示 pin) 由来。admission 毎の
レポートは microSD etcd 保護のため無効化している (`features.admissionReports: false`) —
検出は最大 1h 遅延する。background scan の PolicyReport 生成は reports-controller の担当
(background-controller は generate / mutateExisting 専任のため無効化済み)。

### ポリシーの追加

1. `kubernetes/policy/kyverno-policies/<name>.yaml` に ClusterPolicy を置く (`failureAction: Audit` で開始、`webhookConfiguration.failurePolicy: Ignore`)
2. commit → push → Argo CD sync
3. PolicyReport で violation を観測してから Enforce を検討

### 例外の追加

1. `kubernetes/policy/kyverno-policies/exceptions/<workload>.yaml` に PolicyException を置く (namespace は `kyverno` 固定)
2. `podSecurity` の `controlName` で exempt 範囲を最小に絞る
3. Pod controller を exempt する場合は `ruleNames` に `autogen-<rule>` も併記する

### Enforce 昇格 (Phase 3)

1. 対象ポリシーの violation が PolicyReport 上でゼロであることを確認
2. `validate.failureAction: Audit` → `Enforce` に変更して PR
3. app tier から先行、platform 系は例外整備済みのものから順次
4. 全ポリシー Enforce 後、webhook `failurePolicy: Fail` への昇格を再検討 (ADR-012 Trade-offs)

## 既知の残課題

- **app-kensan の unprefixed labels** (`team` / `app` / `environment`): prefixed 正準 labels は PR #368 で追記済み (`kubernetes/apps/app-kensan/resources/namespace.yaml`)。unprefixed 側は既存 selector 互換のため残置中 — 参照が無いことを確認できたら削除
- **platform 側の requests 未設定** (~50 container: argocd / cert-manager / longhorn 等): mutate ではなく各 values.yaml への追記で解消する (ADR-012 §2)
- **local-path-provisioner の helper pod** (hostPath 使用、ephemeral): pss-baseline の scope 内で exception なし。local-path 自体が段階的廃止のため放置 — 廃止前に Enforce する場合は exception が必要
- **verifyImages (image 署名検証)**: 将来課題。Backstage CI への cosign 導入とセットで検討
