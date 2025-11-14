FROM debian:bookworm

# Version pinning for cache stability
ARG KUBECTL_VERSION=v1.31.3
ARG ARGOCD_VERSION=v2.13.2
ARG KUSTOMIZE_VERSION=v5.5.0
ARG KUBESEAL_VERSION=v0.27.2
ARG HELM_VERSION=v3.16.3
ARG K9S_VERSION=v0.32.7
ARG NODE_MAJOR=22

# Install base packages first
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    curl \
    wget \
    ca-certificates \
    gnupg2 \
    git \
    vim \
    make \
    unzip \
    sudo \
    bash-completion \
    python3 \
    python3-pip \
    yamllint \
    dnsutils \
    podman \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js (separate layer for better caching)
RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_MAJOR}.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install uv (Python package manager) - using fixed installer
RUN curl -LsSf https://astral.sh/uv/0.5.11/install.sh | sh

# Install Claude Code globally
RUN npm install -g @anthropic-ai/claude-code

# Install Gemini CLI globally
RUN npm install -g @google/gemini-cli

# Install kubectl with fixed version
RUN curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/arm64/kubectl" && \
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl && \
    rm kubectl

# Install ArgoCD CLI with fixed version
RUN curl -sSL -o argocd-linux-arm64 "https://github.com/argoproj/argo-cd/releases/download/${ARGOCD_VERSION}/argocd-linux-arm64" && \
    install -m 555 argocd-linux-arm64 /usr/local/bin/argocd && \
    rm argocd-linux-arm64

# Install Kustomize with fixed version
RUN curl -sSL "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_arm64.tar.gz" | \
    tar xz -C /usr/local/bin/

# Install kubeseal with fixed version (note: file name uses version without 'v' prefix)
RUN KUBESEAL_VERSION_NUM=$(echo ${KUBESEAL_VERSION} | sed 's/^v//') && \
    curl -sSL "https://github.com/bitnami-labs/sealed-secrets/releases/download/${KUBESEAL_VERSION}/kubeseal-${KUBESEAL_VERSION_NUM}-linux-arm64.tar.gz" | \
    tar xz kubeseal && \
    install -m 755 kubeseal /usr/local/bin/kubeseal && \
    rm kubeseal

# Install Helm with fixed version
RUN curl -sSL "https://get.helm.sh/helm-${HELM_VERSION}-linux-arm64.tar.gz" | \
    tar xz --strip-components=1 -C /usr/local/bin/ linux-arm64/helm

# Install k9s with fixed version
RUN curl -sSL "https://github.com/derailed/k9s/releases/download/${K9S_VERSION}/k9s_Linux_arm64.tar.gz" | \
    tar xz -C /usr/local/bin/ k9s

# Create vscode user and home directory
RUN useradd -m -s /bin/bash vscode && \
    usermod -aG sudo vscode && \
    echo 'vscode ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/vscode && \
    chmod 0440 /etc/sudoers.d/vscode

# Create bash setup script
RUN echo '#!/bin/bash' > /tmp/setup-bash.sh && \
    echo '# Kubernetes alias' >> /tmp/setup-bash.sh && \
    echo 'alias k=kubectl' >> /tmp/setup-bash.sh && \
    echo '' >> /tmp/setup-bash.sh && \
    echo '# kubectl completion' >> /tmp/setup-bash.sh && \
    echo 'source /etc/bash_completion' >> /tmp/setup-bash.sh && \
    echo 'source <(kubectl completion bash)' >> /tmp/setup-bash.sh && \
    echo 'complete -o default -F __start_kubectl k' >> /tmp/setup-bash.sh && \
    chmod +x /tmp/setup-bash.sh

# Apply setup to both users
RUN cat /tmp/setup-bash.sh >> /home/vscode/.bashrc && \
    cat /tmp/setup-bash.sh >> /root/.bashrc && \
    rm /tmp/setup-bash.sh

# Change ownership of vscode home directory
RUN chown -R vscode:vscode /home/vscode

# Set default user 
USER root

# Set working directory
WORKDIR /workspace

