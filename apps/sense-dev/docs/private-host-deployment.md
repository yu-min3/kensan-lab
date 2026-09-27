---
title: "sense private host service — 初回導入・復旧手順"
status: draft
created: 2026-09-27
updated: 2026-09-27
tags: [kensan-lab, sense, autonomous-development, operations]
---

# 結論

初回は既存 k3s に触れず、`sense` の host systemd に simulation-only controller を `127.0.0.1:8787` で配置する。Cloudflare、K8s、GitHub、ルーターは変更しない。導入操作は独立 Release Gate の初回レビュー記録と固定 SHA-256 の照合後に限る。失敗時は service を停止・自動起動無効化し、台帳と token は削除しない。

**現在は実機変更を保留。** 13:16 UTC に別作業で `k3s` が停止・無効化され、14:55 UTC には `sense-llm`（4 GiB）と `sense-desktop`（6 GiB）が稼働・自動起動していた。host の available memory は約4.6 GiB。所有者・用途・資源配分が確定するまで、この runbook を sense に適用しない。固定候補の独立コード審査は `allow` だったが、実機の Release Gate は `needs_human` のまま。

直近の審査済み固定候補は source `b1743e4`、manifest `private-bootstrap-candidate-b1743e4.json`（commit `f294e09`）。独立 Astra reviewer は clean な source archive から Go 1.25.5 / linux-amd64 / CGO 無効 / `-buildvcs=false -trimpath` で再 build し、候補 binary と同じ SHA-256 `6cbbc12fb84faf2dbfee7a83ffc9f1cc39f650ef9f35b93ff01af0e835f3c0b2` を確認した。判定は `private-bootstrap-review-b1743e4.json` に固定した。`code_candidate=allow`、`host_install=needs_human`、`execution_authorized=false`。その後 `ac2a137` で rollback を固定 unit/binary の照合後に限定したため、**この候補は現行 source の導入に使えない**。導入前に候補・hash・独立レビューを更新する。

## 実測前提（2026-09-27）

| 項目 | 観測 | 判断 |
|---|---|---|
| OS / 資源 | Ubuntu 24.04.4、x86_64、4 CPU、15 GiB RAM。14:55 UTC は約4.6 GiB available、root 129 GiB / `/data` 870 GiB 空き | 初回観測の約11 GiB available を現状の見積りに使わない。VM を含む容量・復旧経路を導入直前に再評価 |
| k3s | 13:12 UTC は sense 1台の Ready control plane。13:16 UTC に `systemctl disable --now k3s` と `k3s-killall.sh` が実行され、13:19 UTC は inactive / disabled | 本件では停止も再起動もしない。別作業の変更として扱い、導入前に再棚卸しする |
| libvirt VM | 14:55 UTC に `sense-llm` 4 GiB、`sense-desktop` 6 GiB が `qemu:///system` で稼働・自動起動 | 所有者・用途・依存を確認せず、停止・変更しない。host service 併用の可否を調整する |
| host port 8787 | 棚卸し時に待受なし | 導入直前にも再確認する |
| 既存公開経路 | host の cloudflared/nginx/caddy/traefik/tailscale service なし。Ingress なし、HTTPRoute CRD なし | 未確認経路は非公開証明に使わない。service 起動後に実測する |
| 専用環境 | `kensan-dev` user、`/opt/kensan-dev`、`/var/lib/kensan-dev`、Go/Claude/Codex/bwrap が未配置 | mock-only 初回導入に必要な user・binary・token だけを作る。実推論は別段階 |

## 初回操作の固定範囲

`deploy/private-host-service.sh install` は `sense` 上でのみ実行できる。既存 unit・drop-in・systemd ロード状態・user/group・専用 directory があれば拒否し、並行 install を lock で拒否する。port 調査の失敗も拒否する。SHA-256 を指定した Linux amd64 binary、`tokens.css`、loopback + `-mock-worker` unit を root 専用 staging に複製し、その複製を検証してから専用 OS user と `/opt/kensan-dev`・`/var/lib/kensan-dev` を作る。daemon-reload 後、enable 前に `FragmentPath/DropInPaths/User/Group/ExecStart/Environment` の実効値を照合する。token はローカルで乱数生成し、値は表示しない。`systemctl enable/start` は `kensan-dev-controller.service` に限定する。失敗・中断では当該 service を停止・無効化し、既存 service には触れない。

| 操作 | 判定 | 理由 |
|---|---|---|
| 専用 user/dir、固定 hash の初回 unit 導入 | レビュー後に採用 | G4a の非公開実機検証に必要。専用領域のみ変更 |
| k3s・VM の停止や削除 | 却下 | 別作業の所有者・依存・資源配分が未確認。データと復旧経路を保持する |
| `-isolated-worker` や公式 CLI 認証 | 保留 | bwrap/rootfs/CLI/本人認証/課金経路が未準備 |
| Cloudflare / tunnel / proxy / public bind | 却下 | 公開有効化の承認なし。非公開段階の範囲外 |

## 実行前チェック

1. clean worktree で `go test -race ./... -count=1` と `go vet ./...`。`GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -trimpath -o <staging-binary> ./cmd/sense-dev` で binary を作る。Go の VCS stamp はこの worktree で別 revision を示したため無効化し、候補 manifest の source SHA と再現 build hash を別に固定する。
2. 独立 reviewer が `operation/repo/ref/head_sha/target_environment/policy_version/artifact_hashes/expires_at` を持つ固定 manifest、binary・tokens・unit・導入 script の SHA-256、read-only 棚卸し、変更対象、rollback を記録する。現在の `code_candidate=allow` は実機操作を許可しない。並行作業の調整と直前 baseline の後、別の `host_install=allow` 判定が成立するまで実機への copy/install はしない。作者の自己承認は不可。
3. `ssh sense` で hostname、k3s/libvirt/VM の状態、available memory、`ss -ltn`、既存 unit/user/dir、空き容量を再確認する。対象が変わったら再レビューする。
4. 3入力と script を sense の一時 staging へ転送する。script 自体を root 管理の実行経路へコピー後に hash 照合し、その複製を実行する。3入力は script 内で root 専用 staging へコピー後に再照合される。reviewer に固定された値を `install` 引数へ渡す。secret は引数にしない。

## 導入・検証

1. `sudo bash private-host-service.sh install <binary> <tokens.css> <unit-file> <binary-sha> <tokens-sha> <unit-sha>`。
2. `sudo bash deploy/private-listener-smoke.sh <reviewed-unit-sha256> <reviewed-binary-sha256>` を sense で read-only 実行する。root 権限は socket 所有 PID と `/proc/<MainPID>/exe` の照合にだけ使う。service、unit/binary hash、実効 fragment/drop-in/ExecStart/Environment、MainPID と socket 所有者、`127.0.0.1:8787` だけの待受、loopback `/login` の HTTP 200 を検査する。`journalctl -u ...` も確認。非 loopback 待受なら即 rollback。
3. **別ホスト**から sense の LAN IPv4:8787 と global IPv6:8787 への接続拒否を確認する。Mac の `apps/sense-dev` で `go run ./cmd/private-route-probe -lan-ip <棚卸ししたLAN IPv4> -global-ipv6 <棚卸ししたglobal IPv6>` を実行する。出力 JSON は IP/host 名を含むため保護された非追跡の証拠ファイルへ保存し、公開 PR に貼らない。両方が `direct_refused` のときだけ終了コード0。`reachable` は失敗、timeout・no_route・試験元の障害は `unknown` で失敗とし、「拒否成功」に読み替えない。host 自身から LAN IP を叩く試験で代用しない。8787 の直結拒否だけでは別 port・tunnel・proxy 経由の公開を否定できないため、既存 tunnel/proxy/Ingress/CI、firewall・ルーター、preview 公開の経路を read-only で照合する。smoke script の `EXTERNAL_ROUTE=unverified` はこの別検査が必要な印。経路不明なら `private-ready` は保留。
4. mock task を投入し、Mac 側の terminal/browser を閉じても event が増えることを確認する。`systemctl restart` と host reboot 後に永続台帳・outbox・重複なしを確認する。reboot は共有 workload への影響と復旧手段を再確認してから行う。

## 停止・復旧

`sudo bash private-host-service.sh rollback <reviewed-unit-sha256> <reviewed-binary-sha256>` は固定 unit/binary hash と実効 fragment・drop-in・User/Group・ExecStart・Environment が一致する場合だけ当該 unit を停止・自動起動無効化する。不一致なら止めずに調査へ上げる。token、台帳、binary、unit は保持し、再開前に原因と固定 hash を確認する。`systemctl enable --now kensan-dev-controller.service` は新たな Gate 判断が必要。データ/OS user/cluster の削除はこの runbook の範囲外。

## 未決事項

- 実推論用 rootfs、Claude/Codex 公式 CLI と本人ログイン、subscription 上限・従量無効の実証。
- 日報宛先、外部入口の hostname/IdP/allowlist。公開有効化は別承認。
- reboot 検証の時間帯と、稼働 VM の利用者・依存への影響許容。
