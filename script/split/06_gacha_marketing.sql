-- ============================================================
-- 06_gacha_marketing.sql - 营销域（报价锁/券模板/用户券/忠诚度 + 活动/兑换码/签到/转盘/资格 + dtm_barrier）（gacha_marketing 库终态快照）
-- 表结构 = 本机 MySQL 终态导出；种子 = 历史里程碑脚本按序归并（幂等，可重复执行）。
-- 本文件由 script/split_sql.go 从旧仓 script/split/ 拆库生成。
-- 执行方式：mysql -u root < script/split/06_gacha_marketing.sql
-- ============================================================

SET NAMES utf8mb4;

USE `gacha_marketing`;

SET FOREIGN_KEY_CHECKS = 0;


CREATE TABLE IF NOT EXISTS `campaign_box_entitlements` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `entitlement_no` varchar(96) NOT NULL COMMENT '资格单号（"ce_"+grant_no，uk）',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `box_id` bigint NOT NULL COMMENT '宝箱ID（draw 箱）',
  `campaign_id` bigint NOT NULL COMMENT '来源活动ID',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 AVAILABLE 2 LOCKED 3 USED',
  `locked_by` varchar(64) NOT NULL DEFAULT '' COMMENT '锁定方 quote_token（LOCKED 时非空）',
  `granted_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '资格发放时间',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_entitlement_no` (`entitlement_no`),
  KEY `idx_user_status` (`user_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='活动宝箱资格表';


CREATE TABLE IF NOT EXISTS `campaign_participations` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `idem_key` varchar(96) NOT NULL COMMENT '幂等键（客户端 Idempotency-Key，uk）',
  `campaign_id` bigint NOT NULL COMMENT '活动ID',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `kind` tinyint NOT NULL COMMENT '活动类型（冗余，便于查询）',
  `result_json` text NOT NULL COMMENT '参与结果 JSON（重放原样返回）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_idem_key` (`idem_key`),
  KEY `idx_user_created` (`user_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='活动参与幂等表';


CREATE TABLE IF NOT EXISTS `campaign_prize_grants` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `grant_no` varchar(96) NOT NULL COMMENT '发放单号（"pg_"+idem_key+"#"+序号，uk）',
  `participation_id` bigint NOT NULL COMMENT '参与记录ID',
  `idem_key` varchar(96) NOT NULL COMMENT '参与幂等键（冗余，便于重放补发）',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `prize_kind` tinyint NOT NULL COMMENT '奖品类型：1 CASH 2 POINTS 3 COUPON 4 GACHA_BOX',
  `amount_fen` bigint NOT NULL DEFAULT '0' COMMENT '现金金额（分，prize_kind=1）',
  `points_amount` bigint NOT NULL DEFAULT '0' COMMENT '积分数（prize_kind=2）',
  `coupon_template_no` varchar(64) NOT NULL DEFAULT '' COMMENT '券模板单号（prize_kind=3）',
  `box_id` bigint NOT NULL DEFAULT '0' COMMENT '宝箱ID（prize_kind=4，发放活动宝箱资格）',
  `status` tinyint NOT NULL DEFAULT '0' COMMENT '状态：0 PENDING 1 GRANTED',
  `granted_at` datetime DEFAULT NULL COMMENT '发放完成时间',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_grant_no` (`grant_no`),
  KEY `idx_participation` (`participation_id`),
  KEY `idx_user_status` (`user_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='活动奖品发放流水表';


CREATE TABLE IF NOT EXISTS `campaign_redemption_codes` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `campaign_id` bigint NOT NULL COMMENT '活动ID',
  `code` varchar(64) NOT NULL COMMENT '兑换码（uk）',
  `max_redemptions` int NOT NULL DEFAULT '0' COMMENT '总可兑换次数',
  `redeemed_count` int NOT NULL DEFAULT '0' COMMENT '已兑换次数',
  `per_user_limit` int NOT NULL DEFAULT '1' COMMENT '每用户可兑换次数',
  `prize_config` text NOT NULL COMMENT '奖品数组 JSON：[{"kind":1,"cashFen":100},{"kind":3,"couponTemplateNo":"T-FIX-500"}]',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 启用 0 停用',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_campaign` (`campaign_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='兑换码表';


CREATE TABLE IF NOT EXISTS `campaign_redemption_users` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `campaign_id` bigint NOT NULL COMMENT '活动ID',
  `code` varchar(64) NOT NULL COMMENT '兑换码',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `times` int NOT NULL DEFAULT '0' COMMENT '该用户已兑换次数',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_campaign_code_user` (`campaign_id`,`code`,`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='兑换用户计数表';


CREATE TABLE IF NOT EXISTS `campaign_sign_ins` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `campaign_id` bigint NOT NULL COMMENT '活动ID',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `streak` int NOT NULL DEFAULT '0' COMMENT '连续签到天数（断签重置）',
  `total_days` int NOT NULL DEFAULT '0' COMMENT '累计签到天数',
  `last_signed_on` date DEFAULT NULL COMMENT '最近签到日期',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_campaign_user` (`campaign_id`,`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='签到状态表';


CREATE TABLE IF NOT EXISTS `campaign_versions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `campaign_id` bigint unsigned NOT NULL COMMENT '活动ID（campaigns.id）',
  `version` int unsigned NOT NULL COMMENT '版本号（1 起递增）',
  `kind` tinyint unsigned NOT NULL COMMENT 'CampaignKind：1 SIGN_IN 2 REDEMPTION_CODE 13 PRIZE_WHEEL',
  `name` varchar(128) NOT NULL COMMENT '活动名称',
  `state` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '状态：1 DRAFT 2 PUBLISHED',
  `config_json` text NOT NULL COMMENT 'console 形状配置（protojson，发布时转 player 形状物化）',
  `start_at` datetime NOT NULL COMMENT '开始时间',
  `end_at` datetime NOT NULL COMMENT '结束时间',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_campaign_version` (`campaign_id`,`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='活动版本表（console 草稿/发布）';


CREATE TABLE IF NOT EXISTS `campaign_wheel_daily` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `campaign_id` bigint NOT NULL COMMENT '活动ID',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `spin_date` date NOT NULL COMMENT '日期（服务本地日）',
  `spin_count` int NOT NULL DEFAULT '0' COMMENT '当日已抽次数',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_campaign_user_date` (`campaign_id`,`user_id`,`spin_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='转盘日计数表';


CREATE TABLE IF NOT EXISTS `campaign_wheel_slots` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `campaign_id` bigint NOT NULL COMMENT '活动ID',
  `slot_index` int NOT NULL COMMENT '槽位序号（0 起）',
  `label` varchar(64) NOT NULL DEFAULT '' COMMENT '槽位名称',
  `weight` int NOT NULL DEFAULT '0' COMMENT '权重（只在 remaining≠0 的槽中加权）',
  `prize_kind` tinyint NOT NULL COMMENT '奖品类型：1 CASH 2 POINTS 3 COUPON',
  `cash_fen` bigint NOT NULL DEFAULT '0' COMMENT '现金金额（分）',
  `points_amount` bigint NOT NULL DEFAULT '0' COMMENT '积分数',
  `coupon_template_no` varchar(64) NOT NULL DEFAULT '' COMMENT '券模板单号',
  `prize_config` text NOT NULL COMMENT '奖品数组 JSON（console 多奖品槽；空=走单奖品列）',
  `daily_stock` int NOT NULL DEFAULT '-1' COMMENT '每日库存（-1 不限量）',
  `remaining` int NOT NULL DEFAULT '-1' COMMENT '剩余（-1 不限量；0 不参与抽取）',
  `daily_spent` int NOT NULL DEFAULT '0' COMMENT '当日已中次数（daily_stock>0 时生效，跨日重置）',
  `daily_stock_date` date DEFAULT NULL COMMENT '日库存计数所属日期（跨日重置依据）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_campaign_slot` (`campaign_id`,`slot_index`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='转盘槽位表';


CREATE TABLE IF NOT EXISTS `campaigns` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `campaign_key` varchar(64) NOT NULL COMMENT '活动唯一键（uk）',
  `name` varchar(128) NOT NULL DEFAULT '' COMMENT '活动名称',
  `kind` tinyint NOT NULL DEFAULT '1' COMMENT '类型：1 签到 2 兑换码 3 转盘',
  `config_json` text NOT NULL COMMENT '按 kind 的配置 JSON（签到阶梯/转盘日限等）',
  `start_at` datetime NOT NULL COMMENT '活动开始时间',
  `end_at` datetime NOT NULL COMMENT '活动结束时间',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 ACTIVE 0 停用',
  `published_version` int unsigned NOT NULL DEFAULT '0' COMMENT '已发布版本（0=未发布；发布后指向 campaign_versions.version）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_campaign_key` (`campaign_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='活动配置表';


CREATE TABLE IF NOT EXISTS `coupon_template_heads` (
  `template_id` bigint unsigned NOT NULL COMMENT '模板ID（console 端指定，跨版本稳定）',
  `next_version` int unsigned NOT NULL DEFAULT '1' COMMENT '下一个版本号',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`template_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='券模板头表（不可变版本指针）';


CREATE TABLE IF NOT EXISTS `coupon_template_versions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `template_id` bigint unsigned NOT NULL COMMENT '模板ID',
  `version` int unsigned NOT NULL COMMENT '版本号（1 起递增）',
  `kind` tinyint unsigned NOT NULL COMMENT '类型：1 FIXED_MINOR 2 PERCENT_BPS 3 FREE 4 CUSTOM_MINOR 5 DRAW_REWARD',
  `amount_minor` bigint NOT NULL DEFAULT '0' COMMENT '直减金额（分）',
  `percent_bps` int NOT NULL DEFAULT '0' COMMENT '折扣万分比（10000=100%）',
  `min_gross_minor` bigint NOT NULL DEFAULT '0' COMMENT '使用门槛（分）',
  `currency` varchar(3) NOT NULL COMMENT '币种（3 字母）',
  `funder` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '出资方：1 PLATFORM 2 MERCHANT',
  `scene` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '结算场景：0 全场景',
  `min_unit_price_minor` bigint NOT NULL DEFAULT '0' COMMENT '单件价下限（分），0=不限',
  `max_unit_price_minor` bigint NOT NULL DEFAULT '0' COMMENT '单件价上限（分），0=不限',
  `min_item_count` int unsigned NOT NULL DEFAULT '0' COMMENT '件数下限，0=不限',
  `max_item_count` int unsigned NOT NULL DEFAULT '0' COMMENT '件数上限，0=不限',
  `target_business_key` varchar(64) NOT NULL DEFAULT '' COMMENT '业务键匹配（空=全匹配）',
  `validity_days` int unsigned NOT NULL DEFAULT '0' COMMENT '发券后有效天数，0=无期限',
  `reward_template_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT 'DRAW_REWARD 引用的券模板ID',
  `reward_count` int unsigned NOT NULL DEFAULT '0' COMMENT 'DRAW_REWARD 发放张数',
  `request_id` varchar(64) NOT NULL COMMENT '发布请求号（幂等键）',
  `published_at` datetime NOT NULL COMMENT '发布时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_template_version` (`template_id`,`version`),
  UNIQUE KEY `uk_request_id` (`request_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='券模板版本表（不可变，发布即生效）';


CREATE TABLE IF NOT EXISTS `coupon_templates` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `template_no` varchar(64) NOT NULL COMMENT '模板单号（uk，确定性）',
  `name` varchar(128) NOT NULL DEFAULT '' COMMENT '券名称',
  `kind` tinyint NOT NULL DEFAULT '1' COMMENT '类型：1 FIXED_MINOR 2 PERCENT_BPS 3 FREE 4 CUSTOM_MINOR 5 DRAW_REWARD',
  `value` bigint NOT NULL DEFAULT '0' COMMENT '面值：kind1/4=直减金额（分）；kind2=折扣 bps（万分比，如 5000=5折）；kind3=0',
  `min_gross` bigint NOT NULL DEFAULT '0' COMMENT '使用门槛：订单原价下限（分），0=无门槛',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：0 草稿 1 已发布 2 已停用',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_template_no` (`template_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='券模板表';


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


CREATE TABLE IF NOT EXISTS `loyalty_offset_config` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键（恒为 1）',
  `points_per_yuan` int NOT NULL DEFAULT '10' COMMENT '每元消耗积分（10=1 积分抵 0.1 元，即 1 积分=10 分）',
  `max_ratio_bps` int NOT NULL DEFAULT '7000' COMMENT '最高抵扣比例 bps（7000=订单原价 70%）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 启用 0 停用',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='积分抵扣配置表';


CREATE TABLE IF NOT EXISTS `quote_locks` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `quote_token` varchar(64) NOT NULL COMMENT '报价令牌："q_"+requestId 确定性派生',
  `request_id` varchar(64) NOT NULL COMMENT '全局事务 gid（幂等键）',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `box_id` bigint NOT NULL COMMENT '宝箱ID',
  `gross_fen` bigint NOT NULL COMMENT '宝箱原价（分，网关取自 draw 服务端定价）',
  `coupon_no` varchar(64) NOT NULL DEFAULT '' COMMENT '锁定的券号（空=未用券）',
  `coupon_discount_fen` bigint NOT NULL DEFAULT '0' COMMENT '券抵扣金额（分）',
  `entitlement_no` varchar(64) NOT NULL DEFAULT '' COMMENT '活动宝箱资格单号（空=未用资格）',
  `loyalty_points_debit` bigint NOT NULL DEFAULT '0' COMMENT '实际扣积分（点）',
  `loyalty_fen` bigint NOT NULL DEFAULT '0' COMMENT '积分抵扣金额（分，LOYALTY 腿）',
  `payable_fen` bigint NOT NULL COMMENT '实付总额（分）= cash_fen + loyalty_fen',
  `cash_fen` bigint NOT NULL COMMENT '现金腿金额（分）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 OPEN 2 LOCKED 3 USED 4 RELEASED 5 EXPIRED',
  `expires_at` datetime NOT NULL COMMENT '报价过期时间（创建+15 分钟）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_quote_token` (`quote_token`),
  UNIQUE KEY `uk_request_id` (`request_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='报价锁表';


CREATE TABLE IF NOT EXISTS `user_coupons` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `coupon_no` varchar(64) NOT NULL COMMENT '券号："uc_"+claimRequestId 确定性派生',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `template_no` varchar(64) NOT NULL COMMENT '券模板单号',
  `custom_amount_minor` bigint NOT NULL DEFAULT '0' COMMENT '自定义面额（分，CUSTOM_MINOR 券；0=用模板面额）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 AVAILABLE 2 LOCKED 3 USED 4 EXPIRED',
  `locked_by` varchar(64) NOT NULL DEFAULT '' COMMENT '锁定方 quote_token（LOCKED 时非空）',
  `expires_at` datetime DEFAULT NULL COMMENT '过期时间（NULL=无期限）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '领取时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_coupon_no` (`coupon_no`),
  KEY `idx_user_status` (`user_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户券表';


-- ============================================================
-- 种子数据（幂等；按里程碑序归并）
-- ============================================================

-- 券模板 ×2：直减 5 元（无门槛）/ 5 折券（满 10 元可用）
INSERT INTO `coupon_templates` (`template_no`, `name`, `kind`, `value`, `min_gross`, `status`)
VALUES ('T-FIX-500', '无门槛 5 元直减券', 1, 500, 0, 1),
       ('T-PCT-5000', '5 折券（满 10 元）', 2, 5000, 1000, 1)
ON DUPLICATE KEY UPDATE
    name=VALUES(name), kind=VALUES(kind), value=VALUES(value),
    min_gross=VALUES(min_gross), status=VALUES(status);


-- 积分抵扣配置：10 积分=1 元，最高抵订单原价 70%
INSERT INTO `loyalty_offset_config` (`id`, `points_per_yuan`, `max_ratio_bps`, `status`)
VALUES (1, 10, 7000, 1)
ON DUPLICATE KEY UPDATE
    points_per_yuan=VALUES(points_per_yuan), max_ratio_bps=VALUES(max_ratio_bps), status=VALUES(status);


-- 活动 ×3：签到（7 天积分阶梯，第 7 天加现金）/ 兑换码 / 转盘（日限 2）
INSERT INTO `campaigns` (`campaign_key`, `name`, `kind`, `config_json`, `start_at`, `end_at`, `status`)
VALUES ('C-SIGN-IN-001', '每日签到', 1,
        '{"rewards":[{"prizes":[{"kind":2,"points":10}]},{"prizes":[{"kind":2,"points":20}]},{"prizes":[{"kind":2,"points":30}]},{"prizes":[{"kind":2,"points":40}]},{"prizes":[{"kind":2,"points":50}]},{"prizes":[{"kind":2,"points":60}]},{"prizes":[{"kind":2,"points":100},{"kind":1,"cashFen":100}]}]}',
        '2026-01-01 00:00:00', '2036-01-01 00:00:00', 1),
       ('C-REDEEM-001', '兑换码活动', 2, '{}',
        '2026-01-01 00:00:00', '2036-01-01 00:00:00', 1),
       ('C-WHEEL-001', '幸运转盘', 13, '{"dailyLimit":2}',
        '2026-01-01 00:00:00', '2036-01-01 00:00:00', 1)
ON DUPLICATE KEY UPDATE
    name=VALUES(name), kind=VALUES(kind), config_json=VALUES(config_json),
    start_at=VALUES(start_at), end_at=VALUES(end_at), status=VALUES(status);


-- 兑换码 ×3（幂等覆盖；redeemed_count 复原为 0，测试前 reset 也可清空用户计数）
INSERT INTO `campaign_redemption_codes` (`campaign_id`, `code`, `max_redemptions`, `redeemed_count`, `per_user_limit`, `prize_config`, `status`)
SELECT id, 'GACHA666', 100, 0, 2,
       '[{"kind":3,"couponTemplateNo":"T-FIX-500"}]', 1 FROM `campaigns` WHERE `campaign_key`='C-REDEEM-001'
UNION ALL
SELECT id, 'BOX888', 2, 0, 1,
       '[{"kind":4,"boxId":1}]', 1 FROM `campaigns` WHERE `campaign_key`='C-REDEEM-001'
UNION ALL
SELECT id, 'LUCKY10', 1, 0, 1,
       '[{"kind":1,"cashFen":10},{"kind":2,"points":50}]', 1 FROM `campaigns` WHERE `campaign_key`='C-REDEEM-001'
ON DUPLICATE KEY UPDATE
    prize_config=VALUES(prize_config), status=VALUES(status), redeemed_count=VALUES(redeemed_count);


-- 转盘槽 ×3：现金 1 元 w10 / 积分 50 w80 / 5 元券 w10（daily_stock=-1 不限量）
INSERT INTO `campaign_wheel_slots` (`campaign_id`, `slot_index`, `label`, `weight`, `prize_kind`, `cash_fen`, `points_amount`, `coupon_template_no`, `prize_config`, `daily_stock`, `remaining`)
SELECT id, 0, '现金 1 元', 10, 1, 100, 0, '', '', -1, -1 FROM `campaigns` WHERE `campaign_key`='C-WHEEL-001'
UNION ALL
SELECT id, 1, '积分 50', 80, 2, 0, 50, '', '', -1, -1 FROM `campaigns` WHERE `campaign_key`='C-WHEEL-001'
UNION ALL
SELECT id, 2, '5 元券', 10, 3, 0, 0, 'T-FIX-500', '', -1, -1 FROM `campaigns` WHERE `campaign_key`='C-WHEEL-001'
ON DUPLICATE KEY UPDATE
    label=VALUES(label), weight=VALUES(weight), prize_kind=VALUES(prize_kind),
    cash_fen=VALUES(cash_fen), points_amount=VALUES(points_amount),
    coupon_template_no=VALUES(coupon_template_no), prize_config=VALUES(prize_config),
    daily_stock=VALUES(daily_stock), remaining=VALUES(remaining);


-- kind 对齐 server_v2 CampaignKind：转盘 3 → 13（PRIZE_WHEEL）
UPDATE `campaigns` SET `kind` = 13 WHERE `kind` = 3;

SET FOREIGN_KEY_CHECKS = 1;
