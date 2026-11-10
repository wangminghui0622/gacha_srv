# 代码评审清单

> PR review 时逐项核对；未通过项要求修改后重新评审。

## 通用（前后端）

- [ ] 接口响应走统一信封 `{code, msg, data}`，`code == 0` 成功
- [ ] 错误码已按域分段登记，无裸 `errors.New` 返回给调用方 / 无未映射的 HTTP 状态码
- [ ] 金额用字符串两位小数，无 float 传输
- [ ] 无密钥/令牌硬编码或进入日志、提交
- [ ] 分页字段为 `page`/`pageSize`，列表响应含 `total`
- [ ] 注释为中文，关键业务逻辑有步骤注释
- [ ] 无调试残留（console.log、fmt.Println、临时文件）
- [ ] 生成文件未手改；接口变更已重新生成
- [ ] PR 描述完整（背景/改动/影响/验证）

## 后端（Go）

- [ ] handler/server 层薄，业务在 logic；logic 用 `%w` 包装错误并带中文上下文
- [ ] model 层用 DAO 模式，`ctx` 第一参数；余额更新用原子 `gorm.Expr` + 守卫条件
- [ ] 查询不存在返回 `(nil, nil)`，未误判为错误
- [ ] 涉及跨服务资金操作的接口走 DTM SAGA，Action/Revert 成对且可幂等
- [ ] 新增 SQL 有中文 COMMENT、索引命名 `uk_`/`idx_`、金额 `decimal(12,2)`
- [ ] 配置项已在 `pkg/etcs/<svc>.yaml` 登记，无散落硬编码
- [ ] 单测覆盖纯逻辑；e2e 场景覆盖主流程与失败路径（余额不足、补偿）

## 前端（Vue 3）

- [ ] SFC 用 `<script setup lang="ts">`，文件 PascalCase 与组件同名，script 首行中文 JSDoc
- [ ] props 用类型声明式 `defineProps<XxxProps>()`，接口无 `I` 前缀；类型导入用 `import type`
- [ ] 状态用域 store + `storeToRefs` 取用；加载/错误态在组件 `ref` 内
- [ ] 请求只从 `services/` 发起；函数带类型与中文 JSDoc；错误统一转为中文提示
- [ ] UI 文案全部走 `t()`，新增 key 已同步 locale 文件
- [ ] 路由懒加载 + 守卫用 `beforeEach` + `meta.public`；`base` 与部署路径一致
- [ ] 主题色用 `tokens.scss` 令牌，Antdv 主题走 `ConfigProvider`，无散落任意色值
- [ ] `npm run lint` 与 `npm run build`（vue-tsc 类型检查）通过
