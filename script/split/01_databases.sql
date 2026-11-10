-- ============================================================
-- 01_databases.sql - 创建全部数据库（唯一权威建库点）
-- gacha_identity    账号域（用户/手机号/第三方身份 + dtm_barrier）          → identity-rpc 9311
-- gacha_money       资金域（钱包/资产/支付单/退款单/支付路由 + dtm_barrier） → money-rpc 9312
-- gacha_draw        抽奖域（宝箱/奖池条目/抽赏记录/吉仓 + dtm_barrier）      → draw-rpc 9313
-- gacha_base        基础域（资格策略 + 风控规则/案例/事件 + dtm_barrier）    → base-rpc 9314
-- gacha_marketing   营销域（报价锁/券/忠诚度 + 活动/兑换码/签到/转盘/资格 + dtm_barrier） → marketing-rpc 9315
-- gacha_play        玩法域（挖宝/满车/扭蛋/连击链/拼团/记忆翻牌 + dtm_barrier） → play-rpc 9316
-- gacha_task        任务域（任务进度/事件/签到/通行证/会员 + dtm_barrier）   → task-rpc 9317
-- gacha_catalog     商品目录域（SPU/SKU/修订/发布 + dtm_barrier）           → catalog-rpc 9318
-- gacha_thirdparty  第三方域（provider 指标 + 腾讯广告点击）                → thirdparty-rpc 9319
-- gacha_ops         运营域（控制台/权限/任务计划/看板 + dtm_barrier）        → ops-rpc 9320
-- gacha_dtm         DTM Server 存储库（kv/trans_branch_op/trans_global）
-- 后续编号脚本只 USE 不建库（幂等；从零初始化与重复执行均安全）。
-- 执行方式：mysql -u root < script/split/01_databases.sql
-- ============================================================

SET NAMES utf8mb4;

CREATE DATABASE IF NOT EXISTS `gacha_identity`    DEFAULT CHARACTER SET utf8mb4;
CREATE DATABASE IF NOT EXISTS `gacha_money`       DEFAULT CHARACTER SET utf8mb4;
CREATE DATABASE IF NOT EXISTS `gacha_draw`        DEFAULT CHARACTER SET utf8mb4;
CREATE DATABASE IF NOT EXISTS `gacha_base`        DEFAULT CHARACTER SET utf8mb4;
CREATE DATABASE IF NOT EXISTS `gacha_marketing`   DEFAULT CHARACTER SET utf8mb4;
CREATE DATABASE IF NOT EXISTS `gacha_play`        DEFAULT CHARACTER SET utf8mb4;
CREATE DATABASE IF NOT EXISTS `gacha_task`        DEFAULT CHARACTER SET utf8mb4;
CREATE DATABASE IF NOT EXISTS `gacha_catalog`     DEFAULT CHARACTER SET utf8mb4;
CREATE DATABASE IF NOT EXISTS `gacha_thirdparty`  DEFAULT CHARACTER SET utf8mb4;
CREATE DATABASE IF NOT EXISTS `gacha_ops`         DEFAULT CHARACTER SET utf8mb4;
CREATE DATABASE IF NOT EXISTS `gacha_dtm`         DEFAULT CHARACTER SET utf8mb4;

-- dtm_barrier 屏障库：dtmcli 子事务屏障默认表名 dtm_barrier.barrier（schema.table）。
-- SAGA 分支（settle/grant/cancelPayment 等）经 BarrierFromGrpc + CallWithDB 落屏障行
-- 防悬挂/重复；各业务库内的 `dtm_barrier` 表（02-12 各建）服务于 GORM 侧屏障模型，
-- 与本全局屏障库并存。缺此库时屏障 insert 报 Unknown database → 分支 Internal →
-- DTM 无限重试，属必建项。
CREATE DATABASE IF NOT EXISTS `dtm_barrier` DEFAULT CHARACTER SET utf8mb4;
CREATE TABLE IF NOT EXISTS `dtm_barrier`.`barrier` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `trans_type` varchar(45) DEFAULT '' COMMENT '事务类型：saga/xa/tcc/msg',
  `gid` varchar(128) DEFAULT '' COMMENT '全局事务ID',
  `branch_id` varchar(128) DEFAULT '' COMMENT '分支事务ID：01/02...',
  `op` varchar(45) DEFAULT '' COMMENT '操作类型：action=正向操作, compensate=补偿操作',
  `barrier_id` varchar(45) DEFAULT '' COMMENT '屏障序号：同一分支内多次barrier调用的序号，从01开始递增',
  `reason` varchar(45) DEFAULT '' COMMENT '描述信息，便于调试',
  `create_time` datetime DEFAULT CURRENT_TIMESTAMP COMMENT '记录创建时间',
  `update_time` datetime DEFAULT CURRENT_TIMESTAMP COMMENT '记录更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_barrier` (`gid`,`branch_id`,`op`,`barrier_id`),
  KEY `idx_create_time` (`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC COMMENT='DTM子事务屏障表（dtmcli 默认）';
