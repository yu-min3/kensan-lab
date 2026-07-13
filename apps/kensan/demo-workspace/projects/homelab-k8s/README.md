---
type: project
tags: [kubernetes, gitops, homelab]
created: 2026-01-10
updated: 2026-07-13
status: active
deadline: 2026-09-30
repo: "https://github.com/example/homelab-k8s"
---

## 概要

Raspberry Pi 5 ×3 + mini PC の bare-metal クラスタを、ArgoCD の GitOps だけで運用する。手作業の `kubectl apply` をゼロにし、「Git が唯一の真実」を徹底する。

## 目標

2026 Q3 末までに、全インフラ変更を Git 経由に統一し、直接 kubectl での変更が発生しない状態（selfHeal で自動復旧）を達成する。

## マイルストーン

- [x] Cilium CNI + kube-proxy 置換
- [x] ArgoCD App-of-Apps 構成
- [x] Istio Gateway + mTLS
- [ ] Backstage による app scaffolding の自動化
- [ ] Ansible での node bootstrap 自動化

## タスク

- [ ] ApplicationSet の generator を config.json ベースに寄せる @today @p(10)
- [ ] root-app の prune 保護を PVC 個別 annotation に統一 @due(2026-07-20) @p(20)
- [ ] cert-manager の Let's Encrypt staging → production 切り替えを検証 @due(2026-07-22) @p(30)
- [ ] 新 app 追加フローを README に 1 枚図で整理する @p(40)
- [ ] argocd の repo-server のメモリ上限を調整（OOM 対策） @p(50)
- [ ] 週次バックアップの R2 リストアを 1 本試す @p(60)

## ログ

- 2026-07-11: Istio Gateway 経由で全 UI を集約。裸の NodePort を撤去
- 2026-06-28: local-path を全廃し Longhorn に一本化
- 2026-05-07: 有線メイン + WiFi fallback の 2 系統に移行
