# syntax=docker/dockerfile:1.7
FROM node:22-bookworm-slim AS web-build
WORKDIR /src/web
COPY web/package*.json ./
RUN if [ -f package-lock.json ]; then npm ci --no-audit --no-fund; else npm install --no-audit --no-fund; fi
COPY web/ ./
RUN npm run build

FROM golang:1.26.3-bookworm AS go-build
ARG VERSION=container
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/forge ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/forge-worker ./cmd/worker \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/forge-migrate ./cmd/migrate

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates tzdata \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --system --uid 10001 --create-home forge \
 && mkdir -p /app/web /app/data /tmp/forge-nacos \
 && chown -R forge:forge /app /tmp/forge-nacos
WORKDIR /app
COPY --from=go-build /out/forge /app/forge
COPY --from=go-build /out/forge-worker /app/forge-worker
COPY --from=go-build /out/forge-migrate /app/forge-migrate
COPY --from=web-build /src/web/dist /app/web
COPY configs/minimal.yaml /app/config.yaml
USER 10001
EXPOSE 8080
ENV FORGE_CONFIG=/app/config.yaml FORGE_WEB_DIR=/app/web
ENTRYPOINT ["/app/forge"]
