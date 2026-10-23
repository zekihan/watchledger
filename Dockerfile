FROM node:24.22.1-bookworm-slim AS web
WORKDIR /src
COPY package.json package-lock.json ./
COPY web/package.json ./web/package.json
RUN npm ci
COPY web ./web
RUN npm run build

FROM golang:1.27.3-bookworm AS api
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY db ./db
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /watchledger ./cmd/watchledger

FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=api /watchledger /app/watchledger
COPY --from=web /src/web/dist /app/web
ENV WATCHLEDGER_ADDR=0.0.0.0:8080 WATCHLEDGER_WEB_DIR=/app/web
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/watchledger"]
