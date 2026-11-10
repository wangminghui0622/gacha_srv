-- ============================================================
-- 03_gacha_draw.sql - 抽赏域（宝箱/奖池/抽赏记录/吉仓/奖池发布）（gacha_draw 库终态快照）
-- 表结构 = 本机 MySQL 终态导出（mysqldump --no-data，2026-10-26）；
-- 种子 = 历史里程碑编号脚本按序归并（幂等，可重复执行）。
-- 执行方式：mysql -u root < script/split/03_gacha_draw.sql
-- ============================================================

SET NAMES utf8mb4;

USE `gacha_draw`;

SET FOREIGN_KEY_CHECKS = 0;

CREATE TABLE IF NOT EXISTS `draw_boxes` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `box_name` varchar(64) NOT NULL COMMENT '宝箱名称',
  `price` bigint NOT NULL COMMENT '单抽价格（单位：分）',
  `description` varchar(255) NOT NULL DEFAULT '' COMMENT '宝箱描述',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1=上架 0=下架',
  `rolling_rules_json` text COMMENT '滚动中奖规则 JSON（NULL=非滚动盒）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='宝箱表';

CREATE TABLE IF NOT EXISTS `draw_pool_draft_requests` (
  `idempotency_key` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL COMMENT '幂等键（= HTTP Idempotency-Key 头）',
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池 id',
  `version` bigint unsigned NOT NULL COMMENT '创建的版本号',
  `request_hash` binary(32) NOT NULL COMMENT '请求载荷哈希（同 key 不同载荷=冲突）',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (`idempotency_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='赏池草稿创建幂等表';

CREATE TABLE IF NOT EXISTS `draw_pool_entries` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `box_id` bigint NOT NULL COMMENT '所属宝箱ID（关联 draw_boxes.id）',
  `name` varchar(64) NOT NULL COMMENT '条目名称（奖品名或"谢谢参与"）',
  `weight` int NOT NULL COMMENT '权重（整数），公示概率 = weight / Σweight',
  `quantity` int NOT NULL COMMENT '总库存：-1=不限量（如谢谢参与）',
  `remaining` int NOT NULL COMMENT '剩余库存：-1=不限量；0=售罄不参与抽取',
  `is_prize` tinyint NOT NULL DEFAULT '1' COMMENT '是否奖品：1=中奖入吉仓 0=谢谢参与只记录',
  `display_value_fen` bigint NOT NULL DEFAULT '0' COMMENT '展示价（分），滚动中奖等级判定用：0=不参与高档判定',
  `rolling_limited` tinyint NOT NULL DEFAULT '0' COMMENT '滚动限量标记：1=限量稀有，feed 等级 LIMITED、可被剔除规则剔除',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_box_id` (`box_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='奖池条目表';

CREATE TABLE IF NOT EXISTS `draw_pool_heads` (
  `pool_id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '赏池 id（=宝箱 id；新建池自动分配）',
  `next_version` bigint unsigned NOT NULL DEFAULT '1' COMMENT '下一个版本号（创建草稿时分配）',
  PRIMARY KEY (`pool_id`)
) ENGINE=InnoDB AUTO_INCREMENT=1000000 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='赏池版本头表（AUTO_INCREMENT 起始 1000000：赏池 id 空间避开种子宝箱 id 1-3）';

CREATE TABLE IF NOT EXISTS `draw_pool_publication_approvals` (
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池 id',
  `version` bigint unsigned NOT NULL COMMENT '版本号',
  `config_hash` binary(32) NOT NULL COMMENT '审批对象配置哈希',
  `requester_id` bigint unsigned NOT NULL COMMENT '申请人（控制台主体 id）',
  `approver_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '审批人（0=待审批）',
  `status` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '状态：1=PENDING 待审批 2=APPROVED 已批准',
  `requested_at` datetime(3) NOT NULL COMMENT '提请时间',
  `approved_at` datetime(3) DEFAULT NULL COMMENT '批准时间（未批准为 NULL）',
  PRIMARY KEY (`pool_id`,`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='赏池发布审批表（单审批，审批人≠申请人）';

CREATE TABLE IF NOT EXISTS `draw_pool_versions` (
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池 id（=宝箱 id）',
  `version` bigint unsigned NOT NULL COMMENT '版本号（从 1 递增）',
  `status` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '状态：1=DRAFT 草稿 2=PUBLISHED 已发布',
  `config_hash` binary(32) NOT NULL COMMENT '配置哈希 SHA256（确定性序列化草稿）',
  `config_json` json NOT NULL COMMENT '完整草稿配置（protojson）',
  `published_at` datetime(3) DEFAULT NULL COMMENT '发布时间（未发布为 NULL）',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (`pool_id`,`version`),
  UNIQUE KEY `uk_pool_config` (`pool_id`,`config_hash`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='赏池版本表';

CREATE TABLE IF NOT EXISTS `draw_records` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `request_id` varchar(64) NOT NULL COMMENT '请求ID（= DTM 全局事务 gid），唯一键幂等',
  `user_id` bigint NOT NULL COMMENT '抽奖用户ID',
  `box_id` bigint NOT NULL COMMENT '宝箱ID',
  `entry_name` varchar(64) NOT NULL COMMENT '开奖条目名称',
  `entry_id` bigint NOT NULL DEFAULT '0' COMMENT '中奖条目ID（=draw_pool_entries.id；0=旧数据回退按 name）',
  `is_prize` tinyint NOT NULL COMMENT '是否中奖：1=是 0=谢谢参与',
  `price` bigint NOT NULL COMMENT '本次抽盒价格（分）',
  `seed_hex` varchar(64) NOT NULL DEFAULT '' COMMENT '引擎种子（hex，64 字符；确定性开奖，可复算审计）',
  `audit_hash` varchar(64) NOT NULL DEFAULT '' COMMENT '审计哈希（hex，64 字符；SHA256 覆盖种子+条目+结果，公平性可验证）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 正常 2 已撤销',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '开奖时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_request_id` (`request_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='抽赏记录表';

CREATE TABLE IF NOT EXISTS `draw_warehouse_items` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `user_id` bigint NOT NULL COMMENT '所属用户ID',
  `record_id` bigint NOT NULL COMMENT '关联抽赏记录ID（draw_records.id）',
  `box_id` bigint NOT NULL COMMENT '来源宝箱ID',
  `item_name` varchar(64) NOT NULL COMMENT '奖品名称',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1=在仓 0=已消耗（v1 暂不使用）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '入仓时间',
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='吉仓表';

CREATE TABLE IF NOT EXISTS `dtm_barrier` (
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC COMMENT='DTM子事务屏障表';

SET FOREIGN_KEY_CHECKS = 1;

-- ============================================================
-- 种子数据（幂等；按里程碑序归并）
-- ============================================================

-- 种子数据：3 个主题宝箱（幂等插入）
INSERT INTO `draw_boxes` (`id`, `box_name`, `price`, `description`, `status`) VALUES
    (1, '新手体验盒', 100,   '入门之选，人人有份', 1),
    (2, '夏日限定盒', 1000,  '清凉一夏，泳装来袭', 1),
    (3, '传说宝箱',   10000, '传说品质，欧皇专属', 1)
ON DUPLICATE KEY UPDATE `id` = `id`;

-- 奖池条目种子（本表无业务唯一键：DELETE 种子范围后插入，保证幂等）；
-- weight 合计 = 公示概率分母
DELETE FROM `draw_pool_entries` WHERE `box_id` IN (1, 2, 3);
INSERT INTO `draw_pool_entries` (`box_id`, `name`, `weight`, `quantity`, `remaining`, `is_prize`) VALUES
    -- 新手体验盒：50% 谢谢参与
    (1, '谢谢参与',     50, -1, -1, 0),
    (1, '金币×100',     30, 10000, 10000, 1),
    (1, '限定头像框',   15, 100, 100, 1),
    (1, '限定皮肤',     5,  10, 10, 1),
    -- 夏日限定盒：40% 谢谢参与
    (2, '谢谢参与',     40, -1, -1, 0),
    (2, '金币×500',     44, 5000, 5000, 1),
    (2, '夏日皮肤·泳装', 10, 50, 50, 1),
    (2, '夏日皮肤·冲浪', 5,  20, 20, 1),
    (2, '典藏皮肤·海神', 1,  5,  5,  1),
    -- 传说宝箱：30% 谢谢参与
    (3, '谢谢参与',     30, -1, -1, 0),
    (3, '金币×5000',    64, 1000, 1000, 1),
    (3, '典藏时装',     3,  15, 15, 1),
    (3, '传说武器',     2,  10, 10, 1),
    (3, '传说坐骑',     1,  5,  5,  1);

-- 种子：box1 为滚动盒（阈值 3000 分；剔除规则默认关闭，场景 18 会临时开启验证）
UPDATE `draw_boxes`
SET `rolling_rules_json` = '{"high_value_threshold_fen":3000,"single_draw_exclusion_enabled":false,"single_draw_excluded_percent":0,"user_excluded_percent":0}'
WHERE `id` = 1;

-- 种子：box1 条目展示价与限量标记（幂等 UPDATE）
UPDATE `draw_pool_entries` SET `display_value_fen` = 0,     `rolling_limited` = 0 WHERE `box_id` = 1 AND `name` = '谢谢参与';

UPDATE `draw_pool_entries` SET `display_value_fen` = 100,   `rolling_limited` = 0 WHERE `box_id` = 1 AND `name` = '金币×100';

UPDATE `draw_pool_entries` SET `display_value_fen` = 500,   `rolling_limited` = 0 WHERE `box_id` = 1 AND `name` = '限定头像框';

UPDATE `draw_pool_entries` SET `display_value_fen` = 5000,  `rolling_limited` = 1 WHERE `box_id` = 1 AND `name` = '限定皮肤';
-- ============================================================
-- 增量升级（幂等，可重复执行）：已有库补列
-- draw_records 增加 seed_hex / audit_hash（确定性开奖引擎，可复算审计）。
-- ============================================================

DROP PROCEDURE IF EXISTS `__upgrade_draw_records_fairness`;

DELIMITER $$
CREATE PROCEDURE `__upgrade_draw_records_fairness`()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'gacha_draw' AND TABLE_NAME = 'draw_records' AND COLUMN_NAME = 'seed_hex'
  ) THEN
    ALTER TABLE `draw_records`
      ADD COLUMN `seed_hex` varchar(64) NOT NULL DEFAULT '' COMMENT '引擎种子（hex，64 字符；确定性开奖，可复算审计）' AFTER `price`;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'gacha_draw' AND TABLE_NAME = 'draw_records' AND COLUMN_NAME = 'audit_hash'
  ) THEN
    ALTER TABLE `draw_records`
      ADD COLUMN `audit_hash` varchar(64) NOT NULL DEFAULT '' COMMENT '审计哈希（hex，64 字符；SHA256 覆盖种子+条目+结果，公平性可验证）' AFTER `seed_hex`;
  END IF;
END$$
DELIMITER ;

CALL `__upgrade_draw_records_fairness`();
DROP PROCEDURE `__upgrade_draw_records_fairness`;