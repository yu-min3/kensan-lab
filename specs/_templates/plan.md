# Plan: <機能名>

> Spec: [./spec.md](./spec.md)
> how を書く。spec.md の what/why を満たす技術設計。

## Constitution check

適用される既存ルール（複製せずリンクで参照。設計はこれらに従う）:

- [ ] [`gitops-workflow.md`](../../.claude/rules/gitops-workflow.md) — GitOps only / worktree / deploy order
- [ ] [`helm-multisource.md`](../../.claude/rules/helm-multisource.md) — 3-file パターン
- [ ] [`security-secrets.md`](../../.claude/rules/security-secrets.md) — Vault / ESO / SealedSecret
- [ ] [`network-ingress.md`](../../.claude/rules/network-ingress.md) — Gateway / HTTPRoute / cert
- [ ] [`kubernetes-cluster.md`](../../.claude/rules/kubernetes-cluster.md) — node / scheduling / storage
- [ ] [`environment-separation.md`](../../.claude/rules/environment-separation.md) — namespace / Argo CD project

該当しない行は削除し、各チェックで「この設計がどうルールを満たすか」を一言添える。

## 配置 (Layout)

- **category**: <observability | network | security | storage | environments | gitops | ...>
- **component path**: `kubernetes/<category>/<component>/`
- **パターン**: <Pattern A (Helm multi-source) | Pattern B (raw YAML) | ApplicationSet>
- **Application CR**: `kubernetes/argocd/applications/<category>/<component>/app.yaml`

## Chart & version

- **chart repo**: <例: https://prometheus-community.github.io/helm-charts>
- **chart**: <例: kube-prometheus-stack>
- **targetRevision (固定 version)**: <例: 79.5.0>   ← `main`/`latest`/`HEAD` 禁止

(Pattern B / 自作マニフェストのみの場合はこの節を「N/A（raw YAML）」に)

## Argo CD project / namespace / syncPolicy

- **Argo CD project**: <platform-project | app-project>
- **destination namespace**: <namespace>
- **syncPolicy**: <automated prune/selfHeal の有無、ServerSideApply、CreateNamespace 等>
- **sync wave / 依存**: <先行が必要な CRD / コンポーネント>

## ネットワーク (Gateway / HTTPRoute)

- **Gateway**: <gateway-platform (192.168.0.242) | gateway-prod (192.168.0.243) | なし>
- **host**: <例: foo.platform.yu-min3.com>
- **HTTPRoute**: `kubernetes/<category>/<component>/resources/httproute.yaml`

## Secrets 方式

- **方式**: <Vault dynamic | Vault static (ESO) | Vault Transit | SealedSecret | なし>
- **Vault path / secret 名**: <例: secret/foo, foo-credentials>
- **Reloader**: <annotation を付けるか>

## Affected paths

> /sdd-impl の検証フェーズが対象を確定するためのシグナル。触る / 作るパスを列挙。

- `kubernetes/<category>/<component>/values.yaml`
- `kubernetes/<category>/<component>/resources/`
- `kubernetes/argocd/applications/<category>/<component>/app.yaml`

## Risks / rollback

- **リスク**: <破壊的変更・既存への影響>
- **rollback**: <revert で戻せるか / state を持つか>
