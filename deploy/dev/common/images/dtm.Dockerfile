# DTM 服务镜像：COPY 本地编译好的 dtm 二进制 + 配置模板。
# DTM 用 `-c` 标志加载配置（go-zero 服务用 `-f`），entrypoint 通过 APP_FLAG 区分。
ARG CONFIG=conf/dtm.yml

FROM alpine:3.20
ARG CONFIG
RUN apk add --no-cache ca-certificates tzdata gettext

COPY bin/dtm /app/server
COPY ${CONFIG} /app/conf/config.yaml
COPY common/images/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh /app/server

ENV APP_FLAG=-c
ENTRYPOINT ["/app/entrypoint.sh"]
