---
type: memo
tags: [whiteboard]
created: 2026-07-10
updated: 2026-07-13
---

## いま考えていること

- ノード追加を Ansible playbook 一発にしたい。kubeadm join + label 付与 + Cilium 確認まで
- observability のマルチテナント化はまだ早い。単一テナントで十分回っている
- 次のハード投資は 256GB SSD ×4 で Longhorn の replica を物理分散させる案

```
[master]---[switch]---[worker1]
                |------[worker2]
                |------[m4neo(NVMe)]
```

- DR 演習: 「クラスタ全消し → Git から再構築」を四半期に 1 回やる
