---
title: "sense private host service — 初回導入・復旧手順"
status: draft
created: 2026-09-27
updated: 2026-09-27
tags: [kensan-lab, sense, autonomous-development, operations]
---

# 結論

初回は既存 k3s に触れず、`sense` の host systemd に simulation-only controller を `127.0.0.1:8787` で配置する。Cloudflare、K8s、GitHub、ルーターは変更しない。導入操作は独立 Release Gate の初回レビュー記録と固定 SHA-256 の照合後に限る。失敗時は service を停止・自動起動無効化し、台帳と token は削除しない。

**現在は実機変更を保留。** 13:16 UTC に別作業で `k3s` が停止・無効化され、libvirt 関連の導入が始まった。誰の作業かと今後の host 用途が確定するまで、この runbook を sense に適用しない。script・policy・固定 manifest の独立コード審査は `allow` だが、これは実機操作の許可ではない。実機の Release Gate は `needs_human` のまま。

独立 reviewer は source `e3b022f` から Go 1.25.5 / linux-amd64 / CGO 無効 / `-buildvcs=false -trimpath` で再 build し、候補 binary と同じ SHA-256 `c634683ec048574b8b41d33cb8b01600e1d332bec210b0a1073ef57c09b8389c` を確認した。source commit から候補記録 commit `75087de` までの変更は JSON 候補だけで、Go source/module/static 資産は不変。

## 実測前提（2026-09-27）

| 項目 | 観測 | 判断 |
|---|---|---|
| OS / 資源 | Ubuntu 24.04.4、x86_64、4 CPU、15 GiB RAM（約 11 GiB available）、root 146 GiB / `/data` 870 GiB 空き | mock controller は共存を試せる。常時負荷は導入後に測る |
| k3s | 13:12 UTC は sense 1台の Ready control plane。13:16 UTC に `systemctl disable --now k3s` と `k3s-killall.sh` が実行され、13:19 UTC は inactive / disabled | 本件では停止も再起動もしない。別作業の変更として扱い、導入前に再棚卸しする |
| host port 8787 | 棚卸し時に待受なし | 導入直前にも再確認する |
| 既存公開経路 | host の cloudflared/nginx/caddy/traefik/tailscale service なし。Ingress なし、HTTPRoute CRD なし | 未確認経路は非公開証明に使わない。service 起動後に実測する |
| 専用環境 | `kensan-dev` user、`/opt/kensan-dev`、`/var/lib/kensan-dev`、Go/Claude/Codex/bwrap が未配置 | mock-only 初回導入に必要な user・binary・token だけを作る。実推論は別段階 |

## 初回操作の固定範囲

`deploy/private-host-service.sh install` は `sense` 上でのみ実行できる。既存 unit・drop-in・systemd ロード状態・user/group・専用 directory があれば拒否し、並行 install を lock で拒否する。port 調査の失敗も拒否する。SHA-256 を指定した Linux amd64 binary、`tokens.css`、loopback + `-mock-worker` unit を root 専用 staging に複製し、その複製を検証してから専用 OS user と `/opt/kensan-dev`・`/var/lib/kensan-dev` を作る。daemon-reload 後、enable 前に `FragmentPath/DropInPaths/User/Group/ExecStart/Environment` の実効値を照合する。token はローカルで乱数生成し、値は表示しない。`systemctl enable/start` は `kensan-dev-controller.service` に限定する。失敗・中断では当該 service を停止・無効化し、既存 service には触れない。

| 操作 | 判定 | 理由 |
|---|---|---|
| 専用 user/dir、固定 hash の初回 unit 導入 | レビュー後に採用 | G4a の非公開実機検証に必要。専用領域のみ変更 |
| k3s 停止・Coder 削除 | 却下 | 既存 workload と PVC に影響。資源不足の証拠なし |
| `-isolated-worker` や公式 CLI 認証 | 保留 | bwrap/rootfs/CLI/本人認証/課金経路が未準備 |
| Cloudflare / tunnel / proxy / public bind | 却下 | 公開有効化の承認なし。非公開段階の範囲外 |

## 実行前チェック

1. clean worktree で `go test -race ./... -count=1` と `go vet ./...`。`GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -trimpath -o <staging-binary> ./cmd/sense-dev` で binary を作る。Go の VCS stamp はこの worktree で別 revision を示したため無効化し、候補 manifest の source SHA と再現 build hash を別に固定する。
2. 独立 reviewer が `operation/repo/ref/head_sha/target_environment/policy_version/artifact_hashes/expires_at` を持つ固定 manifest、binary・tokens・unit・導入 script の SHA-256、read-only 棚卸し、変更対象、rollback を記録して `allow` とする。作者の自己承認は不可。未成立なら実機への copy/install はしない。
3. `ssh sense` で hostname、`systemctl status k3s`、`ss -ltn`、既存 unit/user/dir、空き容量を再確認する。対象が変わったら再レビューする。
4. 3入力と script を sense の一時 staging へ転送する。script 自体を root 管理の実行経路へコピー後に hash 照合し、その複製を実行する。3入力は script 内で root 専用 staging へコピー後に再照合される。reviewer に固定された値を `install` 引数へ渡す。secret は引数にしない。

## 導入・検証

1. `sudo bash private-host-service.sh install <binary> <tokens.css> <unit-file> <binary-sha> <tokens-sha> <unit-sha>`。
2. `sudo bash deploy/private-listener-smoke.sh <reviewed-unit-sha256> <reviewed-binary-sha256>` を sense で read-only 実行する。root 権限は socket 所有 PID と `/proc/<MainPID>/exe` の照合にだけ使う。service、unit/binary hash、実効 fragment/drop-in/ExecStart/Environment、MainPID と socket 所有者、`127.0.0.1:8787` だけの待受、loopback `/login` の HTTP 200 を検査する。`journalctl -u ...` も確認。非 loopback 待受なら即 rollback。
3. **別ホスト**から sense の LAN IPv4:8787 と global IPv6:8787 への接続拒否を確認する。host 自身から LAN IP を叩く試験で代用しない。timeout、IPv6 到達経路なし、試験元の障害は「拒否成功」でなく試験不能と記録する。8787 の直結拒否だけでは別 port・tunnel・proxy 経由の公開を否定できないため、既存 tunnel/proxy/Ingress/CI、firewall・ルーター、preview 公開の経路を read-only で照合する。smoke script の `EXTERNAL_ROUTE=unverified` はこの別検査が必要な印。経路不明なら `private-ready` は保留。
4. mock task を投入し、Mac 側の terminal/browser を閉じても event が増えることを確認する。`systemctl restart` と host reboot 後に永続台帳・outbox・重複なしを確認する。reboot は共有 workload への影響と復旧手段を再確認してから行う。

## 停止・復旧

`sudo bash private-host-service.sh rollback` は当該 unit のみ停止・自動起動無効化する。token、台帳、binary、unit は保持し、再開前に原因と固定 hash を確認する。`systemctl enable --now kensan-dev-controller.service` は新たな Gate 判断が必要。データ/OS user/cluster の削除はこの runbook の範囲外。

## 未決事項

- 実推論用 rootfs、Claude/Codex 公式 CLI と本人ログイン、subscription 上限・従量無効の実証。
- 日報宛先、外部入口の hostname/IdP/allowlist。公開有効化は別承認。
- reboot 検証の時間帯と、既存 Coder 利用者への影響許容。
