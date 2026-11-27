-- ============================================================
-- 14_play_modes.sql - 玩法域按玩法拆分（无版本）
--
-- 设计：
--   play_pool_catalog   公共索引：只放身份、状态、列表字段
--   play_pool_history   修改历史：只读审计，不参与开奖
--   play_pool_requests  创建/修改幂等
--   play_<mode>_pools   每个玩法自己的当前配置
--   play_<mode>_prizes  每个玩法自己的当前奖池
--
-- 不再使用 play_pool_versions / play_pool_prizes。
-- 执行方式：mysql --default-character-set=utf8mb4 -u root < 14_play_modes.sql
-- ============================================================

SET NAMES utf8mb4;

USE `gacha_play`;

CREATE TABLE IF NOT EXISTS `play_pool_catalog` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '赏池ID（从 1000000 开始）',
  `mode` varchar(32) NOT NULL COMMENT '玩法代码：memorymatch/gashapon/fullcar/combochain/dig/groupbuy',
  `name` varchar(128) NOT NULL DEFAULT '' COMMENT '赏池名称',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1=草稿 2=上线 3=下架 4=作废',
  `price_fen` bigint NOT NULL DEFAULT '0' COMMENT '当前价格（分，冗余字段，便于列表展示）',
  `ip_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT 'IP ID',
  `tags_json` varchar(1024) NOT NULL DEFAULT '[]' COMMENT '标签 JSON 数组',
  `weight` int NOT NULL DEFAULT '0' COMMENT '排序权重（越大越靠前）',
  `admin_json` mediumtext COMMENT '后台扩展字段 JSON（活动时间/开关/折扣/封面/优惠券）',
  `void_reason` varchar(255) NOT NULL DEFAULT '' COMMENT '作废原因',
  `voided_at` datetime DEFAULT NULL COMMENT '作废时间',
  `voided_by` bigint unsigned NOT NULL DEFAULT '0' COMMENT '作废操作人',
  `created_by` bigint unsigned NOT NULL DEFAULT '0' COMMENT '创建人',
  `updated_by` bigint unsigned NOT NULL DEFAULT '0' COMMENT '最后修改人',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_mode_status` (`mode`,`status`),
  KEY `idx_weight` (`weight`)
) ENGINE=InnoDB AUTO_INCREMENT=1000000 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='玩法池统一索引（仅身份/状态/列表，不含玩法配置）';

CREATE TABLE IF NOT EXISTS `play_pool_history` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID',
  `mode` varchar(32) NOT NULL COMMENT '玩法代码',
  `action` varchar(32) NOT NULL COMMENT '动作：CREATE/UPDATE/APPLY/OFFLINE/RELIST/VOID',
  `before_json` longtext COMMENT '修改前快照 JSON',
  `after_json` longtext COMMENT '修改后快照 JSON',
  `reason` varchar(255) NOT NULL DEFAULT '' COMMENT '操作原因/备注',
  `operator_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '操作人',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_pool_created` (`pool_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='玩法池修改历史（只读审计，不参与开奖）';

CREATE TABLE IF NOT EXISTS `play_pool_requests` (
  `idempotency_key` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL COMMENT '幂等键',
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID',
  `request_hash` char(64) NOT NULL COMMENT '请求哈希 SHA256',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`idempotency_key`),
  KEY `idx_pool` (`pool_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='玩法池创建/修改幂等表';

-- ── 对对碰 ────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS `play_memorymatch_pools` (
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID（play_pool_catalog.id）',
  `price_fen` bigint NOT NULL DEFAULT '0' COMMENT '单次购买价格（分）',
  `config_json` json NOT NULL COMMENT '玩法配置 JSON：回合数/最大碰数/牌堆/外观/全局跟随等',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`pool_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='对对碰当前配置';

CREATE TABLE IF NOT EXISTS `play_memorymatch_prizes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID（play_pool_catalog.id）',
  `prize_index` int NOT NULL COMMENT '奖品位序（0 起）',
  `name` varchar(128) NOT NULL COMMENT '奖档名称',
  `total_pair_count` int NOT NULL COMMENT '总碰数档位（开奖时精确匹配）',
  `prize_kind` tinyint NOT NULL DEFAULT '0' COMMENT '奖励类型：0=无奖 1=现金 2=积分 3=优惠券 5=实物SKU',
  `cash_fen` bigint NOT NULL DEFAULT '0' COMMENT '现金金额（分）',
  `points_amount` bigint NOT NULL DEFAULT '0' COMMENT '积分数量',
  `coupon_template_no` varchar(64) NOT NULL DEFAULT '' COMMENT '优惠券模板号',
  `sku_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '实物 SKU ID',
  `quantity` int NOT NULL DEFAULT '0' COMMENT '总库存（-1=不限量）',
  `weight` int NOT NULL DEFAULT '1' COMMENT '权重（保留字段）',
  `is_prize` tinyint NOT NULL DEFAULT '1' COMMENT '是否奖品：1=有奖 0=谢谢参与',
  `image_url` varchar(512) NOT NULL DEFAULT '' COMMENT '奖档图片',
  `cost_fen` bigint NOT NULL DEFAULT '0' COMMENT '成本价（分）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_pool_index` (`pool_id`,`prize_index`),
  UNIQUE KEY `uk_pool_pairs` (`pool_id`,`total_pair_count`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='对对碰当前奖档';
