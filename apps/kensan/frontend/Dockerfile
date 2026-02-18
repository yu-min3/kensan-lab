# Frontend Dockerfile (Production Build)
FROM node:22-alpine

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

# Observability proxy targets (Docker internal network)
ENV LOKI_URL=http://loki:3100
ENV TEMPO_URL=http://tempo:3200

# Expose preview server port
EXPOSE 5173

# Start preview server (serves production build)
CMD ["npm", "run", "preview", "--", "--host", "0.0.0.0", "--port", "5173"]
