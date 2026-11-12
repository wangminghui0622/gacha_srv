# 玩法框架（play-rpc + gateway 购买流）

> 2026-11 接口化：36 个 typed SAGA 分支 RPC + 5 个 List 收敛为 **7 个通用 RPC**；
> 读/玩法/管理接口保留 20 个，共 **27 个 RPC**。goctl 可安全重生成。

## 1. 分层

```
pkg/proto/play.proto
├── enum PlayMode                 # 6 种玩法
├── PlayLock/Release/Settle/SettleRevert/Grant/GrantRevert
├── PlayListRecords
└── 20 个读/玩法/管理 RPC（Get*/List*/记忆翻牌玩法/Assign/CheckPrizesGranted）

app/play/
├── internal/logic/               # goctl 生成：27 个桩 + playDispatch.go
│   ├── playLockLogic.go          #   通用分支桩 → doPlayLock
│   ├── ...
│   └── getGashaponLogic.go       #   读接口桩 → gameimpl.NewGetGashaponLogic
├── internal/gameimpl/            # 手写实现（按 SAGA 阶段拆文件）
│   ├── gashapon_{lock,release,settle,grant,query,ops}.go
│   ├── fullcar_{lock,release,settle,grant,query,ops}.go
│   ├── combochain_{...}.go
│   ├── dig_{...}.go
│   ├── groupbuy_{...}.go
│   ├── memorymatch_{lock,settle,grant,session,round,champion,ops}.go
│   └── shared.go
└── internal/playkit/             # BarrierTx / LoadQuote / LockQuote / GrantAll
```

`logic` 只做转发，`gameimpl` 是唯一实现层。goctl 重生成只会覆盖 `logic` 桩和 pb/client/server，不会碰 `gameimpl`。

## 2. 7 个通用 SAGA 分支

| RPC | payload 内容 | 补偿 |
|---|---|---|
| `PlayLock` | typed `LockXxxReq` | `PlayRelease`（**复用 PlayLockReq**，DTM 透传正向 payload） |
| `PlaySettle` | typed `SettleXxxReq` | `PlaySettleRevert`（同一 `PlaySettleReq`） |
| `PlayGrant` | typed `GrantXxxReq` | `PlayGrantRevert`（同一 `PlayGrantReq`） |
| `PlayListRecords` | mode + requestId/userId/page | — |

固定顺序：`lock → authorize → settle → grant → capture`。  
补偿由 DTM 反向执行：`captureRevert → grantRevert → settleRevert → cancel → release`。

## 3. 网关购买流

```
app/gateway/internal/saga/
├── mode.go        # PurchaseMode[Req] + RunBuy
├── branches.go    # PlayBranches：固定 5 分支 + 通用 Play* 路径
└── wallet.go      # 02 冻结 / 05 捕获

app/gateway/internal/playapi/
└── playapi.go     # PlayListRecords 封装（payload → typed 响应）
```

每个玩法实现一个 `purchaseMode`：

```go
type purchaseMode struct{ log logx.Logger }

func (m purchaseMode) BuildBranches(...) ([]saga.Branch, error) {
    lockBytes, _ := proto.Marshal(&playClient.LockGashaponReq{...})
    return saga.PlayBranches(..., saga.PlayBranchSet{
        LockPath:    c.PlayRpcDirect + saga.PathPlayLock,
        ReleasePath: c.PlayRpcDirect + saga.PathPlayRelease,
        LockPayload: &play.PlayLockReq{Mode: ..., Payload: lockBytes, ...},
        ...
    }), nil
}
```

## 4. 新增玩法的标准步骤

1. `play.proto`：`PlayMode` 加枚举值；通用分支不需要加 RPC。
2. `gameimpl`：新增 `<mode>_lock.go / _settle.go / _grant.go / _query.go / _ops.go`。
3. `logic/playDispatch.go`：在 `doPlayLock/Release/Settle/Grant/ListRecords` 加 case，反序列化 payload → 调 `gameimpl`。
4. 网关：新增 `<mode>/purchaseMode.go`，实现 `saga.PurchaseMode`。
5. 读/玩法接口：按需在 `play.proto` 加 typed RPC（读接口仍保留类型安全）。
6. 测试：core 单测 + `saga.PlayBranches` 顺序测试 + 场景冒烟。

## 5. goctl 约束

- `bash script/goctl.sh play` 会重新生成 `logic` 27 个桩 + pb/client/server；
- **不要再把实现写回 `logic`**，否则重生成会冲突；
- 实现只写 `gameimpl`，桩只做转发；
- 通用分支桩改的是 `playDispatch.go`（手写文件，goctl 不会覆盖）。

## 6. 验证

```bash
go build ./...
go test ./...
go vet ./app/play/... ./app/gateway/...
```

部署冒烟：扭蛋 / 满车 / 连击链 / 挖宝 / 拼团 / 记忆翻牌各买一次；
并验证一次失败补偿（`PlaySettleRevert` + `PlayRelease`）。
