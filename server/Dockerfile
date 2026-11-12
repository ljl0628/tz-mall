# syntax=docker/dockerfile:1
# ---------- 构建阶段:编译静态二进制 ----------
FROM golang:1.22-alpine AS builder

# 默认国内代理加速依赖下载;海外构建可 --build-arg GOPROXY=https://proxy.golang.org,direct
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

WORKDIR /app

# 先拷贝依赖清单,利用 Docker 层缓存加速重复构建
COPY go.mod go.sum ./
RUN go mod download

# 再拷贝源码编译;CGO_ENABLED=0 产出静态二进制,运行阶段不依赖 libc
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w" \
    -o /out/fawn-server ./cmd/server

# ---------- 运行阶段:最小镜像 ----------
FROM alpine:3.20

# ca-certificates:外呼 HTTPS 需要;tzdata:日志时间戳使用本地时区
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=builder /out/fawn-server ./fawn-server
# 内置默认配置;生产通过 TZ_* 环境变量覆盖(docker-compose.yml),密钥不进镜像
COPY config/config.yaml ./config/config.yaml

USER app
EXPOSE 8080

ENTRYPOINT ["./fawn-server"]
