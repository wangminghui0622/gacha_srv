#!/bin/bash
# ============================================================
# 场景29：M15a 手机号/微信登录（player gateway 9310 / identity-rpc 9311 / thirdparty-rpc 9319）
#
# 覆盖：SQL 复位（三张 M15a 表整表清空——手机号列为密文无法按明文值清，
#       冒烟残留绑定可能属于任意用户，且三表仅 M15a 使用，整表清空最稳）+ Redis 频控复位
#       → 注册用户 A/B → phonebind 工具绑定（A 绑 13800001111 / B 抢同号 → 1012 / 非法号 → 400）
#       → 发码（POST /auth/phone/code）→ DryRun 从 Redis 读码（phonebind devcode）
#       → 错码登录 9001 → 对码登录 token → GET /user/profile 200
#       → 同码重放 9001（一次性消费）→ 同号 60s 内重发 9002 → clear 后恢复
#       → 未绑定手机号登录 1011
#       → 微信：本地 mock server（Mini APIBase 指向；App 未配置）
#         link A kind=1 openid=dev-openid-1 → scene=2 登录 token → profile 200
#         → 未绑定 openid 1013 → mock errcode 9006 → scene=1 未配置 9005 → scene 非法 400
#       → resolve/get 断言 → 现场清理（三表整表清空 + Redis + mock 进程）
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景29：M15a 手机号/微信登录"

PHONE_A=13800001111
PHONE_B=13800002222
WX_OPENID=dev-openid-1

# phonebind 工具：多次调用，先编译成 exe（比 go run 逐次编译快；场景 27 evalpol 为单次 go run 先例）
PB=/tmp/phonebind_m15a.exe
go build -o "$PB" "${SCRIPT_DIR}/tools/phonebind/main.go" || { print_error "构建 phonebind 失败"; exit $FAILED; }

# ---------- 1. 复位 ----------
print_step "1. 复位（三张 M15a 表整表清空 / Redis 频控清 / 微信 mock 启动）"
# 手机号列为密文，无法按明文值清；冒烟残留绑定可能属于任意用户（非本场景前缀），
# 且三表数据仅 M15a 使用（其他场景零依赖）→ 整表清空最稳（reset_test_data.sql 同思路）
do_mysql "DELETE FROM gacha_identity.user_phone_grams;"
do_mysql "DELETE FROM gacha_identity.user_phones;"
do_mysql "DELETE FROM gacha_identity.user_third_parties;"
"$PB" clear 1 "$PHONE_A" >/dev/null
"$PB" clear 1 "$PHONE_B" >/dev/null
# mock：起 test/tools/wechatmock（thirdparty-rpc yaml Mini.APIBase 已指向 9410）
go build -o /tmp/wechatmock_m15a.exe "${SCRIPT_DIR}/tools/wechatmock/main.go" || { print_error "构建 wechatmock 失败"; exit $FAILED; }
mock_pid=$(netstat -ano | grep "127.0.0.1:9410" | grep LISTENING | awk '{print $5}' | head -1)
[ -n "$mock_pid" ] && taskkill //F //PID "$mock_pid" 2>/dev/null
sleep 1
/tmp/wechatmock_m15a.exe > /tmp/wechatmock_m15a.log 2>&1 &
for i in $(seq 1 20); do
    netstat -ano | grep -q "127.0.0.1:9410.*LISTENING" && break
    sleep 0.5
done

# ---------- 2. 注册用户 A/B ----------
print_step "2. 注册用户 A/B"
a_line=$(register_user e2e_m15a_a) || exit $FAILED
uid_a=$(echo "$a_line" | cut -d' ' -f1)
b_line=$(register_user e2e_m15a_b) || exit $FAILED
uid_b=$(echo "$b_line" | cut -d' ' -f1)

# ---------- 3. 手机号绑定（内部 RPC 工具） ----------
print_step "3. 绑定：A 绑 13800001111 / B 抢同号 1012 / 非法号 400 / 精确查 + 片段搜"
out=$("$PB" set 1 "$uid_a" "$PHONE_A" 2>&1); rc=$?
[ $rc -eq 0 ] && echo "$out" | grep -q "userId=${uid_a}" || { print_error "A 绑定应成功: ${out}"; exit $FAILED; }
out=$("$PB" set 1 "$uid_b" "$PHONE_A" 2>&1); rc=$?
[ $rc -eq 1 ] && echo "$out" | grep -q "code=1012" || { print_error "B 抢同号应 1012，实际: ${out}"; exit $FAILED; }
out=$("$PB" set 1 "$uid_a" "12345" 2>&1); rc=$?
[ $rc -eq 1 ] && echo "$out" | grep -q "code=400" || { print_error "非法号应 400，实际: ${out}"; exit $FAILED; }
out=$("$PB" find 1 "$PHONE_A")
echo "$out" | grep -q "userId=${uid_a}" || { print_error "精确查应命中 A，实际: ${out}"; exit $FAILED; }
out=$("$PB" search 1 "0011")
echo "$out" | grep -q "masked=138\*\*\*\*1111" || { print_error "片段搜索应含掩码命中，实际: ${out}"; exit $FAILED; }
out=$("$PB" search 1 "9999")
echo "$out" | grep -q "^hits=0" || { print_error "无命中片段应 hits=0，实际: ${out}"; exit $FAILED; }
print_step "绑定/查询断言通过"

# ---------- 4. 发码 + 错码/对码/重放登录 ----------
print_step "4. 发码 → 错码 9001 → 对码 token → profile 200 → 同码重放 9001"
resp=$(do_post /auth/phone/code "{\"tenantId\":1,\"phone\":\"${PHONE_A}\"}")
[ "$(extract_code "$resp")" == "0" ] || { print_error "发码应 0: ${resp}"; exit $FAILED; }
expires=$(extract_field "$resp" expiresAtUnix)
[ -n "$expires" ] && [ "$expires" -gt 0 ] || { print_error "发码应回 expiresAtUnix>0: ${resp}"; exit $FAILED; }
code=$( "$PB" devcode 1 "$PHONE_A" | sed 's/code=//')
[ -n "$code" ] && [ ${#code} -eq 6 ] || { print_error "应读到 6 位验证码，实际: ${code}"; exit $FAILED; }
wrong=000000
[ "$code" == "000000" ] && wrong=111111
resp=$(do_post /auth/phone/login "{\"tenantId\":1,\"phone\":\"${PHONE_A}\",\"code\":\"${wrong}\"}")
[ "$(extract_code "$resp")" == "9001" ] || { print_error "错码登录应 9001，实际: ${resp}"; exit $FAILED; }
resp=$(do_post /auth/phone/login "{\"tenantId\":1,\"phone\":\"${PHONE_A}\",\"code\":\"${code}\"}")
[ "$(extract_code "$resp")" == "0" ] || { print_error "对码登录应 0，实际: ${resp}"; exit $FAILED; }
token=$(extract_field "$resp" token)
[ -n "$token" ] || { print_error "登录应回 token: ${resp}"; exit $FAILED; }
[ "$(extract_field "$resp" userId)" == "${uid_a}" ] || { print_error "登录 userId 应 A(${uid_a}): ${resp}"; exit $FAILED; }
profile=$(do_get /user/profile "$token")
[ "$(extract_code "$profile")" == "0" ] && echo "$profile" | grep -q "\"userId\":${uid_a}" || { print_error "token 应可查 A 资料: ${profile}"; exit $FAILED; }
resp=$(do_post /auth/phone/login "{\"tenantId\":1,\"phone\":\"${PHONE_A}\",\"code\":\"${code}\"}")
[ "$(extract_code "$resp")" == "9001" ] || { print_error "同码重放应 9001（一次性消费），实际: ${resp}"; exit $FAILED; }
print_step "手机号登录断言通过"

# ---------- 5. 频控 ----------
print_step "5. 同号 60s 内重发 9002 → clear 后恢复"
resp=$(do_post /auth/phone/code "{\"tenantId\":1,\"phone\":\"${PHONE_A}\"}")
[ "$(extract_code "$resp")" == "9002" ] || { print_error "60s 内重发应 9002，实际: ${resp}"; exit $FAILED; }
"$PB" clear 1 "$PHONE_A" >/dev/null
resp=$(do_post /auth/phone/code "{\"tenantId\":1,\"phone\":\"${PHONE_A}\"}")
[ "$(extract_code "$resp")" == "0" ] || { print_error "clear 后发码应恢复 0，实际: ${resp}"; exit $FAILED; }
print_step "频控断言通过"

# ---------- 6. 未绑定手机号登录 ----------
print_step "6. 未绑定手机号：发码 + 登录 → 1011"
resp=$(do_post /auth/phone/code "{\"tenantId\":1,\"phone\":\"${PHONE_B}\"}")
[ "$(extract_code "$resp")" == "0" ] || { print_error "未绑定号发码应 0（发码不查绑定），实际: ${resp}"; exit $FAILED; }
code2=$( "$PB" devcode 1 "$PHONE_B" | sed 's/code=//')
resp=$(do_post /auth/phone/login "{\"tenantId\":1,\"phone\":\"${PHONE_B}\",\"code\":\"${code2}\"}")
[ "$(extract_code "$resp")" == "1011" ] || { print_error "未绑定登录应 1011，实际: ${resp}"; exit $FAILED; }
print_step "未绑定断言通过"

# ---------- 7. 微信登录（mock + 内部 link） ----------
print_step "7. 微信：link → scene=2 登录 → 未绑定 1013 → errcode 9006 → scene=1 未配置 9005 → scene 非法 400"
out=$("$PB" link 1 "$uid_a" 1 "$WX_OPENID" 2>&1); rc=$?
[ $rc -eq 0 ] && echo "$out" | grep -q "linked" || { print_error "link 应成功: ${out}"; exit $FAILED; }
resp=$(do_post /auth/wechat/login "{\"tenantId\":1,\"code\":\"${WX_OPENID}\",\"scene\":2}")
[ "$(extract_code "$resp")" == "0" ] || { print_error "微信登录应 0，实际: ${resp}"; exit $FAILED; }
wxtoken=$(extract_field "$resp" token)
[ "$(extract_field "$resp" userId)" == "${uid_a}" ] || { print_error "微信登录 userId 应 A(${uid_a}): ${resp}"; exit $FAILED; }
profile=$(do_get /user/profile "$wxtoken")
[ "$(extract_code "$profile")" == "0" ] && echo "$profile" | grep -q "\"userId\":${uid_a}" || { print_error "微信 token 应可查 A 资料: ${profile}"; exit $FAILED; }
resp=$(do_post /auth/wechat/login '{"tenantId":1,"code":"dev-openid-999","scene":2}')
[ "$(extract_code "$resp")" == "1013" ] || { print_error "未绑定 openid 应 1013，实际: ${resp}"; exit $FAILED; }
resp=$(do_post /auth/wechat/login '{"tenantId":1,"code":"err","scene":2}')
[ "$(extract_code "$resp")" == "9006" ] || { print_error "mock errcode 应 9006，实际: ${resp}"; exit $FAILED; }
resp=$(do_post /auth/wechat/login '{"tenantId":1,"code":"any","scene":1}')
[ "$(extract_code "$resp")" == "9005" ] || { print_error "App 未配置应 9005，实际: ${resp}"; exit $FAILED; }
resp=$(do_post /auth/wechat/login '{"tenantId":1,"code":"any","scene":9}')
[ "$(extract_code "$resp")" == "400" ] || { print_error "scene 非法应 400，实际: ${resp}"; exit $FAILED; }
out=$("$PB" resolve 1 1 "$WX_OPENID")
echo "$out" | grep -q "userId=${uid_a}" || { print_error "resolve 应命中 A，实际: ${out}"; exit $FAILED; }
out=$("$PB" get 1 "$uid_a" 1)
echo "$out" | grep -q "openId=${WX_OPENID}" || { print_error "get 应回 openid，实际: ${out}"; exit $FAILED; }
print_step "微信登录断言通过"

# ---------- 8. 现场清理 ----------
print_step "8. 现场清理（三张 M15a 表整表清空 / Redis / mock 进程）"
do_mysql "DELETE FROM gacha_identity.user_phone_grams;"
do_mysql "DELETE FROM gacha_identity.user_phones;"
do_mysql "DELETE FROM gacha_identity.user_third_parties;"
"$PB" clear 1 "$PHONE_A" >/dev/null
"$PB" clear 1 "$PHONE_B" >/dev/null
mock_pid=$(netstat -ano | grep "127.0.0.1:9410" | grep LISTENING | awk '{print $5}' | head -1)
[ -n "$mock_pid" ] && taskkill //F //PID "$mock_pid" 2>/dev/null

if [ "$FAILED" -eq 0 ]; then
    echo -e "${GREEN}√ 场景29 M15a 手机号/微信登录 通过${NC}"
    exit 0
else
    echo -e "${RED}× 场景29 失败${NC}"
    exit 1
fi
