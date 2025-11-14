FROM debian:bookworm

# Install basic packages (bash-completion moved to correct position)
RUN apt-get update && apt-get install -y \
    curl \
    wget \
    vim \
    git \
    make \
    ca-certificates \
    iproute2 \
    iputils-ping \
    net-tools \
    lsof \
    unzip \
    bash-completion \
    dnsutils \
    sudo \
    podman \
    yamllint \
    python3 \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js and npm (v22 for Backstage compatibility)
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y nodejs

# Install Claude Code globally
RUN npm install -g @anthropic-ai/claude-code

# Install Gemini CLI globally
RUN npm install -g @google/gemini-cli


# uvのインストール
RUN curl -LsSf https://astral.sh/uv/install.sh | sh

# kubectlのインストール
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/arm64/kubectl" && \
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# k9sのインストール
RUN curl -sS https://webinstall.dev/k9s | bash && \
    bash -c "source ~/.config/envman/PATH.env"

# argocd cliのインストール
RUN curl -sSL -o argocd-linux-arm64 https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-arm64 && \
    install -m 555 argocd-linux-arm64 /usr/local/bin/argocd && \
    rm argocd-linux-arm64

# kustomizeのインストール
RUN curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh" | bash && \
    mv kustomize /usr/local/bin/

# kubeseal
RUN wget https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.26.0/kubeseal-0.26.0-linux-arm64.tar.gz && \
    tar xfz kubeseal-0.26.0-linux-arm64.tar.gz && \
    sudo install -m 755 kubeseal /usr/local/bin/kubeseal && \
    rm kubeseal-0.26.0-linux-arm64.tar.gz kubeseal

# helmのインストール
RUN curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

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

