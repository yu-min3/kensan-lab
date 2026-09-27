---
title: "sense 自動開発基盤 — private-ready 判定台帳"
status: active
created: 2026-09-27
updated: 2026-09-27
tags: [kensan-lab, sense, autonomous-development, audit]
---

# 結論

**private-ready は未達。** `sense`（192.168.0.113）への SSH が復旧し、2026-09-27 に read-only 棚卸しを実施した。単一ノード k3s control plane と Coder・監視・Flux・ローカル PVC が稼働しているため、k3s は停止しない。専用 service は未導入で、IPv4/IPv6・既存公開経路の実測も未完了。`private-ready` や v0 完了を宣言しない。Cloudflare・既存公開経路は変更していない。

棚卸し時の実装 SHA: `2f39119`（専用 worktree `feat/sense-autonomous-development`）。実装と検証はローカル段階で継続中。

## 今回の判断

| 選択肢 | 判定 | 理由 |
|---|---|---|
| k3s を維持し、loopback の host service を共存させる | 採用 | k3s は唯一の control plane で、Coder・監視・PVC に依存がある。RAM は約 11 GiB 空き、専用 port 8787 は未使用 |
| k3s を停止して資源を確保 | 却下 | 既存 workload と復旧に影響し、現時点で資源不足の証拠がない |
| sense を未確認のまま `private-ready` と記録 | 却下 | AC-01/19 の service・待受・既存到達経路に実機証拠がない |
| 別 tunnel / Cloudflare route で到達性を補う | 却下 | 明示的な公開承認がなく、非公開境界を迂回する |
| GitHub push/PR/merge を現時点で実行 | 保留 | scan は一次検査に過ぎず、独立 Gate の実推論・CI/可視性/配備影響の照合と限定 publisher が未完成 |

## 証拠と未達

| 領域 / AC | 状態 | 現在の証拠 | private-ready までに必要なもの |
|---|---|---|---|
| ● 高: sense 稼働・非公開性（AC-01, 08, 19） | 部分 | SSH 復旧。Ubuntu 24.04.4、4 CPU / 15 GiB RAM、root 空き 146 GiB、`/data` 空き 870 GiB。単一ノード k3s は Ready。host の 8787 は未使用、Ingress なし、HTTPRoute CRD なし。手元の unit は loopback + `-mock-worker`、未導入 | host service 導入、IPv4/IPv6・既存 tunnel/proxy/Ingress/CI の実測、reboot/復旧 E2E。Cloudflare の設定を触らずに既存公開経路も別途確認 |
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

1. k3s と既存 workload は止めず、bootstrap 手順と rollback を独立レビューする。`kensan-dev` OS user、CLI・bubblewrap・rootfs・認証ホームの欠落を導入前提として扱う。
2. worker の別OSユーザー隔離、restart時の orphan turn 照合、credential なし verifier、限定 publisher/outbox を完成させる。
3. loopback service を導入し、実機 listen・継続 event・再起動/復旧・既存到達経路を検証してから `private-ready` を判定する。

## Yu が決めるべき未決事項

- 実認証時の Claude upgrade/モデル利用枠、運転時間帯（private-ready の模擬運転には不要）。
- 日報の宛先、将来の hostname/Access IdP/本人 allowlist（外部公開は別承認）。
