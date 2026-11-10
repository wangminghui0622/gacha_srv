#!/usr/bin/env bash
# 身份域模型生成（无缓存）：账号/手机号盲索引/三方绑定 + 屏障表，一致性优先不用缓存
# 来源：旧仓 user_nocache.sh 拆出（identity-rpc 段，库 gacha_identity）
# 用法：bash script/model/identity_nocache.sh
# 连接参数可用环境变量覆盖：USER / PASS / HOST（默认 root / we7root / 127.0.0.1:3306）
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

USER="${USER:-root}"
PASS="${PASS:-we7root}"
HOST="${HOST:-127.0.0.1:3306}"
DB="gacha_identity"
URL="${USER}:${PASS}@tcp(${HOST})/${DB}"
DIR="../../pkg/model_nocache/gacha_identity"

for t in user_users user_phones user_phone_grams user_third_parties dtm_barrier; do
    echo "=== ${t} ==="
    goctl model mysql datasource -url="$URL" -table="$t" -dir="$DIR" -cache=false --style=go_zero --home="../../pkg/template"
done
echo "√ 身份域 nocache 模型生成完成 -> ${DIR}"
