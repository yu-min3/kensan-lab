---
title: "sense 自動開発基盤 — private-ready 判定台帳"
status: active
created: 2026-09-27
updated: 2026-09-27
tags: [kensan-lab, sense, autonomous-development, audit]
---

# 結論

**private-ready は未達。** `sense`（192.168.0.113）は同じ LAN（Mac 192.168.0.103）から ARP 未解決、ping 応答なし、SSH タイムアウト。sense の OS・旧 k3s 依存・待受・既存公開経路を実測できず、service の配置も未実施。`private-ready` や v0 完了を宣言しない。Cloudflare・既存公開経路は変更していない。

基準実装 SHA: `7a35337e3e189efbaee58c1c0be43b81aadf0383`（専用 worktree `feat/sense-autonomous-development`）。実装と検証はローカル段階で継続中。

## 今回の判断

| 選択肢 | 判定 | 理由 |
|---|---|---|
| ローカルの模擬運転・検証を継続 | 採用 | 実認証・公開・sense の稼働を必要としない G1/G2 を前進できる |
| sense を未確認のまま `private-ready` と記録 | 却下 | AC-01/19 の service・待受・既存到達経路に実機証拠がない |
| 別 tunnel / Cloudflare route で到達性を補う | 却下 | 明示的な公開承認がなく、非公開境界を迂回する |
| GitHub push/PR/merge を現時点で実行 | 保留 | scan は一次検査に過ぎず、独立 Gate の実推論・CI/可視性/配備影響の照合と限定 publisher が未完成 |

## 証拠と未達

| 領域 / AC | 状態 | 現在の証拠 | private-ready までに必要なもの |
|---|---|---|---|
| ● 高: sense 稼働・非公開性（AC-01, 08, 19） | 未検証 | 192.168.0.113 の ARP が incomplete。SSH は timeout / no route。手元の service unit は loopback + `-mock-worker`、未導入 | read-only 棚卸し、共有依存判定、可逆停止、systemd 導入、IPv4/IPv6 と tunnel/proxy/Ingress/CI の実測、reboot/復旧 E2E |
| ● 高: 実モデル・費用（AC-04, 06, 07） | 未検証 | Claude/Codex の購読専用 adapter と fake CLI/App Server 試験。Mac/sense 同時利用は未試験 | sense で本人が公式ログイン。契約・モデル利用可能性・従量無効を確認後に4工程とMac優先を実走 |
| ● 高: Release Gate（AC-09, 18） | 部分 | 全 commit の Git scan、対象 SHA/操作に固定した scan artifact、独立 Gate 入力 manifest、mock転用・誤操作・別SHAの拒否試験。publish は intent の dry-run のみ | CI・PR本文/添付・repo可視性・render設定・公開到達経路の実照合、worker/publisher credential 分離、限定 publisher と reconcile |
| ▲ 中: 独立 context と工程（AC-13〜16） | 部分 | App/Platform の別 profile/knowledge/memo/session、immutable artifact、manifest hash、前工程の成果物配送、依存配車・provider枠・Mac優先・restart試験 | 実 worker のOS分離、review session の実運転、session消失後の再作成、App不合格→Platform修正→再試験 |
| ▲ 中: モバイル UI（AC-03, 17） | 部分 | Chrome Playwright 360/390/430px で依頼・Mac優先・停止・日報preview、横はみ出しなし。質問/回答/承認はHTTP試験 | 実機 Safari/Chrome、切断・Access再認証、回答草稿/再送、案件詳細・受入feedbackを E2E |
| ▲ 中: 日報（AC-11） | preview のみ | JST日付で1件の未配信原本と判断リンク。送信結果は `not_configured` | 宛先確定、timer/outbox、送信不明の照合、2日連続の到達確認 |
| ▲ 中: canary/pilot（AC-05, 10, 12） | 未着手 | Golden Path と既存 app の契約は実装 goal に定義済み | 対象・rollback固定、App feedback/再試験、GitOps後の利用者経路、48時間pilot |
| ● 高: 外出先入口（AC-02） | 意図的保留 | loopbackのみ、Cloudflare未変更 | Yu の hostname/IdP/本人allowlist承認後に専用 Access/JWT/route を有効化して実機検証。private-ready の条件には含めない |

## 再現コマンドと作業場所

- Worktree: `/Users/yu/kensan-workspace/.worktrees/sense-autonomous-development`
- 単体・競合検査: `cd apps/sense-dev && go test -race ./... -count=1 && go vet ./...`
- モバイルブラウザ: `cd apps/kensan/e2e && npm ci && npx playwright test --config playwright.sense-dev.config.ts`（2026-09-27: 3/3 pass）
- `apps/sense-dev/deploy/kensan-dev-controller.service` は設定例で、sense には未配置。GitHub remote・Cloudflare・K8s の変更はしていない。

## 次の作業

1. sense が到達したら OS/service/volume/k3s membership と既存公開経路を read-only で記録し、他ホスト依存がない対象だけ可逆停止する。
2. worker の別OSユーザー隔離、restart時の orphan turn 照合、credential なし verifier、限定 publisher/outbox を完成させる。
3. 非公開 service を導入し、実機 listen・継続 event・再起動/復旧・既存到達経路を検証してから `private-ready` を判定する。

## Yu が決めるべき未決事項

- sense の電源/LAN接続の確認（現時点の作業を前進させる最短入力）。
- 実認証時の Claude upgrade/モデル利用枠、運転時間帯（private-ready の模擬運転には不要）。
- 日報の宛先、将来の hostname/Access IdP/本人 allowlist（外部公開は別承認）。
