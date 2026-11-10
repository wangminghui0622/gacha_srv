#!/usr/bin/env bash
# 资金域模型生成（无缓存）：钱包/资产/流水/支付单全 nocache（资金一致性优先，行锁资源）
# 来源：旧仓 user_nocache.sh 资产段 + payment_nocache.sh 合并（money-rpc，库 gacha_money）
# payment_route_* 3 表走 internal/repo 原始 gorm 层（旧仓惯例），不生成模型
# 用法：bash script/model/money_nocache.sh
# 连接参数可用环境变量覆盖：USER / PASS / HOST（默认 root / we7root / 127.0.0.1:3306）
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

USER="${USER:-root}"
PASS="${PASS:-we7root}"
HOST="${HOST:-127.0.0.1:3306}"
DB="gacha_money"
URL="${USER}:${PASS}@tcp(${HOST})/${DB}"
DIR="../../pkg/model_nocache/gacha_money"

for t in wallet_accounts wallet_holds wallet_ledgers \
         user_balances user_balance_logs \
         assets asset_reservations pickup_codes \
         payment_orders payment_legs payment_refunds \
         dtm_barrier; do
    echo "=== ${t} ==="
    goctl model mysql datasource -url="$URL" -table="$t" -dir="$DIR" -cache=false --style=go_zero --home="../../pkg/template"
done
echo "√ 资金域 nocache 模型生成完成 -> ${DIR}"
