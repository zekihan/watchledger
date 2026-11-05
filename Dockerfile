FROM --platform=$BUILDPLATFORM node:24.22.2-bookworm-slim@sha256:01d566e8d94d8c619591d78652babfccd3c6992a3dffa9ed5ab39e11cf6c70e9 AS web
WORKDIR /src
COPY package.json package-lock.json ./
COPY web/package.json ./web/package.json
RUN npm ci
COPY web ./web
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27.3-bookworm@sha256:d9b6a1beb9474adc2a1ce842573639b44b2a0dd49b6669f51d3821baf9ec37c9 AS api
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY db ./db
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o /watchledger ./cmd/watchledger

FROM debian:trixie-slim@sha256:f6b08b09a05725f9431502d3b5714f5c4576e82c1a84fa1debe0c24a7e0c3ec4
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && rm -rf /var/lib/apt/lists/*
ARG VERSION=dev
LABEL org.opencontainers.image.source="https://github.com/zekihan/watchledger" \
      org.opencontainers.image.version=$VERSION \
      org.opencontainers.image.licenses="MIT"
WORKDIR /app
COPY --from=api /watchledger /app/watchledger
COPY --from=web /src/web/dist /app/web
ENV WATCHLEDGER_ADDR=0.0.0.0:8080 WATCHLEDGER_WEB_DIR=/app/web
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/watchledger"]
