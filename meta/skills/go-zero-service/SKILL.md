---
name: go-zero-service
description: Scaffolds or extends go-zero microservices in this repo — new RPC service, new gateway endpoint, or new proto RPC — following the project's backend standard (layout, config, error codes, deployment scripts). Use when the user asks to create a new service, add an API endpoint, define a proto RPC, or mentions goctl, .api, or .proto files.
---

# go-zero-service：按规范脚手架后端服务/端点

目标：产出符合本项目后端规范的代码骨架。规范细节见 `../../../docs/standards/backend/`（01 架构、02 布局、03 api-proto、05 错误）。

## 第一步：明确任务类型

| 任务 | 改哪里 |
|------|--------|
| 网关新增 HTTP 端点 | `pkg/api/desc/<域>.api`（或新建 desc 文件并在 `gateway.api` 注册 import） |
| 新增 RPC 方法 | `pkg/proto/<svc>.proto` |
| 新增整个 RPC 服务 | 新建 proto + 按第二步全流程 |
| 纯逻辑改动 | 不重新生成，直接改 logic/model |

先向用户确认域（user/game/payment/agent/vip/ops…）与端点清单，再动手。

## 第二步：编写接口定义

`.api` 端点按此模板（模块横幅 → 类型块 → @server → service）：

```go
// ==================== 用户模块 ====================

type (
    GetProfileReq {
        UserId int64 `json:"userId" validate:"required"`
    }
    UserInfo {
        Id       int64  `json:"id"`
        Username string `json:"username"`
        Balance  string `json:"balance"`   // 金额一律 string
    }
)

@server (
    prefix:     /<项目名>/api/v1
    group:      user
    middleware: JwtAuth
    tags:   user
)
service gateway-api {
    @doc "获取用户信息"
    @handler GetProfile
    get /user/profile returns (UserInfo)
}
```

`.proto` 按此模板（包名=服务名、camelCase 字段、string 金额、SAGA 成对 Revert）：

```proto
syntax = "proto3";
package user;
option go_package = "./user";

service User {
    rpc GetProfile(GetProfileReq) returns (GetProfileResp);
    // SAGA 参与的操作成对定义：
    rpc DeductBalance(DeductBalanceReq) returns (DeductBalanceResp);
    rpc DeductBalanceRevert(DeductBalanceReq) returns (DeductBalanceResp);
}
```

必守规则：字段 camelCase；金额 `string`；分页 `page`/`pageSize` + `list`/`total`；公开接口不挂 JwtAuth。

## 第三步：生成与清理

1. 运行 `./script/goctl.sh <all|gateway|<svc>>`（新增服务先在该脚本中登记服务名）。
2. 生成后立即：删除 `app/<svc>/etc`，确认 `main.go` 配置路径为 `../../pkg/etcs/<svc>-rpc.yaml`。
3. 在 `pkg/etcs/` 补/改配置文件：端口按架构规范顺延（6007 起）、Etcd key、MySQL DSN、JwtAuth；密钥只写占位符。
4. 禁止手改 `types.go`、`routes.go`、`*.pb.go`、`*Client/`；handler 与 logic 可以改。

## 第四步：实现业务

- 网关 handler 薄：调 logic，`httpx.OkJsonCtx` / `httpx.ErrorCtx` 写响应。
- RPC server 纯委托：`l := logic.NewXxxLogic(ctx, s.svcCtx); return l.Xxx(in)`。
- Logic 用 `XxxLogic` 结构体模式；错误用 `errorx`（未登记错误码先去 `../../../docs/standards/backend/05-errors.md` 的段表登记）。
- Model 用 DAO 模式，`ctx` 第一参数；余额更新原子 `gorm.Expr` + 守卫。
- 跨服务资金操作走 DTM SAGA（见架构规范第 11-14 条），业务库需有 `dtm_barrier` 表。

## 第五步：收尾（缺一不可）

1. `pkg/etcs/<svc>.yaml` 配置齐全（含中文注释的可调参数）。
2. `deploy/swarm/*.sh` 部署脚本的服务列表同步（原 deploy/linux、deploy/windows 已废弃）。
3. `README.md` 端口表登记新服务。
4. e2e 场景：新增 `test/scenario_NN_<主题>.sh`（头部注释写流程/前置/生产代码路径），`run_all.sh` 登记。
5. `go test ./...` 通过；涉及接口的跑 `bash test/run_all.sh smoke`。
