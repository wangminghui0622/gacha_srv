# M15a 设计：手机号/微信登录 + thirdparty-rpc 激活

> 2026-10-24。M15 = A 类补全（service-mapping.md §3，用户拍板全部补克隆）。
> M15a 首发 = 手机号/微信登录 + integration（短信/微信）克隆 + thirdparty-rpc 9317 激活。
> 老项目依据（只读）：identity 服务（user_session/phone/third_party）+ integration 服务（sms/wechat）。
> 归属（已定）：登录/账号 → user-rpc 9311；发码/微信接口 → thirdparty-rpc 9317。**不开新服务**。

## 老项目行为盘点（已逐文件核实）

- **验证码**：integration/sms —— RedisCaptchaStore（TTL 15min；key `luckybox:sms:captcha:{tenant}:{md5(phone)}`；频控列表 24h>10 次或 60s 内重发 → busy；Verify 用 Lua GET+比对+DEL **一次性消费**）；Sender 双通道（阿里云/腾讯），**DryRun 直接跳过**（开发模式），888 前缀手机号跳过阿里云（测试号），两通道都关且非 DryRun → disabled。
- **手机号登录**（identity）：`RequestPhoneLoginCode` → 调 integration.SendCaptcha(scene=LOGIN) 回 expires_at_unix；`LoginWithPhoneCode` → integration.CheckCaptcha → **phones.FindExact（必须先绑定，不自动注册）** → 会话签发。老项目响应 = UserSessionTokens（6 字段，refresh 家族）。
- **微信登录**（identity）：`LoginWithWeChatCode` → integration.ExchangeWeChatCode（Mini=jscode2session / App=oauth2 access_token，APIBase/OAuthBase 可配）→ **third_party.ResolveUser（必须先 Link，不自动注册）** → 会话签发。
- **绑定入口**：老项目两个网关都**没有**绑定端点——SetUserPhone 是内部 RPC（x-identity-phone-token caller secret 鉴权，campaign smokelinks 工具调用）；LinkThirdPartyIdentity 无任何调用方。即：老项目登录端点已接线，但绑定只能走内部 RPC/工具。
- **手机号存储**：identity phonecrypto 盲索引加密——AES-256-GCM 密文 + HMAC-SHA256 exact/4-gram 索引 token（AAD 绑定 tenant+user，keyID+indexVersion）；exact_token 唯一索引仲裁并发绑定；NormalizeCNMobile（剥 +86、11 位 1[3-9]）。
- **设备依赖**：老项目会话 Start 要求 deviceID 非零且设备 ACTIVE——但**两个网关都没有 RegisterDevice 路由**，HTTP 面上 device_id 永远无来源（老项目此依赖自身就是断的）。本仓会话无设备概念（M7 起）。

## 决策

### D1 范围：M15a = 登录所需最小 thirdparty 面
thirdparty-rpc 首落 3 RPC：SendCaptcha / CheckCaptcha / ExchangeWeChatCode（sms + wechat code 交换，登录所需）。
微信小程序全家（access_token/手机号/二维码/urllink）、支付宝 OAuth、腾讯广告、kuaidi100 → **M15 后续子里程碑**（有业务承接时再补，service-mapping.md §3 注记）。
ClaimPrize 维持 SAGA 自动发奖（用户已拍板，不动）。

### D2 登录语义照抄：绑定前置、不自动注册
手机号登录 = 验证码校验 → 已绑定手机 → 发会话；微信登录 = code 交换 → 已 Link → 发会话。
**绑定入口照抄为 user-rpc 内部 RPC**（无网关路由）：SetUserPhone / LinkThirdPartyIdentity；场景测试用 test/tools 工具直连 RPC 绑定（evalpol 工具先例）。玩家自助绑定端点老项目没有，本仓也不加（未来要加再说，非 M15a）。

### D3 会话体系维持本仓设计（响应 {userId, token}）★ 可复议
老项目手机号/微信登录响应 = UserSessionTokens（access/refresh 双 token + session_id + 过期），依赖 refresh 家族（user_sessions 表 + 轮转 + 指纹 + 设备）。
本仓玩家会话自 M7 起 = 单 JWT（jwtutil HMAC + 网关 VerifyToken），全部既有端点/前端/场景基于此。
**决定**：手机号/微信登录响应与 Login 一致 `{userId, token}`，请求形状保留 tenant_id/device_id/client_fingerprint 字段（接口照抄）——tenant_id 校验 =1（本仓单租户，与 M14d 控制台注入一致），device_id/client_fingerprint 透传不参与会话签发。
理由：refresh 家族/设备是会话架构而非登录业务功能；老项目网关连设备注册端点都没有，该依赖自身断裂；按 M14 裁决"处理方式可以优化"维持本仓会话架构。
**备选（若你要照抄 UserSessionTokens 六字段）**：需 user_sessions 表 + Ed25519 + refresh 轮转/吊销 + 网关 /auth/session/{start,refresh} 端点——量大，且与既有玩家 API 两套会话并存。默认按 D3 实施。

### D4 手机号存储照抄 phonecrypto 盲索引加密
移植 phonecrypto（~160 行纯函数 + 单测）：AES-256-GCM + HMAC-SHA256 exact/4-gram token + AAD(tenant,user) + keyID/version；两表 user_phones（exact_token 唯一索引）+ user_phone_grams（片段搜索）。
密钥在 user-rpc.yaml（enc/index 两把 32B base64 + keyID + version，生产走发布流程）。
PhoneService 三 RPC 照抄（SetUserPhone / FindUserPhoneExact / SearchUserPhones，含 500 候选上限 + 解密切片验证 + 3-4-4 掩码）。
两处偏离：**去掉 caller-secret 头部鉴权**（本仓内部 RPC 无此约定，无路由即内部）；**去掉 GrantRegistration 副作用**（本仓 campaign 无注册奖励挂钩）。

### D5 验证码存储照抄（thirdparty-rpc 内 Redis）
RedisCaptchaStore 全量照抄（key/TTL/频控/Lua 一次性消费）。Sender 照抄：DryRun 默认开（开发 yaml），阿里云/腾讯双通道，888 前缀测试号。错误映射：码错/过期 → 6001；频控 busy → 6002；未配通道且非 DryRun → 6003；手机号格式 → 6004。

### D6 微信 code 交换照抄（thirdparty-rpc 内 HTTP 客户端）
wechat.Gateway 照抄：Mini `jscode2session` / App `oauth2/access_token`；APIBase/OAuthBase 可配（**场景测试指向本地 mock server**——老项目配置能力，非新机制）；未配置 → 6005；微信 API 错误（errcode≠0 / openid 空）→ 6006。token_store（小程序 access_token 缓存）**已由 M15d 交付**（本地缓存→Redis→/cgi-bin/token 三级 + 40001 清缓存重试，见 m15d-integration-channels.md）。

### D7 wire 契约（user.proto 扩展 + thirdparty.proto 新建）
字段名/序号照抄老项目 proto；uint64 → 本仓 int64 风格；枚举照抄。
**user.proto（User 服务 +9 RPC）**：
- `RequestPhoneLoginCode{tenant_id,phone}` → `{expires_at_unix}`（reserved 2 = 老项目 dev_code，照抄）
- `LoginWithPhoneCode{tenant_id,phone,code,device_id,client_fingerprint}` → `{userId,token}`（响应形状偏离见 D3）
- `LoginWithWeChatCode{tenant_id,code,scene,device_id,client_fingerprint}` → `{userId,token}`
- `SetUserPhone{tenant_id,user_id,phone}` → `{user_id}`；`FindUserPhoneExact{tenant_id,phone}` → `{user_id}`；`SearchUserPhones{tenant_id,fragment}` → `{hits[]{user_id,masked_phone}}`
- `LinkThirdPartyIdentity{tenant_id,user_id,kind,open_id,union_id}` → `{}`；`ResolveUserByThirdParty{tenant_id,kind,open_id}` → `{user_id}`；`GetThirdPartyIdentity{tenant_id,user_id,kind}` → `{open_id,union_id}`
- enum `ThirdPartyKind{WECHAT_MINI_PROGRAM=1,WECHAT_APP=2}`；enum `WeChatOAuthScene{APP=1,MINI_PROGRAM=2}`；union_id 空时以 open_id 回填（老项目 Link 语义）
**thirdparty.proto（新服务 Thirdparty，3 RPC）**：
- `SendCaptcha{tenant_id,phone,scene}` → `{expires_at_unix}`；`CheckCaptcha{tenant_id,phone,code,scene}` → `{}`；`ExchangeWeChatCode{tenant_id,scene,code}` → `{open_id,union_id,session_key}`
- enum `CaptchaScene{LOGIN=1}`（老项目 UNSPECIFIED 也按 LOGIN 处理，SceneLogin()）
**玩家网关 3 端点**（/gacha/api/v1 前缀，本仓风格；对照老路径 /v2/api/auth/session/{phone/code,phone/login,wechat/login}）：
- `POST /auth/phone/code`、`POST /auth/phone/login`、`POST /auth/wechat/login`（公开组，无 JWT）

### D8 错误码（05-errors.md 登记）
- 用户域：1011 CodePhoneLoginInvalid（未绑定手机号登录，老项目 ErrInvalidPhoneLogin；验证码错归 6001）；1012 CodePhoneInUse（绑定冲突）；1013 CodeThirdPartyNotLinked；1014 CodePhoneSearchTooBroad
- 第三方域 6000 段首落：6001 CodeCaptchaInvalid；6002 CodeCaptchaBusy（HTTP 429）；6003 CodeSmsDisabled；6004 CodePhoneInvalid；6005 CodeWechatNotConfigured；6006 CodeWechatAPIError

### D9 表结构（20_gacha_user_m15a.sql，幂等）
- `user_phones`：tenant_id,user_id,ciphertext varbinary,nonce varbinary,enc_key_id varchar(64),index_version smallint,exact_token binary(32) **uk**,created_at,updated_at
- `user_phone_grams`：tenant_id,index_version,gram_token binary(32),user_id（组合索引搜索）
- `user_third_parties`：tenant_id,kind tinyint,open_id,union_id,user_id,created_at；**uk(tenant_id,kind,open_id)**（Upsert）
- thirdparty-rpc **无 MySQL**（验证码走 Redis、微信无状态）；模型走 nocache 套（含加密列不缓存）

### D10 服务接线
- **thirdparty-rpc 激活**（M14c catalog 同款）：app/thirdparty/rpc（业务域服务放 app/）；goctl.sh 加 thirdparty 分支；pkg/etcs/thirdparty-rpc.yaml 填充（ListenOn 127.0.0.1:9317、Etcd thirdparty.rpc、Redis、Sms{DryRun:true}+微信空配置、DevServer 9397）；**无网关路由**（老项目 integration 玩家网关仅 kuaidi100 回调接线，M15a 不落）
- **user-rpc**：svc 加 ThirdpartyRpc zrpc 客户端（Etcd thirdparty.rpc）；config 加 PhoneCrypto 段；3 新模型接入；+9 logic；重启
- **玩家网关**：3 路由 + 3 handler（user 包内），desc 类型手写（**禁跑 goctl gateway**）；网关只调 user-rpc，无新客户端

### D11 前端（web）
Login.vue 三 tab：账号登录（现有）/ 手机号登录（手机号 + 验证码 + 60s 倒计时重发 + 登录）/ 微信登录（code 输入 + scene 下拉，注明生产由小程序/App 取 code，web 调试用）。services/auth.ts 三 API；i18n zh/en 全量。

### D12 场景 29（test/scenario_29_phone_wechat_login.sh + run_all/menu 注册，25→26 场景）
1. 注册用户 A → test/tools/phonebind 工具调 SetUserPhone 绑 13800001111（+误用他人手机号 → 1012）
2. POST /auth/phone/code → 200 + expires_at_unix；Redis 读码（helper）
3. 错码登录 → 6001（验证码一次性消费，码错归第三方域）；对码登录 → token → GET /user/profile 带 token 200
4. 同码重放 → 6001（一次性消费）；同号 60s 内重发 → 6002；工具 clear 频控后恢复 0
5. 未绑定手机号（合法格式）发码+登录 → 1011（发码不查绑定）
6. 微信：本地 mock server（yaml APIBase 指向）→ 工具 LinkThirdPartyIdentity(user A, kind=1, openid=dev-openid-1) → POST /auth/wechat/login → token → profile 200；未绑定 openid → 1013
7. 清理现场

### D13 边界门
run_all 26 场景 ×2 全绿；go build ./...；go test 核心包（user phonecrypto / thirdparty core）；npm run build ×2（web 含 vue-tsc）；05-errors.md 登记；docs + memory 更新；提交（Co-Authored-By，排除清单，.smoke_risk/ 不提交）。

## 偏离记录汇总（m15a 全量）
1. **D3 会话**：响应 {userId,token} 而非 UserSessionTokens；不引入 refresh 家族/设备/指纹（★ 唯一可复议项）
2. PhoneService 无 caller-secret 头部鉴权（内部 RPC 无路由即内部）
3. SetUserPhone 无 GrantRegistration 副作用
4. 玩家网关路径走本仓 /gacha/api/v1 风格（M7 起既定）
5. 微信小程序全家/支付宝/腾讯广告/kuaidi100 → M15 后续（范围选择，非放弃）
6. ClaimPrize 维持自动发奖（已拍板，service-mapping.md §3）

## 实施顺序
1. 20_gacha_user_m15a.sql + 模型脚本接入 + 执行
2. errorx 1011-1014 / 6001-6006 + 05-errors.md 登记
3. thirdparty.proto 新建 + user.proto 扩展 → goctl.sh 加 thirdparty → 跑 `bash script/goctl.sh user` + `thirdparty`
4. thirdparty-rpc 激活：core 纯函数（归一化/随机码）+ sms store/sender + wechat gateway + 3 logic + yaml → 启动冒烟
5. user-rpc：phonecrypto 移植 + 单测 + 3 模型 + 9 logic + ThirdpartyRpc 客户端 → 重启
6. 玩家网关 3 路由 3 handler
7. 前端 Login.vue 三 tab + api + i18n
8. test/tools/phonebind + 场景 29 + run_all/menu
9. 边界门 + 文档 + 提交
