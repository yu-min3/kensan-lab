---
type: book
title: "Kubernetes完全ガイド"
author: "青山真也"
status: finished
started: 2026-02-01
finished: 2026-03-10
rating: 5
tags: [kubernetes, infrastructure]
created: 2026-02-01
updated: 2026-03-10
---

## 総評

homelab を始める前に通読しておいて正解だった。個々の resource の意味が「なぜそう設計されているか」まで腹落ちする。辞書としても使える。

## 学び

- Service の種類（ClusterIP / NodePort / LoadBalancer）の使い分けが、実際に自分で LB を用意する段になって効いた
- Pod の disruption budget と affinity をちゃんと理解していないと、ノード drain で事故る
- CRD と Operator パターンの章で、ArgoCD や cert-manager の挙動が読めるようになった

## その他

- 手を動かしながら読むのが前提。読むだけだと 3 割しか残らない
