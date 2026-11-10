# M15d 设计：integration 渠道收尾（微信小程序全家 + 支付宝 OAuth + 腾讯广告 + provider 指标 → thirdparty-rpc 9317 / gacha_thirdparty）

> 老项目参考：F:\workspace\src\server_v2 app/service/integration（alipay/tencentad/wechat 子包 + biz 指标 + data 存储/outbox）。
> 新仓落地：thirdparty-rpc 9317 扩 8 RPC + 首建 gacha_thirdparty 库（5 表）；ops-rpc 报表侧补 3 表 + ApplyMetricEvent 内部 RPC（收掉 M14e 报表 D4 偏离：provider 指标桶恒空）。
> M15a 已交付短信+微信登录（3 RPC）；本里程碑交付 integration 全套剩余：**微信小程序全家（access_token/手机号/二维码/urllink）+ 支付宝 OAuth + 腾讯广告点击 + provider 指标接入**。kuaidi100 承接方 delivery 属 B 类不克隆，留占位（决策不变）。

## 核心语义（照抄，常量不可改）

- **微信 access_token**：本地内存缓存（2h）→ Redis token store（key `luckybox:integration:wechat:mini:access_token`）→ `/cgi-bin/token`（grant_type=client_credential，expires_in 减 60s 余量，`expires_in<=60` → 2h）。`/wxa/business/getuserphonenumber`、`/wxa/getwxacodeunlimit`、`/wxa/generate_urllink` 均带 access_token；**errcode 40001/42001 → 清 token（本地+Redis）重试一次**。手机号：`phone_info.purePhoneNumber` 优先，空则 `phoneNumber`；二维码：scene 必填、page 空用 DefaultPage、width 0→430、响应 JSON（以 `{` 开头且有 errmsg）→ API 错误；urllink：path 必填、回 `url_link`。
- **支付宝**：`alipay.system.oauth.token`（grant_type=authorization_code 带 code / refresh_token 带 refresh_token，均 trim 非空）；响应 `user_id/access_token/refresh_token/open_id`，reply.union_id=open_id。**GetAliPayUserInfo 老项目是空回复 stub**——照抄（RPC 存在、返回空结构）。
- **腾讯广告**：TxAdClick → RecordClick：wechat_open_id 必填 → `CountByOpenID>0` 直接跳过（**按 openid 去重，uk 唯一键仲裁并发**）→ 落一行 ad_click。**ReportAction（SDK 上报转化）老项目零调用方（死代码），不搬**。
- **provider 指标**：请求校验（tenant>0、request_id≤64/idempotency_key≤128/provider_code≤64/external_account_id≤128 非空无控制符；samples 1..200；每样本 provider_metric_code≤128/external_object_id≤128/dimension_key≤256、bucket_start 合法 UTC 对齐（nanosecond=0、unix%seconds=0）、**seconds∈{3600,86400}**、value≥0、计数指标（impressions/clicks/conversions）currency 空、金额指标（spend/revenue）currency 3 位大写、槽位（object,metric,start,seconds,dimension）不重复）；request_hash=SHA256(deterministic proto marshal)。**幂等**：request_id 或 idempotency_key 命中且全字段+hash 一致 → 原批重放（replayed=true）；不一致 → 幂等冲突。**新旧仲裁**：`ON DUPLICATE KEY UPDATE` 的 `IF(VALUES(source_revision)>source_revision,...)`——新修订号严格大于才更新 current（affected==0 ⇒ stale，`>=` 不成立），stale 也落 raw（is_stale=1）但不发事件。
- **报表 Apply（ops）**：inbox 按 event_id 去重 → snapshot `FOR UPDATE` + `SourceRevision<=old` 跳过 → 货币差量 `Deltas(old,new,oldCur,newCur)`（旧币减旧值、新币加新值、0 值剔除）→ bucket 按币种 `value+=delta` → snapshot upsert。
- **看板查询**：widget 按 (metric,seconds,dimension,from,to) 去重取桶；NUMBER 按币种求和（币种字典序）；范围 floor/ceil + maxDataPoints 截断。

## 决策 D1-D9

- **D1 库与服务**：thirdparty-rpc 首建 **gacha_thirdparty**（01_databases.sql 增库）：`integration_metric_batch`（PK id）、`integration_metric_raw`（PK id）、`integration_metric_current`（PK 8 列复合）、`integration_metric_outbox`（PK id）、`integration_tx_ad_click`（PK id + uk wechat_open_id）。ops-rpc gacha_ops 加 `reporting_metric_inbox`（PK event_id）、`reporting_metric_snapshot`（PK 8 列复合）、`reporting_metric_bucket`（PK 8 列含 currency）。8 业务服务+2 网关不变。
- **D2 事件总线去 Kafka（偏离 2，M15c D4 同款）**：integration 侧 outbox 表 + **thirdparty-rpc 内置单实例轮询 dispatcher**（1s ticker：claim 最早可用行 → 直连 ops-rpc `ApplyMetricEvent` → 成功 published / 失败退避重试；启动时 status=2 残留回 status=1）。老项目 Kafka 消费者=ops reporting（inbox 去重 + revision 守卫），业务终点一致（指标终到 reporting_metric_bucket）。至少一次投递安全（inbox event_id 幂等 + snapshot revision 守卫）；单实例无租约（老项目多节点 lease 简化为单实例，冲突面更小）。
- **D3 支付宝不引 gopay（实现优化）**：crypto/rsa 手写 RSA2（参数按 k=v 字典序拼接 → SHA256withRSA → base64 签名；响应有 alipay_public_key 时验签），APIBase 可配（空=openapi.alipay.com，场景测试指本地 mock；公钥空跳过验签=开发模式，老项目同款宽容）。
- **D4 腾讯广告不搬死代码**：ReportAction/营销 SDK 零调用方不搬；只克隆 RecordClick 语义（RPC 面=TxAdClick）。
- **D5 ID 生成**：AUTO_INCREMENT 替代雪花（M14e 看板先例）：batch/raw/outbox 行顺序插入读 LastInsertId；outbox 行 id 即 event_id（MetricSnapshotChanged 的事件 id 语义保持）。
- **D6 errorx**（thirdparty 6000 段续、ops 7000 段续，05-errors.md 登记）：`6007 支付宝未配置` `6008 支付宝 API 错误（含入参非法）` `6009 腾讯广告未配置` `6010 广告点击记录失败（openid 缺失等）` `6011 指标请求非法` `6012 指标幂等冲突`；ops `7026 指标事件非法（event_id/tenant 缺失）`。微信四 RPC 复用 M15a 6005/6006 映射。全部内部 RPC，无网关路由 → 无 HTTP 码面。
- **D7 无 HTTP 路由（照抄）**：老项目玩家/控制台网关对这 8 个 RPC **零端点**（回调/上报方为外部 adapter 与小程序自有后端）。新仓同样全部内部 RPC，admin gateway fail-closed 清单不变；场景测试用 test/tools/thirdpartycli 直连 9317。
- **D8 场景 32**（test/scenario_32_integration_channels.sh + run_all/menu 注册，28→29 场景）：复位（integration 5 表 + reporting 3 表清空）→ 微信全家（token 两次同值+本地缓存、phone mock 首次 40001 重试后成功、qr PNG 魔数、urllink 字符串、未配置负例走配置开关不测）→ 支付宝（authorization_code 换 token 字段断言、空 code 6008、非法 grant 6008）→ 腾讯广告（同 openid 两次 → 1 行、异 openid → 2 行、空 openid 6010）→ 指标（合法批 3 样本 accepted=3 → batch/raw/current/outbox 断言 → **轮询 dispatcher** → ops inbox/snapshot/bucket 断言 → 场景内建已发布看板 query 数据点非空（providerCode=mock）→ 高修订 accepted 桶=差量、低修订 stale 桶不变 → 同 request_id 重放同批号 → 同幂等键异内容 6012 → 非法样本 6011（seconds/重复槽/计数带币种/负值）→ 复位。
- **D9 工具与 mock**：wechatmock（9410）扩 4 端点（/cgi-bin/token、/wxa/business/getuserphonenumber、/wxa/getwxacodeunlimit、/wxa/generate_urllink，含 errcode 计数与可切换故障）；新 **alipaymock**（9411，回 alipay_system_oauth_token_response JSON，公钥空不验签）；新 **thirdpartycli**（grpc 直连 9317：wechat-token/wechat-phone/wechat-qr/wechat-urllink/alipay-token/tx-ad-click/ingest）。

## DDL — 24_gacha_thirdparty_m15d.sql + 24_gacha_ops_m15d.sql（幂等，init_all.sql 聚合）

表形状照抄老项目 schema.sql（DATETIME(3) → DATETIME 本仓惯例；id BIGINT UNSIGNED AUTO_INCREMENT）：

- `integration_metric_batch`：id PK AUTO_INCREMENT、tenant_id、request_id(64)、idempotency_key(128)、request_hash BINARY(32)、provider_code(64)、external_account_id(128)、accepted_count、stale_count、created_at
- `integration_metric_raw`：id PK AUTO_INCREMENT、tenant_id、batch_id、provider_code、external_account_id、external_object_id(128)、provider_metric_code(128)、standard_metric TINYINT、bucket_start、bucket_seconds、dimension_key(256)、value BIGINT、currency CHAR(3)、source_revision BIGINT UNSIGNED、is_stale TINYINT、created_at
- `integration_metric_current`：PK(tenant_id,provider_code,external_account_id,external_object_id,standard_metric,bucket_start,bucket_seconds,dimension_key)、value、currency、source_revision、raw_id、updated_at
- `integration_metric_outbox`：id PK AUTO_INCREMENT、aggregate_id CHAR(64)、event_type(128)、payload BLOB、status TINYINT(1 待发/2 处理中/3 已发)、attempts、available_at、lease_owner(96)、locked_until NULL、last_error(512)、published_at NULL、created_at
- `integration_tx_ad_click`：id PK AUTO_INCREMENT、tenant_id、account_id(64)、click_id(128)、click_time BIGINT、wechat_open_id(128) uk、device_os_type(32)、created_at

- `reporting_metric_inbox`：event_id PK、tenant_id、event_type、source_revision、received_at
- `reporting_metric_snapshot`：PK(tenant_id,provider_code,external_account_id,external_object_id,standard_metric,bucket_start,bucket_seconds,dimension_key)、value、currency、source_revision、event_id、updated_at
- `reporting_metric_bucket`：PK(tenant_id,provider_code,external_account_id,standard_metric,bucket_seconds,bucket_start,dimension_key,currency)、value、updated_at

模型：新 script/model/thirdparty_nocache.sh（单 PK 4 表 goctl 生成；integration_metric_current 复合 PK 手写，reporting_metric_dimension 先例）；ops_nocache.sh 增 3 表（snapshot/bucket 复合 PK 手写、inbox goctl）。

## proto 扩展（`bash script/goctl.sh thirdparty` / `bash script/goctl.sh ops`，禁跑 gateway）

- thirdparty.proto：`GetWeChatMiniProgramAccessToken{tenant_id}→{access_token}`、`GetWeChatMiniProgramPhoneNumber{tenant_id,phone_code}→{phone_number,country_code}`、`GenerateWeChatMiniProgramQrCode{tenant_id,scene,page,width,check_path}→{image_png bytes}`、`GenerateWeChatMiniProgramUrlLink{tenant_id,path,query}→{url_link}`、`TxAdClick{tenant_id,account_id,callback,click_id,click_time,promoted_object_id,wechat_open_id,device_os_type}→{}`（字段照抄老项目）、`GetAliPayOauthToken{grant_type enum,auth_code,refresh_token}→{user_id,access_token,refresh_token,open_id,union_id}`、`GetAliPayUserInfo{access_token}→{open_id,avatar,nick_name}`（空回复 stub）、`IngestProviderMetrics{request_id,idempotency_key,provider_code,external_account_id,samples[]{provider_metric_code,standard_metric enum,external_object_id,bucket_start Timestamp,bucket_seconds,dimension_key,value,currency,source_revision},tenant_id}→{batch_id,accepted_count,stale_count,replayed}`。
- ops.proto：`ApplyMetricEvent{event_id,tenant_id,provider_code,external_account_id,external_object_id,standard_metric,bucket_start,bucket_seconds,dimension_key,value,currency,source_revision}→{applied}`（内部 RPC 无路由）。

## 实现顺序

1. DDL 两文件 + 01_databases.sql 加 gacha_thirdparty + init_all.sql 聚合 → 跑 → 模型脚本（thirdparty_nocache.sh 新建 / ops_nocache.sh 扩展）+ 手写复合 PK 模型 → serviceContext 接入（thirdparty 加 gorm 连接 + dispatcher 启动）。
2. errorx 6007-6012/7026 + 05-errors.md 登记。
3. thirdparty.proto +8 RPC → `bash script/goctl.sh thirdparty`；ops.proto +1 RPC → `bash script/goctl.sh ops`。
4. thirdparty：wechat Gateway 扩展（token 缓存/手机号重试/二维码/urllink + TokenStore）+ alipay OAuth 客户端 + tencentad 服务 + core/metrics 校验纯函数（单测）+ repo（batch/raw/current/outbox/ad_click）+ dispatcher + 8 logic + config/yaml（Mysql/Alipay/TencentAd/Wechat.DefaultPage）。
5. ops：ApplyMetricEvent logic + store.ApplyMetric（inbox/快照锁/差量）+ ListBuckets 实查询 + queryDashboardLogic 接线（per-widget 查询去重）。
6. 单测：wechat 解析、alipay 签名、metrics 校验、ops Deltas/Apply 纯函数。
7. 工具：wechatmock 扩 4 端点 + alipaymock 新建 + thirdpartycli 新建（build 到 /tmp）。
8. 场景 32 → run_all 29 场景 ×2 全绿。
9. 边界门 + 提交（Co-Authored-By，排除清单）+ docs/memory 更新。

## 偏离记录汇总（m15d）

1. **D2**：Kafka 事件总线 → thirdparty-rpc 内置 dispatcher 直连 ops-rpc ApplyMetricEvent（业务终点一致：指标终到 reporting_metric_bucket；单实例无租约）。
2. **D3**：gopay SDK → crypto/rsa 手写 RSA2 + APIBase 可配（测试可 mock；公钥空跳过验签=开发模式，老项目同款宽容）。
3. **D4**：腾讯广告 ReportAction（营销 SDK 上报）零调用方死代码，不搬。
4. **D5**：雪花 ID → AUTO_INCREMENT（M14e 看板先例）；outbox 行 id 即事件 id。
5. **GetAliPayUserInfo 空回复**：老项目 stub 照抄（非 bug，接口面保持）。

## 测试

- 单测：thirdparty core（metrics 校验/wechat 解析/alipay 签名）、ops core（Deltas）。
- 场景 32（D8）+ run_all/menu 注册（29 场景）。
- 边界门：run_all 29 场景 ×2 全绿、go build ./...、go test 核心包、npm run build ×2。

## 交付状态（2026-10-25）

- **已交付**：实现顺序 1-9 全部完成；场景 32 八步全绿（微信全家 40001 重试/二维码魔数/urllink、支付宝双 grant 指纹+6008 负例、腾讯广告去重、指标管线 3 表→outbox→ops 3 表、看板 query 数据点非空、仲裁/重放/冲突/非法）。
- **修复的真 bug（场景 32 仲裁步骤揪出）**：ops `core.Deltas` 原实现 `if oldCurrency != ""` 跳过旧值侧——计数指标（币种 `""`）高修订替换时桶差量变成 `+新值` 而非 `新-旧`（arb-b 桶 400≠300）。已改双侧无条件入账+同键相抵（dashboardConsole.go + TestDeltas 补「计数指标替换」「计数转金额」两案例）。金额指标同币种路径不受影响（旧币种非空），故 3 样本批断言此前未暴露。
- 场景断言口径说明：仲裁步骤用同一对象高修订替换验证差量（桶=终值 300），3 样本批用 3 个不同对象验证多行落库（桶=50/500/20）。
