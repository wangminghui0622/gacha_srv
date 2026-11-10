# 测试规范

## 单元测试

1. 纯逻辑（utils、errorx、uuid、格式化）必须配单测；标准库 `testing`，测试函数 `TestXxx`，断言用 `t.Fatal`/`t.Fatalf`（中文上下文信息）。
2. 测试文件与被测文件同目录（`xxx_test.go`）；不引入第三方断言库（保持零额外依赖）。
3. 覆盖重点：错误码映射、边界值（余额恰好为零、金额精度）、幂等性。

## 端到端测试（test/）

4. 场景脚本命名 `scenario_NN_<主题>.sh`（`scenario_02_spin_saga.sh`），编号顺序 = 依赖顺序。
5. 每个场景头部注释必须写明：**流程、前置条件、对应的生产代码路径**。
6. 公共工具集中在 `test/common.sh`：
   - `BASE_URL="${BASE_URL:-http://localhost:8888/<项目>/api/v1}"`（可覆盖）
   - `do_get/do_post/do_put <url> [token]`、`extract_field <json> <dot-path>`、`is_ok`（断言信封 `code == 0`）、彩色 `print_step/print_ok/print_error`
7. 断言必须打在统一信封上（`code`、`data.xxx`），不允许只检查 HTTP 200。
8. 编排入口 `test/run_all.sh`：`smoke` 参数只跑冒烟子集（健康检查 + 只读），默认跑全部；`menu.sh` 提供交互选择。
9. 测试用户用时间戳后缀避免冲突（`tester_${TS}`）；令牌经临时文件（`.test_token`）在场景间传递；临时产物必须 gitignore。
10. 失败路径必测：余额不足、重复请求、补偿触发；SAGA 场景验证最终一致（余额、流水、注单三方一致）。
11. 手动 RPC 调试指南（grpcurl 用法）写成 `<服务>/TEST.md` 随服务归档。

## 验收口径

12. 发布前必须通过：`go test ./...` + `bash test/run_all.sh`（全场景）。
