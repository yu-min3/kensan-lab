---
title: "sense 自動開発基盤 — private-ready 判定台帳"
status: active
created: 2026-09-27
updated: 2026-09-27
tags: [kensan-lab, sense, autonomous-development, audit]
---

# 結論

**private-ready は未達。** `sense`（192.168.0.113）への SSH が復旧し、2026-09-27 に read-only 棚卸しを実施した。ただし 13:16 UTC にこの作業と別の操作で単一ノード k3s が停止・無効化され、libvirt の導入が始まった。導入対象の前提が変化したため、このスレッドからの sense 変更は保留する。専用 service は未導入で、IPv4/IPv6・既存公開経路の実測も未完了。`private-ready` や v0 完了を宣言しない。Cloudflare・既存公開経路は変更していない。

最新の固定 source SHA: `4918d15`（専用 worktree `feat/sense-autonomous-development`）。manifest `private-bootstrap-candidate-4918d15.json` と独立 Astra 判定 `private-bootstrap-review-4918d15.json` は、再現 build と固定 hash を確認済み。ただし `code_candidate=allow` に限り、`host_install=needs_human`・`execution_authorized=false`。実装と検証はローカル段階で継続中。

## 今回の判断

| 選択肢 | 判定 | 理由 |
|---|---|---|
| k3s の現状態を変更せず、loopback の host service を計画する | 条件付き採用 | 13:12 UTC は稼働・約11 GiB空き、13:19 UTC は inactive/disabled・約14 GiB空き。別作業の変更を上書きしない |
| k3s を本件から停止または再起動 | 却下 | 既存 workload と別作業に影響し、所有者・目的が未確定 |
| 初回 host 導入案 | ローカル審査のみ合格 | source `4918d15` を独立 Astra reviewer が再buildし、候補 binary の hash 一致。TOCTOU、既存領域、systemd 実効設定、固定 policy/manifest を確認し `code_candidate=allow`。実機操作は別作業との調整が必要で `host_install=needs_human` |
| sense を未確認のまま `private-ready` と記録 | 却下 | AC-01/19 の service・待受・既存到達経路に実機証拠がない |
| 別 tunnel / Cloudflare route で到達性を補う | 却下 | 明示的な公開承認がなく、非公開境界を迂回する |
| GitHub push/PR/merge を現時点で実行 | 保留 | scan は一次検査に過ぎず、独立 Gate の実推論・CI/可視性/配備影響の照合と限定 publisher が未完成 |

## 証拠と未達

| 領域 / AC | 状態 | 現在の証拠 | private-ready までに必要なもの |
|---|---|---|---|
| ● 高: sense 稼働・非公開性（AC-01, 08, 19） | 部分 | SSH 復旧。Ubuntu 24.04.4、4 CPU / 15 GiB RAM、root 空き 146 GiB、`/data` 空き 870 GiB。k3s は 13:12 UTC に Ready、13:19 UTC に inactive/disabled。14:08 UTC の再確認でも k3s inactive、libvirtd active、8787 待受なし。global IPv6 あり。unit は未導入。ローカル実 process の mock 配車→SIGTERM→再起動/台帳復元→再配車は race 検査込みで合格。listener/実効 unit/MainPID/binary hash の sense smoke は独立コード審査済み、未実行 | 別作業との調整、独立 Gate 再審査、host service 導入、IPv4/IPv6・既存 tunnel/proxy/Ingress/CI の実測、reboot/復旧 E2E |
| ● 高: 実モデル・費用（AC-04, 06, 07） | 未検証 | Claude/Codex の購読専用 adapter と fake CLI/App Server 試験。Mac/sense 同時利用は未試験 | sense で本人が公式ログイン。契約・モデル利用可能性・従量無効を確認後に4工程とMac優先を実走 |
| ● 高: Release Gate（AC-09, 18） | 部分 | 全 commit の Git scan、対象 SHA/操作に固定した scan artifact、独立 Gate 入力 manifest、mock転用・誤操作・別SHAの拒否試験。publish は intent の dry-run のみ | CI・PR本文/添付・repo可視性・render設定・公開到達経路の実照合、worker/publisher credential 分離、限定 publisher と reconcile |
| ▲ 中: 独立 context と工程（AC-13〜16） | 部分 | App/Platform の別 profile/knowledge/memo/session、immutable artifact、manifest hash、前工程の成果物配送、依存配車・provider枠・Mac優先・restart試験。App不合格→Platform修正→同一シナリオ再試験をローカル試験で確認 | 実 worker のOS分離、review session の実運転、session消失後の再作成、実モデル/実 repo での再試験 |
| ▲ 中: モバイル UI（AC-03, 17） | 部分 | Chrome Playwright 360/390/430px で依頼・Mac優先・停止・日報preview、横はみ出しなし。質問/回答/承認はHTTP試験 | 実機 Safari/Chrome、切断・Access再認証、回答草稿/再送、案件詳細・受入feedbackを E2E |
| ▲ 中: 日報（AC-11） | preview のみ | JST日付で1件の未配信原本と判断リンク。送信結果は `not_configured` | 宛先確定、timer/outbox、送信不明の照合、2日連続の到達確認 |
| ▲ 中: canary/pilot（AC-05, 10, 12） | 部分 | Golden Path と既存 app の契約は実装 goal に定義済み。AC-05 の成果物配送・差し戻し・修正・再試験は固定 SHA のローカル試験まで | 対象・rollback固定、実 App feedback/再試験、GitOps後の利用者経路、48時間pilot |
| ● 高: 外出先入口（AC-02） | 意図的保留 | loopbackのみ、Cloudflare未変更 | Yu の hostname/IdP/本人allowlist承認後に専用 Access/JWT/route を有効化して実機検証。private-ready の条件には含めない |

## 再現コマンドと作業場所

- Worktree: `/Users/yu/kensan-workspace/.worktrees/sense-autonomous-development`
- 単体・競合検査: `cd apps/sense-dev && go test -race ./... -count=1 && go vet ./...`
- 2026-09-27: `TestMockControllerSurvivesProcessRestart` を含む `go test -race ./... -count=1` は全 package pass。これはローカル mock process の証拠で、sense の systemd/reboot 証拠ではない。
- モバイルブラウザ: `cd apps/kensan/e2e && npm ci && npx playwright test --config playwright.sense-dev.config.ts`（2026-09-27: 3/3 pass）
- `apps/sense-dev/deploy/kensan-dev-controller.service` は設定例で、sense には未配置。GitHub remote・Cloudflare・K8s の変更はしていない。

## 既存到達経路の静的棚卸し（実機証明ではない）

| 経路 | repo で確認した事実 | 残る実機・外部証拠 |
|---|---|---|
| Cloudflare Tunnel | `kubernetes/network/cloudflare-tunnel/deployment.yaml` は token で起動する K8s pod。`README.md` は public hostname と origin が Cloudflare Zero Trust dashboard 管理と明記。静的 manifest に `hostNetwork` 指定はない | dashboard の read-only route/Access 照合、pod の実効 spec と起動状態。Git に hostname がないことを「route なし」と扱わない |
| Istio / Cilium / K8s | repo の K8s・CI・chart に `sense-dev`、`kensan-dev`、port `8787` の参照は見つからない。既存 Gateway は公開・LAN 経路を持つ | k3s が再稼働した場合の実効 Ingress/HTTPRoute/Service/Pod と、host への転送経路。静的検索は runtime drift を否定しない |
| GitHub Pages / CI | `.github/workflows/docs.yml` は `main` への対象 path push で Pages deploy。現在の作業は専用 branch・ローカルのみで remote 操作なし | push/PR/merge 前に送信する全 commit、workflow、公開 repo 可視性、artifact/preview を専用 Release Gate が再照合 |
| host / IPv6 | sense に global IPv6 があり、候補 unit は `127.0.0.1:8787`。導入後の smoke は実効 unit・socket owner・binary を照合する | 別ホストから IPv4/IPv6 の直結拒否と、別 port・proxy・tunnel 経由を確認。timeout や経路なしは拒否成功とみなさない |

Cloudflare の設定を変更しないことと、既存公開経路が無いことは別。公開 hostname は Git だけでは列挙できず、K8s が停止中の現在は実効 Pod 経路も確認できないため、AC-19 は未達とする。

## 次の作業

1. 別作業による k3s 停止・libvirt 導入の目的と host 利用範囲を確認する。このスレッドから k3s を再起動しない。source `4918d15` の独立コード審査は合格済みだが、実機操作の Gate は直前 baseline・公開経路・期限・固定 artifact を確認して別に判定する。候補期限は 2026-09-29 14:15 UTC で、切れたら再作成・再審査する。
2. worker の別OSユーザー隔離、restart時の orphan turn 照合、credential なし verifier、限定 publisher/outbox を完成させる。
3. loopback service を導入し、実機 listen・継続 event・再起動/復旧・既存到達経路を検証してから `private-ready` を判定する。

## Yu が決めるべき未決事項

- 同時進行している sense の k3s 停止・libvirt 導入と、本件の host 利用の調整。
- 実認証時の Claude upgrade/モデル利用枠、運転時間帯（private-ready の模擬運転には不要）。
- 日報の宛先、将来の hostname/Access IdP/本人 allowlist（外部公開は別承認）。
