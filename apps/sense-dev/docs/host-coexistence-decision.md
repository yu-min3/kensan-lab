---
title: "sense 自動開発基盤 — omarkey VM との併用判断"
status: review
created: 2026-09-29
updated: 2026-09-29
tags: [kensan-lab, sense, autonomous-development, decision]
---

# 結論と Yu への依頼

**止まっているのは実機への初回配置だけ。** `sense-llm` と `sense-desktop` は Yu が依頼した omarkey 用 VM と判明した。自動開発チームの一部ではない。App/Platform の独立 context・版付き成果物交換と、独立 Release Gate のローカル実装・試験は進んでいるが、sense に controller はまだ存在せず、実モデルの自動開発は一度も始めていない。

**推奨判断: 条件付きで host の併用を許可する。** omarkey の両 VM と k3s の現状態を一切変更せず、`kensan-dev-controller.service` を host systemd に新設する。最初は `127.0.0.1:8787` の **mock-only**。モデル推論、GitHub、Cloudflare、外部公開、日報送信は起動しない。直前の read-only 棚卸しと、コード審査とは別の独立 `host_install=allow` 判定が通らなければ転送・導入しない。予想外の競合・資源不足なら `needs_human` へ戻す。

Yu が同意する場合は、次の範囲に対して「**VM を止めずに、条件付き host 併用で進めてよい**」と回答してほしい。これは reboot、VM 設定変更、k3s 再起動、実モデル運転、公開、GitHub 操作の承認ではない。

## なぜ止まったか

2026-09-27 に、この作業とは別に k3s が停止・無効化され、libvirt と VM 2台が稼働し始めた。当時は用途・所有者が不明だったので、固定ポリシー `private-bootstrap-v1` の「並行作業と host 利用を調整する」必要条件を満たせず、`host_install=needs_human` で止めた。今回、**所有者・用途は判明**したが、「VM を動かしたまま host service を併用してよいか」はまだ明示されていない。これは僕の確認の切り方が曖昧だった点でもある。

## 現在地（2026-09-29 10:35 UTC、read-only）

| 領域 | 確認できた事実 | 判定 |
|---|---|---|
| sense host | 4 CPU / 15 GiB。available memory 約7.9 GiB。`sense-llm` と `sense-desktop` が稼働 | 共存可能性はあるが、負荷時の余裕は未実測 |
| omarkey VM | `sense-llm` 4 GiB、`sense-desktop` 6 GiB。両方自動起動設定 | 変更・停止しない |
| k3s / controller | k3s は inactive/disabled。`kensan-dev-controller.service` は `not-found`、8787 待受なし | 既存 controller はない。k3s には触れない |
| コード候補 | source `ee431b9` の binary・script・unit・CSS を独立 Astra reviewer が再現検証。`code_candidate=allow`、`host_install=needs_human` | コード合格は実機操作の許可ではない |
| 候補期限 | 2026-09-29 15:00 UTC（JST 9/30 00:00） | 期限を過ぎたら候補・hash・独立レビューを再作成 |
| 非公開性 | 固定 unit は loopback/mock-only。既存 tunnel/proxy/IPv6/ルーター経路の否定証拠と実機試験は未完 | `private-ready=false` |

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

## 未決事項

- Yu: omarkey VM 2台を稼働・自動起動のまま維持し、本件の mock-only host service を条件付きで併用してよいか。
- 運用: 直前 baseline と別の `host_install=allow`、既存公開経路の調査、実機停止・復元の証拠。
- 後段: 実モデル認証・費用条件、日報宛先、外部入口は今回の承認外。

詳細: [private-ready 判定台帳](private-ready-audit.md)、[導入・復旧手順](private-host-deployment.md)、実装契約 `projects/kensan-lab/docs/goal.md`。
