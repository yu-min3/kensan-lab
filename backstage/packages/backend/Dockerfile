# Build the backend bundle inside the image build. Explore must execute custom
# scaffolder actions from the checkout passed to `make try`, not from whichever
# published Backstage tag happens to be pinned in a manifest.
FROM --platform=$BUILDPLATFORM node:22-bookworm-slim AS bundle-builder

ENV PYTHON=/usr/bin/python3
RUN apt-get update && \
    apt-get install -y --no-install-recommends python3 g++ build-essential && \
    rm -rf /var/lib/apt/lists/*

USER node
WORKDIR /build
COPY --chown=node:node . .
RUN node .yarn/releases/yarn-4.4.1.cjs install --immutable && \
    node .yarn/releases/yarn-4.4.1.cjs workspace backend build

FROM node:22-bookworm-slim

# Set Python interpreter for `node-gyp` to use
ENV PYTHON=/usr/bin/python3

# Install isolate-vm dependencies, sqlite3, and mkdocs-techdocs-core
# Consolidated into single RUN to optimize build time (avoid redundant apt-get update)
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    python3 python3-pip g++ build-essential libsqlite3-dev && \
    pip3 install --no-cache-dir --break-system-packages mkdocs-techdocs-core==1.4.0 && \
    rm -rf /var/lib/apt/lists/*

# From here on we use the least-privileged `node` user to run the backend.
USER node

# This should create the app dir as `node`.
# If it is instead created as `root` then the `tar` command below will fail: `can't create directory 'packages/': Permission denied`.
# If this occurs, then ensure BuildKit is enabled (`DOCKER_BUILDKIT=1`) so the app dir is correctly created as `node`.
WORKDIR /app

# Copy files needed by Yarn
# Note: Build context is backstage/ (this Makefile's directory)
COPY --chown=node:node .yarn ./.yarn
COPY --chown=node:node .yarnrc.yml ./
COPY --chown=node:node backstage.json ./

# This switches many Node.js dependencies to production mode.
ENV NODE_ENV=production

# This disables node snapshot for Node 20 to work with the Scaffolder
ENV NODE_OPTIONS="--no-node-snapshot"

# Copy repo skeleton first, to avoid unnecessary docker cache invalidation.
# The skeleton contains the package.json of each package in the monorepo,
# and along with yarn.lock and the root package.json, that's enough to run yarn install.
COPY --chown=node:node yarn.lock package.json ./
COPY --from=bundle-builder --chown=node:node /build/packages/backend/dist/skeleton.tar.gz ./
RUN tar xzf skeleton.tar.gz && rm skeleton.tar.gz

RUN yarn workspaces focus --all --production && rm -rf "$(yarn cache clean)"

# NOTE: Examples are NOT included in the Docker image
# They are only available in local development environment (make dev)

# Copy production catalog (domains and organizations)
COPY --chown=node:node catalog ./catalog

# Copy templates directory (Backstage templates)
COPY --chown=node:node templates ./templates

# Both environments select an entrypoint at runtime. A missing file otherwise
# leaves the backend Healthy while the Create page quietly shows zero templates.
RUN test -f templates/fastapi-template/template.yaml && \
    test -f templates/fastapi-template/template-explore.yaml

# Then copy the rest of the backend bundle, along with any other files we might want.
COPY --from=bundle-builder --chown=node:node /build/packages/backend/dist/bundle.tar.gz ./
COPY --chown=node:node app-config*.yaml ./
RUN tar xzf bundle.tar.gz && rm bundle.tar.gz

# Links the published package to this repository, which is what lets the
# repository's GITHUB_TOKEN push to it from Actions. A package created by a
# manual `docker push` belongs to no repository, and the release job is denied
# with `permission_denied: write_package` until somebody connects the two by
# hand — this label makes that connection happen on its own from now on.
LABEL org.opencontainers.image.source="https://github.com/yu-min3/kensan-lab"

CMD ["node", "packages/backend", "--config", "app-config.yaml", "--config", "app-config.kubernetes.yaml"]
