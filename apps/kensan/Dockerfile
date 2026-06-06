# kensan — 単一 image（Go API + ビルド済み SPA）
# multi-arch: docker buildx build --platform linux/amd64,linux/arm64

# ---- frontend build ----
FROM --platform=$BUILDPLATFORM node:22-slim AS frontend
WORKDIR /build
# tokens は相対 import（adoption-status.md の interim 方針）のため packages/ も必要
COPY packages/design-tokens /build/packages/design-tokens
COPY apps/kensan/frontend /build/apps/kensan/frontend
WORKDIR /build/apps/kensan/frontend
RUN npm ci && npm run build

# ---- backend build（クロスコンパイル: BUILDPLATFORM 上で TARGETARCH 向けに）----
FROM --platform=$BUILDPLATFORM golang:1.25 AS backend
ARG TARGETARCH
WORKDIR /build
COPY apps/kensan/backend /build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /kensan ./cmd/kensan

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /kensan /usr/local/bin/kensan
COPY --from=frontend /build/apps/kensan/frontend/dist /srv/dist
ENV KENSAN_STATIC_DIR=/srv/dist
ENV KENSAN_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/kensan"]
CMD ["serve"]
