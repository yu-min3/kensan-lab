# konro — 単一 image（Go API + ビルド済み SPA）。apps/kensan/Dockerfile と同型
# multi-arch: docker buildx build --platform linux/amd64,linux/arm64

# ---- frontend build ----
FROM --platform=$BUILDPLATFORM node:22-slim AS frontend
WORKDIR /build
COPY apps/konro/frontend /build
RUN npm ci && npm run build

# ---- backend build（クロスコンパイル: BUILDPLATFORM 上で TARGETARCH 向けに）----
FROM --platform=$BUILDPLATFORM golang:1.25 AS backend
ARG TARGETARCH
WORKDIR /build
COPY apps/konro/backend /build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /konro ./cmd/konro

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /konro /usr/local/bin/konro
COPY --from=frontend /build/dist /srv/dist
ENV KONRO_STATIC_DIR=/srv/dist
ENV KONRO_ADDR=:8080
ENV KONRO_DATA_DIR=/data
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/konro"]
