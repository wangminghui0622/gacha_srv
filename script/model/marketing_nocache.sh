#!/usr/bin/env bash
# 营销域模型生成（无缓存）：用户券/报价锁是行锁资源（FOR UPDATE 防重复锁），
# 积分配置/屏障表低频强一致，campaign 参与/计数/资格/流水（行锁+幂等键）——全 nocache
# campaign_versions/coupon_template_heads/coupon_template_versions 走 internal/repo
# 原始 gorm 层（旧仓惯例 M14d），不生成模型
# 用法：bash script/model/marketing_nocache.sh
# 连接参数可用环境变量覆盖：USER / PASS / HOST（默认 root / we7root / 127.0.0.1:3306）
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

USER="${USER:-root}"
PASS="${PASS:-we7root}"
HOST="${HOST:-127.0.0.1:3306}"
DB="gacha_marketing"
URL="${USER}:${PASS}@tcp(${HOST})/${DB}"
DIR="../../pkg/model_nocache/gacha_marketing"

for t in user_coupons quote_locks loyalty_offset_config dtm_barrier \
         campaign_sign_ins campaign_redemption_codes campaign_redemption_users \
         campaign_participations campaign_prize_grants campaign_wheel_slots \
         campaign_wheel_daily campaign_box_entitlements; do
    echo "=== ${t} ==="
    goctl model mysql datasource -url="$URL" -table="$t" -dir="$DIR" -cache=false --style=go_zero --home="../../pkg/template"
done
echo "√ 营销域 nocache 模型生成完成 -> ${DIR}"
