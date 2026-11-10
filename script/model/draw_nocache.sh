#!/usr/bin/env bash
# 抽奖域模型生成（无缓存）：抽赏记录/吉仓（行锁 FOR UPDATE + uk request_id 幂等）+ 屏障表
# draw_pool 4 表走 internal/repo 原始 gorm 层（旧仓惯例），不生成模型
# 用法：bash script/model/draw_nocache.sh
# 连接参数可用环境变量覆盖：USER / PASS / HOST（默认 root / we7root / 127.0.0.1:3306）
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

USER="${USER:-root}"
PASS="${PASS:-we7root}"
HOST="${HOST:-127.0.0.1:3306}"
DB="gacha_draw"
URL="${USER}:${PASS}@tcp(${HOST})/${DB}"
DIR="../../pkg/model_nocache/gacha_draw"

for t in draw_records draw_warehouse_items dtm_barrier; do
    echo "=== ${t} ==="
    goctl model mysql datasource -url="$URL" -table="$t" -dir="$DIR" -cache=false --style=go_zero --home="../../pkg/template"
done
echo "√ 抽奖域 nocache 模型生成完成 -> ${DIR}"
