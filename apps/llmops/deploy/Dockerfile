# Build context: apps/llmops. No credentials or operator configuration in image.
FROM golang:1.25.5-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY contracts/ contracts/
COPY internal/ internal/
COPY cmd/llmops-api/ cmd/llmops-api/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/llmops-api ./cmd/llmops-api
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/llmops-api /usr/local/bin/llmops-api
USER 65532:65532
EXPOSE 8443
ENTRYPOINT ["/usr/local/bin/llmops-api"]
