---
id: NNN-<slug>
domain: infra
status: draft   # draft | ready-for-plan | planned | implemented
created: YYYY-MM-DD
updated: YYYY-MM-DD
---

# Spec: <機能名>

> what / why のみを書く。chart 名・version・マニフェスト構造などの技術選定は書かない（→ plan.md）。

## Problem (Why)

<なぜこの変更が必要か。今ない能力 / 困りごと / きっかけ。1〜3 文で本質的に。>

## Goals

- <達成したいこと。観測可能な状態で>

## Non-goals

- <今回やらないこと。スコープ境界を明示>

## 対象 namespace / ドメイン / ネットワーク影響

- **namespace**: <例: monitoring / 新規 app-foo>
- **公開ドメイン**: <例: foo.platform.yu-min3.com / なし（クラスタ内のみ）>
- **入口**: <例: gateway-platform 経由 / Cilium L2 LB / 公開なし>

## セキュリティ影響

- **secrets**: <扱う秘密情報の有無と種類。例: GHCR pull token / DB cred / なし>
- **認証**: <Gateway-level OIDC を通すか / クラスタ内通信のみか>

## Acceptance criteria

> 検証フェーズの oracle になる。観測可能・確認可能な形で書く。

- [ ] Argo CD Application が `Synced` かつ `Healthy`
- [ ] <例: `https://foo.platform.yu-min3.com` が 200 / 認証画面にリダイレクト>
- [ ] <例: Certificate `foo-tls` が `Ready`>
- [ ] <その機能固有の確認項目>

## Open questions

- [NEEDS CLARIFICATION: <未確定事項。残っている間は /spec-plan に進めない>]
