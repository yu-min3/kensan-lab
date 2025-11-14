# This dockerfile builds an image for the backend package.
# It should be executed with the root of the repo as docker context.
#
# Before building this image, be sure to have run the following commands in the repo root:
#
# yarn install --immutable
# yarn tsc
# yarn build:backend
#
# Once the commands have been run, you can build the image using `yarn build-image`
#
# Alternatively, there is also a multi-stage Dockerfile documented here:
# https://backstage.io/docs/deployment/docker#multi-stage-build

FROM node:22-bookworm-slim

# Set Python interpreter for `node-gyp` to use
ENV PYTHON=/usr/bin/python3

# Install isolate-vm dependencies, these are needed by the @backstage/plugin-scaffolder-backend.
# Also install mkdocs-techdocs-core for TechDocs local generation
RUN apt-get update && \
    apt-get install -y --no-install-recommends python3 python3-pip g++ build-essential && \
    pip3 install --no-cache-dir --break-system-packages mkdocs-techdocs-core==1.4.0 && \
    rm -rf /var/lib/apt/lists/*

# Install sqlite3 dependencies. You can skip this if you don't use sqlite3 in the image,
# in which case you should also move better-sqlite3 to "devDependencies" in package.json.
RUN apt-get update && \
    apt-get install -y --no-install-recommends libsqlite3-dev && \
    rm -rf /var/lib/apt/lists/*

# From here on we use the least-privileged `node` user to run the backend.
USER node

# This should create the app dir as `node`.
# If it is instead created as `root` then the `tar` command below will fail: `can't create directory 'packages/': Permission denied`.
# If this occurs, then ensure BuildKit is enabled (`DOCKER_BUILDKIT=1`) so the app dir is correctly created as `node`.
WORKDIR /app

# Copy files needed by Yarn
# Note: Build context is now k8s-platform-config/ (parent dir)
COPY --chown=node:node backstage-app/.yarn ./.yarn
COPY --chown=node:node backstage-app/.yarnrc.yml ./
COPY --chown=node:node backstage-app/backstage.json ./

# This switches many Node.js dependencies to production mode.
ENV NODE_ENV=production

# This disables node snapshot for Node 20 to work with the Scaffolder
ENV NODE_OPTIONS="--no-node-snapshot"

# Copy repo skeleton first, to avoid unnecessary docker cache invalidation.
# The skeleton contains the package.json of each package in the monorepo,
# and along with yarn.lock and the root package.json, that's enough to run yarn install.
COPY --chown=node:node backstage-app/yarn.lock backstage-app/package.json backstage-app/packages/backend/dist/skeleton.tar.gz ./
RUN tar xzf skeleton.tar.gz && rm skeleton.tar.gz

RUN yarn workspaces focus --all --production && rm -rf "$(yarn cache clean)"

# NOTE: Examples are NOT included in the Docker image
# They are only available in local development environment (make dev)

# TODO: Re-enable catalog copy after testing with more memory
# Copy production catalog (domains and organizations)
# COPY --chown=node:node backstage-app/catalog ./catalog

# Copy app-templates directory (Backstage templates)
COPY --chown=node:node app-templates ./app-templates

# Then copy the rest of the backend bundle, along with any other files we might want.
COPY --chown=node:node backstage-app/packages/backend/dist/bundle.tar.gz backstage-app/app-config*.yaml ./
RUN tar xzf bundle.tar.gz && rm bundle.tar.gz

CMD ["node", "packages/backend", "--config", "app-config.yaml", "--config", "app-config.kubernetes.yaml"]
