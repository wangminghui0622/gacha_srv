#!/usr/bin/env bash
# 玩法域模型生成（缓存版）：6 玩法配置表纯种子，运行时只读，无失效需求
# 来源：旧仓 game_cache.sh 玩法段拆出（play-rpc，库 gacha_play）
# 用法：bash script/model/play_cache.sh
# 连接参数可用环境变量覆盖：USER / PASS / HOST（默认 root / we7root / 127.0.0.1:3306）
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

USER="${USER:-root}"
PASS="${PASS:-we7root}"
HOST="${HOST:-127.0.0.1:3306}"
DB="gacha_play"
URL="${USER}:${PASS}@tcp(${HOST})/${DB}"
DIR="../../pkg/model/gacha_play"

for t in dig_box_configs dig_box_cells \
         full_car_configs full_car_entries gashapon_configs gashapon_entries \
         combo_chain_configs group_buy_configs group_buy_entries \
         memory_match_configs memory_match_entries; do
    echo "=== ${t} ==="
    goctl model mysql datasource -url="$URL" -table="$t" -dir="$DIR" -cache=true --style=go_zero --home="../../pkg/template"
done
echo "√ 玩法域 cache 模型生成完成 -> ${DIR}"
