# go-zero 服务通用镜像：COPY 本地已编译好的二进制 + 配置模板（不在镜像内编译）。
#
# 用法（构建上下文 = deploy/<env> 环境目录，含 bin/conf/common/images）：
#   docker build -f common/images/go.Dockerfile \
#     --build-arg BINARY=draw-rpc --build-arg CONFIG=conf/draw-rpc.yaml \
#     -t gacha/draw-rpc:dev .
#
# 配置里用 ${MYSQL_USER}/${MYSQL_PASS}/${REDIS_PASS}/${ETCD_ENDPOINTS} 占位，运行时由
# entrypoint 用 envsubst 替换（services-stack.yaml 的 environment 注入），密码不打进镜像。
ARG BINARY
ARG CONFIG

FROM alpine:3.20
ARG BINARY
ARG CONFIG
RUN apk add --no-cache ca-certificates tzdata gettext

COPY bin/${BINARY} /app/server
COPY ${CONFIG} /app/conf/config.yaml
COPY common/images/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh /app/server

ENTRYPOINT ["/app/entrypoint.sh"]
