---
title: "private-bootstrap-v1 — sense host 初回導入ポリシー"
status: active
created: 2026-09-27
updated: 2026-09-27
tags: [kensan-lab, sense, autonomous-development, release-gate, policy]
---

# private-bootstrap-v1

このポリシーは `projects/kensan-lab/docs/goal.md` の G4a、AC-01/08/19 と、初回基盤構築における独立 reviewer 判定の契約を具体化する。候補 manifest の `policy_version` はこの文書の SHA-256 に結び付ける。本文を変更した場合は旧判定を転用しない。

## 許可し得る唯一の操作

`sense` の host へ **初回** `kensan-dev-controller.service` を配置し、専用 user/group、`/opt/kensan-dev`、`/var/lib/kensan-dev` を作る。service は `127.0.0.1:8787` の `-mock-worker` のみ。モデル推論、GitHub、K8s、Cloudflare、proxy、tunnel、DNS、ルーター、公開経路、既存 unit、既存データには触れない。reboot は含まない。

## allow の必要条件

1. 独立 reviewer が `allow` を明記した判定記録を作る。作者の自己判定、`pending`、`needs_human`、`deny` は実行許可ではない。
2. 判定は `operation/repo/ref/head_sha/target_environment/policy_version/policy_hash/artifact_hashes/expires_at` に固定し、対象 binary・script・unit・tokens を同じ hash で実行側が照合する。source commit は実在し、Go binary の build provenance と再現試験を確認する。
3. Yu または該当作業者が、並行する k3s 停止・libvirt 導入と host 利用の調整を確認する。実機 baseline が候補作成時から変わったら再棚卸し・再判定する。
4. 導入直前の read-only 調査で、host の unit/user/group/専用 path/port が未使用、既存 workload と復旧経路に不意の影響がないことを確認する。global IPv6、既存 tunnel/proxy/Ingress/CI・firewall/ルーターの経路が不明な場合、service 導入を検証目的に限定し `private-ready` は宣言しない。
5. script 自体は root 管理の実行経路へ copy 後に hash 検証し、その複製を実行する。script 内は root 専用 staging に入力を copy→hash→install し、systemd の実効 unit を enable/start 前に照合する。token の値をログ・argv・manifest へ出さない。
6. 期限内であり、判定対象の source/artifact/policy hash と環境・操作が完全一致する。差分、期限切れ、別操作への転用は再レビューする。

## deny / needs_human

secret/個人情報の送信、従量モデル利用、公開経路追加、既存 service やデータへの変更、権限拡大、強制停止・削除、reboot はこの操作では `deny`。既存 workload や並行作業への影響が未確定、公開経路を判別できない、判定対象が変化した場合は `needs_human` または `deny` とし、未確認を安全とみなさない。

## 実行後

loopback IPv4/IPv6 と LAN/public の拒否、service event の継続、停止・復元を実測して証拠を残す。失敗時は当該 service のみ停止・無効化し、state/token/binary/unit を保持する。`private-ready` は AC-19 の到達経路検査まで保留し、mock 導入を v0 完了や実モデル運転の証拠にしない。
