---
name: troubleshoot
description: Troubleshoot a specific infrastructure component — resolve namespace, check pods, events, logs, and Argo CD sync status
argument-hint: <component>
---

# Troubleshoot Component

Diagnose issues with `$ARGUMENTS[0]`.

## Steps

1. **Resolve component → namespace**:

   | Component | Namespace |
   |-----------|-----------|
   | cilium | kube-system |
   | istio, istiod | istio-system |
   | argocd | argocd |
   | prometheus, grafana, loki, tempo, otel-collector | monitoring |
   | cert-manager | cert-manager |
   | sealed-secrets | kube-system |
   | keycloak | keycloak-prod / keycloak-dev |
   | backstage | backstage |

   If component is an app name, check `app-dev` and `app-prod`.

2. **Check Argo CD Application status**:
   ```bash
   kubectl get application $ARGUMENTS[0] -n argocd -o yaml
   ```
   - Look at sync status, health status, and any error conditions

3. **Check pods**:
   ```bash
   kubectl get pods -n <namespace> -l app.kubernetes.io/name=$ARGUMENTS[0]
   ```
   - If no label match, fall back to `kubectl get pods -n <namespace>`

4. **Check events**:
   ```bash
   kubectl get events -n <namespace> --sort-by='.lastTimestamp' | tail -20
   ```

5. **Check logs** (if pods exist but unhealthy):
   ```bash
   kubectl logs -n <namespace> <pod-name> --tail=50
   ```

6. **Diagnosis**:
   - Summarize findings
   - Suggest remediation steps
   - If GitOps-related, remind that fixes should go through Git (not kubectl apply)
