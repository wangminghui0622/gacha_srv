# M16 设计：玩家端 web/ → 微信小程序（uni-app，另立 mini/ 目录）

> 2026-10-26。用户拍板：服务端 M15d 已收官（6f9ebb4，run_all 29 场景），玩家端转微信小程序另立里程碑。
> 技术路线（用户已定）：**uni-app 新建 mini/ 目录，保留 web/ 作演示对照**（2026-10-26 走查后用户拍板删除 web/，git 历史可恢复）。
> 后端零改动：M15a 微信登录（jscode2session）与 M15d 微信小程序全家（token/手机号/二维码/urllink）已就绪，网关 API 客户端无关。

## 0. 为什么是小程序（已与用户确认）

- 老项目 server_v2 仓库内**没有 H5 客户端**——玩家端形态在仓库外，从 integration 服务的微信小程序全家（token 缓存/手机号解密/二维码/urllink）可断定其真实客户端是微信小程序；本仓 web/ 是自建 H5 演示。
- 盲盒/抽盒业务天然依赖微信生态（拼团抽/连击链等社交裂变玩法、免注册微信登录获客）。
- 29 个 e2e 场景全部 curl 直打网关，不依赖前端；web/ 无验收责任，替换无测试风险。

## 1. web/ 资产盘点（迁移依据，2026-10-26 实测）

| 资产 | 规模 | 迁移结论 |
|---|---|---|
| pages/ 19 个 .vue | 6749 行（MemoryMatch 959 最大，玩法页 550-960） | 模板全部重写（无 DOM/antd），script 逻辑 40-60% 可复用 |
| services/ 13 个 .ts | 1456 行 | **types.ts（832 行）100% 复用**；12 个域 service 复用（只把 api.ts 的 axios 换成 uni.request 适配器，接口形状不变） |
| stores/authStore + pinia | pinia 4.0.3 | pinia 兼容 uni-app，authStore 复用（js-cookie → uni storage 适配） |
| i18n（zh-CN/en-US） | 全量词条 | 词条文件复用；vue-i18n 用 **v9**（uni-app 官方兼容版本，web 的 v11 不迁移） |
| utils/coupon.ts、format.ts | 纯函数 | 100% 复用；storage.ts 换 uni.setStorageSync |
| router（19 路由+2 布局） | — | 换成 pages.json + uni.navigateTo；MainLayout → tabBar（首页/我的）+ 玩法宫格 |
| ant-design-vue 4 / axios / vue-router 5 / js-cookie | — | **全部弃用**：组件换 uni-ui + 玩法自绘；路由换 uni 导航；cookie 换 storage |

## 2. 决策

### D1 技术栈与目录
- uni-app CLI 模板（Vue 3 + Vite + TS：`npx degit dcloudio/uni-preset-vue#vite-ts mini`），双产物 `mp-weixin`（主）+ `h5`（兜底演示）。
- 新建 `mini/`；`web/` 保留不动（演示对照 + 组件语义参考）——**2026-10-26 已删除**（走查通过后拍板，对照使命完成；删除前核查零硬依赖：test/script/README/后端均不引用）。依赖：pinia（与 web 同版 4.x）、vue-i18n@9、uni-ui；**不装** axios/antd/vue-router/js-cookie。
- scripts：`dev:mp-weixin`、`build:mp-weixin`、`dev:h5`、`type-check`（vue-tsc）。

### D2 请求层（services 移植核心）
- `services/api.ts` 重写为 uni.request Promise 适配器，语义与 web 版逐条对齐：baseURL `/gacha/api/v1`、Authorization: Bearer 注入、`{code,msg,data}` 信封判定（code≠0 → uni.showToast(msg) + reject）、401 → 清 token 跳登录。
- 12 个域 service 文件**拷贝即用**（import 自同一适配器）；types.ts 原样拷贝。
- 小程序生产环境 request 合法域名需 https 白名单（微信后台配置）；开发工具可关校验。

### D3 登录闭环（小程序形态，后端零改动）
- **微信一键登录**：启动/未登录时 `uni.login()` 静默拿 code → `POST /auth/wechat/login`（M15a 已交付）→ 存 token。未绑定 openid（1013）→ 引导注册页。
- **注册/登录兜底**：注册（账号+密码，Register RPC）+ 手机号验证码登录（/auth/phone/*，M15a 已交付）+ 账号密码登录——同 web 三 tab 能力，但微信 tab 从"手输 code"变为真实 uni.login 按钮。
- 绑定语义照抄老项目：绑定无网关端点（M15a D2），新用户注册后**不自动绑微信**；后续可通过 test/tools 工具直连绑定（开发期），产品化绑定端点老项目没有、本仓也不加（要加再说，非 M16）。

### D4 页面映射与导航
- pages.json：tabBar 2 个（首页 = 宝箱列表 + 玩法宫格 + 滚动中奖 marquee；我的 = Profile）。
- 其余 17 页 navigateTo；登录页为启动页（token 有效直接进首页）。

| web 页面 | 小程序页面 | 组件替换要点 |
|---|---|---|
| Auth/Login.vue | pages/login | uni-ui 表单；微信 tab = uni.login 按钮 |
| Boxes/index.vue + RollingMarquee | pages/index（tab） | swiper 纵向滚动替代 marquee 动画 |
| Boxes/Detail.vue | pages/box-detail | 概率公示列表化 |
| Buy/index.vue（购买 phases） | pages/buy | 弹窗改 uni-popup；轮询揭晓动画自绘 |
| Coupons / Tasks / Records / Warehouse / Recharge / Profile | 同名页 | uni-list/uni-card/uni-forms |
| Campaigns/index + Wheel | pages/campaigns、pages/wheel | 转盘自绘（纯 CSS 扇形拼接，见偏离 9） |
| DigBox（格子棋盘） | pages/dig | 格子 grid 自绘（view 网格） |
| FullCar（座位网格） | pages/full-car | 座位网格自绘 |
| Gashapon（进度条+连抽） | pages/gashapon | 进度条 uni-progress 定制 |
| ComboChain / GroupBuy | 同名页 | 列表+弹窗 |
| MemoryMatch（翻牌棋盘，959 行最大页） | pages/memory-match | 卡牌翻转自绘（CSS 3d / 视图切换），逻辑层大量复用 |

### D5 支付
- 充值维持模拟通道（与 web 一致，Recharge RPC）；`wx.requestPayment` 真支付需商户号，**范围外**，留待后续里程碑（要接时改 payment-rpc 渠道层）。

### D6 合规与上线前置（外部依赖，需用户推进）
- 微信类目资质：盲盒/抽奖类目需对应资质材料，**开工前确认**（影响审核周期）。
- AppID/密钥：小程序 AppID 与 secret 配进 thirdparty-rpc yaml（APIBase 可配 mock 已有）；request 域名白名单、业务域名配置。

### D7 里程碑切分
- **M16a 核心闭环**：工程骨架 + 请求层/登录闭环 + 首页（选盒/详情/购买/揭晓/滚动）+ 我的（余额/充值/记录/仓库）→ 微信开发者工具人工走查核心链路。
- **M16b 全玩法**：六玩法（dig/full-car/gashapon/combo-chain/group-buy/memory-match）+ 活动（campaigns/wheel）+ 券 + 任务 → 全量走查 + 双语 + 收尾。

### D8 边界门（后端零改动，门禁与 M15 系列不同）
- M16a：`npm run build:mp-weixin` 成功 + `vue-tsc` 绿；微信开发者工具走查清单通过（登录→选盒→购买→揭晓→仓库→充值）；web/ `npm run build` 照旧绿（不受影响——web/ 删除后此门取消）。
- M16b：同上 + 全玩法走查；i18n 双语核对；docs + memory 更新；提交（Co-Authored-By，排除清单，.smoke_risk/ 不提交）。
- 不新增 e2e 场景（后端未变）；`go build ./...` 与 run_all 不强制（可跑一次确认无回归）。

## 3. 风险

1. **vue-i18n 版本**：web 用 v11，uni-app 官方兼容 v9——迁移时降级，词条文件不变。
2. **转盘实现**：canvas 2d 双端差异大且难自动验证，改用纯 CSS 扇形拼接（偏离 9）；需开发者工具目测走查一次。
3. **轮询揭晓体验**：小程序页面栈有限（10 层），购买→揭晓流程避免多级跳转，用单页内弹窗完成（web 的 polling 模式照搬）。
4. **分享裂变**（拼团/连击）：小程序 `onShareAppMessage` 配置，M16b 拼团页加分享按钮（增强项，非克隆范围——老项目客户端在仓库外，无参考，按产品常识做）。
5. **微信开发者工具**：Windows 环境可用；预览/真机调试需小程序管理员权限（需用户提供 AppID 或先用测试号）。

## 4. 实施顺序

1. 骨架：uni-preset-vue#vite-ts → mini/；deps 锁定（pinia/vue-i18n@9/uni-ui）；目录结构、pages.json（2 tab + 17 页注册）、请求适配器 + 错误提示统一。✅（M16a）
2. services 移植：types.ts 原样拷贝 + 12 个域 service 拷贝（api.ts 重写适配器）+ storage 适配 + authStore。✅（M16a）
3. M16a：登录页（微信一键/手机号/账号注册登录）→ 首页 + 详情 + 购买（SAGA 轮询）→ 我的（余额/充值/记录/仓库）→ 走查核心闭环。✅（b59d591）
4. M16b：六玩法逐页迁移（dig → full-car → gashapon → combo-chain → group-buy → memory-match）→ campaigns/wheel → coupons → tasks。✅（10 页全落地 + 玩法宫格接线）
5. 边界门 + 走查清单归档（`mini/CHECKLIST.md`）+ 文档 + memory 更新 + 提交。✅（构建/类型/web 四门全绿；走查待用户在微信开发者工具执行）

## 5. 偏离记录（预计）

1. 客户端技术栈为自研选择（老项目客户端在仓库外，无参考可克隆）——uni-app 而非未知的老客户端实现。
2. 微信一键登录按钮为小程序形态适配（老项目语义：Link 前置、不自动注册，语义不变，只是拿 code 的方式从"手输"变 uni.login）。
3. wx.requestPayment 真支付范围外（商户号依赖）。
4. 分享按钮为增强项（老项目无参考）。
5. **pinia 用 2.3.x 而非 web 的 4.x**：uni-app 模板锁定 vue ^3.4.21，pinia 4 要求 vue 3.5+；pinia 2 与 4 的 defineStore API 同形，authStore 代码零改动。web 侧不降级（不受影响）。
6. **pages.json 按里程碑增量注册**：设计写"2 tab + 17 页注册"，但注册未落地页面会导致构建失败——M16a 注册 8 页（login/index/mine/box-detail/buy/records/warehouse/recharge），M16b 补齐其余 10 页（六玩法+campaigns/wheel+coupons+tasks），共 18 页（web 的 19 个 .vue 含 MainLayout，小程序由 tabBar 替代不单计页）。
7. **tabBar 文案与导航标题为静态中文**：微信 tabBar 不支持运行时 i18n；页面标题可用 uni.setNavigationBarTitle 运行时覆盖（后续按需）。en-US 词条已全量就位。
8. **marquee 用 swiper 纵向循环**替代 web 的 CSS 动画（D4 原定方案）；idempotencyKey 生成器本地 v4 兜底（mp-weixin 无 crypto.randomUUID）。
9. **转盘 canvas → 纯 CSS 扇形拼接**：WXSS 无 conic-gradient，canvas 2d 双端差异大；改为「零尺寸三角形 + 旋转」拼接扇形——border-bottom 三角形朝下（conic 约定 180°），rotate((i+0.5)·seg−180°) 落扇区中心，r=200rpx、底边 r·tan(θ/2)、transform-origin 50% 0%（0×0 元素 left:50% 定位 border box 左缘，apex 在圆心右侧 w，margin-left:−w 归位）；槽位几何与落点数学（align=(360−center)%360）与 web 完全一致；槽位 <3 时隐藏转盘只留槽位说明（tan 发散，种子 ≥4 槽不受影响）。
10. **玩法页样式基建**：a-progress/a-tag/a-modal/CSS grid 小程序无对应 → App.vue 全局工具类（.progress-track/.progress-fill、.tag-chip 色变体、.modal-mask/.modal-card）+ 页面内 flex-wrap 均匀边距网格（width: calc(X% − 12rpx); margin: 6rpx + 容器 −6rpx）与自绘 spinner/shake keyframes。
11. **记录时间格式**：web 用 `toLocaleString()`（浏览器 locale 输出）；mini 全局统一 `formatUnix`（YYYY-MM-DD HH:mm:ss，0 值显示 '-'）——展示级差异，属 mini 端统一替换。
12. **disabled 语义双保险**：web 按钮 `:disabled` 真正阻断点击；mini 的 `.disabled` 类名仅降透明 → 行为层在 onXxxClick/selectXxx 入口镜像 web 的 disabled 条件早退（dig 零格/满车已出发/扭蛋轮询期重复下单等边界与 web 行为一致）。
