# ADR-011: Vault server image tag explicit pin (chart 0.32.0 + Vault 2.0.0)

## Status

**Accepted** (2026-04-14, 暫定対応として採用)

## Date

2026-04-14

## Context

`infrastructure/security/vault/values.yaml` で `server.image.tag` を `"2.0.0"` に明示 pin している。本 ADR はその経緯と解除条件を残す。

### 背景: silent upgrade 事故

HashiCorp Vault Helm chart `0.32.0` (2026-01-14 リリース) の template は、`server.image.tag` に空文字 `""` が入っていると `default "latest"` を発動して `image: hashicorp/vault:latest` を render する仕様。

このリポでは当初 `tag: ""` (= chart default に従う) を期待して空文字を指定していたが、実際には `:latest` が選択されていた。2026-04-14 に Vault 2.0.0 が `:latest` tag に publish されたタイミングで、3 replica 全 Pod が `RollingUpdate` で 2.0.0 に silent upgrade された。PR #271 のレビュー中に発覚。

### 影響

- Vault major 跨ぎの upgrade (1.x → 2.x) を予期せず実施
- 公式は major 跨ぎ downgrade をサポートしていない (raft snapshot / storage format 互換性なし)
- 戻すには手動 storage migration が必要、homelab スケールで現実的でない

## Decision

`server.image.tag: "2.0.0"` を明示 pin する。

```yaml
server:
  image:
    repository: hashicorp/vault
    tag: "2.0.0"
```

### 本来の原則

- `tag` 行は削除して chart default に従う (chart 0.32.0 では `server.image.tag = 1.21.2`)
- これにより chart upgrade と image upgrade を 1 つのレビュー単位に揃えられる

### 暫定対応の理由

- すでに 2.0.0 で稼働中、1.21 への downgrade は raft storage format 非互換でリスクあり
- 動いている major version を維持することを優先

## 解除条件

以下を全て満たした時点で本 pin を解除する:

1. chart 0.33+ 系がリリースされ、`server.image.tag` の chart default が 2.x line に追従していること
2. その時点で稼働中の Vault major version と chart default が同じ major であること

解除手順:
- `server.image.tag` 行を削除して chart default に従わせる
- chart upgrade PR の中で `helm template` 出力を diff 確認し、image tag が想定通り 2.x 系であることを check

## 教訓

- Helm chart の `default "latest"` 挙動は silent upgrade の温床
- `tag: ""` は「chart default に従う」ではなく「`:latest` を引く」になる場合がある
- 空文字渡しは避け、明示 pin か行削除のどちらかにする

## References

- PR #271: silent upgrade 発見・対応
- HashiCorp Vault chart 0.32.0 (2026-01-14 リリース)
- Vault 2.0.0 (2026-04-14 GA)
