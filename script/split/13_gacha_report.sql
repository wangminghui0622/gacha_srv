-- ============================================================
-- 13_gacha_report.sql - 报表读模型库（经营日报 + 留存报表）
-- 由 ops-rpc 定时从 identity/money/draw/catalog 聚合写入；
-- 报表接口只读本库，不再跨库查询业务表。
-- ============================================================

SET NAMES utf8mb4;

USE `gacha_report`;

SET FOREIGN_KEY_CHECKS = 0;

CREATE TABLE IF NOT EXISTS `business_daily` (
  `report_date` date NOT NULL COMMENT '统计日期（东八区）',
  `registrations` bigint NOT NULL DEFAULT '0' COMMENT '注册用户数',
  `active_users` bigint NOT NULL DEFAULT '0' COMMENT '活跃用户数',
  `paying_users` bigint NOT NULL DEFAULT '0' COMMENT '付费用户数（当日）',
  `refundable_fen` bigint NOT NULL DEFAULT '0' COMMENT '用户可退款金额（分，快照）',
  `balance_fen` bigint NOT NULL DEFAULT '0' COMMENT '金额剩余（分，快照）',
  `inventory_value_fen` bigint NOT NULL DEFAULT '0' COMMENT '库存价值（分）',
  `draw_count` bigint NOT NULL DEFAULT '0' COMMENT '抽赏次数',
  `draw_amount_fen` bigint NOT NULL DEFAULT '0' COMMENT '总抽赏金额（分）',
  `prize_amount_fen` bigint NOT NULL DEFAULT '0' COMMENT '奖品金额（分）',
  `gross_profit_fen` bigint NOT NULL DEFAULT '0' COMMENT '毛利（分）',
  `total_received_fen` bigint NOT NULL DEFAULT '0' COMMENT '总到账（分）',
  `total_recharge_fen` bigint NOT NULL DEFAULT '0' COMMENT '总充值（分）',
  `total_refund_fen` bigint NOT NULL DEFAULT '0' COMMENT '总退款（分）',
  `alipay_recharge_fen` bigint NOT NULL DEFAULT '0' COMMENT '支付宝充值（分）',
  `huifu_recharge_fen` bigint NOT NULL DEFAULT '0' COMMENT '汇付充值（分）',
  `alipay_refund_fen` bigint NOT NULL DEFAULT '0' COMMENT '支付宝退款（分）',
  `huifu_refund_fen` bigint NOT NULL DEFAULT '0' COMMENT '汇付退款（分）',
  `recharge_original_refund_fen` bigint NOT NULL DEFAULT '0' COMMENT '充值原路退款（分）',
  `user_refund_fen` bigint NOT NULL DEFAULT '0' COMMENT '用户退款（分）',
  `recharge_minus_refund_fen` bigint NOT NULL DEFAULT '0' COMMENT '充值减退款（分）',
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`report_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='经营日报每日聚合';

CREATE TABLE IF NOT EXISTS `business_daily_payer` (
  `report_date` date NOT NULL COMMENT '统计日期（东八区）',
  `user_id` bigint NOT NULL COMMENT '当日有成功充值的用户ID',
  PRIMARY KEY (`report_date`,`user_id`),
  KEY `idx_user_date` (`user_id`,`report_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='经营日报付费用户明细（累计/本月去重）';

-- ── 留存报表（5 个 Tab 各一张表）──
-- 统一结构：day_no=0 存 cohort 总人数，day_no=1/2/.../180 存第 N 天留存人数。
-- 由 ops-rpc 从业务库定时聚合写入，后台接口只读本库。

CREATE TABLE IF NOT EXISTS `retention_new` (
  `cohort_date` date NOT NULL COMMENT 'cohort 日期（注册日，东八区）',
  `day_no` int NOT NULL COMMENT '留存天数：0=cohort 总数，1/2/.../180',
  `total_users` bigint NOT NULL DEFAULT '0' COMMENT 'cohort 总用户数',
  `retained_users` bigint NOT NULL DEFAULT '0' COMMENT '第 N 天留存用户数',
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`cohort_date`,`day_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='新增留存：注册 cohort 后第 N 天活跃';

CREATE TABLE IF NOT EXISTS `retention_active` (
  `cohort_date` date NOT NULL COMMENT 'cohort 日期（活跃日，东八区）',
  `day_no` int NOT NULL COMMENT '留存天数：0=cohort 总数，1/2/.../180',
  `total_users` bigint NOT NULL DEFAULT '0' COMMENT 'cohort 总用户数',
  `retained_users` bigint NOT NULL DEFAULT '0' COMMENT '第 N 天留存用户数',
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`cohort_date`,`day_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='有效用户：活跃 cohort 后第 N 天活跃';

CREATE TABLE IF NOT EXISTS `retention_new_pay` (
  `cohort_date` date NOT NULL COMMENT 'cohort 日期（首次成功充值日，东八区）',
  `day_no` int NOT NULL COMMENT '留存天数：0=cohort 总数，1/2/.../180',
  `total_users` bigint NOT NULL DEFAULT '0' COMMENT 'cohort 总用户数',
  `retained_users` bigint NOT NULL DEFAULT '0' COMMENT '第 N 天留存用户数',
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`cohort_date`,`day_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='新增消费：首次成功充值 cohort 后第 N 天成功充值';

CREATE TABLE IF NOT EXISTS `retention_join` (
  `cohort_date` date NOT NULL COMMENT 'cohort 日期（抽赏日，东八区）',
  `day_no` int NOT NULL COMMENT '留存天数：0=cohort 总数，1/2/.../180',
  `total_users` bigint NOT NULL DEFAULT '0' COMMENT 'cohort 总用户数',
  `retained_users` bigint NOT NULL DEFAULT '0' COMMENT '第 N 天留存用户数',
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`cohort_date`,`day_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='参与留存：抽赏 cohort 后第 N 天抽赏';

CREATE TABLE IF NOT EXISTS `retention_pay` (
  `cohort_date` date NOT NULL COMMENT 'cohort 日期（成功充值日，东八区）',
  `day_no` int NOT NULL COMMENT '留存天数：0=cohort 总数，1/2/.../180',
  `total_users` bigint NOT NULL DEFAULT '0' COMMENT 'cohort 总用户数',
  `retained_users` bigint NOT NULL DEFAULT '0' COMMENT '第 N 天留存用户数',
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`cohort_date`,`day_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='消费留存：成功充值 cohort 后第 N 天成功充值';

-- 营收报表每日聚合（统一大盘趋势图 + 营收明细）
CREATE TABLE IF NOT EXISTS `revenue_daily` (
  `report_date` date NOT NULL COMMENT '统计日期（东八区）',
  `registrations` bigint NOT NULL DEFAULT '0' COMMENT '注册用户数',
  `active_users` bigint NOT NULL DEFAULT '0' COMMENT '日活',
  `new_active_users` bigint NOT NULL DEFAULT '0' COMMENT '日活（新）',
  `old_active_users` bigint NOT NULL DEFAULT '0' COMMENT '日活（旧）',
  `active_users_7d` bigint NOT NULL DEFAULT '0' COMMENT '有效用户数（近7天）',
  `active_users_15d` bigint NOT NULL DEFAULT '0' COMMENT '有效用户数（近15天）',
  `churn_1d_bps` bigint NOT NULL DEFAULT '0' COMMENT '前1天流失率（万分比）',
  `churn_3d_bps` bigint NOT NULL DEFAULT '0' COMMENT '前3天流失率（万分比）',
  `pay_amount_fen` bigint NOT NULL DEFAULT '0' COMMENT '总付费（分）',
  `new_pay_amount_fen` bigint NOT NULL DEFAULT '0' COMMENT '总付费（新，分）',
  `old_pay_amount_fen` bigint NOT NULL DEFAULT '0' COMMENT '总付费（旧，分）',
  `refund_amount_fen` bigint NOT NULL DEFAULT '0' COMMENT '退款金额（分）',
  `refund_users` bigint NOT NULL DEFAULT '0' COMMENT '退款人数',
  `paying_users` bigint NOT NULL DEFAULT '0' COMMENT '付费用户数',
  `new_paying_users` bigint NOT NULL DEFAULT '0' COMMENT '付费用户数（新）',
  `old_paying_users` bigint NOT NULL DEFAULT '0' COMMENT '付费用户数（旧）',
  `net_revenue_fen` bigint NOT NULL DEFAULT '0' COMMENT '总营收（分，总付费-退款）',
  `wechat_pay_fen` bigint NOT NULL DEFAULT '0' COMMENT '微信充值（分）',
  `alipay_pay_fen` bigint NOT NULL DEFAULT '0' COMMENT '支付宝充值（分）',
  `huifu_pay_fen` bigint NOT NULL DEFAULT '0' COMMENT '汇付充值（分）',
  `wechat_gift_fen` bigint NOT NULL DEFAULT '0' COMMENT '微信赠送（分）',
  `alipay_gift_fen` bigint NOT NULL DEFAULT '0' COMMENT '支付宝赠送（分）',
  `huifu_gift_fen` bigint NOT NULL DEFAULT '0' COMMENT '汇付赠送（分）',
  `wechat_refund_fen` bigint NOT NULL DEFAULT '0' COMMENT '退微信充值（分）',
  `alipay_refund_fen` bigint NOT NULL DEFAULT '0' COMMENT '退支付宝充值（分）',
  `huifu_refund_fen` bigint NOT NULL DEFAULT '0' COMMENT '退汇付充值（分）',
  `wechat_gift_refund_fen` bigint NOT NULL DEFAULT '0' COMMENT '退微信赠送（分）',
  `alipay_gift_refund_fen` bigint NOT NULL DEFAULT '0' COMMENT '退支付宝赠送（分）',
  `huifu_gift_refund_fen` bigint NOT NULL DEFAULT '0' COMMENT '退汇付赠送（分）',
  `tax_fen` bigint NOT NULL DEFAULT '0' COMMENT '总税收（分，抽赏毛利）',
  `new_tax_fen` bigint NOT NULL DEFAULT '0' COMMENT '税收（新，按新付费占比分摊）',
  `old_tax_fen` bigint NOT NULL DEFAULT '0' COMMENT '税收（旧）',
  `wallet_balance_fen` bigint NOT NULL DEFAULT '0' COMMENT 'WalletBalance / 余额（分）',
  `bonus_balance_fen` bigint NOT NULL DEFAULT '0' COMMENT '吉豆余额',
  `redeem_point_balance` bigint NOT NULL DEFAULT '0' COMMENT '积分余额',
  `loyalty_point_balance` bigint NOT NULL DEFAULT '0' COMMENT '吉祥值余额',
  `platform_currency_pay_fen` bigint NOT NULL DEFAULT '0' COMMENT '总吉祥币付费（分）',
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`report_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='营收报表每日聚合';

SET FOREIGN_KEY_CHECKS = 1;
