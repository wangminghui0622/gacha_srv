-- ============================================================
-- 08_gacha_catalog.sql - 商品目录域（SPU/SKU/修订/发布快照）（gacha_catalog 库终态快照）
-- 表结构 = 本机 MySQL 终态导出（mysqldump --no-data，2026-10-26）；
-- 种子 = 历史里程碑编号脚本按序归并（幂等，可重复执行）。
-- 执行方式：mysql -u root < script/split/08_gacha_catalog.sql
-- ============================================================

SET NAMES utf8mb4;

USE `gacha_catalog`;

SET FOREIGN_KEY_CHECKS = 0;

CREATE TABLE IF NOT EXISTS `option_values` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '选项值ID',
  `option_id` bigint unsigned NOT NULL COMMENT '所属选项ID',
  `value` varchar(128) NOT NULL COMMENT '值',
  `sort_order` int NOT NULL DEFAULT '0' COMMENT '排序',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_catalog_option_value` (`option_id`,`value`),
  KEY `idx_catalog_value_option_sort` (`option_id`,`sort_order`,`id`),
  CONSTRAINT `fk_catalog_value_option` FOREIGN KEY (`option_id`) REFERENCES `product_options` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='商品选项值表';

CREATE TABLE IF NOT EXISTS `product_options` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '选项ID',
  `product_id` bigint unsigned NOT NULL COMMENT '所属商品ID',
  `name` varchar(64) NOT NULL COMMENT '选项名',
  `sort_order` int NOT NULL DEFAULT '0' COMMENT '排序',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_catalog_option_name` (`product_id`,`name`),
  KEY `idx_catalog_option_product_sort` (`product_id`,`sort_order`,`id`),
  CONSTRAINT `fk_catalog_option_product` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='商品规格选项表';

CREATE TABLE IF NOT EXISTS `product_published_snapshots` (
  `product_id` bigint unsigned NOT NULL COMMENT '商品ID',
  `version` bigint unsigned NOT NULL COMMENT '快照版本',
  `snapshot_json` json NOT NULL COMMENT '完整商品快照（protojson）',
  `published_at` datetime(3) NOT NULL COMMENT '发布时间',
  PRIMARY KEY (`product_id`,`version`),
  KEY `idx_catalog_snapshot_published` (`published_at`,`product_id`,`version`),
  CONSTRAINT `fk_catalog_snapshot_product` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='商品已发布快照表';

CREATE TABLE IF NOT EXISTS `product_revision_drafts` (
  `product_id` bigint unsigned NOT NULL COMMENT '商品ID',
  `base_version` bigint unsigned NOT NULL COMMENT '修订基准版本（= 当前已发布版本）',
  `target_version` bigint unsigned NOT NULL COMMENT '目标版本（= base+1）',
  `draft_json` json NOT NULL COMMENT '完整草稿（protojson）',
  `created_at` datetime(3) NOT NULL COMMENT '创建时间',
  `updated_at` datetime(3) NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`product_id`),
  CONSTRAINT `fk_catalog_revision_product` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`),
  CONSTRAINT `chk_catalog_revision_version` CHECK ((`target_version` = (`base_version` + 1)))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='商品修订草稿表';

CREATE TABLE IF NOT EXISTS `products` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '商品ID',
  `product_code` varchar(64) NOT NULL COMMENT '商品编码',
  `title` varchar(255) NOT NULL COMMENT '标题',
  `status` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '状态：1=DRAFT 2=ACTIVE 3=ARCHIVED',
  `version` bigint unsigned NOT NULL DEFAULT '1' COMMENT '版本号（发布后快照版本）',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_catalog_product_code` (`product_code`),
  KEY `idx_catalog_product_status_updated` (`status`,`updated_at`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='商品头表';

CREATE TABLE IF NOT EXISTS `catalog_ips` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'IP ID',
  `name` varchar(128) NOT NULL COMMENT 'IP 名称',
  `sort_weight` int NOT NULL DEFAULT '0' COMMENT '排序权重（越大越靠前）',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_catalog_ip_name` (`name`),
  KEY `idx_catalog_ip_sort` (`sort_weight`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='IP 列表';

CREATE TABLE IF NOT EXISTS `catalog_brands` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '品牌 ID',
  `name` varchar(128) NOT NULL COMMENT '品牌名称',
  `logo` varchar(512) NOT NULL DEFAULT '' COMMENT 'Logo 图片地址',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_catalog_brand_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='品牌列表';

CREATE TABLE IF NOT EXISTS `sku_selections` (
  `sku_id` bigint unsigned NOT NULL COMMENT 'SKU ID',
  `option_id` bigint unsigned NOT NULL COMMENT '选项ID',
  `option_value_id` bigint unsigned NOT NULL COMMENT '选项值ID',
  PRIMARY KEY (`sku_id`,`option_id`),
  UNIQUE KEY `uk_catalog_sku_option_value` (`sku_id`,`option_value_id`),
  KEY `idx_catalog_sov_value_sku` (`option_value_id`,`sku_id`),
  KEY `fk_catalog_sov_option` (`option_id`),
  CONSTRAINT `fk_catalog_sov_option` FOREIGN KEY (`option_id`) REFERENCES `product_options` (`id`),
  CONSTRAINT `fk_catalog_sov_sku` FOREIGN KEY (`sku_id`) REFERENCES `skus` (`id`),
  CONSTRAINT `fk_catalog_sov_value` FOREIGN KEY (`option_value_id`) REFERENCES `option_values` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='SKU 规格选择表';

CREATE TABLE IF NOT EXISTS `skus` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'SKU ID',
  `product_id` bigint unsigned NOT NULL COMMENT '所属商品ID',
  `sku_code` varchar(64) NOT NULL COMMENT 'SKU 编码（全局唯一）',
  `title` varchar(255) NOT NULL COMMENT 'SKU 标题',
  `status` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '状态：1=DRAFT 2=ACTIVE 3=ARCHIVED',
  `fulfillment_mode` tinyint unsigned NOT NULL COMMENT '履约方式：1=PHYSICAL 2=DIGITAL 3=NONE',
  `sale_amount_minor` bigint NOT NULL COMMENT '售价（最小单位）',
  `reference_amount_minor` bigint NOT NULL COMMENT '参考价（最小单位）',
  `currency` char(3) NOT NULL COMMENT '币种（3 字母）',
  `external_code` varchar(128) NOT NULL DEFAULT '' COMMENT '外部编码',
  `version` bigint unsigned NOT NULL DEFAULT '1' COMMENT 'SKU 版本',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_catalog_sku_code` (`sku_code`),
  KEY `idx_catalog_sku_product_status` (`product_id`,`status`,`id`),
  KEY `idx_catalog_sku_external` (`external_code`),
  CONSTRAINT `fk_catalog_sku_product` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='商品 SKU 表';

-- IP 列表演示数据：仅在表为空时插入（可重复执行）
INSERT INTO `catalog_ips` (`name`, `sort_weight`)
SELECT seed.name, seed.sort_weight
FROM (
  SELECT '布兰兔' AS name, 44 AS sort_weight
  UNION ALL SELECT '宇宙的猜想', 43
  UNION ALL SELECT 'Handhandhand', 42
  UNION ALL SELECT '欧舒丹', 41
  UNION ALL SELECT '野兽派', 40
  UNION ALL SELECT 'BYREDO', 39
  UNION ALL SELECT 'Baccarat', 38
  UNION ALL SELECT '奢侈品', 37
  UNION ALL SELECT '富士山', 36
  UNION ALL SELECT '蝙蝠侠', 35
  UNION ALL SELECT '英雄联盟', 34
  UNION ALL SELECT '变形金刚', 33
  UNION ALL SELECT '剑风传奇', 32
  UNION ALL SELECT '生化危机', 31
  UNION ALL SELECT '明日方舟', 30
) AS seed
WHERE NOT EXISTS (SELECT 1 FROM `catalog_ips`);

-- 品牌列表演示数据：仅在表为空时插入（可重复执行；倒序插入以匹配 ID 倒序展示）
INSERT INTO `catalog_brands` (`name`, `logo`)
SELECT seed.name, seed.logo
FROM (
  SELECT 'OPPO' AS name, '' AS logo
  UNION ALL SELECT 'vivo', ''
  UNION ALL SELECT '荣耀', ''
  UNION ALL SELECT '三星', ''
  UNION ALL SELECT '寻找独角兽', ''
  UNION ALL SELECT '蜂蜜罐', ''
  UNION ALL SELECT '线条小狗', ''
  UNION ALL SELECT 'MORTEN SOLUM', ''
  UNION ALL SELECT '星辰铸造', ''
  UNION ALL SELECT '志扬玩具', ''
  UNION ALL SELECT '共鸣GONG', ''
  UNION ALL SELECT '影之刃零', ''
  UNION ALL SELECT '飞智 FLYDIGI', ''
  UNION ALL SELECT '玩家国度', ''
  UNION ALL SELECT '盖世小鸡', ''
) AS seed
WHERE NOT EXISTS (SELECT 1 FROM `catalog_brands`);

SET FOREIGN_KEY_CHECKS = 1;

