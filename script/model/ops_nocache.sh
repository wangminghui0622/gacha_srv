#!/usr/bin/env bash
# 运营域模型生成（无缓存）：控制台账号/会话/RBAC/审计/任务计划/看板——一致性优先，不用 Redis 缓存
# 来源：旧仓 ops_nocache.sh 原样迁移（ops-rpc，库 gacha_ops）
# reporting_metric_dimension / reporting_metric_snapshot / reporting_metric_bucket 为联合主键，
# goctl model 不支持——手写模型 pkg/model_nocache/gacha_ops/ 下 *_model.go（勿重生成覆盖）
# 用法：bash script/model/ops_nocache.sh
# 连接参数可用环境变量覆盖：USER / PASS / HOST（默认 root / we7root / 127.0.0.1:3306）
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

USER="${USER:-root}"
PASS="${PASS:-we7root}"
HOST="${HOST:-127.0.0.1:3306}"
DB="gacha_ops"
URL="${USER}:${PASS}@tcp(${HOST})/${DB}"
DIR="../../pkg/model_nocache/gacha_ops"

for t in console_accounts console_sessions roles role_permissions principal_roles ops_audit_logs \
         task_plan_head task_plan_version task_plan_publication_approval \
         reporting_metric_definition reporting_dimension_definition \
         reporting_dashboard reporting_dashboard_widget \
         reporting_metric_inbox; do
    echo "=== ${t} ==="
    goctl model mysql datasource -url="$URL" -table="$t" -dir="$DIR" -cache=false --style=go_zero --home="../../pkg/template"
done
echo "√ 运营域 nocache 模型生成完成 -> ${DIR}"
