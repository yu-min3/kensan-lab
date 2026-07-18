# Cilium DaemonSet update strategy (the maxUnavailable=1 constraint)

## Symptom

During a Cilium chart upgrade, the whole 4-node cluster briefly becomes unreachable. Concretely:

- ClusterIP routing disappears (since kube-proxy replacement is in use, routing goes down when the Cilium Agent goes down)
- The Cilium Operator CrashLoops from being unable to reach the apiserver

## Cause

The Cilium chart's DaemonSet defaults `updateStrategy.rollingUpdate.maxUnavailable` to `2`. On a 4-node cluster, that means half the Agents restart simultaneously, during which the ClusterIP routing path disappears entirely.

In particular, while the Agent on the node running the Operator is restarting, apiserver access stalls, triggering a connection chain that sends the Operator into CrashLoop.

## Configuration (current)

`kubernetes/network/cilium/values.yaml`:

```yaml
updateStrategy:
  type: RollingUpdate
  rollingUpdate:
    maxUnavailable: 1
```

This keeps at least 3 nodes' Agents running at all times throughout the rolling update.

## Supplementary Operator tuning

The Operator itself is also trimmed down:

```yaml
operator:
  replicas: 1                    # default is 2; excessive on a 4-node cluster
  extraArgs:
    - --operator-k8s-client-qps=15
    - --operator-k8s-client-burst=30
```

Why `replicas: 1`: on 4 nodes, leader-election overhead and the apiserver load from CrashLoop cycles are bigger problems than losing HA on the Operator. The `extraArgs` rate limits tame the API burst right after the Operator restarts.

## Lesson

On a cluster with kube-proxy replacement enabled, a Cilium Agent rolling update **is equivalent to a rolling update of the routing layer itself**. Never leave `maxUnavailable` at its default — derive it from cluster size instead.

## Related

- [Cilium WiFi stability](./cilium-wifi-stability.md)
