-- ============================================================
-- 10_gacha_thirdparty.sql - 第三方域（provider 指标/腾讯广告点击）（gacha_thirdparty 库终态快照）
-- 表结构 = 本机 MySQL 终态导出（mysqldump --no-data，2026-10-26）；
-- 种子 = 历史里程碑编号脚本按序归并（幂等，可重复执行）。
-- 执行方式：mysql -u root < script/split/10_gacha_thirdparty.sql
-- ============================================================

SET NAMES utf8mb4;

USE `gacha_thirdparty`;

SET FOREIGN_KEY_CHECKS = 0;

CREATE TABLE IF NOT EXISTS `integration_metric_batch` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '批次ID（自增）',
  `tenant_id` bigint unsigned NOT NULL COMMENT '租户ID',
  `request_id` varchar(64) NOT NULL COMMENT '请求ID（幂等键一）',
  `idempotency_key` varchar(128) NOT NULL COMMENT '幂等键二',
  `request_hash` binary(32) NOT NULL COMMENT '请求内容哈希（deterministic proto marshal SHA256）',
  `provider_code` varchar(64) NOT NULL COMMENT '渠道编码',
  `external_account_id` varchar(128) NOT NULL COMMENT '外部账户ID',
  `accepted_count` int unsigned NOT NULL DEFAULT '0' COMMENT '接受样本数（新修订）',
  `stale_count` int unsigned NOT NULL DEFAULT '0' COMMENT '过期样本数（旧修订）',
  `created_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_metric_batch_request` (`tenant_id`,`request_id`),
  KEY `idx_metric_batch_key` (`tenant_id`,`provider_code`,`idempotency_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='指标接入批次（幂等/重放）';

CREATE TABLE IF NOT EXISTS `integration_metric_current` (
  `tenant_id` bigint unsigned NOT NULL,
  `provider_code` varchar(64) NOT NULL,
  `external_account_id` varchar(128) NOT NULL,
  `external_object_id` varchar(128) NOT NULL,
  `standard_metric` tinyint unsigned NOT NULL,
  `bucket_start` datetime(3) NOT NULL,
  `bucket_seconds` int unsigned NOT NULL,
  `dimension_key` varchar(256) NOT NULL,
  `value` bigint NOT NULL,
  `currency` char(3) NOT NULL DEFAULT '',
  `source_revision` bigint unsigned NOT NULL,
  `raw_id` bigint unsigned NOT NULL COMMENT '最新接受的样本ID',
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`tenant_id`,`provider_code`,`external_account_id`,`external_object_id`,`standard_metric`,`bucket_start`,`bucket_seconds`,`dimension_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='指标当前值（槽位唯一，修订号仲裁）';

CREATE TABLE IF NOT EXISTS `integration_metric_outbox` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '事件ID（自增，即 MetricSnapshotChanged.event_id）',
  `aggregate_id` char(64) NOT NULL COMMENT '槽位聚合ID（sha256 hex，保序）',
  `event_type` varchar(128) NOT NULL COMMENT '事件类型（integration.metric.snapshot.changed.v1）',
  `payload` blob NOT NULL COMMENT '事件载荷（proto marshal）',
  `status` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '1 待发 / 2 处理中 / 3 已发',
  `attempts` int unsigned NOT NULL DEFAULT '0',
  `available_at` datetime(3) NOT NULL,
  `lease_owner` varchar(96) NOT NULL DEFAULT '',
  `locked_until` datetime(3) DEFAULT NULL,
  `last_error` varchar(512) NOT NULL DEFAULT '',
  `published_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_metric_outbox_due` (`status`,`available_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='指标事件投递 outbox（Kafka 替代）';

CREATE TABLE IF NOT EXISTS `integration_metric_raw` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '样本ID（自增）',
  `tenant_id` bigint unsigned NOT NULL,
  `batch_id` bigint unsigned NOT NULL COMMENT '批次ID',
  `provider_code` varchar(64) NOT NULL,
  `external_account_id` varchar(128) NOT NULL,
  `external_object_id` varchar(128) NOT NULL COMMENT '外部对象ID（广告计划/创意）',
  `provider_metric_code` varchar(128) NOT NULL COMMENT '渠道原始指标编码',
  `standard_metric` tinyint unsigned NOT NULL COMMENT '标准指标 1-5（impressions/clicks/conversions/spend/revenue）',
  `bucket_start` datetime(3) NOT NULL COMMENT '时间桶起点（UTC 对齐）',
  `bucket_seconds` int unsigned NOT NULL COMMENT '桶长秒（3600/86400）',
  `dimension_key` varchar(256) NOT NULL COMMENT '维度键（all 或细分值）',
  `value` bigint NOT NULL COMMENT '指标值',
  `currency` char(3) NOT NULL DEFAULT '' COMMENT '币种（计数指标空）',
  `source_revision` bigint unsigned NOT NULL COMMENT '源修订号（新旧仲裁）',
  `is_stale` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '0 接受 / 1 过期（旧修订）',
  `created_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_metric_raw_batch` (`batch_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='指标原始样本流水';

CREATE TABLE IF NOT EXISTS `integration_tx_ad_click` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `account_id` varchar(64) NOT NULL COMMENT '广告账户ID',
  `click_id` varchar(128) NOT NULL COMMENT '点击ID',
  `click_time` bigint NOT NULL COMMENT '点击时间（unix 秒）',
  `wechat_open_id` varchar(128) NOT NULL COMMENT '微信 openid（去重键）',
  `device_os_type` varchar(32) NOT NULL DEFAULT '',
  `created_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_integration_tx_ad_open_id` (`wechat_open_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='腾讯广告点击记录（按 openid 去重）';

SET FOREIGN_KEY_CHECKS = 1;

