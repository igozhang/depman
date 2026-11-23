# syntax=docker/dockerfile:1.7
# 2C2G：前端与 Go 串行；go/npm 用 cache mount；编译限 1 并行

FROM node:20-alpine AS web
WORKDIR /web
ENV NODE_OPTIONS=--max-old-space-size=384
COPY web/package.json web/package-lock.json* ./
RUN --mount=type=cache,target=/root/.npm \
    npm install --no-audit --no-fund --prefer-offline
COPY web/ ./
RUN npm run build

FROM golang:1.22-alpine AS build
WORKDIR /src
# 先等 web 完成，避免与 npm 并行抢内存；webdist 仅写入镜像构建层，不改宿主机 git 树
COPY --from=web /web/dist/. ./cmd/server/webdist/
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
COPY --from=web /web/dist/. ./cmd/server/webdist/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOMAXPROCS=1 \
    go build -p=1 -ldflags="-s -w" -o /out/depman ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
  && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
  && echo Asia/Shanghai > /etc/timezone
COPY --from=build /out/depman /depman
ENV DEPMAN_ADDR=:8080 \
    DEPMAN_DATA_DIR=/data \
    DEPMAN_DB_PATH=/data/depman.db \
    DEPMAN_ETC_DIR=/etc/depman \
    TZ=Asia/Shanghai
COPY etc/depman.conf /etc/depman/depman.conf
EXPOSE 8080
VOLUME ["/data"]
# 自定义 banner / conf：docker run -v /host/etc:/etc/depman
ENTRYPOINT ["/depman"]
