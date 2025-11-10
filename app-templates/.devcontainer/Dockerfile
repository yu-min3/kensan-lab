FROM debian:bookworm

# Install basic packages (bash-completion moved to correct position)
RUN apt-get update && apt-get install -y \
    curl \
    wget \
    vim \
    git \
    make \
    lsof \
    unzip \
    bash-completion \
    podman \
    yamllint \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js and npm
RUN curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - \
    && apt-get install -y nodejs

# Install Claude Code globally
RUN npm install -g @anthropic-ai/claude-code

# Create vscode user and home directory
RUN useradd -m -s /bin/bash vscode

# Set working directory
WORKDIR /workspace
