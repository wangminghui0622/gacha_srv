#!/usr/bin/env bash
# 玩法域模型生成（无缓存）：6 玩法状态与记录（行锁 FOR UPDATE + uk request_id 幂等）
# 来源：旧仓 game_nocache.sh 玩法段拆出（play-rpc，库 gacha_play）
# 用法：bash script/model/play_nocache.sh
# 连接参数可用环境变量覆盖：USER / PASS / HOST（默认 root / we7root / 127.0.0.1:3306）
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

USER="${USER:-root}"
PASS="${PASS:-we7root}"
HOST="${HOST:-127.0.0.1:3306}"
DB="gacha_play"
URL="${USER}:${PASS}@tcp(${HOST})/${DB}"
DIR="../../pkg/model_nocache/gacha_play"

for t in dig_box_sessions dig_box_records \
         full_car_state full_car_records \
         gashapon_state gashapon_records gashapon_lucky_assignments \
         prize_grant_marks \
         combo_chain_state combo_chain_records combo_chain_streaks \
         group_buy_state group_buy_records \
         memory_match_sessions memory_match_rounds memory_match_settlements \
         memory_match_appearance_selections memory_match_appearance_entitlements \
         memory_match_champion memory_match_champion_events memory_match_champion_payouts \
         dtm_barrier; do
    echo "=== ${t} ==="
    goctl model mysql datasource -url="$URL" -table="$t" -dir="$DIR" -cache=false --style=go_zero --home="../../pkg/template"
done
echo "√ 玩法域 nocache 模型生成完成 -> ${DIR}"
