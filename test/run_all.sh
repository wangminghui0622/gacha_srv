#!/bin/bash
# ============================================================
# 一键回归：服务预检 → 重置测试数据 → 顺序执行全部场景
# 任一场景失败整体退出码为 1（CI 友好）。
#
# 场景 5（概率公示核对，1000 抽约 4-6 分钟）、场景 6（故障注入，
# 约 2 分钟）、场景 10（捕获故障注入，两段锁约 4-5 分钟）属 M6 验收脚本，
# 耗时较长/依赖时序，不纳入一键回归（RUN_ALL_SKIP 见 scenarios.sh），
# 用 bash test/menu.sh 单独执行。
#
# 前置条件（真实启动顺序）：
#   MySQL(3306) / Redis(6379) / Etcd(2379) → DTM(36789/36790)
#   → identity-rpc(9311) → money-rpc(9312) → draw-rpc(9313) → base-rpc(9314) → marketing-rpc(9315) → play-rpc(9316) → task-rpc(9317) → catalog-rpc(9318) → thirdparty-rpc(9319) → ops-rpc(9320) → gateway(9310)
# 详见 docs/standards/backend/08-deploy-config.md 与 07-testing.md。
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"
source "${SCRIPT_DIR}/scenarios.sh"

# 一键回归排除慢/时序场景（见 scenarios.sh RUN_ALL_SKIP）
RUN_SCENARIOS=()
for s in "${SCENARIOS[@]}"; do
    skip=0
    for x in "${RUN_ALL_SKIP[@]}"; do
        [ "$s" == "$x" ] && skip=1 && break
    done
    [ $skip -eq 0 ] && RUN_SCENARIOS+=("$s")
done

echo ""
echo -e "${BLUE}============================================================${NC}"
echo -e "${BLUE}  Gacha e2e 一键回归（${#RUN_SCENARIOS[@]} 场景）${NC}"
echo -e "${BLUE}  网关: ${BASE_URL}${NC}"
echo -e "${BLUE}============================================================${NC}"

# ---------- 1. 服务存活预检 ----------
print_step "1. 服务存活预检（GET /boxes）"
resp=$(do_get /boxes)
if [ "$(extract_code "$resp")" != "0" ]; then
    echo -e "${RED}网关不可用或返回异常: ${resp}${NC}" >&2
    echo "请按顺序启动：MySQL/Redis/Etcd → DTM(app/dtm, 36789/36790)" >&2
    echo "             → identity-rpc(9311) → money-rpc(9312) → draw-rpc(9313) → base-rpc(9314) → marketing-rpc(9315) → play-rpc(9316) → task-rpc(9317) → catalog-rpc(9318) → thirdparty-rpc(9319) → ops-rpc(9320) → gateway(9310)" >&2
    exit 1
fi
print_step "网关存活 ✓（宝箱列表 code=0）"

# ---------- 2. 重置测试数据 ----------
print_step "2. 重置奖池库存（reset_test_data.sql）"
"${MYSQL_BIN}" --connect-timeout=5 -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" -p"${DB_PASS}" \
    --default-character-set=utf8mb4 gacha_draw < "${SCRIPT_DIR}/../script/sql/reset_test_data.sql"
if [ $? -ne 0 ]; then
    echo -e "${RED}重置 SQL 执行失败，检查 MySQL 是否在 ${DB_HOST}:${DB_PORT} 运行${NC}" >&2
    exit 1
fi
print_step "库存已复位"

# ---------- 3. 顺序执行场景 ----------
PASS=0
FAIL=0
for s in "${RUN_SCENARIOS[@]}"; do
    echo ""
    bash "${SCRIPT_DIR}/${s}.sh"
    rc=$?
    if [ $rc -eq 0 ]; then
        PASS=$((PASS + 1))
    else
        FAIL=$((FAIL + 1))
    fi
done

# ---------- 4. 汇总 ----------
echo ""
echo -e "${BLUE}============================================================${NC}"
echo -e "${BLUE}  回归汇总：通过 ${GREEN}${PASS}${BLUE} / 失败 ${RED}${FAIL}${BLUE} / 共 ${#RUN_SCENARIOS[@]}${NC}"
echo -e "${BLUE}============================================================${NC}"
if [ $FAIL -gt 0 ]; then
    exit 1
fi
exit 0
