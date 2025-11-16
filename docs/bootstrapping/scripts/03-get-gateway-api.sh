#!/bin/bash
# Gateway API CRDs をダウンロード
curl -sL https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.0/standard-install.yaml \
  -o ../../base-infra/gateway-api/gateway-api-crds.yaml
