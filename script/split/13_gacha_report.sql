-- ============================================================
-- 13_gacha_report.sql - 报表读模型库（经营日报）
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

SET FOREIGN_KEY_CHECKS = 1;
