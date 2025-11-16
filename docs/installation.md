# Prerequisites

### Hardware

- **Minimum Configuration**: 1 Master + 1 or more Workers
- **Recommended Configuration**: 1 Master + 2 or more Workers
- **Memory**: Minimum 4GB per node, 8GB or more recommended
- **Storage**: Minimum 50GB per node, 100GB or more recommended
- **Network**: An environment where L2 network communication is possible

### Software

**For an existing cluster environment:**
- Kubernetes 1.27 or higher (built with kubeadm)
- kubectl (a version compatible with the cluster version)
- kubeconfig configured

**On a development machine:**
- kubectl
- helm 3.x
- kubeseal (Sealed Secrets CLI)
- docker or podman (for building container images)
- make
- Python 3.8 or higher (for CRD splitting script)

**Optional (for Backstage development):**
- Node.js 18.x or higher
- Yarn 4.x

### Accounts & Credentials

- GitHub Account (for container registry GHCR)
- GitHub Personal Access Token (with `packages:write` permissions)
- DNS Provider (for Cert-Manager + Let's Encrypt, e.g., AWS Route53)
- Domain Name (for issuing TLS certificates)

### Network Requirements

- IP range for Cilium LoadBalancer (a range that does not overlap with DHCP)
- Externally accessible IP address (for Istio Gateway)

> **Note**: For detailed configuration items, please refer to the [Environment-Specific Configuration Guide](./configuration.md).
