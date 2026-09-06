FROM --platform=$BUILDPLATFORM golang:1.27.1-trixie AS build
ARG TARGETOS=linux
ARG TARGETARCH
WORKDIR /src
ENV GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/douyin-mcp .

FROM debian:13-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends chromium ca-certificates fonts-noto-cjk tini curl \
    && apt-get clean \
    && useradd --create-home --uid 10001 --shell /usr/sbin/nologin publisher \
    && install -d -o publisher -g publisher -m 0700 /app/data /app/media
WORKDIR /app
COPY --from=build /out/douyin-mcp /usr/local/bin/douyin-mcp
COPY LICENSE THIRD_PARTY_NOTICES.md /usr/share/doc/douyin-mcp/
COPY licenses/ /usr/share/doc/douyin-mcp/licenses/
USER 10001:10001
ENV ROD_BROWSER_BIN=/usr/bin/chromium \
    DY_HOST=0.0.0.0 \
    DY_PORT=18070 \
    DY_HEADLESS=true \
    DY_DATA_DIR=/app/data \
    DY_MEDIA_ROOT=/app/media
EXPOSE 18070
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl --fail --silent --output /dev/null http://127.0.0.1:18070/health || exit 1
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/douyin-mcp"]
