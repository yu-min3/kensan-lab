#!/bin/bash
# Helm リポジトリ追加
helm repo add cilium https://helm.cilium.io/
helm repo update

# マニフェスト生成（Hubble UI/Relay を含む）
helm template cilium cilium/cilium \
  --namespace kube-system \
  --set kubeProxyReplacement=true \
  --set k8sClientRateLimit.qps=10 \
  --set k8sClientRateLimit.burst=20 \
  --set k8s.cluster.cidr=10.244.0.0/16 \
  --set ipam.mode=kubernetes \
  --set l2announcements.enabled=true \
  --set externalIPs.enabled=true \
  --set devices=wlan0 \
  --set hubble.enabled=true \
  --set hubble.relay.enabled=true \
  --set hubble.ui.enabled=true \
  > ../../base-infra/cilium/cilium.yaml
