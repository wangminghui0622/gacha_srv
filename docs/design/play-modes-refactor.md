# 玩法域重构：按玩法拆分（无版本）

## 目标

- 每个玩法拥有自己的配置表、奖池表、代码包和后台表单。
- 不共享 `play_pool_versions` / `play_pool_prizes`，不搞配置版本。
- 只保留池状态和修改历史：
  - `1 DRAFT` 草稿
  - `2 LIVE` 上线
  - `3 OFFLINE` 下架（可重新上架）
  - `4 VOID` 作废（永久）
- 线上出问题：旧池下架/作废，新建新池，不复用旧池 ID。
- 冗余可以接受，模式之间不互相影响。

## 表结构

公共索引，只放身份、状态、列表字段，不掺玩法配置：

```text
play_pool_catalog
  id, mode, name, status, price_fen, ip_id, tags_json, weight,
  admin_json, void_reason, voided_at, voided_by, created_at, updated_at
```

公共历史与幂等：

```text
play_pool_history
  id, pool_id, mode, action, before_json, after_json,
  reason, operator_id, created_at

play_pool_requests
  idempotency_key, pool_id, request_hash, created_at
```

每个玩法自己的当前配置和奖池，**所有玩法表结构完全一致**（模式差异只体现在 `config_json` 和代码校验/投影）：

```text
play_memorymatch_pools / play_memorymatch_prizes
play_gashapon_pools    / play_gashapon_prizes
play_fullcar_pools     / play_fullcar_prizes
play_combochain_pools  / play_combochain_prizes
play_dig_pools         / play_dig_prizes
play_groupbuy_pools    / play_groupbuy_prizes
```

统一字段：

```text
play_<mode>_pools
  pool_id, price_fen, config_json, created_at, updated_at

play_<mode>_prizes
  id, pool_id, prize_index, name, prize_type, sku_id,
  cash_fen, points_amount, coupon_template_no,
  weight, quantity, is_prize, image_url, cost_fen,
  special_kind, target_value, parent_index, total_pair_count, extra_json,
  created_at, updated_at
```

模式只解释自己关心的字段：

```text
memorymatch : total_pair_count
gashapon    : special_kind(LAST/MVP/LUCKY), target_value(Lucky顺序)
fullcar     : special_kind(SETTLEMENT), target_value(结算档位)
combochain  : special_kind(GLOBAL/...), target_value(槽位/阈值), extra_json
dig         : target_value(格子索引)
groupbuy    : special_kind(LUCKY/LUCKY_CHILD), target_value(父位序)
```

所有玩法共用同一套生命周期、历史、幂等和审核逻辑。

运行表保持现状，生效时覆盖：

```text
memory_match_configs / memory_match_entries
gashapon_configs / gashapon_entries
full_car_configs / full_car_entries
combo_chain_configs / combo_chain_global_slots ...
dig_box_configs / dig_box_cells
group_buy_configs / group_buy_entries
```

## 状态流转

```text
DRAFT --生效--> LIVE
LIVE --下架--> OFFLINE --重新生效--> LIVE
LIVE/OFFLINE --作废--> VOID
VOID 不可编辑、不可重新上架
```

编辑规则：

```text
DRAFT / OFFLINE 可编辑
LIVE 不可直接编辑，先下架
VOID 只读
```

## 代码结构

```text
app/play/internal/playpool/          公共：索引、状态、历史、生效编排
app/play/internal/modes/registry.go  玩法注册表
app/play/internal/modes/memorymatch/
app/play/internal/modes/gashapon/
app/play/internal/modes/fullcar/
app/play/internal/modes/combochain/
app/play/internal/modes/dig/
app/play/internal/modes/groupbuy/
```

每个玩法实现统一接口：

```go
type Mode interface {
    Code() string
    Validate(pool *Pool) error
    Save(tx *gorm.DB, pool *Pool) error
    Apply(tx *gorm.DB, pool *Pool) error
    Void(tx *gorm.DB, pool *Pool) error
}
```

新增玩法：

1. 新建 `modes/<mode>` 包。
2. 新建 `play_<mode>_pools` / `play_<mode>_prizes`。
3. 注册到 registry。
4. 新增后台 `<mode>/CreateForm.vue`。

## 接口

```text
CreatePlayPool
UpdatePlayPool
ApplyPlayPool
SetPlayPoolStatus
GetPlayPool
ListPlayPools
```

移除：

```text
CreatePlayPoolDraft
PublishPlayPool
GetPlayPoolVersion
ListPoolVersions
按版本审批
```

## 迁移顺序

1. 新建公共表和 memorymatch 两张表。
2. 实现对对碰的创建、编辑、生效、下架、作废、上新。
3. 后台表单拆出 `MemoryMatchCreateForm.vue`。
4. 验收后按同一模板迁移 gashapon。
5. 再迁移 fullcar / combochain / dig / groupbuy。
6. 删除旧 `play_pool_versions` / `play_pool_prizes`，只保留 `play_pool_catalog`。
7. 删除旧版本接口和前端版本逻辑。
