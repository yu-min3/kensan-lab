FROM --platform=$BUILDPLATFORM golang:1.25 AS build
ARG TARGETARCH
WORKDIR /build
COPY apps/feed/go.mod apps/feed/go.sum ./
RUN go mod download
COPY apps/feed ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /feed ./cmd/feed

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /feed /usr/local/bin/feed
ENTRYPOINT ["/usr/local/bin/feed"]
CMD ["run"]
