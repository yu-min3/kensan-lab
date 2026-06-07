# Policy Enforcement (Kyverno 統一設計)

クラスタの Policy as Code の SoT。設計判断の経緯は [ADR-012](../adr/012-policy-enforcement-kyverno.md) を参照。

**v2 統一設計**: PSS の強制は Kyverno に一本化する。PSA (Pod Security Admission) は
label を撤去して不活性化し、ns の PSS level は Kyverno 専用 label
`kensan-lab.platform/pss-level` で宣言する (PSA の label key を流用すると PSA 本体が
再活性化するため、独自 key を使う)。

## pss-level label のセマンティクス

| `kensan-lab.platform/pss-level` | 意味 | 宣言できる条件 | 現在の対象 |
|---|---|---|---|
| (無印 = デフォルト) | `pss-baseline` の床が適用 | — | 大多数の ns |
| `privileged` | 床から除外 (host アクセス前提の PE 専管 ns) | `tier=platform` のみ (`ns-label-contract` rule 3 が強制) | kube-system / istio-system / longhorn-system / local-path-storage |
| `restricted` | 床より厳格な PSS restricted を適用 (opt-in) | — | app-kensan |

未知の値は何にも match せず床に落ちる (安全側フォールバック)。

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
| `pss-baseline` | `pss-level=privileged` の ns **以外すべて** (床) | **Audit** | PSS baseline (`validate.podSecurity`) |
| `pss-restricted` | `pss-level=restricted` の ns (opt-in) | **Audit** | PSS restricted (旧 PSA enforce=restricted の引き継ぎ) |
| `disallow-latest-tag` | app tier | **Audit** | `:latest` / tag 省略禁止 (ADR-011 の教訓) |
| `require-requests` | app tier | **Audit** | cpu / memory requests 必須 |
| `ns-label-contract` (3 rules) | rule1: 全 ns (K8s 組み込み除く) / rule2: `app-*` (app-prod 除外) / rule3: `pss-level=privileged` の ns | **Audit** | rule1: environment + tier (値域込み) 必須 / rule2: ADR-006 3-axis + tier=application / rule3: privileged 宣言は tier=platform のみ |

「app tier」= `kensan-lab.platform/tier=application` label の ns ∪ ns 名 `app-*` / `kensan`。
label が SoT で、name match は label 欠落 ns が静かに scope 外へ落ちるのを防ぐ defense-in-depth
(label 欠落自体は ns-label-contract が検出する)。scope 定義は disallow-latest-tag /
require-requests の match block と本 doc の 3 ヶ所にあり、変更時は同期すること。

全ポリシー `webhookConfiguration.failurePolicy: Ignore` (admission controller replica 1 のため。ADR-012 §3)。

## PolicyException Inventory

| 例外 | 対象 | exempt する control | 理由 |
|---|---|---|---|
| `node-exporter-host-access` | monitoring / label selector (`prometheus-node-exporter`) | Host Namespaces / Host Ports (images 限定) / HostPath Volumes | node metrics 収集に host アクセスが必須 |

**PolicyException の podSecurity 指定の注意**: container レベルの control (Host Ports /
Capabilities 等) は `controlName` だけでは exempt されず **`images` の併記が必須**
(pod レベルの Host Namespaces / HostPath Volumes は controlName のみで効く)。
exception を書いたら `kyverno apply <policy> --resource <live pod> --exceptions <polex>`
のオフライン評価で skip になることを確認してから commit する (background scan の 1h を待たずに検証できる)。

## 運用

### violation の確認

```bash
kubectl get cpol                                  # ClusterPolicy の Ready 状態
kubectl get polr -A                               # namespaced リソースの PolicyReport
kubectl get cpolr                                 # cluster-scoped リソース (Namespace 等) は
                                                  # ClusterPolicyReport — ns-label-contract はこちら
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

### ns の PSS level を変える

1. 対象 ns の `namespace.yaml` で `kensan-lab.platform/pss-level` label を設定 (`privileged` / `restricted`。無印 = baseline の床)
2. `privileged` は `tier=platform` の ns でないと `ns-label-contract` rule 3 に違反する (Enforce 後は拒否)
3. PR → Argo CD sync

### Enforce 昇格 (Phase 3)

1. 対象ポリシーの violation が PolicyReport 上でゼロであることを確認
2. **`ns-label-contract` を最初に Enforce する** (他 policy の scope が label に依存するため、土台から)
3. `validate.failureAction: Audit` → `Enforce` に変更して PR — pss-baseline / pss-restricted → latest / requests の順
4. **PSA label の撤去は Enforce 昇格と同じ PR で行う** (atomic swap — 強制の空白期間を作らない)
5. app-tier policy の name match (`app-*` / `kensan`) を撤去して label selector に一本化
6. webhook `failurePolicy: Fail` への昇格条件 (ADR-012 改訂): Enforce 安定数週間 + `replicas: 2` + `config.webhooks` で kube-system を webhook レベル除外 + Fail は app-tier policy のみ

## 既知の残課題

- **PSA label の撤去** (kube-system / istio-system / longhorn-system / local-path-storage / app-kensan / app-prod / kensan / argocd / auth 系 / secrets 系 / kyverno ns): Phase 3 の Enforce 昇格と同時に実施 (atomic swap)。それまでは pss-level label と併存
- **bare ns の label remediation** — `ns-label-contract` rule 1 の violation。**初回 scan で実測済み (2026-06-07): `cilium-secrets` / `local-path-storage` / `reloader` / `sealed-secrets` の 4 ns**:
  - `reloader` / `sealed-secrets`: ns manifest の追加 or `managedNamespaceMetadata` で対応 (kubernetes/secrets/ 配下)
  - `cilium-secrets`: cilium が自動生成。ns 管理方法を確認して対応
  - `local-path-storage`: Git の manifest は Argo CD 管理外 (Application CR なし、bootstrap 期の kubectl apply 由来) のため Git 修正が live に届かない。廃止完了による ns 削除が本筋の解消
  - ※ `backstage` / `kensan` は実測で**準拠を確認** (どちらも live に environment + tier あり) — 事前予測から除外
- **blackbox-exporter の pss-baseline FAIL は履歴ノイズ** (実測 2026-06-07): violator は旧 revision の ReplicaSet (replicas=0、template に NET_RAW が残存)。**現行 Pod は NET_RAW を落とした準拠版**のため PolicyException は作らない (live が不要な権限の恒久付与になる)。Deployment の revision rotation で自然消滅する。**Phase 3 の「violation ゼロ」判定は replicas=0 の履歴 ReplicaSet を除外して行う**
- **app-kensan の unprefixed labels** (`team` / `app` / `environment`): prefixed 正準 labels は PR #368 で追記済み。unprefixed 側は既存 selector 互換のため残置中 — 参照が無いことを確認できたら削除
- **platform 側の requests 未設定** (~50 container: argocd / cert-manager / longhorn 等): mutate ではなく各 values.yaml への追記で解消する (ADR-012 §2)
- **verifyImages (image 署名検証)**: 将来課題。Backstage CI への cosign 導入とセットで検討
