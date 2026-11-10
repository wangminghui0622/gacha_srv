# syntax=docker/dockerfile:1
# 数据库迁移镜像：COPY 仓库 script/split/*.sql（建库建表 + 种子数据），
# 作为一次性 job 容器在服务启动前执行（depends_on mysql healthy）。
FROM alpine:3.20
RUN apk add --no-cache mysql-client

COPY script/split/ /migrations/
COPY deploy/common/images/migrate.sh /migrate.sh
RUN chmod +x /migrate.sh

ENTRYPOINT ["/migrate.sh"]
