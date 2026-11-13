-- ============================================================
-- 03_gacha_money.sql - 资金域（钱包/冻结/账本/余额/资产/支付单/退款单/支付路由 + dtm_barrier）（gacha_money 库终态快照）
-- 表结构 = 本机 MySQL 终态导出；种子 = 历史里程碑脚本按序归并（幂等，可重复执行）。
-- 本文件由 script/split_sql.go 从旧仓 script/split/ 拆库生成。
-- 执行方式：mysql -u root < script/split/03_gacha_money.sql
-- ============================================================

SET NAMES utf8mb4;

USE `gacha_money`;

SET FOREIGN_KEY_CHECKS = 0;


CREATE TABLE IF NOT EXISTS `asset_reservations` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `reservation_no` varchar(64) NOT NULL COMMENT '预留单号（确定性派生，唯一键幂等）',
  `asset_id` bigint NOT NULL COMMENT '资产ID（关联 assets.id）',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `purpose` tinyint NOT NULL COMMENT '预留目的：1 TRADE 2 DELIVERY 3 EXCHANGE 4 NESTED_BOX_OPEN',
  `biz_id` varchar(64) NOT NULL DEFAULT '' COMMENT '业务单号',
  `state` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 ACTIVE 2 RELEASED 3 CONSUMED',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_reservation_no` (`reservation_no`),
  KEY `idx_asset_id` (`asset_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='资产预留表';


CREATE TABLE IF NOT EXISTS `assets` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `asset_no` varchar(64) NOT NULL COMMENT '资产单号：draw="a_"+requestId；play="pa_"+grantNo',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `box_id` bigint NOT NULL DEFAULT '0' COMMENT '宝箱ID',
  `record_id` bigint NOT NULL DEFAULT '0' COMMENT '抽赏记录ID（draw 域）',
  `pool_id` bigint NOT NULL DEFAULT '0' COMMENT '玩法池/配置ID（play 域；draw 域=0）',
  `sku_id` bigint NOT NULL DEFAULT '0' COMMENT 'catalog SKU ID（实物奖；0=虚拟/无）',
  `source` varchar(16) NOT NULL DEFAULT 'draw' COMMENT '来源域：draw/play',
  `item_name` varchar(64) NOT NULL COMMENT '奖品名称',
  `state` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 AVAILABLE 2 RESERVED 3 CONSUMED 4 DELIVERED 5 CANCELED',
  `pickup_code` varchar(32) DEFAULT NULL COMMENT '自提码（实物奖品，可空）',
  `reserved_for` tinyint DEFAULT NULL COMMENT '预留目的：1 TRADE 2 DELIVERY 3 EXCHANGE 4 NESTED_BOX_OPEN',
  `reserved_biz_id` varchar(64) NOT NULL DEFAULT '' COMMENT '预留业务单号',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_asset_no` (`asset_no`),
  UNIQUE KEY `uk_pickup_code` (`pickup_code`),
  KEY `idx_user_state` (`user_id`,`state`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='数字资产表';


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


CREATE TABLE IF NOT EXISTS `pickup_codes` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `code` varchar(32) NOT NULL COMMENT '自提码',
  `asset_id` bigint NOT NULL COMMENT '资产ID（关联 assets.id）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 ACTIVE 2 USED',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  UNIQUE KEY `uk_asset_id` (`asset_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='自提码表';


CREATE TABLE IF NOT EXISTS `user_balance_logs` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `log_no` varchar(64) NOT NULL COMMENT '流水号：由业务单号确定性派生，重试幂等',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `amount` bigint NOT NULL COMMENT '变动金额（分）：正=入账，负=出账',
  `biz_type` varchar(16) NOT NULL COMMENT '业务类型：recharge=充值 deduct=扣款 refund=退款',
  `status` varchar(16) NOT NULL COMMENT '状态：success=成功 refunded=已退款（deduct 被补偿）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_log_no` (`log_no`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户余额流水表';


CREATE TABLE IF NOT EXISTS `user_balances` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `user_id` bigint NOT NULL COMMENT '用户ID（关联 user_users.id）',
  `balance` bigint NOT NULL DEFAULT '0' COMMENT '余额（单位：分，int64）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户余额表';


CREATE TABLE IF NOT EXISTS `wallet_accounts` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `user_id` bigint NOT NULL COMMENT '用户ID（关联 user_users.id）',
  `wallet_kind` tinyint NOT NULL COMMENT '钱包类型：1 CASH 2 BONUS 3 REDEEM_POINT 4 LOYALTY_POINT',
  `currency` varchar(8) NOT NULL DEFAULT 'CNY' COMMENT '币种（CASH 用 CNY，积分用 POINT）',
  `available` bigint NOT NULL DEFAULT '0' COMMENT '可用余额（单位：分）',
  `held` bigint NOT NULL DEFAULT '0' COMMENT '冻结余额（单位：分）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_kind` (`user_id`,`wallet_kind`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='钱包账户表';


CREATE TABLE IF NOT EXISTS `wallet_holds` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `hold_no` varchar(64) NOT NULL COMMENT '冻结单号：由业务单号确定性派生（"h_"+biz_id）',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `wallet_kind` tinyint NOT NULL COMMENT '钱包类型（同 wallet_accounts）',
  `amount` bigint NOT NULL COMMENT '冻结金额（分）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 PENDING 2 POSTED 3 VOIDED 4 EXPIRED',
  `biz_type` varchar(16) NOT NULL COMMENT '业务类型（如 payment_leg）',
  `biz_id` varchar(64) NOT NULL COMMENT '业务单号（冻结请求幂等键）',
  `expires_at` datetime DEFAULT NULL COMMENT '过期时间（NULL=不过期）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_hold_no` (`hold_no`),
  UNIQUE KEY `uk_biz` (`biz_type`,`biz_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='钱包冻结表';


CREATE TABLE IF NOT EXISTS `wallet_ledgers` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `log_no` varchar(64) NOT NULL COMMENT '流水号：由业务单号确定性派生，重试幂等',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `wallet_kind` tinyint NOT NULL COMMENT '钱包类型',
  `amount` bigint NOT NULL COMMENT '变动金额（分）：正=可用余额增加，负=减少',
  `biz_type` varchar(16) NOT NULL COMMENT '业务类型：recharge credit freeze unfreeze deduct',
  `biz_id` varchar(64) NOT NULL DEFAULT '' COMMENT '关联业务单号',
  `hold_no` varchar(64) NOT NULL DEFAULT '' COMMENT '关联冻结单号（非冻结流水为空）',
  `status` varchar(16) NOT NULL DEFAULT 'success' COMMENT '状态：success=成功 refunded=已退款（deduct 被补偿）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_log_no` (`log_no`),
  KEY `idx_user_kind` (`user_id`,`wallet_kind`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='钱包流水表';


INSERT INTO `user_balances` (`id`, `user_id`, `balance`) VALUES
    (1, 1, 100000)
ON DUPLICATE KEY UPDATE `id` = `id`;


-- v1 单余额 user_balances → wallet_accounts(CASH)。
-- INSERT IGNORE：已迁移过的账户不覆盖（重复执行安全；后续余额以钱包为准）
INSERT IGNORE INTO `wallet_accounts` (`user_id`, `wallet_kind`, `currency`, `available`, `held`, `created_at`, `updated_at`)
SELECT `user_id`, 1, 'CNY', `balance`, 0, NOW(), NOW() FROM `user_balances`;


-- 演示用户（id=1）忠诚点种子：1000 积分（M10 积分抵现联调，10 积分=1 元）
INSERT INTO `wallet_accounts` (`user_id`, `wallet_kind`, `currency`, `available`, `held`) VALUES
    (1, 4, 'POINT', 1000, 0)
ON DUPLICATE KEY UPDATE `id` = `id`;


CREATE TABLE IF NOT EXISTS `payment_legs` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `leg_no` varchar(64) NOT NULL COMMENT '资金腿单号：确定性派生（payment_no+"_"+wallet_kind）',
  `payment_no` varchar(64) NOT NULL COMMENT '支付单号',
  `wallet_kind` tinyint NOT NULL COMMENT '钱包类型：1 CASH 4 LOYALTY_POINT（同用户服务）',
  `amount` bigint NOT NULL COMMENT '腿金额（分）',
  `status` tinyint NOT NULL COMMENT '状态：1 PENDING 2 POSTED 3 VOIDED',
  `hold_no` varchar(64) NOT NULL DEFAULT '' COMMENT '冻结单号（user 服务 wallet_holds.hold_no）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_leg_no` (`leg_no`),
  KEY `idx_payment_no` (`payment_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='支付资金腿表（本阶段仅钱包腿，微信/支付宝留第三方阶段）';


CREATE TABLE IF NOT EXISTS `payment_orders` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `payment_no` varchar(64) NOT NULL COMMENT '支付单号：由请求单号确定性派生（"P"+requestId）',
  `order_no` varchar(64) NOT NULL COMMENT '业务订单号（= DTM 全局事务 gid）',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `box_id` bigint NOT NULL COMMENT '宝箱ID',
  `status` tinyint NOT NULL COMMENT '状态：1 CREATED 2 AWAITING_PAYMENT 3 PAID 4 REFUNDING 5 REFUNDED 6 CANCELED',
  `gross_amount` bigint NOT NULL COMMENT '订单总额（分）',
  `cash_amount` bigint NOT NULL COMMENT '现金部分（分）',
  `loyalty_amount` bigint NOT NULL DEFAULT '0' COMMENT '积分抵现部分（分，M10 起用）',
  `loyalty_points_debit` bigint NOT NULL DEFAULT '0' COMMENT '抵扣积分数（M10 起用）',
  `currency` varchar(8) NOT NULL DEFAULT 'CNY' COMMENT '币种',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_payment_no` (`payment_no`),
  UNIQUE KEY `uk_order_no` (`order_no`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='支付单表';


CREATE TABLE IF NOT EXISTS `payment_refunds` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `refund_no` varchar(64) NOT NULL COMMENT '退款单号："R"+refundRequestId 确定性派生',
  `payment_no` varchar(64) NOT NULL COMMENT '支付单号',
  `amount` bigint NOT NULL COMMENT '退款金额（分）',
  `status` tinyint NOT NULL COMMENT '状态：1 PROCESSING 2 SUCCEEDED 3 FAILED',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_refund_no` (`refund_no`),
  KEY `idx_payment_no` (`payment_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='退款单表';


CREATE TABLE IF NOT EXISTS `payment_route_audits` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `policy_id` bigint unsigned NOT NULL COMMENT '策略ID',
  `tenant_id` bigint unsigned NOT NULL COMMENT '租户',
  `old_version` int unsigned NOT NULL DEFAULT '0' COMMENT '旧版本（首建 0）',
  `new_version` int unsigned NOT NULL COMMENT '新版本',
  `operator_user_id` bigint unsigned NOT NULL COMMENT '操作员ID',
  `old_snapshot` json DEFAULT NULL COMMENT '旧快照（首建 NULL）',
  `new_snapshot` json NOT NULL COMMENT '新快照',
  `changed_at` datetime NOT NULL COMMENT '变更时间',
  PRIMARY KEY (`id`),
  KEY `idx_policy` (`policy_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='支付路由变更审计表';


CREATE TABLE IF NOT EXISTS `payment_route_candidates` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `policy_id` bigint unsigned NOT NULL COMMENT '策略ID',
  `provider` tinyint unsigned NOT NULL COMMENT '渠道商：1 NATIVE_WECHAT 2 NATIVE_ALIPAY',
  `merchant_account_id` varchar(64) NOT NULL COMMENT '商户号',
  `priority` int unsigned NOT NULL DEFAULT '0' COMMENT '优先级（小值优先）',
  `weight` int unsigned NOT NULL DEFAULT '1' COMMENT '权重（1..10000）',
  `enabled` tinyint(1) NOT NULL DEFAULT '1' COMMENT '是否启用',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_policy` (`policy_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='支付路由候选表';


CREATE TABLE IF NOT EXISTS `payment_route_policies` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL COMMENT '租户（单租户固定 1）',
  `channel` tinyint unsigned NOT NULL COMMENT '渠道：1 WECHAT 2 ALIPAY',
  `scene` tinyint unsigned NOT NULL COMMENT '场景：1 APP 2 MINI_PROGRAM 3 H5',
  `mode` tinyint unsigned NOT NULL COMMENT '模式：1 AUTO 2 PINNED',
  `pinned_provider` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '固定渠道商（PINNED 时非 0）',
  `pinned_merchant_account_id` varchar(64) NOT NULL DEFAULT '' COMMENT '固定商户号（PINNED 时非空）',
  `version` int unsigned NOT NULL DEFAULT '0' COMMENT '版本号（每次 PUT +1）',
  `updated_by` bigint unsigned NOT NULL COMMENT '操作员ID',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_channel_scene` (`tenant_id`,`channel`,`scene`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='支付路由策略表';

SET FOREIGN_KEY_CHECKS = 1;
