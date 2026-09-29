---
title: "sense 自動開発基盤 — omarkey VM との併用判断"
status: accepted-with-scope
created: 2026-09-29
updated: 2026-09-29
tags: [kensan-lab, sense, autonomous-development, decision]
---

# 結論

**Yu の条件付き許可に基づき、mock-only の host service を導入した。** `sense-llm` と `sense-desktop` は omarkey 用 VM で、自動開発チームの一部ではない。App/Platform の独立 context・版付き成果物交換と独立 Release Gate は実装・試験済み。今回 sense に配置したのは `127.0.0.1:8787` の mock controller だけで、実モデルの自動開発は一度も始めていない。

**許可範囲: VM と k3s を変更せず host の mock-only service を併用する。** 独立 `host_install=allow` 判定後、固定 hash の候補を導入した。モデル推論、GitHub、Cloudflare、外部公開、日報送信は起動しない。予想外の競合・資源不足が見つかった場合は専用 service だけを停止し、VM/k3s は変更しない。

Yu は「**VM を止めずに、資源が足りれば条件付き host 併用で進めてよい**」と判断した。これは reboot、VM 設定変更、k3s 再起動、実モデル運転、公開、GitHub 操作の承認ではない。

## なぜ止まったか

2026-09-27 に、この作業とは別に k3s が停止・無効化され、libvirt と VM 2台が稼働し始めた。当時は用途・所有者が不明だったので、固定ポリシー `private-bootstrap-v1` の「並行作業と host 利用を調整する」必要条件を満たせず、`host_install=needs_human` で止めた。その後、**所有者・用途が判明し、Yu の条件付き判断と独立 Gate allow を得たため導入へ進んだ**。確認の切り方が曖昧だった点は記録しておく。

## 現在地（2026-09-29 10:35 UTC、read-only）

| 領域 | 確認できた事実 | 判定 |
|---|---|---|
| sense host | 4 CPU / 15 GiB。導入直前 available memory 約8.0 GiB、導入後も約7.9 GiB。`sense-llm` と `sense-desktop` が稼働 | 固定 mock controller の共存は確認。実推論・build の余力を保証しない |
| omarkey VM | `sense-llm` 4 GiB、`sense-desktop` 6 GiB。両方自動起動設定 | 変更・停止しない |
| k3s / controller | k3s は inactive/disabled。導入後 controller は専用 systemd のみ、8787 は loopback | 既存 k3s は変更しない |
| コード候補 | source `ee431b9` の binary・script・unit・CSS を独立 Astra reviewer が再現検証。独立 host Gate は `host_install=allow` | 固定候補以外は導入しない |
| 候補期限 | 2026-09-29 15:00 UTC（JST 9/30 00:00） | 期限を過ぎたら候補・hash・独立レビューを再作成 |
| 非公開性 | Mac から LAN IPv4:8787 は `direct_refused`。global IPv6:8787 は `no_route` で未知。既存 tunnel/proxy/Cloudflare/router は未証明 | `private-ready=false` |

## 選択肢

| 選択肢 | 判定 | 理由 |
|---|---|---|
| A. VM はそのまま、host に mock-only service を条件付き導入 | **採用推奨** | goal の host systemd 配置を維持。専用 user/dir/unit のみ新設し、直前 baseline と別 Gate で止められる |
| B. omarkey 作業が落ち着くまで導入を延期 | 採用可 | VM への影響を完全に避けるが、`private-ready` の実機検証は進まない |
| C. VM を停止して容量を作る | 却下 | omarkey の別目的と稼働状態を損なう。今回の権限に含めない |
| D. controller を omarkey VM 内や cluster Pod に配置 | 却下 | `goal.md` の host systemd・障害分離の契約と異なる |

## A を選んだ後の手順と停止条件

1. sense の VM/k3s/資源・専用 user/group/path/unit/port と復旧経路を **read-only** で直前確認する。公開経路は Cloudflare dashboard 等も含めて別途照合する。不明・変化・資源逼迫があれば停止する。
2. 固定 source/artifact/policy/期限と直前 baseline について、作者とは別の reviewer が `host_install` を判定する。`allow` 以外では **転送もしない**。
3. allow 後、固定 hash の script を root 管理下にコピーして照合し、専用 user/dir/unit の mock-only service を導入する。VM・k3s・Cloudflare は操作しない。rollback はレビュー済み unit/binary/実効設定の一致後だけ当該 service を停止する。
4. service の継続 event と停止・復元、別ホストからの IPv4/IPv6 直結拒否、既存 tunnel/proxy/Ingress/CI・ルーター経路を実測する。公開経路不明なら `private-ready` は宣言しない。host reboot は今回の許可に含めず、必要時に別途調整する。

## 自動開発チームとの関係

App/Platform は **別 profile・知識・メモ・session を持つ論理 agent**で、controller が版付き成果物を配送する。Fable→Astra→Sol→Opus の工程配車もローカル模擬試験まで。今回の最初の host 配置は mock-only で、**実モデルで動く常駐チームの開始ではない**。実モデル運転には sense の本人ログイン、subscription/従量無効、隔離 worker と OS 境界、Mac 併用試験を別に満たす必要がある。

## 実施済みの検証

- `kensan-dev-controller.service` の active/enabled、unit/binary hash、effective user/group/ExecStart、MainPID、loopback listener、`/login` HTTP 200 を確認。
- token を表示せず mock task を作成し、attempt/event が増えることを確認。
- 専用 service の stop→start 後に task/attempts を保持し、VM 2台と k3s の状態が変わらないことを確認。
- Mac から `192.168.0.113:8787` は `direct_refused`。global IPv6 は Mac 側 `no_route` で、拒否成功には数えていない。

## 未決事項

- 運用: global IPv6 を別到達点から再試験し、Cloudflare dashboard・router・既存公開経路を read-only で照合する。
- 後段: 実モデル認証・費用条件、日報宛先、外部入口は今回の承認外。

詳細: [private-ready 判定台帳](private-ready-audit.md)、[導入・復旧手順](private-host-deployment.md)、実装契約 `projects/kensan-lab/docs/goal.md`。
