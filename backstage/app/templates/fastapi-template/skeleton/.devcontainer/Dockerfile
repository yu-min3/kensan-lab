FROM python:3.11-bullseye

# Avoid warnings by switching to noninteractive
ENV DEBIAN_FRONTEND=noninteractive

# Install basic packages
RUN apt-get update && apt-get install -y \
    curl \
    wget \
    vim \
    git \
    make \
    ca-certificates \
    lsof \
    unzip \
    bash-completion \
    sudo \
    podman \
    uidmap \
    slirp4netns \
    && rm -rf /var/lib/apt/lists/*

# <<< 追加: podman を systemd なしで動かす設定 >>>
# /etc/containers/containers.conf を作って、
# cgroup_manager と events_logger を上書き
RUN mkdir -p /etc/containers && \
    printf '[engine]\n\
    cgroup_manager = "cgroupfs"\n\
    events_logger = "file"\n' \
    > /etc/containers/containers.conf

# Install uv
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

# Install kustomize
RUN curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh" | bash \
    && mv kustomize /usr/local/bin/

# Create non-root user
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

RUN groupadd --gid $USER_GID $USERNAME \
    && useradd --uid $USER_UID --gid $USER_GID -m $USERNAME \
    && echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME

# Switch back to dialog for any ad-hoc use of apt-get
ENV DEBIAN_FRONTEND=dialog

# Set user
USER $USERNAME

# Set working directory
WORKDIR /workspace

# Configure git to be safe
RUN git config --global --add safe.directory '*'

# Display versions
RUN echo "Python: $(python --version)" \
    && echo "uv: $(uv --version)" \
    && echo "kustomize: $(kustomize version --short)"
