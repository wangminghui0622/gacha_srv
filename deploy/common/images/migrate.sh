#!/bin/sh
# 迁移脚本：按文件名顺序执行 script/split/*.sql（建库建表 + 种子数据）。
# 01_databases.sql 建库，后续 02-12 只 USE 不建库，幂等可重复执行。
set -e

HOST="${MYSQL_HOST:-mysql}"
PORT="${MYSQL_PORT:-3306}"
USER="${MYSQL_USER:-root}"
PASS="${MYSQL_PASSWORD}"

echo ">> 迁移 MySQL ${HOST}:${PORT}（用户 ${USER}）"
for f in /migrations/*.sql; do
  echo ">> 执行 $(basename "$f")"
  mysql -h"$HOST" -P"$PORT" -u"$USER" -p"$PASS" --default-character-set=utf8mb4 < "$f"
done
echo ">> 迁移完成"
