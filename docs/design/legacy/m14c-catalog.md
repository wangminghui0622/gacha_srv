# M14c 商品目录控制台（catalog-rpc 激活 + SPU/SKU 草稿→发布→修订链路）

## Context

M14 管理控制台第 3 步：激活 catalog-rpc :9316（此前纯占位，无任何 Go 文件），克隆 server_v2
CatalogService 全部 9 个 RPC 与 8 条 console 路由（/console/v2/api/catalog/products/*）。
业务功能照抄 server_v2（M14 裁决：业务功能不动、接口不变、服务划分与实现可优化），
服务落在 catalog-rpc / gacha_catalog（本仓自建栈 go-zero+GORM），无 SAGA、无 DTM（单库事务）。

## 已验证的核心设计决策（server_v2 语义逐条提取）

- **D1 七张表照抄 server_v2 形状，命名按本仓风格**（去 catalog_ 前缀）：
  `products`（head：code/title/status 1草稿2已发布3已归档/version）、`product_options`、
  `option_values`、`skus`（status/fulfillment_mode/sale_amount_minor/reference_amount_minor/
  currency/external_code/version）、`sku_selections`（sku_id+option_id uk，sku_id+option_value_id uk）、
  `product_revision_drafts`（product_id 主键，base_version/target_version，CHECK target=base+1，
  draft_json 只存草稿 blob）、`product_published_snapshots`（product_id+version 主键，snapshot_json）。
  主键 AUTO_INCREMENT（server_v2 用 Snowflake——实现可优化）；status/fulfillment 取值与 proto 枚举同值。
- **D2 ACTIVE 商品读快照、live 表是草稿工作区**：GetProduct 读 head，head ACTIVE →
  读 published_snapshot(head.Version)；否则拼 live 五表。发布=在 live 表物化后插入快照，
  之后所有已发布版本只读快照（与 server_v2 Find/FindPublished 一致）。
- **D3 首次发布 Publish**：单事务，FOR UPDATE 锁 head；head ACTIVE 且 version==expected+1 →
  回滚+Find（幂等重放，expected+1 防回绕）；否则要求 DRAFT && version==expected；激活 SKU
  （DRAFT→ACTIVE，version+1，updated==count）、激活 head（DRAFT→ACTIVE **version=version+1**
  where version==expected）——**版本号随发布前进：首次发布后 head 即 v2，首个快照是 v2，
  不存在 v1 快照**（server_v2 ActivateProduct SQL 原样）；插快照（head 新版本）；提交。
  因此后续 SaveRevision 的 expectedVersion 从 2 开始，PublishRevision 的 expected≥2 与之自洽。
- **D4 修订 SaveRevision**：先 Find 现版本，要求 ACTIVE && version==expected &&
  修订 SKU 数==现 SKU 数（否则冲突）；clone+version+1+DRAFT；每个修订 SKU 必须按 ID 命中现 SKU、
  不重复、title 非空、fulfillment 非 UNSPECIFIED、金额 ≥0、币种 3 字母且 sale==reference。
  落库：锁 head 后要求 ACTIVE && version==base && draft.ID==id && draft.Version==base+1；
  已有草稿 → 内容相同幂等返回，不同 → 冲突；否则插 draft 行。
- **D5 变体修订 SaveVariantRevision**：expectedVersion ≥2；现版本 ACTIVE && version==expected &&
  请求 Code==现 Code；buildProduct 生成全新 ID；选项**名字集合**必须与现版本一致（option 数相等
  且每个新 option 名在旧版中存在）→ 旧 option/value 按名字复用旧 ID，新 value 用新 ID；
  SKU 按 code 匹配：旧 code 复用旧 ID 且组合不可变（换了组合 → 冲突），新 code 新 ID；
  走 D4 同一 SaveRevision 落库（幂等语义相同）。
- **D6 修订发布 PublishRevision**：expected ≥2；单事务锁 head；head.Version==expected+1 →
  回滚+FindPublished(id, head.Version)（幂等重放，head 前进正是上一笔发布留下的）；
  head 非 ACTIVE 或 version!=expected → 冲突；draft 必须存在且 base==expected、target==expected+1、
  blob 内 ID==id/Version==target/Status==DRAFT；读上一版本快照；选项 ID 与名字必须与上版一致；
  插入新增 option_values；`UPDATE products SET title,status=ACTIVE,version=version+1 WHERE id AND
  version=expected`（head 前进到 expected+1——这就是重放分支的判据）；保留 SKU 按 ID 更新为 ACTIVE
  （code 必须不变）、新 SKU 插 ACTIVE+selections、旧版未保留 SKU ACTIVE→ARCHIVED；插快照（target 版）；
  删 draft 行；提交。
- **D7 校验规则（Create/buildProduct，照抄 biz）**：code/title 非空、至少 1 SKU；option 名非空
  不重复、每 option ≥1 value、value 非空不重复；sku code 非空不重复、fulfillment 必须指定、
  sale 币种非空且 sale==reference 币种、selections 数与 option 数相等且每 option 恰选一个合法 value；
  组合不得重复（同 option 集合选同 value 集合）。
- **D8 错误码 4000 段（商城域）**：`4001 商品不存在`→404 PRODUCT_NOT_FOUND、
  `4002 商品版本或状态冲突`→409 PRODUCT_CONFLICT、`4003 商品校验失败`→400 PRODUCT_INVALID
  （server_v2 一律 500——本仓按 D9/M14b 先例细化状态码，信封形状不变）。
- **D9 无 requester/approver 语义、无 SAGA**：catalog RPC 全为单库事务（D3/D4/D5/D6 的 tx
  只写 gacha_catalog），无跨服务写 → 不进 DTM。审计由网关 ConsoleAuthorize 中间件按
  operation_id 自动落（catalog 8 操作已在 M14a 登记 adminop，MilestoneOf=M14c）。
- **D10 fail-closed 里程碑推进**：pkg/etcs_admin/gateway-api.yaml `FailClosedMilestone: M14c`，
  8 条路由 + 8 个目录操作双向闭合后启动。

## DDL — script/split/08_gacha_catalog.sql（现按库终态快照组织；原 18_gacha_catalog_m14c.sql 已并入）

七张表（见 D1），全部 InnoDB utf8mb4；外键同 server_v2（option→product、value→option、
sku→product、selection 三向、snapshot/draft→product）。无种子数据（场景 26 自建）。

## proto — pkg/proto/catalog.proto（照抄 server_v2 catalog.proto 字段名/编号）

- 枚举 `ProductStatus`（DRAFT=1/ACTIVE=2/ARCHIVED=3）、`FulfillmentMode`（PHYSICAL=1/DIGITAL=2/NONE=3），
  protojson 名字面量输出；`message Money { int64 amountMinor=1; string currency=2; }`。
- 9 RPC：CreateProduct / GetProduct / ListProducts（afterProductId+pageSize 游标，size 默认 20
  封顶 100，nextAfterProductId=0 表尾）/ PublishProduct / GetPublishedProduct /
  SaveProductRevision / SaveProductVariantRevision / GetProductRevision / PublishProductRevision。
- 生成：`bash script/goctl.sh catalog`（新增 case，gen_rpc 复用）。

## catalog-rpc 实现（app/catalog/rpc，骨架克隆 draw-rpc）

- catalog.go / internal/config / internal/svc（仅 DB，无缓存模型——管理侧低频读写，
  repo 直用 GORM 行结构，同 M14b pool.go 先例）/ internal/server（goctl 生成）。
- internal/core/catalog.go：buildProduct 校验 + 变体身份映射 + sameSelections（纯函数，单测）。
- internal/repo/catalog.go：Save/Find/List/FindPublished/Publish/SaveRevision/FindRevision/
  PublishRevision（D2-D6 的 tx 流；行锁 `clause.Locking{Strength:"UPDATE"}`）。
- internal/logic：9 个 logic，校验错→4003、冲突→4002、未找到→4001。

## admin gateway 8 路由（pkg/api_admin/desc/catalog.api + goctl admin-gateway）

| 方法/路径 | 操作（M14a 已登记） | RPC |
|---|---|---|
| GET /catalog/products | console.catalog.list (read) | ListProducts |
| POST /catalog/products | console.catalog.create (create) | CreateProduct |
| GET /catalog/products/:productId | console.catalog.get (read) | GetProduct |
| POST /catalog/products/:productId/publish | console.catalog.publish (publish) | PublishProduct |
| POST /catalog/products/:productId/revision | console.catalog.saverevision (write) | SaveProductRevision |
| POST /catalog/products/:productId/variant-revision | console.catalog.savevariantrevision (write) | SaveProductVariantRevision |
| GET /catalog/products/:productId/revision | console.catalog.getrevision (read) | GetProductRevision |
| POST /catalog/products/:productId/revision/publish | console.catalog.publishrevision (publish) | PublishProductRevision |

handler 用 restx.ParseProto 直接收发 catalog proto 消息（protojson camelCase），
productId 用 restx.PathUint64；中间件 RequestId+ConsoleAuth+ConsoleAuthorize（@server 声明）。
svc/config/yaml 增加 CatalogRpc 客户端（etcd Key catalog.rpc）。

## 测试 — test/scenario_26_catalog.sh（run_all 23 场景）

- SQL 复位（gacha_catalog 表清空）→ 登录 → 角色（catalog-writer：catalog.product 全动作；
  catalog-reader：read；noperm）→ 403 负例。
- 创建商品（2 option×2 value + 4 SKU 全组合）→ 校验负例 400（缺编码/空 skus/币种不一致）→
  读草稿（live 读）→ 列表游标（第二商品驱动两页，末页 nextAfterProductId 省略）。
- 首发 expected=1 → **head 前进到 v2、首个快照即 v2（无 v1 快照，SQL 断言）** → 重放幂等
  （快照不新增）→ 错误 expectedVersion 409。GetPublishedProduct 无 HTTP 路由（内部 RPC），
  ACTIVE 商品控制台读即快照。
- 保存修订（改 title+SKU 价）→ 取修订 → 同内容幂等 → 异内容 409 → 期望版本不符 409。
- 发布修订 expected=2 → head v3 → 重放幂等 → 修订草稿已删（getrevision 404）。
- 变体修订 expected=3（新增 option value「绿」+ 新增 SKU S-绿-大、删除 S-蓝-小；选项名
  不符 409 / 期望版本不符 409 负例）→ 发布 v4。
- SQL 断言：snapshot 3 行（v2/v3/v4）、draft 已删、被删 SKU 归档 status=3、新值/新 SKU
  落库且规格选择解析为真实值 ID。
- run_all.sh/menu.sh 注册 scenario_26。

## 实施顺序

1. errorx 4001-4003 + 05-errors.md 登记 + restx MapError 映射。
2. pkg/proto/catalog.proto → goctl.sh 加 catalog → 生成 app/catalog/rpc 骨架。
3. pkg/etcs/catalog-rpc.yaml；01_databases.sql + init_all.sql + 18_gacha_catalog_m14c.sql；
   mysql 建 gacha_catalog 库并执行 DDL。
4. catalog-rpc：catalog.go/config/svc + core/catalog.go（含单测）+ repo/catalog.go + 9 logic。
5. 网关：desc/catalog.api + gateway.api import → goctl admin-gateway → svc/config/yaml 加
   CatalogRpc + FailClosedMilestone M14c → 8 handler + 8 logic。
6. go build ./... + 启动 catalog-rpc + 重启 admin gateway → 场景 26 → run_all 23 场景 ×2。
7. go test 核心包 + npm run build → 文档更新 → 提交（Co-Authored-By，排除清单）。

## 风险与边界

1. 所有发布/修订事务内 FOR UPDATE 锁 head 行，并发串行化（server_v2 同款）。
2. 快照只读不写：live 表在 ACTIVE 后仅由修订事务触碰，与 server_v2 数据流一致。
3. 场景 26 全程确定性（无随机抽奖），断言用状态/版本/行数。
4. 不在本里程碑做：trade 下单消费目录（阶段 3）、web_admin 前端（M14e）、任务/报表（M14e）。
5. 永远只跑 `bash script/goctl.sh catalog` 与 `bash script/goctl.sh admin-gateway`，
   不跑玩家 gateway。
