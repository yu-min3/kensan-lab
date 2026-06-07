# ADR-012: Kyverno による Policy Enforcement (validate-only / Audit → Enforce)

## Status

**Accepted** (2026-06-06)

## Date

2026-06-06

## Context

### PSA (Pod Security Admission) の限界

クラスタの Pod Security は PSA の namespace label で管理してきたが、PSA は **namespace 単位の all-or-nothing** しかできない。`kubernetes/observability/namespace.yaml` では node-exporter (hostNetwork / hostPID / hostPort / hostPath を要求) が baseline と非互換のため、monitoring ns 全体の enforce を断念し、以下の TODO を残していた:

```
TODO(kyverno): Kyverno 統合後に ClusterPolicy で baseline 強制 + 既知 workload
  (node-exporter / vault 等) を per-workload exempt する形で復活させる
```

### その他の動機

1. **ADR-011 の silent upgrade 事故**: Vault が `:latest` tag で major 跨ぎの silent upgrade を起こした (PR #271)。image tag 規律を engine で強制したい
2. **ADR-006 の label 強制**: ADR-006 自身が「new-namespace creation procedure must enforce it as a required field」と要求している。実機照合 (2026-06-06) では app-kensan が unprefixed の `team` / `app` label を使っており、強制機構の不在が既に表記ゆれを生んでいる
3. **AD tenant のガードレール**: Backstage 経由デプロイの品質 (requests 必須等) を Git 管理のポリシーで担保する

### 検討した選択肢

| 観点 | Kyverno (採用) | OPA Gatekeeper | PSA のみ (現状) |
|---|---|---|---|
| ポリシー記述 | YAML ネイティブ | Rego 言語 | label のみ |
| PSS 対応 | `validate.podSecurity` 組み込み | ConstraintTemplate 自作 | ◯ (ただし ns 単位) |
| per-workload 例外 | `PolicyException` CRD | label ベースのみ | ✗ |
| GitOps 親和性 | CRD ベース、Argo CD 実績多数 | 同等 (2 段構成) | — |

## Decision

**Kyverno (chart 3.8.1 / v1.16 系) を validate-only で採用し、Audit → Enforce の段階ロールアウトを行う。**

### 1. 構成

- 新 category `kubernetes/policy/` を作成。engine (`kyverno` app、Pattern A) とポリシー本体 (`kyverno-policies` app、Pattern B) を **別 Application に分離** — ポリシー変更の PR が engine の Helm sync を触らない (network-policy と同じ思想)
- `PolicyException` は `features.policyExceptions.namespace=kyverno` で **kyverno ns のみ受理**。例外も必ず Git (`kubernetes/policy/kyverno-policies/exceptions/`) を経由する
- sync-wave: ns bootstrap `-4` → engine `-2` → policies `-1` (CRD 登録順を保証)

### 2. validate のみ。mutate は使わない

mutate は Argo CD の reconciliation loop と競合する (CNCF "GitOps and mutating policies: the tale of two loops")。mutate → live が Git と乖離 → selfHeal が再 apply → 再 mutate のループ / OutOfSync 常態化が起きる。

- 注入したい値は **Git の manifest に直接書く** のが本リポの正道 (全リソース Git 管理のため mutate の出番が構造的にない)。例: platform 側の requests 未設定 ~50 container は values.yaml への追記で解消する
- 将来 mutate が必要になった場合は、対象 Application に限定して Argo CD 2.10+ の Server-Side Diff を併用する: `argocd.argoproj.io/compare-options: ServerSideDiff=true,IncludeMutationWebhook=true` (diff 計算が SSA dry-run を通り、mutation 込みの predicted live state で比較される)。全 app 一括有効化は admission controller への負荷増の既知 issue があるため per-app annotation に限定する

### 3. ホームラボ制約への対応

| 項目 | 設定 | 理由 |
|---|---|---|
| `features.admissionReports` | **無効化** | master の etcd は microSD。admission 毎の EphemeralReport 書き込みを止め、violation 可視化は background scan (1h) 由来の PolicyReport に一本化 |
| `backgroundScanInterval` | `1h` を明示 pin | 設計値を chart default 任せにしない (ADR-011 と同型の silent change 防止) |
| `features.reporting` | validate のみ | mutate / generate / imageVerify は未使用 |
| 各 controller replicas | 1 | homelab に HA 不要 |
| webhook `failurePolicy` | `Ignore` (policy 側で指定) | admission controller (replica 1) 停止時に cluster 全体の Pod 作成を止めない。Enforce 安定後に `Fail` 昇格を判断 |
| `cleanupController` | 無効化 | CleanupPolicy 未使用。Pi のリソース節約 |
| `backgroundController` | 無効化 | 担当は generate / mutateExisting のみで validate-only 構成では恒久 idle (background scan の PolicyReport 生成は reports-controller の担当) |
| nodeAffinity | preferred `hardware-class=high-performance` (weight 80) | webhook latency 抑制。Medium カテゴリの scheduling rule に準拠 |

### 4. 初期ポリシーと scope

| ポリシー | scope | 初期 mode |
|---|---|---|
| `pss-baseline` | PSA `enforce=privileged` label の ns **以外の全 ns** (label selector 除外) | Audit |
| `disallow-latest-tag` | app tier (`tier=application` label ∪ `app-*` / `kensan` ns 名) | Audit |
| `require-requests` | 同上 | Audit |
| `require-ns-labels` | `app-*` ns (ADR-006)。app-prod は env-shared landing zone のため exclude | Audit |

privileged 設計 ns (現状 kube-system / istio-system / longhorn-system) は PSA `enforce: privileged` を明示済みの PE 専管領域なので、**ns 側の PSA 宣言を SoT とした label selector** で scope 外とする (name list のハードコードを避け、4 つ目の privileged ns 追加時にこの policy の編集を不要にする)。これにより必要な PolicyException は **node-exporter 1 件のみ** (実機照合 2026-06-06。kensan の hook Job 2 件に requests 欠落があったが、本 PR で requests を追記して解消)。

### 5. 段階的ロールアウト

1. **Phase 1**: 全ポリシー Audit で投入、PolicyReport で violation と etcd 負荷を 1〜2 週間観測
2. **Phase 2**: violation を棚卸しし、PolicyException の追加 or Git 側 remediate で violation ゼロ化
3. **Phase 3**: app tier から Enforce 昇格 → platform 系は例外整備が済んだものから順次。webhook failurePolicy の `Fail` 昇格もここで判断

## Consequences

### Positive

- monitoring ns の TODO(kyverno) を回収: baseline 監査 + node-exporter のみ最小 control (Host Namespaces / Host Ports / HostPath Volumes) で exempt
- 例外が Git 管理の宣言的リソースになる (PSA 時代は「ns ごと諦める」しかなかった)
- ADR-006 / ADR-011 の規律に enforcement 機構がつく
- 全ポリシー Audit 開始 + failurePolicy Ignore のため、導入時の blast radius がゼロ

### Trade-offs

- Audit 期間中は強制力がない (Phase 3 まで violation は report されるだけ)
- failurePolicy Ignore の間は、admission controller 停止中に violation が素通りする (background scan が事後検出はする)。Enforce 昇格時に Fail 化とのバランスを再検討
- admissionReports 無効化により、violation の検出は最大で background scan 間隔 (1h) 遅延する
- Kyverno 分のリソース消費 (~200m / 256Mi requests、3 controller)。実測でキャパシティに問題ないことを確認済み

## References

- [ADR-006](006-namespace-naming.md): Namespace Naming (3-axis labels)
- [ADR-011](011-vault-version-pinning.md): Vault `:latest` silent upgrade 事故
- [`docs/concepts/policy-enforcement.md`](../concepts/policy-enforcement.md): ポリシー inventory / 運用手順 (SoT)
- [CNCF: GitOps and mutating policies — the tale of two loops](https://www.cncf.io/blog/2024/01/18/gitops-and-mutating-policies-the-tale-of-two-loops/)
- [Argo CD Diff Strategies (Server-Side Diff)](https://argo-cd.readthedocs.io/en/stable/user-guide/diff-strategies/)
