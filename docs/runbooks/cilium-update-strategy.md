# Cilium DaemonSet update strategy (maxUnavailable=1 制約)

## 症状

Cilium chart upgrade 時、4 ノードクラスタでクラスタ全体が一時的に unreachable になる。具体的には:

- ClusterIP routing が消失 (kube-proxy replacement 環境のため、Cilium Agent が落ちると routing も落ちる)
- Cilium Operator が apiserver 到達不能で CrashLoop

## 原因

Cilium chart の DaemonSet `updateStrategy.rollingUpdate.maxUnavailable` の default 値は `2`。4 ノードクラスタでは半数の Agent が同時に再起動することになり、その間 ClusterIP routing 経路が消失する。

特に Operator が乗っているノードの Agent が再起動中に apiserver アクセスが詰まり、Operator が CrashLoop に入る connection chain が起きる。

## 設定 (現状)

`infrastructure/network/cilium/values.yaml`:

```yaml
updateStrategy:
  type: RollingUpdate
  rollingUpdate:
    maxUnavailable: 1
```

これにより rolling 中も常に 3 ノードの Agent が稼働している状態を維持できる。

## Operator 設定の補助調整

Operator 自体も以下で軽量化:

```yaml
operator:
  replicas: 1                    # default 2、4ノードクラスタでは過剰
  extraArgs:
    - --operator-k8s-client-qps=15
    - --operator-k8s-client-burst=30
```

`replicas: 1` の理由: 4 ノードでは leader election の overhead と CrashLoop 時の apiserver 負荷の方が問題。`extraArgs` の rate limit は Operator 再起動直後の API burst を抑えるため。

## 教訓

kube-proxy replacement を有効にしている cluster では、Cilium Agent の rolling update は **routing 層の rolling** と等価。`maxUnavailable` を一律 default に任せず cluster size から逆算すること。

## 関連

- [Cilium WiFi stability](./cilium-wifi-stability.md)
