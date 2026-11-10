#!/usr/bin/env bash
# 营销域模型生成（缓存版）：券模板走 gorm-zero 缓存模型（短 TTL 30s，模板变更少、读取热），
# campaigns 纯种子配置运行时只读（旧仓 game_cache.sh 迁来）
# 用法：bash script/model/marketing_cache.sh
# 连接参数可用环境变量覆盖：USER / PASS / HOST（默认 root / we7root / 127.0.0.1:3306）
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

USER="${USER:-root}"
PASS="${PASS:-we7root}"
HOST="${HOST:-127.0.0.1:3306}"
DB="gacha_marketing"
URL="${USER}:${PASS}@tcp(${HOST})/${DB}"
DIR="../../pkg/model/gacha_marketing"

for t in coupon_templates campaigns; do
    echo "=== ${t} ==="
    goctl model mysql datasource -url="$URL" -table="$t" -dir="$DIR" -cache=true --style=go_zero --home="../../pkg/template"
done
echo "√ 营销域 cache 模型生成完成 -> ${DIR}"
