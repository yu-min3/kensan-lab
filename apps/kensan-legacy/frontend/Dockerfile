# Frontend Dockerfile (Production Build)
# Multi-stage: build runs on host arch (no QEMU), runtime uses nginx on target platform.

# --- Build stage (runs natively on host architecture) ---
FROM --platform=$BUILDPLATFORM node:22-alpine AS builder

WORKDIR /app

# Build arguments for Vite environment variables
ARG VITE_USER_SERVICE_URL
ARG VITE_TASK_SERVICE_URL
ARG VITE_TIMEBLOCK_SERVICE_URL
ARG VITE_ANALYTICS_SERVICE_URL
ARG VITE_AI_SERVICE_URL
ARG VITE_MEMO_SERVICE_URL
ARG VITE_NOTE_SERVICE_URL

# Set environment variables for build
ENV VITE_USER_SERVICE_URL=$VITE_USER_SERVICE_URL
ENV VITE_TASK_SERVICE_URL=$VITE_TASK_SERVICE_URL
ENV VITE_TIMEBLOCK_SERVICE_URL=$VITE_TIMEBLOCK_SERVICE_URL
ENV VITE_ANALYTICS_SERVICE_URL=$VITE_ANALYTICS_SERVICE_URL
ENV VITE_AI_SERVICE_URL=$VITE_AI_SERVICE_URL
ENV VITE_MEMO_SERVICE_URL=$VITE_MEMO_SERVICE_URL
ENV VITE_NOTE_SERVICE_URL=$VITE_NOTE_SERVICE_URL

# Install dependencies
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

# Copy source code
COPY . .

# Build for production (MSW disabled because import.meta.env.DEV is false)
RUN npm run build

# --- Runtime stage (target platform) ---
FROM nginx:alpine

# Copy built assets
COPY --from=builder /app/dist /usr/share/nginx/html

# SPA fallback: all routes serve index.html
RUN printf 'server {\n\
    listen 80;\n\
    root /usr/share/nginx/html;\n\
    index index.html;\n\
    location / {\n\
        try_files $uri $uri/ /index.html;\n\
    }\n\
}\n' > /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
