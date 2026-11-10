# 数据库规范

## 库与连接

1. **一服务一库**，库名 `Gacha_<域>`（如 `Gacha_user`）；服务只连自己的库，禁止跨库 join、跨库事务。
2. 字符集统一 `utf8mb4` / `utf8mb4_unicode_ci`。
3. DTM 事务状态库独立（`Gacha_dtm`）；每个业务库内建 `dtm_barrier` 表（纯表名，无前缀）。

## 表设计

4. 表名蛇形**复数**（`users`、`user_balance_logs`、`game_bets`），按域加前缀区分（`user_*`、`game_*`）。
5. 每张表、每个字段必须有中文 `COMMENT`；状态字段注释注明取值（`COMMENT '1正常 0禁用'`）。
6. 字段约定：
   - 主键 `id bigint AUTO_INCREMENT`
   - 时间 `created_at` / `updated_at`，`DEFAULT CURRENT_TIMESTAMP` / `ON UPDATE CURRENT_TIMESTAMP`
   - 金额 `decimal(12,2)`（注单/抽赏记录类可 `decimal(12,4)`），禁止 float/double
   - 状态用 `tinyint` + 注释，禁止魔法字符串
7. 索引命名：唯一 `uk_<列>`（`uk_username`），普通 `idx_<列>`（`idx_user_id`），组合唯一按角色命名（`uk_barrier (gid,branch_id,op,barrier_id)`）。

## 脚本管理（script/split/）

8. 终态快照脚本：`01_databases.sql`（唯一权威建库点，11 个库 + dtm_barrier 一次建齐）+ 每库一个文件（`02_gacha_identity.sql` … `12_gacha_dtm.sql`：该库全部表的最终结构 + 全部种子），按序号逐个执行；库文件只 `USE` 不建库。
9. 建表语句幂等：`CREATE TABLE IF NOT EXISTS`，重复执行安全；`AUTO_INCREMENT=N` 只在有业务语义时保留（如 `draw_pool_heads` 起始 1000000 让赏池 id 空间避开种子宝箱 id 1-3），mysqldump 导出的无意义当前计数值一律剥离。
10. 种子数据内联在对应库文件中。幂等写法按表选型：**显式 id** 或**有业务唯一键**的表走 `INSERT ... ON DUPLICATE KEY UPDATE`；**无业务唯一键**的表（如 `*_entries`）走 `DELETE` 种子范围后 `INSERT`，禁止用 `ON DUPLICATE KEY UPDATE id = id` 假幂等（主键自增不会冲突，重跑种子翻倍）。测试专用数据放 `test/` 下的独立 SQL，不进初始化。
11. 不引入迁移框架：schema 变更直接改对应库文件（新表/列/种子在 02-12 对应文件内追加并保持幂等），01 无需改动；破坏性变更提供 `cleanup_*.sql` 并注明执行顺序。
12. 脚本内禁止写真实生产密钥、测试密码以外的明文凭据。

## 模型映射

13. GORM 结构体与表一一对应（见编码规范第 6 条）；`TableName()` 必须与 SQL 脚本一致。
14. 逻辑删除用 `deleted_at`（GORM 软删），不用物理删除日志/资金类数据；余额变动必须写流水表。
