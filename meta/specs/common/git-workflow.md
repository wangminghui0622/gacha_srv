# Git 工作流规范

## 分支

1. 长期分支：`main`（生产）；开发分支按需从 `main` 切出。
2. 分支命名（kebab-case）：
   - `feature/<简述>` — 新功能，如 `feature/user-pix-bind`
   - `fix/<简述>` — 缺陷修复，如 `fix/spin-deadlock`
   - `hotfix/<简述>` — 生产紧急修复
   - `refactor/<简述>` — 重构（行为不变）
3. 禁止直接向 `main` 提交；合并一律走 PR。

## 提交信息

4. 格式：`<type>(<scope>): <中文摘要>`，type 取 `feat` / `fix` / `refactor` / `docs` / `test` / `chore` / `perf` / `style`；scope 为受影响服务或模块（如 `user`、`gateway`、`frontend`）。
5. 摘要用祈使句、中文、50 字以内；正文说明"为什么"而非"改了什么"（代码 diff 已说明后者）。
6. 一次提交只做一件事；不混入无关改动（如顺手的格式化）。

## PR 要求

7. PR 标题与提交信息同格式；描述写清：背景 → 改动 → 影响范围 → 验证方式。
8. 合并前必须通过：lint、单元测试、相关 e2e 场景（后端）。
9. 生成代码（goctl 产物）的变更与手写代码分开提交，便于 review 跳过。
10. review 对照 `review-checklist.md` 逐项检查。

## 禁止事项

11. 禁止提交：密钥/令牌/证书（.env、*.pem）、构建产物（bin/、dist/、node_modules/）、日志、本地临时文件（.test_token、.scenario_*.log）。
12. `.gitignore` 至少覆盖：`.env`、`bin/`、`dist/`、`node_modules/`、`logs/`、`.idea/`、`*.log`、测试临时文件。
13. 禁止 `git push -f` 到共享分支；禁止在提交中夹带大文件（>1MB 二进制走独立存储）。
