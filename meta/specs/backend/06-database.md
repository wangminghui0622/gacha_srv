# 数据库规范

## 库与连接

1. **一服务一库**，库名 `<项目>_<域>`（如 `<项目>_user`）；服务只连自己的库，禁止跨库 join、跨库事务。
2. 字符集统一 `utf8mb4` / `utf8mb4_unicode_ci`。
3. DTM 事务状态库独立（`<项目>_dtm`）；每个业务库内建 `dtm_barrier` 表（纯表名，无前缀）。

## 表设计

4. 表名蛇形**复数**（`users`、`user_balance_logs`、`game_bets`），按域加前缀区分（`user_*`、`game_*`）。
5. 每张表、每个字段必须有中文 `COMMENT`；状态字段注释注明取值（`COMMENT '1正常 0禁用'`）。
6. 字段约定：
   - 主键 `id bigint AUTO_INCREMENT`
   - 时间 `created_at` / `updated_at`，`DEFAULT CURRENT_TIMESTAMP` / `ON UPDATE CURRENT_TIMESTAMP`
   - 金额 `decimal(12,2)`（注单类可 `decimal(12,4)`），禁止 float/double
   - 状态用 `tinyint` + 注释，禁止魔法字符串
7. 索引命名：唯一 `uk_<列>`（`uk_username`），普通 `idx_<列>`（`idx_user_id`），组合唯一按角色命名（`uk_barrier (gid,branch_id,op,barrier_id)`）。

## 脚本管理（script/split/）

8. 编号顺序脚本：`01_create_databases.sql`、`02_user.sql`、`03_game.sql`……由 `init_all.sql` 逐个 `SOURCE`。
9. 建表语句幂等：`CREATE TABLE IF NOT EXISTS`，重复执行安全。
10. 种子数据（等级配置、字典表）内联 `INSERT INTO` 在对应编号脚本中；测试专用数据放 `test/` 下的独立 SQL，不进初始化。
11. 不引入迁移框架：schema 变更新增编号脚本，`init_all.sql` 同步登记；破坏性变更提供 `cleanup_*.sql` 并注明执行顺序。
12. 脚本内禁止写真实生产密钥、测试密码以外的明文凭据。

## 模型映射

13. GORM 结构体与表一一对应（见编码规范第 6 条）；`TableName()` 必须与 SQL 脚本一致。
14. 逻辑删除用 `deleted_at`（GORM 软删），不用物理删除日志/资金类数据；余额变动必须写流水表。
