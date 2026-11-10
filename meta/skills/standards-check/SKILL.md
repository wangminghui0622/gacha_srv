---
name: standards-check
description: Reviews code changes against this project's engineering standards — backend (Go/go-zero), frontend (Vue 3/TS), and common conventions — and reports violations with locations and fixes. Use when reviewing a diff, before committing, or when the user asks to check code against the standard or 规范检查.
---

# standards-check：按工程规范检查代码变更

目标：对给定变更（diff、PR、路径）逐项核对规范，输出可执行的整改清单。判断依据是 `../../../docs/standards/` 下的规范与 `../../rules/` 中的规则；**凭规范说话，每条结论都引用规范条目**。

## 执行流程

1. 确定变更范围：`git diff <目标>` 或用户指定路径；无参数时检查未提交变更。
2. 按变更内容加载对应规范：
   - Go/`.api`/`.proto`/`.sql` → `../../../docs/standards/backend/`（01-08）
   - vue/ts/scss/css → `../../../docs/standards/frontend/`（01-07）
   - 涉及接口契约、错误码、金额 → 加读 `../../../docs/standards/common/api-conventions.md`
3. 逐项核对重点规则，输出报告。

## 检查清单（高频违规点）

**通用**
- 响应信封 `{code, msg, data}`；`code == 0` 成功；HTTP 恒 200
- 错误码已按域段登记；无裸 `errors.New` 透出 / 无未映射错误
- 金额 string 两位小数；时间格式 `"2006-01-02 15:04:05"`
- 分页 `page`/`pageSize`；列表含 `total`
- 密钥无硬编码、无日志泄漏；注释中文；无调试残留

**后端**
- 配置文件在 `pkg/etcs/`；生成文件未手改
- handler/server 薄、logic 分层正确、`%w` 包装错误
- model DAO 模式、`ctx` 第一参数、余额原子更新带守卫
- 跨服务资金操作走 DTM SAGA，Action/Revert 成对
- SQL：表名蛇形复数、中文 COMMENT、`uk_`/`idx_`、`decimal(12,2)`
- deploy 双平台脚本、README 端口表同步

**前端**
- SFC 用 `<script setup lang="ts">`；PascalCase 文件名=组件名；script 首行中文 JSDoc
- props 用类型声明式 `defineProps<XxxProps>()`、无 I 前缀；`import type`
- 请求只在 `services/`；store 用 `storeToRefs` 取用；loading/error 在组件 `ref`
- 文案全走 `t()` 且 locale 已同步；色值用 `tokens.scss` 令牌，Antdv 主题走 `ConfigProvider`
- router 懒加载 + `beforeEach` 守卫 + `meta.public`；`base`/`base`/nginx 三者一致
- `npm run lint`、`npm run build` 通过

## 报告格式

```markdown
## 规范检查报告

范围：<diff/PR/路径>
结论：通过 N 项 / 问题 M 项

### 问题（按严重程度排序）

1. [必须改] <file>:<line> — 违反「<规范名>第 N 条」：<一句话说明>
   建议：<具体改法>

### 提示（可不改）
- ...
```

严重程度定义：必须改 = 会导致线上故障/资金风险/安全风险/接口契约破坏；建议改 = 风格与一致性。

## 边界

- 只报告**有规范依据**的问题，不凭个人偏好提意见。
- 生成代码（goctl 产物）直接跳过，不列入问题。
- 检查完成后若用户要求，再按建议逐项修复（修复后重跑 lint/测试）。
