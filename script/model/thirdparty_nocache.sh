#!/usr/bin/env bash
# 第三方域模型生成（无缓存）：provider 指标 batch/raw/outbox + 腾讯广告点击，一致性优先
# integration_metric_current 为手写扩展模型（联合主键），勿重生成覆盖
# 用法：bash script/model/thirdparty_nocache.sh
# 连接参数可用环境变量覆盖：USER / PASS / HOST（默认 root / we7root / 127.0.0.1:3306）
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

USER="${USER:-root}"
PASS="${PASS:-we7root}"
HOST="${HOST:-127.0.0.1:3306}"
DB="gacha_thirdparty"
URL="${USER}:${PASS}@tcp(${HOST})/${DB}"
DIR="../../pkg/model_nocache/gacha_thirdparty"

for t in integration_metric_batch integration_metric_raw integration_metric_outbox integration_tx_ad_click; do
    echo "=== ${t} ==="
    goctl model mysql datasource -url="$URL" -table="$t" -dir="$DIR" -cache=false --style=go_zero --home="../../pkg/template"
done
echo "√ 第三方域 nocache 模型生成完成 -> ${DIR}"
