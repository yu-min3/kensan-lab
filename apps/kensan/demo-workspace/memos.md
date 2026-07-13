---
type: memo
tags: [memo]
created: 2026-05-01
updated: 2026-07-12
---

## Pinned

- 📌 これは kensan のデモ用サンプル workspace です（スクリーンショット・お試し用）。実在の予定や個人情報は含みません
- kubectl の context 切り替えは `kubectx`。prod と lab を間違えない
- Vault unseal key は 1Password の「homelab」vault。KMS auto-unseal 前提だが手動 unseal 手順も控えておく

## Scratch

ArgoCD の Application を rename すると旧 prune → 新 create になって namespace ごと巻き添えになる。rename せず Prune=false で逃がすのが安全。

Longhorn の replica は NVMe ノードだけに置く。Pi の microSD に replica を置くと I/O で詰まる。

Istio sidecar が Postgres の wire protocol を壊す件、`sidecar.istio.io/inject: false` で除外するのを毎回忘れる。テンプレに入れたい。

来月の LT ネタ候補: 「homelab で学ぶ SRE の三種の神器（SLO / エラーバジェット / dead-man's switch）」
