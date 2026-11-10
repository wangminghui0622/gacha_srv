#!/usr/bin/env bash
# 基础域模型生成（无缓存）：资格决策流水/风控规则/案例/事件 + 屏障表，一致性优先
# eligibility 三表（policies/rules/versions）走 internal/repo 原始 gorm 层（M14d），不生成模型
# 用法：bash script/model/base_nocache.sh
# 连接参数可用环境变量覆盖：USER / PASS / HOST（默认 root / we7root / 127.0.0.1:3306）
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

USER="${USER:-root}"
PASS="${PASS:-we7root}"
HOST="${HOST:-127.0.0.1:3306}"
DB="gacha_base"
URL="${USER}:${PASS}@tcp(${HOST})/${DB}"
DIR="../../pkg/model_nocache/gacha_base"

for t in eligibility_decision_logs risk_rules risk_cases risk_case_events dtm_barrier; do
    echo "=== ${t} ==="
    goctl model mysql datasource -url="$URL" -table="$t" -dir="$DIR" -cache=false --style=go_zero --home="../../pkg/template"
done
echo "√ 基础域 nocache 模型生成完成 -> ${DIR}"
