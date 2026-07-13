---
type: note
title: "Cilium L2 Announcements で LoadBalancer を LAN に出す"
tags: [cilium, networking, loadbalancer]
status: active
created: 2026-06-18
updated: 2026-06-18
---

## 概要

外部 LB を持たない homelab で、Cilium の L2 Announcements を使って LoadBalancer Service に LAN 内の VIP を割り当てる。kube-proxy 置換と組み合わせると、NodePort を使わずにサービスを公開できる。

## ポイント

- IP Pool は DHCP レンジと重ならない範囲を切る（例: `192.168.0.240-249`）
- 有線 / WiFi 両方の interface を announce 対象にすると、有線断でも WiFi fallback で VIP が生き残る
- L2 lease renewal が WiFi でタイムアウトすることがある。詰まったら Cilium daemonset を再起動

## 参考

- kube-proxy replacement を有効にしておくこと（L2 announce と前提が揃う）
