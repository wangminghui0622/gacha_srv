#!/usr/bin/env bash
# 抽奖域模型生成（缓存版）：宝箱/奖池条目只读热门，走 gorm-zero 缓存模型
# 写路径一致性靠「事务内删缓存 + commit 后延迟双删 + 短 TTL」兜底（见 dtm_shop stock 服务）
# 用法：bash script/model/draw_cache.sh
# 连接参数可用环境变量覆盖：USER / PASS / HOST（默认 root / we7root / 127.0.0.1:3306）
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

USER="${USER:-root}"
PASS="${PASS:-we7root}"
HOST="${HOST:-127.0.0.1:3306}"
DB="gacha_draw"
URL="${USER}:${PASS}@tcp(${HOST})/${DB}"
DIR="../../pkg/model/gacha_draw"

for t in draw_boxes draw_pool_entries; do
    echo "=== ${t} ==="
    goctl model mysql datasource -url="$URL" -table="$t" -dir="$DIR" -cache=true --style=go_zero --home="../../pkg/template"
done
echo "√ 抽奖域 cache 模型生成完成 -> ${DIR}"
