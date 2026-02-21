---
name: cluster-status
description: Quick health check of the Kubernetes cluster — nodes, Argo CD apps, unhealthy pods, gateways, and certificates
argument-hint:
---

# Cluster Status Check

## Steps

1. **Node status**:
   ```bash
   kubectl get nodes -o wide
   ```

2. **Argo CD applications**:
   ```bash
   kubectl get applications -n argocd -o custom-columns='NAME:.metadata.name,SYNC:.status.sync.status,HEALTH:.status.health.status'
   ```
   - Flag any app not `Synced` + `Healthy`

3. **Unhealthy pods** (across all namespaces):
   ```bash
   kubectl get pods -A --field-selector=status.phase!=Running,status.phase!=Succeeded
   ```

4. **Gateway & LoadBalancer status**:
   ```bash
   kubectl get gateway -A
   kubectl get svc -A --field-selector spec.type=LoadBalancer
   ```

5. **Certificate status**:
   ```bash
   kubectl get certificate -A
   kubectl get clusterissuer
   ```

6. **Summary**:
   - Report overall health: nodes ready, apps synced, pods running, certs valid
   - Highlight any issues found with suggested next steps
