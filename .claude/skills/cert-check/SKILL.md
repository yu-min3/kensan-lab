---
name: cert-check
description: Check all TLS certificates — status, expiration, ClusterIssuer health, and certificate-to-gateway mapping
argument-hint:
---

# Certificate Health Check

## Steps

1. **ClusterIssuer status**:
   ```bash
   kubectl get clusterissuer -o wide
   kubectl describe clusterissuer
   ```
   - Verify ACME account is registered and ready

2. **All certificates**:
   ```bash
   kubectl get certificate -A -o custom-columns='NAMESPACE:.metadata.namespace,NAME:.metadata.name,READY:.status.conditions[0].status,EXPIRY:.status.notAfter,SECRET:.spec.secretName'
   ```

3. **Certificate-to-Gateway mapping**:

   | Certificate | Secret | Used By |
   |------------|--------|---------|
   | wildcard-platform | wildcard-platform-tls | gateway-platform |
   | wildcard-apps | wildcard-apps-tls | gateway-prod |

4. **Check for expiring certificates**:
   - Flag any cert expiring within 30 days
   - cert-manager should auto-renew at 2/3 lifetime

5. **Check certificate secrets**:
   ```bash
   kubectl get secret -A | grep tls
   ```
   - Verify referenced secrets exist

6. **Summary**:
   - Report certificate health
   - If issues found, suggest: delete secret to trigger renewal, or check cert-manager logs
