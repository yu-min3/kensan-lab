# Documentation Layout

このリポジトリのドキュメントは **層ごとに役割が分かれている**。新しい情報を書くときに「どこに書けばいいか」が機械的に決まるよう、各層の責務を以下に固定する。**同じ事実を複数の場所に書かない** のが基本ルール。

> **読む時 vs 書く時**: このページは「新しい事実を **どこに書くか**」の authoring policy。既存の事実が **どこにあるか** を引く時は、AI/operator 向けの目次 [agents-index.md](../agents-index.md) を入口にする。docs サイトは人間向けの **📖 Understand**（概要・図・narrative）と、AI/operator 向けの **🤖 Operate & Reference**（決定的・網羅・引く）の 2 ゾーンに分かれている。事実は二重化せず、両ゾーンは link で繋ぐ。

## 5 つの層

| Layer | 役割 | 配置 | 書く内容 | 書かない内容 |
|---|---|---|---|---|
| **Top README** | リファレンスポータル | `/README.md` | 全体像、hardware、各層への入口リンク、外向けの "what is this" | 詳細な how-to、決定の理由、コンポーネント仕様 |
| **Per-directory README** | co-located な what / where | `kubernetes/<cat>/README.md`、`kubernetes/<cat>/<comp>/README.md` | このディレクトリは何で、どんな component が並んでて、深掘りは docs/ のどこ | 横断的な背景、ADR で書くべき決定の理由 |
| **`docs/` (MkDocs SoT)** | 横断的な how / why の **人間向け SoT** | `docs/<topic>/` | 詳細マトリクス、ワークフロー、図、複数 component にまたがる説明 | 決定の歴史 (ADR にリンク)、ファイル配置規約 |
| **`docs/adr/`** | **why** (不変の決定記録) | `docs/adr/NNN-*.md` | 決定の背景、却下案、トレードオフ、Consequences | 現状のファイルパス、how-to (時点で固定) |
| **`.claude/rules/`** | **AI 向けの薄いサマリ + glob** | `.claude/rules/<topic>.md` | 3-5 行のルール、適用 glob、`docs/` への link | 詳細表 (docs/ に置く)、人間向け解説 |
| **`CLAUDE.md`** | **AI のエントリポイント** | `/CLAUDE.md` | mandatory 制約、rules/ への index、ディレクトリツリー | 技術スタック表 (Top README に)、構成詳細 (per-dir README に) |

## トピック別の SoT マップ

「この情報はどこに書いてあるか」の決定版。新しい情報を追加するときも、まずこの表に従って配置先を決める。

| トピック | SoT | サマリ参照先 |
|---|---|---|
| Secret management (4 方式) | `docs/secret-management/index.md` | `.claude/rules/security-secrets.md`、`kubernetes/secrets/README.md` |
| Namespace naming | `docs/adr/006-namespace-naming.md` (why) + `.claude/rules/environment-separation.md` (how) | `kubernetes/environments/README.md` |
| Helm multi-source 配置規約 | `kubernetes/README.md` (Pattern A/B) | `.claude/rules/helm-multisource.md` |
| GitOps workflow | `.claude/rules/gitops-workflow.md` | `CLAUDE.md` Mandatory Constraints |
| Network ingress (Cloudflare Tunnel + Cilium L2 + Istio Gateway) | `docs/architecture/network.md` | `.claude/rules/network-ingress.md` |
| Storage (Longhorn + local-path) | `docs/architecture/infrastructure.md` (予定) | `.claude/rules/kubernetes-cluster.md` |
| Cluster topology (node, label, scheduling) | `.claude/rules/kubernetes-cluster.md` | `kubernetes/README.md` |
| Tech stack | Top `README.md` | `docs/index.md` |

## 書き分けの判断基準

ある事実をどこに書くか迷ったら、以下のフローで決める。

```
それは「決定の理由」か？
  └─ Yes → ADR (docs/adr/)
  └─ No  → それは「特定ディレクトリの中だけで完結する説明」か？
            └─ Yes → そのディレクトリの README.md
            └─ No  → それは「複数 component にまたがる横断的な how / why」か？
                      └─ Yes → docs/<topic>/
                      └─ No  → AI に通知すべき glob 付きルールか？
                                └─ Yes → .claude/rules/<topic>.md
                                └─ No  → 多分そもそも書く必要がない
```

## README の書き方ガイド

`kubernetes/<cat>/README.md` および `kubernetes/<cat>/<comp>/README.md` は以下を満たす。

- **長さ**: 5〜30 行程度。これ以上膨らんだら `docs/` に押し出す合図
- **必須セクション**:
  - 1 行サマリ (このディレクトリ / component は何か)
  - 含まれるサブディレクトリ / 主要ファイルの説明
  - 深掘りリンク (関連 docs/、ADR、外部 URL)
- **禁止**:
  - TODO だけ並んだ空テンプレ
  - 他箇所と重複する詳細表 (link で済ます)
  - "Why" の説明 (ADR に書く)

### Component README の最小テンプレ

```markdown
# <component>

<1 行サマリ — 何のために存在するか>

## 構成

- `values.yaml` — Helm chart の主要 override (ポイントだけ)
- `resources/` — chart 外の追加 manifest (HTTPRoute、SealedSecret、…)

## 関連

- 深掘り: [docs/<topic>/index.md](../../../docs/<topic>/index.md)
- 決定の経緯: [ADR-NNN](../../../docs/adr/NNN-*.md)
```

## drift 防止

SoT を一意化することで構造的に drift しない。CI チェックや lint は不要 (重複そのものが存在しないため)。

ただし以下に注意:

- **新しい事実を書くときに必ずこのページに戻ってきて配置先を決める**
- **既存ドキュメント編集時に「他の場所にも同じ事実があるはず」と疑う癖をつける**
- 違反を見つけたら速やかに SoT に集約し、他の場所は link に置き換える
