# Vault Raft auto-join (retry_join 必須化の経緯)

## 症状

`vault-0` を init した後、`vault-1` / `vault-2` が Sealed のまま raft cluster に join できない。`vault status` で `Initialized: false` のまま放置される。

## 原因

`storage "raft"` ブロックに `retry_join` が含まれていないと、各 Pod は自動で peer を発見しない。chart の StatefulSet headless service `vault-internal` は名前解決可能だが、raft プロトコル側で能動的に join しに行く設定がないと cluster に参加できない。

PR #228 までこの設定が漏れており、毎回手動で `vault operator raft join` を実行していた。

## 設定 (現状)

`infrastructure/security/vault/values.yaml` の `server.ha.raft.config` に以下を含める:

```hcl
storage "raft" {
  path = "/vault/data"
  retry_join {
    leader_api_addr = "http://vault-0.vault-internal:8200"
  }
  retry_join {
    leader_api_addr = "http://vault-1.vault-internal:8200"
  }
  retry_join {
    leader_api_addr = "http://vault-2.vault-internal:8200"
  }
}
```

3 Pod 全てに対して `retry_join` を書く。各 Pod は自分自身に対しても join 試行するが noop なので問題ない。

## chart config の注意点

`server.ha.raft.config` は HCL 文字列で chart default を**完全置換** (deep merge できない)。chart upgrade 時は default 内容を確認し、必要な要素 (ui / listener / storage / service_registration) が落ちていないかチェックする必要がある。

`server.affinity` は map syntax なので Helm の deep merge 対象 (chart default の podAntiAffinity を上書きする形になる)。

## 関連

- [Stage 1 Bootstrap](../bootstrapping/12-vault-stage1.md)
- PR #228: retry_join 追加
