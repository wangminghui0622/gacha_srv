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

## 7. 统一赏池/奖品模型

所有玩法共用 `gacha_play.play_pool_*` 四张表，发布时投影到各玩法现有运行表，
玩法运行态代码不改：

| 表 | 用途 |
| --- | --- |
| `play_pool_heads` | 赏池头（mode/name/next_version，ID 从 1000000 起） |
| `play_pool_versions` | 版本（status 1 DRAFT / 2 PUBLISHED / 3 RETIRED，price/config_json/config_hash） |
| `play_pool_prizes` | 奖品（prize_type、sku_id、special_kind、target_value、资产字段、parent_index、extra_json） |
| `play_pool_drafts` | 创建草稿幂等（idempotency_key + request_hash） |

RPC（`play.proto`）：`CreatePlayPoolDraft / PublishPlayPool / GetPlayPoolVersion / ListPlayPools`。
后台路由：`/gacha/play-pools*`，权限 `gacha.pool:{read,create,publish}`，
对应 `pkg/adminop/operations.go` 的 `console.playpool.*`。

### 7.1 config_json 约定（camelCase）

| mode | config_json |
| --- | --- |
| gashapon | `{}`（特别赏在 prizes 里） |
| fullcar | `{"seatCount":4}` |
| combochain | `{"ticketsPerCar":2,"totalCars":10,"tierTriggerProbabilityBps":300}` |
| dig | `{"cellCount":9}` |
| groupbuy | `{"targetBetCount":3,"perBetAwardLimit":0}` |
| memorymatch | `{"roundsPerPurchase":3,"maxPairCount":3,...,"facePresets":[...],"backPresets":[...]}` |

### 7.2 special_kind / target_value

| mode | special_kind | target_value |
| --- | --- | --- |
| gashapon | 空=普通赏；LAST / MVP / LUCKY | LUCKY 的档位 |
| fullcar | 空=普通条目；SETTLEMENT=整车结算奖 | 结算 tier（1 幸运 / 2 普通） |
| combochain | GLOBAL / COMBO_TIER / EMPTY_STREAK / WIN_STREAK | GLOBAL 的槽位 |
| dig | 空 | 格子编号 0..cellCount-1 |
| groupbuy | 空=普通条目；LUCKY；LUCKY_CHILD | LUCKY_CHILD 指向的幸运奖 prizeIndex |
| memorymatch | 空 | 碰数（0=谢谢参与） |

拼团普通子奖用 `has_parent=true + parent_index=父奖 prizeIndex`；
连击链共享奖的 `tierWeight/concentrateThreshold/streakThreshold` 放 `extra_json`。

### 7.3 投影

`PublishPlayPool` 在同一事务内按 mode 写运行表：
gashapon→`gashapon_configs/entries/state`、fullcar→`full_car_*`、
combochain→`combo_chain_configs`、dig→`dig_box_configs/cells`、
groupbuy→`group_buy_configs/entries`、memorymatch→`memory_match_configs/entries`。
已发布版本重复发布幂等；发布新版本会重置对应玩法的运行态（`*_state` /
`gashapon_lucky_assignments`），历史 `*_records` 保留，管理端应在无进行中购买时发布。

实物 SKU：`play_pool_prizes` / `PrizeItem` 已预留 `prize_type=2 + sku_id`。
当前 **挖宝箱 / 记忆翻牌 / 拼团 / 连击链** 已打通：发布投影到运行表 `prize_kind=5 + sku_id`，
发奖时 `playkit.KindGoods` 调 `money.GrantPlayAsset`，按 `asset_no="pa_"+grantNo`
幂等写入 `gacha_money.assets`（`source=play`、`pool_id`、`sku_id`、`state=1 AVAILABLE`）。
其余玩法发布时仍会拒绝实物/箱子奖，待逐步接入。
SKU 存在性/履约方式校验（catalog `GetSKU`）尚未接入，当前按管理员填写的 skuId 落库。

> 旧库升级：`play_pool_prizes` 需补两列
> `parent_index INT NOT NULL DEFAULT -1`、`extra_json JSON NULL`（新装库由 split SQL 直接创建）。
