#!/bin/bash
# ============================================================
# 场景26：商品目录控制台全链路（M14c，admin gateway 9321 / catalog-rpc 9316）
#
# 覆盖：SPU/SKU 草稿→发布→修订→变体修订→修订发布（语义照抄
#       server_v2 CatalogService）——CreateProduct（DRAFT version 1，
#       校验负例 400 PRODUCT_INVALID）→ GetProduct（live 草稿读）→
#       ListProducts（游标分页）→ PublishProduct（版本前进：首发后
#       head 即 v2，首个快照 v2，无 v1 快照；幂等重放 200 不再物化；
#       期望版本不符 409）→ GetPublishedProduct（v2 200 / v1 404）→
#       SaveProductRevision（同内容幂等 / 异内容 409 / 期望版本不符 409）→
#       GetProductRevision → PublishProductRevision（v3；重放幂等；
#       发布后草稿删除 404）→ SaveProductVariantRevision（新增规格值+新
#       SKU、删除旧 SKU；选项名不符 409 / 期望版本不符 409）→ 发布 v4
#       → SQL 断言（3 快照、草稿删除、被删 SKU 归档 status=3、新值/新
#       SKU 落库）→ 权限负例 403 → 审计落库 → 现场清理。
#
# 断言打在 kratos 形错误体 {code,message,reason} 与 protojson 成功体上
# （uint64=JSON 字符串、枚举=名字面量如 PRODUCT_STATUS_DRAFT）。
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

print_title "场景26：商品目录控制台全链路（catalog console）"

ADMIN_PASS="admin12345678"   # ops-rpc.yaml Console.InitialAdminPassword（本地开发种子）

PRODUCT='{"productCode":"E2E-CAT-001","title":"E2E 联名T恤","options":[{"name":"颜色","sortOrder":1,"values":[{"value":"红","sortOrder":1},{"value":"蓝","sortOrder":2}]},{"name":"尺寸","sortOrder":2,"values":[{"value":"大","sortOrder":1},{"value":"小","sortOrder":2}]}],"skus":[{"skuCode":"S-红-大","title":"红色大码","fulfillmentMode":"FULFILLMENT_MODE_PHYSICAL","salePrice":{"amountMinor":2990,"currency":"CNY"},"referencePrice":{"amountMinor":3990,"currency":"CNY"},"selections":[{"optionIndex":0,"valueIndex":0},{"optionIndex":1,"valueIndex":0}]},{"skuCode":"S-红-小","title":"红色小码","fulfillmentMode":"FULFILLMENT_MODE_PHYSICAL","salePrice":{"amountMinor":2990,"currency":"CNY"},"referencePrice":{"amountMinor":3990,"currency":"CNY"},"selections":[{"optionIndex":0,"valueIndex":0},{"optionIndex":1,"valueIndex":1}]},{"skuCode":"S-蓝-大","title":"蓝色大码","fulfillmentMode":"FULFILLMENT_MODE_PHYSICAL","salePrice":{"amountMinor":2990,"currency":"CNY"},"referencePrice":{"amountMinor":3990,"currency":"CNY"},"selections":[{"optionIndex":0,"valueIndex":1},{"optionIndex":1,"valueIndex":0}]},{"skuCode":"S-蓝-小","title":"蓝色小码","fulfillmentMode":"FULFILLMENT_MODE_PHYSICAL","salePrice":{"amountMinor":2990,"currency":"CNY"},"referencePrice":{"amountMinor":3990,"currency":"CNY"},"selections":[{"optionIndex":0,"valueIndex":1},{"optionIndex":1,"valueIndex":1}]}]}'
VARIANT='{"productCode":"E2E-CAT-001","title":"E2E 联名T恤·新色","options":[{"name":"颜色","sortOrder":1,"values":[{"value":"红","sortOrder":1},{"value":"蓝","sortOrder":2},{"value":"绿","sortOrder":3}]},{"name":"尺寸","sortOrder":2,"values":[{"value":"大","sortOrder":1},{"value":"小","sortOrder":2}]}],"skus":[{"skuCode":"S-红-大","title":"红色大码","fulfillmentMode":"FULFILLMENT_MODE_PHYSICAL","salePrice":{"amountMinor":2990,"currency":"CNY"},"referencePrice":{"amountMinor":3990,"currency":"CNY"},"selections":[{"optionIndex":0,"valueIndex":0},{"optionIndex":1,"valueIndex":0}]},{"skuCode":"S-红-小","title":"红色小码","fulfillmentMode":"FULFILLMENT_MODE_PHYSICAL","salePrice":{"amountMinor":2990,"currency":"CNY"},"referencePrice":{"amountMinor":3990,"currency":"CNY"},"selections":[{"optionIndex":0,"valueIndex":0},{"optionIndex":1,"valueIndex":1}]},{"skuCode":"S-蓝-大","title":"蓝色大码","fulfillmentMode":"FULFILLMENT_MODE_PHYSICAL","salePrice":{"amountMinor":2990,"currency":"CNY"},"referencePrice":{"amountMinor":3990,"currency":"CNY"},"selections":[{"optionIndex":0,"valueIndex":1},{"optionIndex":1,"valueIndex":0}]},{"skuCode":"S-绿-大","title":"绿色大码","fulfillmentMode":"FULFILLMENT_MODE_PHYSICAL","salePrice":{"amountMinor":3190,"currency":"CNY"},"referencePrice":{"amountMinor":4190,"currency":"CNY"},"selections":[{"optionIndex":0,"valueIndex":2},{"optionIndex":1,"valueIndex":0}]}]}'

# ---------- 1. SQL 复位管理现场（幂等重跑） ----------
print_step "1. SQL 复位（admin TOTP；e2e 账号/角色；商城 7 表）"
do_mysql "UPDATE gacha_ops.console_accounts SET totp_secret='' WHERE username='admin';"
for u in e2e-cat-writer e2e-cat-reader e2e-cat-noperm; do
    oid=$(do_mysql "SELECT id FROM gacha_ops.console_accounts WHERE username='${u}';")
    if [ -n "$oid" ]; then
        do_mysql "DELETE FROM gacha_ops.console_sessions WHERE account_id=${oid}; DELETE FROM gacha_ops.principal_roles WHERE account_id=${oid}; DELETE FROM gacha_ops.console_accounts WHERE id=${oid};"
    fi
done
for c in e2e-cat-writer e2e-cat-reader e2e-cat-noperm; do
    rid=$(do_mysql "SELECT id FROM gacha_ops.roles WHERE code='${c}';")
    if [ -n "$rid" ]; then
        do_mysql "DELETE FROM gacha_ops.role_permissions WHERE role_id=${rid}; DELETE FROM gacha_ops.roles WHERE id=${rid};"
    fi
done
do_mysql "DELETE FROM gacha_catalog.sku_selections; DELETE FROM gacha_catalog.skus; DELETE FROM gacha_catalog.product_revision_drafts; DELETE FROM gacha_catalog.product_published_snapshots; DELETE FROM gacha_catalog.option_values; DELETE FROM gacha_catalog.product_options; DELETE FROM gacha_catalog.products;"

# ---------- 2. 超管登录 ----------
print_step "2. 超管两步登录（setup TOTP）"
line=$(admin_login_begin admin "$ADMIN_PASS" e2e-fp-admin) || exit 1
ticket=$(echo "$line" | cut -d' ' -f1)
secret=$(echo "$line" | cut -d' ' -f3)
[ "$(echo "$line" | cut -d' ' -f2)" == "true" ] || print_error "复位后应 setupRequired=true，实际: ${line}"
token_admin=$(admin_login_complete admin "$ADMIN_PASS" e2e-fp-admin "$ticket" "$(admin_totp "$secret")") || exit 1
[ -n "$token_admin" ] || print_error "超管第二步未返回 accessToken"

# ---------- 3. 角色 + 权限 ----------
print_step "3. 角色与权限：writer（read/create/write/publish）、reader（read）、noperm（无）"
writer_role=$(admin_role_create "$token_admin" e2e-cat-writer "E2E Cat Writer") || exit 1
reader_role=$(admin_role_create "$token_admin" e2e-cat-reader "E2E Cat Reader") || exit 1
noperm_role=$(admin_role_create "$token_admin" e2e-cat-noperm "E2E Cat No Perm") || exit 1
writer_perms='[{"resource":"catalog.product","action":"read","dataScope":"DATA_SCOPE_ALL","approvalMode":"APPROVAL_MODE_NONE"},{"resource":"catalog.product","action":"create","dataScope":"DATA_SCOPE_ALL","approvalMode":"APPROVAL_MODE_NONE"},{"resource":"catalog.product","action":"write","dataScope":"DATA_SCOPE_ALL","approvalMode":"APPROVAL_MODE_NONE"},{"resource":"catalog.product","action":"publish","dataScope":"DATA_SCOPE_ALL","approvalMode":"APPROVAL_MODE_NONE"}]'
reader_perms='[{"resource":"catalog.product","action":"read","dataScope":"DATA_SCOPE_ALL","approvalMode":"APPROVAL_MODE_NONE"}]'
out=$(admin_request PUT "/authorization/roles/${writer_role}/permissions" "{\"tenantId\":\"1\",\"expectedVersion\":\"1\",\"permissions\":${writer_perms}}" "$token_admin")
[ "$(admin_status "$out")" == "200" ] || print_error "writer 权限写入应 200，实际: $(admin_body "$out")"
out=$(admin_request PUT "/authorization/roles/${reader_role}/permissions" "{\"tenantId\":\"1\",\"expectedVersion\":\"1\",\"permissions\":${reader_perms}}" "$token_admin")
[ "$(admin_status "$out")" == "200" ] || print_error "reader 权限写入应 200，实际: $(admin_body "$out")"

# ---------- 4. 开通账号 + 绑定角色 ----------
print_step "4. 开通账号 writer/reader/noperm 并绑角色"
writer_line=$(admin_open_account "$token_admin" e2e-cat-writer "WriterPass1234") || exit 1
reader_line=$(admin_open_account "$token_admin" e2e-cat-reader "ReaderPass1234") || exit 1
noperm_line=$(admin_open_account "$token_admin" e2e-cat-noperm "NoPermPass1234") || exit 1
writer_id=$(echo "$writer_line" | cut -d' ' -f1)
reader_id=$(echo "$reader_line" | cut -d' ' -f1)
noperm_id=$(echo "$noperm_line" | cut -d' ' -f1)
out=$(admin_bind_roles "$token_admin" "$writer_id" 1 "$writer_role")
[ "$(admin_status "$out")" == "200" ] || print_error "writer 绑角色应 200，实际: $(admin_body "$out")"
out=$(admin_bind_roles "$token_admin" "$reader_id" 1 "$reader_role")
[ "$(admin_status "$out")" == "200" ] || print_error "reader 绑角色应 200，实际: $(admin_body "$out")"
out=$(admin_bind_roles "$token_admin" "$noperm_id" 1 "$noperm_role")
[ "$(admin_status "$out")" == "200" ] || print_error "noperm 绑角色应 200，实际: $(admin_body "$out")"

# ---------- 5. 三方登录 ----------
print_step "5. writer/reader/noperm 两步登录（setup TOTP）"
token_writer=$(admin_login e2e-cat-writer "WriterPass1234") || exit 1
token_reader=$(admin_login e2e-cat-reader "ReaderPass1234") || exit 1
token_noperm=$(admin_login e2e-cat-noperm "NoPermPass1234") || exit 1
[ -n "$token_writer" ] && [ -n "$token_reader" ] && [ -n "$token_noperm" ] || print_error "三方登录应全部拿到 token"

# ---------- 6. 权限负例 ----------
print_step "6. 权限负例：越权 403 CONSOLE_FORBIDDEN"
out=$(admin_request GET /catalog/products '' "$token_noperm")
[ "$(admin_status "$out")" == "403" ] || print_error "noperm 读列表应 403，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "CONSOLE_FORBIDDEN" ] || print_error "越权 reason 应 CONSOLE_FORBIDDEN，实际: $(admin_body "$out")"
out=$(admin_request POST /catalog/products "$PRODUCT" "$token_reader")
[ "$(admin_status "$out")" == "403" ] || print_error "reader 无 create 建商品应 403，实际: $(admin_status "$out") $(admin_body "$out")"
out=$(admin_request GET /catalog/products '' "$token_reader")
[ "$(admin_status "$out")" == "200" ] || print_error "reader 读列表应 200，实际: $(admin_status "$out") $(admin_body "$out")"

# ---------- 7. 创建商品（校验负例 → 成功） ----------
print_step "7. 创建商品：校验负例 400 PRODUCT_INVALID → 成功 DRAFT version 1"
out=$(admin_request POST /catalog/products '{"title":"无编码"}' "$token_writer")
[ "$(admin_status "$out")" == "400" ] || print_error "缺编码应 400，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "PRODUCT_INVALID" ] || print_error "缺编码 reason 应 PRODUCT_INVALID，实际: $(admin_body "$out")"
out=$(admin_request POST /catalog/products '{"productCode":"E2E-X","title":"无SKU"}' "$token_writer")
[ "$(admin_status "$out")" == "400" ] || print_error "无 SKU 应 400，实际: $(admin_status "$out") $(admin_body "$out")"
out=$(admin_request POST /catalog/products '{"productCode":"E2E-X","title":"币种不一致","options":[{"name":"颜色","values":[{"value":"红"}]}],"skus":[{"skuCode":"S-X","title":"X","fulfillmentMode":"FULFILLMENT_MODE_PHYSICAL","salePrice":{"amountMinor":100,"currency":"CNY"},"referencePrice":{"amountMinor":100,"currency":"USD"},"selections":[{"optionIndex":0,"valueIndex":0}]}]}' "$token_writer")
[ "$(admin_status "$out")" == "400" ] || print_error "sale/reference 币种不一致应 400，实际: $(admin_status "$out") $(admin_body "$out")"
out=$(admin_request POST /catalog/products "$PRODUCT" "$token_writer")
[ "$(admin_status "$out")" == "200" ] || print_error "建商品应 200，实际: $(admin_status "$out") $(admin_body "$out")"
cbody=$(admin_body "$out")
pid=$(extract_field "$cbody" productId)
[ "$pid" -ge 1 ] || print_error "productId 应 ≥1，实际: ${pid}"
[ "$(extract_field "$cbody" status)" == "PRODUCT_STATUS_DRAFT" ] || print_error "新商品应 DRAFT，实际: $(extract_field "$cbody" status)"
[ "$(extract_field "$cbody" version)" == "1" ] || print_error "草稿版本应为 1，实际: $(extract_field "$cbody" version)"
[ "$(echo "$cbody" | grep -o '"skuId"' | wc -l)" == "4" ] || print_error "应回 4 个 SKU，实际: ${cbody}"
sku_ids=($(echo "$cbody" | grep -o '"skuId":"[0-9]*"' | sed 's/[^0-9]//g'))
[ "${#sku_ids[@]}" == "4" ] || print_error "应提取 4 个 skuId，实际: ${sku_ids[*]}"
for s in "${sku_ids[@]}"; do [ "$s" -gt 0 ] || print_error "草稿 SKU ID 应为真实 ID，实际: ${sku_ids[*]}"; done
[ "$(echo "$cbody" | grep -o '"value":"' | wc -l)" == "4" ] || print_error "应回 4 个选项值，实际: ${cbody}"
[ "$(echo "$cbody" | grep -c '"value":"红"')" == "1" ] || print_error "应含选项值 红，实际: ${cbody}"

# ---------- 8. 草稿读取（live 读） ----------
print_step "8. GetProduct 草稿读取（ACTIVE 前走 live 表）"
out=$(admin_request GET "/catalog/products/${pid}" '' "$token_writer")
[ "$(admin_status "$out")" == "200" ] || print_error "读草稿应 200，实际: $(admin_status "$out") $(admin_body "$out")"
gbody=$(admin_body "$out")
[ "$(extract_field "$gbody" version)" == "1" ] || print_error "草稿版本应为 1，实际: $(extract_field "$gbody" version)"
[ "$(extract_field "$gbody" productCode)" == "E2E-CAT-001" ] || print_error "编码应 E2E-CAT-001，实际: $(extract_field "$gbody" productCode)"
out=$(admin_request GET /catalog/products/999999 '' "$token_writer")
[ "$(admin_status "$out")" == "404" ] || print_error "读不存在商品应 404，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "PRODUCT_NOT_FOUND" ] || print_error "不存在商品 reason 应 PRODUCT_NOT_FOUND，实际: $(admin_body "$out")"

# ---------- 9. 列表游标分页（建第二个商品驱动两页） ----------
print_step "9. ListProducts 游标分页（第二商品 E2E-CAT-002；pageSize=1 两页；末页 nextAfterProductId=0 省略）"
out=$(admin_request POST /catalog/products '{"productCode":"E2E-CAT-002","title":"E2E 联名帽","options":[{"name":"颜色","values":[{"value":"黑"}]}],"skus":[{"skuCode":"S-黑","title":"黑色均码","fulfillmentMode":"FULFILLMENT_MODE_DIGITAL","salePrice":{"amountMinor":990,"currency":"CNY"},"referencePrice":{"amountMinor":1290,"currency":"CNY"},"selections":[{"optionIndex":0,"valueIndex":0}]}]}' "$token_writer")
[ "$(admin_status "$out")" == "200" ] || print_error "建第二商品应 200，实际: $(admin_status "$out") $(admin_body "$out")"
out=$(admin_request GET '/catalog/products?pageSize=1' '' "$token_writer")
[ "$(admin_status "$out")" == "200" ] || print_error "列表应 200，实际: $(admin_status "$out") $(admin_body "$out")"
lbody=$(admin_body "$out")
[ "$(echo "$lbody" | grep -o '"productId"' | wc -l)" == "1" ] || print_error "pageSize=1 应只回 1 行，实际: ${lbody}"
[ "$(extract_field "$lbody" productCode)" == "E2E-CAT-001" ] || print_error "列表应含 E2E-CAT-001，实际: ${lbody}"
next_pid=$(extract_field "$lbody" nextAfterProductId)
[ "$next_pid" -gt 0 ] || print_error "首页应给出 nextAfterProductId，实际: ${next_pid}"
out=$(admin_request GET "/catalog/products?afterProductId=${next_pid}&pageSize=1" '' "$token_writer")
[ "$(admin_status "$out")" == "200" ] || print_error "末页请求应 200，实际: $(admin_status "$out") $(admin_body "$out")"
lbody=$(admin_body "$out")
[ "$(extract_field "$lbody" productCode)" == "E2E-CAT-002" ] || print_error "末页应含 E2E-CAT-002，实际: ${lbody}"
[ -z "$(extract_field "$lbody" nextAfterProductId)" ] || print_error "末页 nextAfterProductId 应为 0（protojson 省略），实际: ${lbody}"

# ---------- 10. 首次发布（版本前进语义） ----------
print_step "10. 首次发布：expected=1 → v2（首个快照即 v2）→ 重放幂等 → 期望版本不符 409"
out=$(admin_request POST "/catalog/products/${pid}/publish" '{"expectedVersion":"1"}' "$token_writer")
[ "$(admin_status "$out")" == "200" ] || print_error "发布应 200，实际: $(admin_status "$out") $(admin_body "$out")"
pbody=$(admin_body "$out")
[ "$(extract_field "$pbody" version)" == "2" ] || print_error "首发后版本应前进到 2，实际: $(extract_field "$pbody" version)"
[ "$(extract_field "$pbody" status)" == "PRODUCT_STATUS_ACTIVE" ] || print_error "发布后应 ACTIVE，实际: $(extract_field "$pbody" status)"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_catalog.product_published_snapshots WHERE product_id=${pid};")" == "1" ] || print_error "快照应恰 1 行（v2）"
[ "$(do_mysql "SELECT version FROM gacha_catalog.product_published_snapshots WHERE product_id=${pid};")" == "2" ] || print_error "首个快照版本应为 2（无 v1 快照）"
[ "$(do_mysql "SELECT version FROM gacha_catalog.products WHERE id=${pid};")" == "2" ] || print_error "head 版本应前进到 2"
out=$(admin_request POST "/catalog/products/${pid}/publish" '{"expectedVersion":"1"}' "$token_writer")
[ "$(admin_status "$out")" == "200" ] || print_error "重放发布应 200 幂等，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" version)" == "2" ] || print_error "重放应返回 v2，实际: $(admin_body "$out")"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_catalog.product_published_snapshots WHERE product_id=${pid};")" == "1" ] || print_error "重放不应新增快照"
out=$(admin_request POST "/catalog/products/${pid}/publish" '{"expectedVersion":"999"}' "$token_writer")
[ "$(admin_status "$out")" == "409" ] || print_error "期望版本不符应 409，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "PRODUCT_CONFLICT" ] || print_error "版本不符 reason 应 PRODUCT_CONFLICT，实际: $(admin_body "$out")"

# ---------- 11. 快照读取语义（GetPublishedProduct 无 HTTP 路由，内部 RPC；控制台走 live 读） ----------
print_step "11. 快照读取语义：ACTIVE 商品 live 读即快照 v2（无 v1 快照由 SQL 断言）"
out=$(admin_request GET "/catalog/products/${pid}" '' "$token_writer")
[ "$(admin_status "$out")" == "200" ] || print_error "ACTIVE 商品 live 读应 200，实际: $(admin_status "$out") $(admin_body "$out")"
snap=$(admin_body "$out")
[ "$(extract_field "$snap" version)" == "2" ] || print_error "ACTIVE live 读应回快照 v2，实际: ${snap}"
[ "$(extract_field "$snap" status)" == "PRODUCT_STATUS_ACTIVE" ] || print_error "快照状态应 ACTIVE，实际: ${snap}"
[ "$(echo "$snap" | grep -o '"skuCode"' | wc -l)" == "4" ] || print_error "快照应含 4 个 SKU，实际: ${snap}"
[ -z "$(do_mysql "SELECT id FROM gacha_catalog.product_published_snapshots WHERE product_id=${pid} AND version=1;")" ] || print_error "不应存在 v1 快照（首发版本前进语义）"

# ---------- 12. 保存修订 ----------
print_step "12. SaveProductRevision：期望版本不符 409 → 保存 200（DRAFT v3）→ 同内容幂等 → 异内容 409"
revise_skus="[{\"skuId\":\"${sku_ids[0]}\",\"title\":\"红色大码·新\",\"fulfillmentMode\":\"FULFILLMENT_MODE_PHYSICAL\",\"salePrice\":{\"amountMinor\":3090,\"currency\":\"CNY\"},\"referencePrice\":{\"amountMinor\":4090,\"currency\":\"CNY\"}},{\"skuId\":\"${sku_ids[1]}\",\"title\":\"红色小码\",\"fulfillmentMode\":\"FULFILLMENT_MODE_PHYSICAL\",\"salePrice\":{\"amountMinor\":2990,\"currency\":\"CNY\"},\"referencePrice\":{\"amountMinor\":3990,\"currency\":\"CNY\"}},{\"skuId\":\"${sku_ids[2]}\",\"title\":\"蓝色大码\",\"fulfillmentMode\":\"FULFILLMENT_MODE_PHYSICAL\",\"salePrice\":{\"amountMinor\":2990,\"currency\":\"CNY\"},\"referencePrice\":{\"amountMinor\":3990,\"currency\":\"CNY\"}},{\"skuId\":\"${sku_ids[3]}\",\"title\":\"蓝色小码\",\"fulfillmentMode\":\"FULFILLMENT_MODE_PHYSICAL\",\"salePrice\":{\"amountMinor\":2990,\"currency\":\"CNY\"},\"referencePrice\":{\"amountMinor\":3990,\"currency\":\"CNY\"}}]"
out=$(admin_request POST "/catalog/products/${pid}/revision" "{\"expectedVersion\":\"99\",\"title\":\"E2E 联名T恤·升级\",\"skus\":${revise_skus}}" "$token_writer")
[ "$(admin_status "$out")" == "409" ] || print_error "修订期望版本不符应 409，实际: $(admin_status "$out") $(admin_body "$out")"
out=$(admin_request POST "/catalog/products/${pid}/revision" "{\"expectedVersion\":\"2\",\"title\":\"E2E 联名T恤·升级\",\"skus\":${revise_skus}}" "$token_writer")
[ "$(admin_status "$out")" == "200" ] || print_error "保存修订应 200，实际: $(admin_status "$out") $(admin_body "$out")"
dbody=$(admin_body "$out")
[ "$(extract_field "$dbody" status)" == "PRODUCT_STATUS_DRAFT" ] || print_error "修订草稿应 DRAFT，实际: ${dbody}"
[ "$(extract_field "$dbody" version)" == "3" ] || print_error "修订草稿版本应为 3，实际: $(extract_field "$dbody" version)"
echo "$dbody" | grep -q '"title":"E2E 联名T恤·升级"' || print_error "草稿应含新标题，实际: ${dbody}"
out=$(admin_request POST "/catalog/products/${pid}/revision" "{\"expectedVersion\":\"2\",\"title\":\"E2E 联名T恤·升级\",\"skus\":${revise_skus}}" "$token_writer")
[ "$(admin_status "$out")" == "200" ] || print_error "同内容重存应 200 幂等，实际: $(admin_status "$out") $(admin_body "$out")"
out=$(admin_request POST "/catalog/products/${pid}/revision" "{\"expectedVersion\":\"2\",\"title\":\"E2E 联名T恤·改标题\",\"skus\":${revise_skus}}" "$token_writer")
[ "$(admin_status "$out")" == "409" ] || print_error "异内容重存应 409，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "PRODUCT_CONFLICT" ] || print_error "异内容 reason 应 PRODUCT_CONFLICT，实际: $(admin_body "$out")"
out=$(admin_request GET "/catalog/products/${pid}/revision" '' "$token_writer")
[ "$(admin_status "$out")" == "200" ] || print_error "读修订草稿应 200，实际: $(admin_status "$out") $(admin_body "$out")"
echo "$(admin_body "$out")" | grep -q '"title":"E2E 联名T恤·升级"' || print_error "修订草稿应含升级标题，实际: $(admin_body "$out")"

# ---------- 13. 发布修订 ----------
print_step "13. PublishProductRevision：expected=2 → v3 → 重放幂等 → 草稿删除 404"
out=$(admin_request POST "/catalog/products/${pid}/revision/publish" '{"expectedVersion":"2"}' "$token_writer")
[ "$(admin_status "$out")" == "200" ] || print_error "发布修订应 200，实际: $(admin_status "$out") $(admin_body "$out")"
pbody=$(admin_body "$out")
[ "$(extract_field "$pbody" version)" == "3" ] || print_error "修订发布后版本应 3，实际: $(extract_field "$pbody" version)"
[ "$(extract_field "$pbody" status)" == "PRODUCT_STATUS_ACTIVE" ] || print_error "修订发布后应 ACTIVE，实际: ${pbody}"
echo "$pbody" | grep -q '"title":"E2E 联名T恤·升级"' || print_error "v3 应含升级标题，实际: ${pbody}"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_catalog.product_published_snapshots WHERE product_id=${pid};")" == "2" ] || print_error "快照应 2 行（v2/v3）"
[ -z "$(do_mysql "SELECT id FROM gacha_catalog.product_revision_drafts WHERE product_id=${pid};")" ] || print_error "发布后草稿行应删除"
out=$(admin_request POST "/catalog/products/${pid}/revision/publish" '{"expectedVersion":"2"}' "$token_writer")
[ "$(admin_status "$out")" == "200" ] || print_error "重放发布修订应 200 幂等，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" version)" == "3" ] || print_error "重放应返回 v3，实际: $(admin_body "$out")"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_catalog.product_published_snapshots WHERE product_id=${pid};")" == "2" ] || print_error "重放不应新增快照"
out=$(admin_request GET "/catalog/products/${pid}/revision" '' "$token_writer")
[ "$(admin_status "$out")" == "404" ] || print_error "发布后读修订应 404，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "PRODUCT_NOT_FOUND" ] || print_error "无草稿 reason 应 PRODUCT_NOT_FOUND，实际: $(admin_body "$out")"

# ---------- 14. 变体修订（新增规格值+新 SKU、删除旧 SKU） ----------
print_step "14. SaveProductVariantRevision：选项名不符 409 → 保存 200 → 发布 v4（归档被删 SKU）"
out=$(admin_request POST "/catalog/products/${pid}/variant-revision" "{\"expectedVersion\":\"3\",\"product\":{\"productCode\":\"E2E-CAT-001\",\"title\":\"改名测试\",\"options\":[{\"name\":\"色彩\",\"values\":[{\"value\":\"红\"}]}],\"skus\":[{\"skuCode\":\"S-红-大\",\"title\":\"红色大码\",\"fulfillmentMode\":\"FULFILLMENT_MODE_PHYSICAL\",\"salePrice\":{\"amountMinor\":2990,\"currency\":\"CNY\"},\"referencePrice\":{\"amountMinor\":3990,\"currency\":\"CNY\"},\"selections\":[{\"optionIndex\":0,\"valueIndex\":0}]}]}}" "$token_writer")
[ "$(admin_status "$out")" == "409" ] || print_error "选项名不符应 409，实际: $(admin_status "$out") $(admin_body "$out")"
[ "$(extract_field "$(admin_body "$out")" reason)" == "PRODUCT_CONFLICT" ] || print_error "选项名不符 reason 应 PRODUCT_CONFLICT，实际: $(admin_body "$out")"
out=$(admin_request POST "/catalog/products/${pid}/variant-revision" "{\"expectedVersion\":\"99\",\"product\":${VARIANT}}" "$token_writer")
[ "$(admin_status "$out")" == "409" ] || print_error "变体期望版本不符应 409，实际: $(admin_status "$out") $(admin_body "$out")"
out=$(admin_request POST "/catalog/products/${pid}/variant-revision" "{\"expectedVersion\":\"3\",\"product\":${VARIANT}}" "$token_writer")
[ "$(admin_status "$out")" == "200" ] || print_error "保存变体修订应 200，实际: $(admin_status "$out") $(admin_body "$out")"
vbody=$(admin_body "$out")
[ "$(extract_field "$vbody" version)" == "4" ] || print_error "变体草稿版本应为 4，实际: $(extract_field "$vbody" version)"
echo "$vbody" | grep -q '"skuCode":"S-绿-大"' || print_error "变体草稿应含新 SKU S-绿-大，实际: ${vbody}"
echo "$vbody" | grep -q '"skuCode":"S-蓝-小"' && print_error "变体草稿不应含被删 SKU S-蓝-小，实际: ${vbody}"
echo "$vbody" | grep -q '"value":"绿"' || print_error "变体草稿应含新值 绿，实际: ${vbody}"
out=$(admin_request POST "/catalog/products/${pid}/revision/publish" '{"expectedVersion":"3"}' "$token_writer")
[ "$(admin_status "$out")" == "200" ] || print_error "发布变体修订应 200，实际: $(admin_status "$out") $(admin_body "$out")"
pbody=$(admin_body "$out")
[ "$(extract_field "$pbody" version)" == "4" ] || print_error "变体发布后版本应 4，实际: $(extract_field "$pbody" version)"
echo "$pbody" | grep -q '"value":"绿"' || print_error "v4 快照应含新值 绿，实际: ${pbody}"

# ---------- 15. SQL 终态断言 ----------
print_step "15. SQL 终态断言（3 快照、草稿删除、归档 SKU、新值/新 SKU 落库）"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_catalog.product_published_snapshots WHERE product_id=${pid};")" == "3" ] || print_error "终态快照应 3 行（v2/v3/v4）"
[ -z "$(do_mysql "SELECT id FROM gacha_catalog.product_revision_drafts WHERE product_id=${pid};")" ] || print_error "终态草稿应删除"
[ "$(do_mysql "SELECT version FROM gacha_catalog.products WHERE id=${pid};")" == "4" ] || print_error "head 版本应为 4"
[ "$(do_mysql "SELECT status FROM gacha_catalog.products WHERE id=${pid};")" == "2" ] || print_error "head 状态应为 2(ACTIVE)"
[ "$(do_mysql "SELECT status FROM gacha_catalog.skus WHERE product_id=${pid} AND sku_code='S-蓝-小';")" == "3" ] || print_error "被删 SKU S-蓝-小 应归档 status=3"
[ "$(do_mysql "SELECT status FROM gacha_catalog.skus WHERE product_id=${pid} AND sku_code='S-绿-大';")" == "2" ] || print_error "新 SKU S-绿-大 应 ACTIVE status=2"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_catalog.skus WHERE product_id=${pid} AND status=2;")" == "4" ] || print_error "ACTIVE SKU 应为 4 行"
[ -n "$(do_mysql "SELECT id FROM gacha_catalog.option_values WHERE \`value\`='绿';")" ] || print_error "新值 绿 应落库"
new_sku_id=$(do_mysql "SELECT id FROM gacha_catalog.skus WHERE product_id=${pid} AND sku_code='S-绿-大';")
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_catalog.sku_selections WHERE sku_id=${new_sku_id};")" == "2" ] || print_error "新 SKU 应有 2 条规格选择"
[ "$(do_mysql "SELECT COUNT(*) FROM gacha_catalog.sku_selections WHERE sku_id=${new_sku_id} AND option_value_id=0;")" == "0" ] || print_error "新 SKU 规格选择应解析为真实值 ID"

# ---------- 16. 审计落库断言 ----------
print_step "16. 审计落库（ops_audit_logs 按 operation_id 断言 catalog 操作）"
[ -n "$(do_mysql "SELECT id FROM gacha_ops.ops_audit_logs WHERE operation_id='console.catalog.create' AND request_id LIKE 'e2e-rid-%' LIMIT 1;")" ] || print_error "审计应含 console.catalog.create 行"
[ -n "$(do_mysql "SELECT id FROM gacha_ops.ops_audit_logs WHERE operation_id='console.catalog.publish' AND request_id LIKE 'e2e-rid-%' LIMIT 1;")" ] || print_error "审计应含 console.catalog.publish 行"
[ -n "$(do_mysql "SELECT id FROM gacha_ops.ops_audit_logs WHERE operation_id='console.catalog.saverevision' AND request_id LIKE 'e2e-rid-%' LIMIT 1;")" ] || print_error "审计应含 console.catalog.saverevision 行"
[ -n "$(do_mysql "SELECT id FROM gacha_ops.ops_audit_logs WHERE operation_id='console.catalog.savevariantrevision' AND request_id LIKE 'e2e-rid-%' LIMIT 1;")" ] || print_error "审计应含 console.catalog.savevariantrevision 行"
[ -n "$(do_mysql "SELECT id FROM gacha_ops.ops_audit_logs WHERE operation_id='console.catalog.publishrevision' AND request_id LIKE 'e2e-rid-%' LIMIT 1;")" ] || print_error "审计应含 console.catalog.publishrevision 行"

# ---------- 17. 现场清理 ----------
print_step "17. 现场清理（e2e 账号/角色；商城 7 表）"
for pair in "$writer_id e2e-cat-writer" "$reader_id e2e-cat-reader" "$noperm_id e2e-cat-noperm"; do
    set -- $pair
    do_mysql "DELETE FROM gacha_ops.console_sessions WHERE account_id=$1; DELETE FROM gacha_ops.principal_roles WHERE account_id=$1; DELETE FROM gacha_ops.console_accounts WHERE id=$1;"
done
for pair in "$writer_role e2e-cat-writer" "$reader_role e2e-cat-reader" "$noperm_role e2e-cat-noperm"; do
    set -- $pair
    do_mysql "DELETE FROM gacha_ops.role_permissions WHERE role_id=$1; DELETE FROM gacha_ops.roles WHERE id=$1;"
done
do_mysql "DELETE FROM gacha_catalog.sku_selections; DELETE FROM gacha_catalog.skus; DELETE FROM gacha_catalog.product_revision_drafts; DELETE FROM gacha_catalog.product_published_snapshots; DELETE FROM gacha_catalog.option_values; DELETE FROM gacha_catalog.product_options; DELETE FROM gacha_catalog.products;"

if [ "$FAILED" -eq 0 ]; then
    echo -e "${GREEN}场景26通过：商品目录控制台全链路（草稿/发布/修订/变体/归档/幂等/审计）${NC}"
    exit 0
else
    echo -e "${RED}场景26失败${NC}"
    exit 1
fi
