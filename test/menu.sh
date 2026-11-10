#!/bin/bash
# ============================================================
# 交互菜单：单个场景选择执行（run_all.sh 之外的手动入口）
# 用法：bash test/menu.sh
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/scenarios.sh"

echo "=================================================="
echo "  Gacha e2e 场景菜单"
echo "=================================================="
i=1
for n in "${NAMES[@]}"; do
    echo "  $i) $n"
    i=$((i + 1))
done
echo "  0) 退出"
echo "=================================================="
read -r -p "选择场景 [1-32，回车=退出]: " choice
choice=${choice:-0}

if [ "$choice" == "0" ] || [ -z "$choice" ]; then
    echo "已退出"
    exit 0
fi

idx=$((choice - 1))
if [ "$idx" -lt 0 ] || [ "$idx" -ge "${#SCENARIOS[@]}" ]; then
    echo "无效选择: ${choice}" >&2
    exit 1
fi

bash "${SCRIPT_DIR}/${SCENARIOS[$idx]}.sh"
exit $?
