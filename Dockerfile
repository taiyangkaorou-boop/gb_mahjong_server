# 多阶段：编译需要 Go + g++，运行只需要编好的程序和 libstdc++。
# 构建前务必：git submodule update --init --recursive
#
# 母镜像（第一次构建会从 Docker Hub 拉下来）：
#   golang:1.22-bookworm  = Debian 12「Bookworm」+ 官方 Go，用来编译
#   debian:bookworm-slim  = 同一代 Debian 的瘦身版，用来跑
# 不用 Ubuntu：和上面 Go 官方镜像同一套系统，体积更小。
#
# 国内拉模块慢时：
#   docker build --build-arg GOPROXY=https://goproxy.cn,direct -t gbmj-server .
FROM golang:1.22-bookworm AS build
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
ENV CGO_ENABLED=1
ENV GOOS=linux
WORKDIR /src

RUN apt-get update \
    && apt-get install -y --no-install-recommends g++ \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY api ./api
COPY cmd/server ./cmd/server
COPY cgo ./cgo
COPY configs ./configs
COPY internal ./internal
COPY pkg ./pkg
COPY third_party/GB-Mahjong ./third_party/GB-Mahjong

RUN test -f third_party/GB-Mahjong/mahjong/fan.cpp \
    || (echo "third_party/GB-Mahjong is empty; run: git submodule update --init --recursive" >&2 && exit 1)

RUN go build -o /out/server ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends libstdc++6 ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/server /app/server
COPY configs /app/configs

RUN mkdir -p /app/data

EXPOSE 8080
VOLUME ["/app/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/app/server"]
CMD ["configs/prod.yaml"]
