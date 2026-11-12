# 玩法框架（play-rpc + gateway 购买流）

> 本文件记录 2026-11 重构后的玩法结构。目标：新增玩法只加「一个包 + 几张表 + 一个购买流」，
> 不再复制粘贴 6~8 个 logic 文件。

## 1. 分层

```
app/play/internal/
├── playkit/                 # 玩法公共能力（所有玩法共用）
│   ├── barrier.go           #   BarrierTx：DTM 屏障 + raw DB + txutil
│   ├── quote.go             #   LoadQuote / LockQuote（统一错误映射）
│   └── prize.go             #   GrantAll / GrantOne / MarkGranted
└── logic/
    ├── gashaponService.go   # 扭蛋：Lock/Release/Settle/Revert/Grant/Get/List/Assign
    ├── fullCarService.go    # 满车
    ├── comboChainService.go # 连击链
    ├── digService.go        # 挖宝
    ├── groupBuyService.go   # 拼团
    ├── memoryMatchService.go# 记忆翻牌
    └── shared.go            # 公共常量 / CheckPrizesGranted
```

每个 `*Service.go` 内仍然是原来的 `NewXxxLogic` 构造器与 RPC 方法，`playServer.go` 不用改。

## 2. playkit API

| API | 用途 |
|---|---|
| `playkit.BarrierTx(ctx, svc, op, fn)` | 统一 `BarrierFromGrpc → rawDB → CallWithDB → txutil.WithTx`；返回 `ran` 对应原 `businessRan` |
| `playkit.LoadQuote(ctx, svc, quoteToken)` | 读报价，返回 `UserID/GrossFen/BoxID`；业务错误统一映射 Aborted |
| `playkit.LockQuote(ctx, svc, requestID, quoteToken)` | 锁报价；`newLocked=false` 表示已锁（幂等重放） |
| `playkit.GrantAll(ctx, svc, requestID, prefix, []Prize)` | 逐奖幂等发放；`grant_no = prefix_requestID#idx` |
| `playkit.MarkGranted(ctx, svc, requestID, userID)` | 写 `prize_grant_marks`（04 退款前置检查依据） |

奖品统一形状：

```go
playkit.Prize{
    UserID, Kind, CashFen, Points, CouponTemplateNo,
}
// Kind: 1 现金 / 2 积分 / 3 券
```

## 3. 网关购买流

```
app/gateway/internal/saga/
├── builder.go        # DTM 组装器（Add/Submit/Recheck）
├── buyflow.go        # BuyFlow：userId→gid→幂等→单价→报价→预检→分支→Submit
├── wallet.go         # 02 冻结 / 05 捕获（统一资金分支）
└── branches.go       # PlayBranches：固定 5 分支顺序 + 单元测试
```

`BuyFlow` 的玩法差异只剩 hook：

```go
flow := &saga.BuyFlow{
    Ctx, Log, Svc, RequestID, Scene, ConfigID, FailMsg,
    LoadUnitFen: func(ctx, userID) (int64, error) { ... },  // 服务端单价
    Prepare:     func(ctx, userID) error { ... },           // 可选：挖宝格子
    AmountFen:   func(unitFen int64) int64 { ... },         // 默认 unitFen
    Idempotent:  func(ctx, gid, userID) (bool, error) { ... }, // 同 gid 已有记录
    BuildBranches: func(...) ([]saga.Branch, error) { ... },
    Recheck:     func(ctx, gid, userID) error { ... },      // Submit 失败回查
}
```

`BuildBranches` 统一用：

```go
saga.PlayBranches(moneyDirect, gid, userID, boxID, quote, saga.PlayBranchSet{
    LockPath, ReleasePath, LockPayload,
    SettlePath, SettleRevertPath, SettlePayload,
    GrantPath, GrantRevertPath, GrantPayload,
})
```

固定顺序：`lock → authorize → settle → grant → capture`。  
补偿由 DTM 反向执行：`captureRevert → grantRevert → settleRevert → cancel → release`。

## 4. 新增一个玩法的标准步骤

1. **SQL**：`xxx_configs / xxx_entries / xxx_state / xxx_records`（每玩法独立表）。
2. **model**：`pkg/model/gacha_play` + `pkg/model_nocache/gacha_play`。
3. **纯函数**：`app/play/internal/core/xxx.go`（可单测、可回放，不放 DB/RPC）。
4. **服务逻辑**：`app/play/internal/logic/xxxService.go`，提供：
   - `NewLockXxxLogic / NewReleaseXxxLogic / NewSettleXxxLogic / NewSettleXxxRevertLogic / NewGrantXxxPrizesLogic / NewGrantXxxPrizesRevertLogic`
   - `NewGetXxxLogic / NewListXxxRecordsLogic`
   - 内部统一用 `playkit.BarrierTx / LoadQuote / LockQuote / GrantAll / MarkGranted`
5. **RPC**：`pkg/proto/play.proto` 加消息 + `rpc`；生成后 `playServer.go` 加转发（构造器名保持一致）。
6. **网关购买**：`app/gateway/internal/logic/<mode>/xxxBuyLogic.go`，套 `saga.BuyFlow` + `PlayBranches`。
7. **后台配置**：admin-gateway 加配置 CRUD；复用赏池发布管线。
8. **测试**：core 单测 + `saga.PlayBranches` 顺序测试 + 场景脚本冒烟。

## 5. proto 策略

- **现在不改**：本次重构只动实现，RPC 契约不变。
- 新增玩法时二选一：
  - 继续 per-mode RPC（8~19 个）；
  - 增量加通用 RPC：`PlayMode` 枚举 + `PlayLock/PlayRelease/PlaySettle/...` + `payload`，旧 RPC 保留。
- 不建议直接删除旧 per-mode RPC（网关/客户端会断）。

## 6. 验证

```bash
go build ./...
go test ./...
go vet ./app/play/... ./app/gateway/...
```

部署后冒烟：扭蛋 / 满车 / 连击链 / 挖宝 / 拼团各买一次，确认
「扣款 → 开奖 → 发奖 → 记录 → 补偿」链路正常。
