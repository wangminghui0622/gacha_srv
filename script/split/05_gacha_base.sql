-- ============================================================
-- 06_gacha_base.sql - 资格与风控域（eligibility/risk/决策日志）（gacha_base 库终态快照）
-- 表结构 = 本机 MySQL 终态导出（mysqldump --no-data，2026-10-26）；
-- 种子 = 历史里程碑编号脚本按序归并（幂等，可重复执行）。
-- 执行方式：mysql -u root < script/split/06_gacha_base.sql
-- ============================================================

SET NAMES utf8mb4;

USE `gacha_base`;

SET FOREIGN_KEY_CHECKS = 0;

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

CREATE TABLE IF NOT EXISTS `eligibility_decision_logs` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `decision_no` varchar(64) NOT NULL COMMENT '决策单号："d_"+requestId 确定性派生',
  `scene` varchar(32) NOT NULL COMMENT '场景',
  `policy_no` varchar(64) NOT NULL DEFAULT '' COMMENT '评估所用策略单号',
  `user_id` bigint NOT NULL DEFAULT '0' COMMENT '用户ID',
  `box_id` bigint NOT NULL DEFAULT '0' COMMENT '宝箱ID',
  `result` tinyint NOT NULL COMMENT '结果：1 通过 0 拒绝',
  `matched_rule_id` bigint NOT NULL DEFAULT '0' COMMENT '拒绝时命中的规则ID',
  `reason` varchar(512) NOT NULL DEFAULT '' COMMENT '拒绝原因（中文，可透出）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_decision_no` (`decision_no`),
  KEY `idx_user` (`user_id`),
  KEY `idx_scene_result` (`scene`,`result`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='资格决策日志表（只追加不撤销）';

CREATE TABLE IF NOT EXISTS `eligibility_policies` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '政策ID（自增）',
  `policy_code` varchar(64) NOT NULL COMMENT '政策编码（uk，^[a-z][a-z0-9_.-]{1,63}$）',
  `name` varchar(128) NOT NULL COMMENT '政策名称',
  `current_version` bigint unsigned NOT NULL DEFAULT '0' COMMENT '生效版本（0=未发布）',
  `latest_version` bigint unsigned NOT NULL DEFAULT '0' COMMENT '最新版本号',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_policy_code` (`policy_code`),
  KEY `idx_current_scene` (`current_version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='条件政策头表';

CREATE TABLE IF NOT EXISTS `eligibility_policy_rules` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `policy_id` bigint unsigned NOT NULL COMMENT '政策ID',
  `policy_version` bigint unsigned NOT NULL COMMENT '版本号',
  `rule_id` varchar(64) NOT NULL COMMENT '规则ID（版本内唯一）',
  `expression` text NOT NULL COMMENT 'CEL 表达式（facts.xxx）',
  `failure_code` varchar(64) NOT NULL COMMENT '失败原因码（拒绝时透出）',
  `sort_order` int NOT NULL DEFAULT '0' COMMENT '规则顺序',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_policy_version_rule` (`policy_id`,`policy_version`,`rule_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='条件政策规则表（CEL）';

CREATE TABLE IF NOT EXISTS `eligibility_policy_versions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `policy_id` bigint unsigned NOT NULL COMMENT '政策ID',
  `version` bigint unsigned NOT NULL COMMENT '版本号（1 起递增）',
  `scene` tinyint unsigned NOT NULL COMMENT '场景：1 DISCOVER 2 VIEW 3 DRAW 4 BOOK',
  `status` tinyint unsigned NOT NULL COMMENT '状态：1 DRAFT 2 PUBLISHED 3 RETIRED',
  `combination` tinyint unsigned NOT NULL COMMENT '规则组合：1 ALL 2 ANY',
  `required_facts` json NOT NULL COMMENT '必需事实键数组（评估缺失即拒绝）',
  `config_hash` binary(32) NOT NULL COMMENT '内容哈希（发布 CAS 依据）',
  `published_at` datetime DEFAULT NULL COMMENT '发布时间',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_policy_version` (`policy_id`,`version`),
  UNIQUE KEY `uk_policy_hash` (`policy_id`,`config_hash`),
  KEY `idx_scene_status` (`scene`,`status`,`policy_id`,`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='条件政策版本表';

CREATE TABLE IF NOT EXISTS `risk_case_events` (
  `event_id` varchar(64) NOT NULL COMMENT '事件ID（=console operation_id，uk 幂等）',
  `case_id` varchar(64) NOT NULL COMMENT '案件单号',
  `event_type` tinyint unsigned NOT NULL COMMENT '事件类型：1 OPENED 2 CLAIMED 3 RESOLVED 4 DISMISSED',
  `from_status` tinyint unsigned NOT NULL COMMENT '迁移前状态',
  `to_status` tinyint unsigned NOT NULL COMMENT '迁移后状态',
  `actor_id` bigint unsigned NOT NULL COMMENT '操作员ID',
  `resolution_code` varchar(64) NOT NULL DEFAULT '' COMMENT '处置码（RESOLVED/DISMISSED 事件）',
  `occurred_at` datetime NOT NULL COMMENT '发生时间',
  PRIMARY KEY (`event_id`),
  KEY `idx_case` (`case_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='风控案件迁移事件表（幂等重放）';

CREATE TABLE IF NOT EXISTS `risk_cases` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `case_no` varchar(64) NOT NULL COMMENT '案例单号："rk_"+requestId 确定性派生',
  `decision_id` varchar(64) NOT NULL DEFAULT '' COMMENT '关联决策ID（本仓风控无决策流水，置空）',
  `rule_no` varchar(64) NOT NULL COMMENT '命中规则单号',
  `scene` varchar(32) NOT NULL DEFAULT 'DRAW' COMMENT '场景',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `box_id` bigint NOT NULL DEFAULT '0' COMMENT '宝箱ID',
  `amount_fen` bigint NOT NULL DEFAULT '0' COMMENT '订单总额（分）',
  `risk_score` int NOT NULL DEFAULT '0' COMMENT '风险分（severity 映射：LOW 30/MEDIUM 60/HIGH 90）',
  `action` tinyint unsigned NOT NULL DEFAULT '3' COMMENT '决策动作：3 CHALLENGE（本仓案件全为挑战）',
  `status` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '状态：1 OPEN 2 IN_REVIEW 3 RESOLVED 4 DISMISSED',
  `claimant` varchar(64) NOT NULL DEFAULT '' COMMENT '认领人',
  `assigned_actor_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '认领操作员ID（IN_REVIEW 起非 0）',
  `resolution_code` varchar(64) NOT NULL DEFAULT '' COMMENT '处置码（RESOLVED/DISMISSED 时非空）',
  `resolution` varchar(512) NOT NULL DEFAULT '' COMMENT '处置备注',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_case_no` (`case_no`),
  KEY `idx_status` (`status`),
  KEY `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='风控案例表（只增不改，人工处置走 Claim/Resolve）';

CREATE TABLE IF NOT EXISTS `risk_rules` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `rule_no` varchar(64) NOT NULL COMMENT '规则单号',
  `scene` varchar(32) NOT NULL DEFAULT 'DRAW' COMMENT '适用场景',
  `fact_key` varchar(32) NOT NULL COMMENT '事实键：amount_fen/user_id/box_id',
  `op` varchar(16) NOT NULL COMMENT '比较符：eq/neq/gte/lte/in',
  `expected_value` varchar(64) NOT NULL COMMENT '期望值（in 用逗号分隔）',
  `severity` varchar(16) NOT NULL DEFAULT 'MEDIUM' COMMENT '严重级别：LOW/MEDIUM/HIGH',
  `enabled` tinyint NOT NULL DEFAULT '1' COMMENT '是否启用：1 是 0 否',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_rule_no` (`rule_no`),
  KEY `idx_scene_enabled` (`scene`,`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='风控规则表';

SET FOREIGN_KEY_CHECKS = 1;

-- ============================================================
-- 种子数据（幂等；按里程碑序归并）
-- ============================================================

-- 种子：四个场景各一条已发布空规则策略（无条件放行）；空规则列表即放行
INSERT INTO `eligibility_policies` (`id`, `policy_code`, `name`, `current_version`, `latest_version`)
VALUES
    (1, 'P-DISCOVER-1', '发现场景默认放行', 1, 1),
    (2, 'P-VIEW-1',     '查看场景默认放行', 1, 1),
    (3, 'P-DRAW-1',     '抽奖场景默认放行', 1, 1),
    (4, 'P-BOOK-1',     '预订场景默认放行', 1, 1)
ON DUPLICATE KEY UPDATE `name` = VALUES(`name`), `current_version` = VALUES(`current_version`),
    `latest_version` = VALUES(`latest_version`);

-- 种子版本：config_hash 为占位 32 字节（发布 CAS 只比对同版本行存储值；console 新建版本由 Go 端
-- 按 proto Deterministic marshal 计算真实哈希，种子永不经 CreateRevision 修订，无需与 Go 算法一致）
INSERT INTO `eligibility_policy_versions`
    (`policy_id`, `version`, `scene`, `status`, `combination`, `required_facts`, `config_hash`, `published_at`)
SELECT 1, 1, 1, 2, 1, JSON_ARRAY(),
       UNHEX(SHA2(CONCAT('{"combination":"RULE_COMBINATION_ALL","requiredFacts":[],"rules":[],"scene":"ELIGIBILITY_SCENE_DISCOVER"}'), 256)), NOW()
WHERE NOT EXISTS (SELECT 1 FROM `eligibility_policy_versions` WHERE `policy_id` = 1 AND `version` = 1);

INSERT INTO `eligibility_policy_versions`
    (`policy_id`, `version`, `scene`, `status`, `combination`, `required_facts`, `config_hash`, `published_at`)
SELECT 2, 1, 2, 2, 1, JSON_ARRAY(),
       UNHEX(SHA2(CONCAT('{"combination":"RULE_COMBINATION_ALL","requiredFacts":[],"rules":[],"scene":"ELIGIBILITY_SCENE_VIEW"}'), 256)), NOW()
WHERE NOT EXISTS (SELECT 1 FROM `eligibility_policy_versions` WHERE `policy_id` = 2 AND `version` = 1);

INSERT INTO `eligibility_policy_versions`
    (`policy_id`, `version`, `scene`, `status`, `combination`, `required_facts`, `config_hash`, `published_at`)
SELECT 3, 1, 3, 2, 1, JSON_ARRAY(),
       UNHEX(SHA2(CONCAT('{"combination":"RULE_COMBINATION_ALL","requiredFacts":[],"rules":[],"scene":"ELIGIBILITY_SCENE_DRAW"}'), 256)), NOW()
WHERE NOT EXISTS (SELECT 1 FROM `eligibility_policy_versions` WHERE `policy_id` = 3 AND `version` = 1);

INSERT INTO `eligibility_policy_versions`
    (`policy_id`, `version`, `scene`, `status`, `combination`, `required_facts`, `config_hash`, `published_at`)
SELECT 4, 1, 4, 2, 1, JSON_ARRAY(),
       UNHEX(SHA2(CONCAT('{"combination":"RULE_COMBINATION_ALL","requiredFacts":[],"rules":[],"scene":"ELIGIBILITY_SCENE_BOOK"}'), 256)), NOW()
WHERE NOT EXISTS (SELECT 1 FROM `eligibility_policy_versions` WHERE `policy_id` = 4 AND `version` = 1);

