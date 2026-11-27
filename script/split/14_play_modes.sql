-- ============================================================
-- 14_play_modes.sql - 玩法域按玩法拆分（无版本，所有玩法同一套逻辑）
--
-- 统一约定：
--   play_pool_catalog   公共索引：身份、状态、列表字段
--   play_pool_history   修改历史：只读审计
--   play_pool_requests  创建/修改幂等
--   play_<mode>_pools   每个玩法自己的当前配置（结构完全一致）
--   play_<mode>_prizes  每个玩法自己的当前奖池（结构完全一致）
--
-- 所有玩法的生命周期一致：
--   DRAFT -> LIVE -> OFFLINE -> LIVE
--                    \-> VOID
-- 没有 version，没有版本表，没有按版本审批。
--
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
  `published_at` datetime DEFAULT NULL COMMENT '最近一次上线时间',
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

-- ── 所有玩法的配置表结构一致 ───────────────────────────────

CREATE TABLE IF NOT EXISTS `play_memorymatch_pools` (
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID（play_pool_catalog.id）',
  `price_fen` bigint NOT NULL DEFAULT '0' COMMENT '单次购买价格（分）',
  `config_json` json NOT NULL COMMENT '玩法配置 JSON',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`pool_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='对对碰当前配置';

CREATE TABLE IF NOT EXISTS `play_gashapon_pools` (
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID（play_pool_catalog.id）',
  `price_fen` bigint NOT NULL DEFAULT '0' COMMENT '单次购买价格（分）',
  `config_json` json NOT NULL COMMENT '玩法配置 JSON',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`pool_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='吉祥赏当前配置';

CREATE TABLE IF NOT EXISTS `play_fullcar_pools` (
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID（play_pool_catalog.id）',
  `price_fen` bigint NOT NULL DEFAULT '0' COMMENT '单次购买价格（分）',
  `config_json` json NOT NULL COMMENT '玩法配置 JSON',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`pool_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='满车赏当前配置';

CREATE TABLE IF NOT EXISTS `play_combochain_pools` (
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID（play_pool_catalog.id）',
  `price_fen` bigint NOT NULL DEFAULT '0' COMMENT '单次购买价格（分）',
  `config_json` json NOT NULL COMMENT '玩法配置 JSON',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`pool_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='串串赏当前配置';

CREATE TABLE IF NOT EXISTS `play_dig_pools` (
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID（play_pool_catalog.id）',
  `price_fen` bigint NOT NULL DEFAULT '0' COMMENT '单次购买价格（分）',
  `config_json` json NOT NULL COMMENT '玩法配置 JSON',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`pool_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='挖宝赏当前配置';

CREATE TABLE IF NOT EXISTS `play_groupbuy_pools` (
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID（play_pool_catalog.id）',
  `price_fen` bigint NOT NULL DEFAULT '0' COMMENT '单次购买价格（分）',
  `config_json` json NOT NULL COMMENT '玩法配置 JSON',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`pool_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='拼团赏当前配置';

-- ── 所有玩法的奖池表结构一致 ───────────────────────────────

CREATE TABLE IF NOT EXISTS `play_memorymatch_prizes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID（play_pool_catalog.id）',
  `prize_index` int NOT NULL COMMENT '奖品位序（0 起）',
  `name` varchar(128) NOT NULL COMMENT '奖档名称',
  `prize_grade` varchar(32) NOT NULL DEFAULT '' COMMENT '赏别/分类（SP/SSR/Last 等，展示用）',
  `prize_type` tinyint NOT NULL DEFAULT '1' COMMENT '奖品类型：1=虚拟 2=实物SKU 3=箱子',
  `sku_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '实物 SKU ID',
  `child_pool_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '箱中箱子赏池ID（预留）',
  `cash_fen` bigint NOT NULL DEFAULT '0' COMMENT '现金金额（分）',
  `points_amount` bigint NOT NULL DEFAULT '0' COMMENT '积分数量',
  `coupon_template_no` varchar(64) NOT NULL DEFAULT '' COMMENT '优惠券模板号',
  `weight` bigint NOT NULL DEFAULT '1' COMMENT '权重',
  `quantity` int NOT NULL DEFAULT '0' COMMENT '总库存（-1=不限量）',
  `is_prize` tinyint NOT NULL DEFAULT '1' COMMENT '是否奖品：1=有奖 0=谢谢参与',
  `image_url` varchar(512) NOT NULL DEFAULT '' COMMENT '奖档图片',
  `cost_fen` bigint NOT NULL DEFAULT '0' COMMENT '成本价（分）',
  `special_kind` varchar(32) NOT NULL DEFAULT '' COMMENT '特别赏/共享奖类型；空=普通',
  `target_value` int NOT NULL DEFAULT '-1' COMMENT '模式目标值：碰数/格子/槽位/档位等',
  `parent_index` int NOT NULL DEFAULT '-1' COMMENT '父奖品位序；-1=无',
  `total_pair_count` int NOT NULL DEFAULT '-1' COMMENT '对对碰总碰数档位；其它玩法忽略',
  `extra_json` json NOT NULL DEFAULT (JSON_OBJECT()) COMMENT '模式扩展 JSON',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_pool_index` (`pool_id`,`prize_index`),
  KEY `idx_pool_target` (`pool_id`,`target_value`),
  KEY `idx_pool_pairs` (`pool_id`,`total_pair_count`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='对对碰当前奖档';

CREATE TABLE IF NOT EXISTS `play_gashapon_prizes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID（play_pool_catalog.id）',
  `prize_index` int NOT NULL COMMENT '奖品位序（0 起）',
  `name` varchar(128) NOT NULL COMMENT '奖档名称',
  `prize_grade` varchar(32) NOT NULL DEFAULT '' COMMENT '赏别/分类（SP/SSR/Last 等，展示用）',
  `prize_type` tinyint NOT NULL DEFAULT '1' COMMENT '奖品类型：1=虚拟 2=实物SKU 3=箱子',
  `sku_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '实物 SKU ID',
  `child_pool_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '箱中箱子赏池ID（预留）',
  `cash_fen` bigint NOT NULL DEFAULT '0' COMMENT '现金金额（分）',
  `points_amount` bigint NOT NULL DEFAULT '0' COMMENT '积分数量',
  `coupon_template_no` varchar(64) NOT NULL DEFAULT '' COMMENT '优惠券模板号',
  `weight` bigint NOT NULL DEFAULT '1' COMMENT '权重',
  `quantity` int NOT NULL DEFAULT '0' COMMENT '总库存（-1=不限量）',
  `is_prize` tinyint NOT NULL DEFAULT '1' COMMENT '是否奖品：1=有奖 0=谢谢参与',
  `image_url` varchar(512) NOT NULL DEFAULT '' COMMENT '奖档图片',
  `cost_fen` bigint NOT NULL DEFAULT '0' COMMENT '成本价（分）',
  `special_kind` varchar(32) NOT NULL DEFAULT '' COMMENT '特别赏类型：LAST/MVP/LUCKY；空=普通',
  `target_value` int NOT NULL DEFAULT '-1' COMMENT 'LUCKY 顺序',
  `parent_index` int NOT NULL DEFAULT '-1' COMMENT '父奖品位序；-1=无',
  `total_pair_count` int NOT NULL DEFAULT '-1' COMMENT '对对碰总碰数档位；其它玩法忽略',
  `extra_json` json NOT NULL DEFAULT (JSON_OBJECT()) COMMENT '模式扩展 JSON',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_pool_index` (`pool_id`,`prize_index`),
  KEY `idx_pool_target` (`pool_id`,`target_value`),
  KEY `idx_pool_pairs` (`pool_id`,`total_pair_count`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='吉祥赏当前奖档';

CREATE TABLE IF NOT EXISTS `play_fullcar_prizes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID（play_pool_catalog.id）',
  `prize_index` int NOT NULL COMMENT '奖品位序（0 起）',
  `name` varchar(128) NOT NULL COMMENT '奖档名称',
  `prize_grade` varchar(32) NOT NULL DEFAULT '' COMMENT '赏别/分类（SP/SSR/Last 等，展示用）',
  `prize_type` tinyint NOT NULL DEFAULT '1' COMMENT '奖品类型：1=虚拟 2=实物SKU 3=箱子',
  `sku_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '实物 SKU ID',
  `child_pool_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '箱中箱子赏池ID（预留）',
  `cash_fen` bigint NOT NULL DEFAULT '0' COMMENT '现金金额（分）',
  `points_amount` bigint NOT NULL DEFAULT '0' COMMENT '积分数量',
  `coupon_template_no` varchar(64) NOT NULL DEFAULT '' COMMENT '优惠券模板号',
  `weight` bigint NOT NULL DEFAULT '1' COMMENT '权重',
  `quantity` int NOT NULL DEFAULT '0' COMMENT '总库存（-1=不限量）',
  `is_prize` tinyint NOT NULL DEFAULT '1' COMMENT '是否奖品：1=有奖 0=谢谢参与',
  `image_url` varchar(512) NOT NULL DEFAULT '' COMMENT '奖档图片',
  `cost_fen` bigint NOT NULL DEFAULT '0' COMMENT '成本价（分）',
  `special_kind` varchar(32) NOT NULL DEFAULT '' COMMENT 'SETTLEMENT=结算奖；空=普通',
  `target_value` int NOT NULL DEFAULT '-1' COMMENT '结算档位：1=幸运 2=普通',
  `parent_index` int NOT NULL DEFAULT '-1' COMMENT '父奖品位序；-1=无',
  `total_pair_count` int NOT NULL DEFAULT '-1' COMMENT '对对碰总碰数档位；其它玩法忽略',
  `extra_json` json NOT NULL DEFAULT (JSON_OBJECT()) COMMENT '模式扩展 JSON',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_pool_index` (`pool_id`,`prize_index`),
  KEY `idx_pool_target` (`pool_id`,`target_value`),
  KEY `idx_pool_pairs` (`pool_id`,`total_pair_count`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='满车赏当前奖档';

CREATE TABLE IF NOT EXISTS `play_combochain_prizes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID（play_pool_catalog.id）',
  `prize_index` int NOT NULL COMMENT '奖品位序（0 起）',
  `name` varchar(128) NOT NULL COMMENT '奖档名称',
  `prize_grade` varchar(32) NOT NULL DEFAULT '' COMMENT '赏别/分类（SP/SSR/Last 等，展示用）',
  `prize_type` tinyint NOT NULL DEFAULT '1' COMMENT '奖品类型：1=虚拟 2=实物SKU 3=箱子',
  `sku_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '实物 SKU ID',
  `child_pool_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '箱中箱子赏池ID（预留）',
  `cash_fen` bigint NOT NULL DEFAULT '0' COMMENT '现金金额（分）',
  `points_amount` bigint NOT NULL DEFAULT '0' COMMENT '积分数量',
  `coupon_template_no` varchar(64) NOT NULL DEFAULT '' COMMENT '优惠券模板号',
  `weight` bigint NOT NULL DEFAULT '1' COMMENT '权重',
  `quantity` int NOT NULL DEFAULT '0' COMMENT '总库存（-1=不限量）',
  `is_prize` tinyint NOT NULL DEFAULT '1' COMMENT '是否奖品：1=有奖 0=谢谢参与',
  `image_url` varchar(512) NOT NULL DEFAULT '' COMMENT '奖档图片',
  `cost_fen` bigint NOT NULL DEFAULT '0' COMMENT '成本价（分）',
  `special_kind` varchar(32) NOT NULL DEFAULT '' COMMENT 'GLOBAL/COMBO_TIER/EMPTY_STREAK/WIN_STREAK；空=普通',
  `target_value` int NOT NULL DEFAULT '-1' COMMENT '槽位/阈值等模式目标值',
  `parent_index` int NOT NULL DEFAULT '-1' COMMENT '父奖品位序；-1=无',
  `total_pair_count` int NOT NULL DEFAULT '-1' COMMENT '对对碰总碰数档位；其它玩法忽略',
  `extra_json` json NOT NULL DEFAULT (JSON_OBJECT()) COMMENT '模式扩展 JSON（连击权重/阈值等）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_pool_index` (`pool_id`,`prize_index`),
  KEY `idx_pool_target` (`pool_id`,`target_value`),
  KEY `idx_pool_pairs` (`pool_id`,`total_pair_count`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='串串赏当前奖档';

CREATE TABLE IF NOT EXISTS `play_dig_prizes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID（play_pool_catalog.id）',
  `prize_index` int NOT NULL COMMENT '奖品位序（0 起）',
  `name` varchar(128) NOT NULL COMMENT '奖档名称',
  `prize_grade` varchar(32) NOT NULL DEFAULT '' COMMENT '赏别/分类（SP/SSR/Last 等，展示用）',
  `prize_type` tinyint NOT NULL DEFAULT '1' COMMENT '奖品类型：1=虚拟 2=实物SKU 3=箱子',
  `sku_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '实物 SKU ID',
  `child_pool_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '箱中箱子赏池ID（预留）',
  `cash_fen` bigint NOT NULL DEFAULT '0' COMMENT '现金金额（分）',
  `points_amount` bigint NOT NULL DEFAULT '0' COMMENT '积分数量',
  `coupon_template_no` varchar(64) NOT NULL DEFAULT '' COMMENT '优惠券模板号',
  `weight` bigint NOT NULL DEFAULT '1' COMMENT '权重',
  `quantity` int NOT NULL DEFAULT '0' COMMENT '总库存（-1=不限量）',
  `is_prize` tinyint NOT NULL DEFAULT '1' COMMENT '是否奖品：1=有奖 0=谢谢参与',
  `image_url` varchar(512) NOT NULL DEFAULT '' COMMENT '奖档图片',
  `cost_fen` bigint NOT NULL DEFAULT '0' COMMENT '成本价（分）',
  `special_kind` varchar(32) NOT NULL DEFAULT '' COMMENT '保留字段；挖宝为空',
  `target_value` int NOT NULL DEFAULT '-1' COMMENT '挖宝格子索引（0 起）',
  `parent_index` int NOT NULL DEFAULT '-1' COMMENT '父奖品位序；-1=无',
  `total_pair_count` int NOT NULL DEFAULT '-1' COMMENT '对对碰总碰数档位；其它玩法忽略',
  `extra_json` json NOT NULL DEFAULT (JSON_OBJECT()) COMMENT '模式扩展 JSON',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_pool_index` (`pool_id`,`prize_index`),
  KEY `idx_pool_target` (`pool_id`,`target_value`),
  KEY `idx_pool_pairs` (`pool_id`,`total_pair_count`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='挖宝赏当前奖档';

CREATE TABLE IF NOT EXISTS `play_groupbuy_prizes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID（play_pool_catalog.id）',
  `prize_index` int NOT NULL COMMENT '奖品位序（0 起）',
  `name` varchar(128) NOT NULL COMMENT '奖档名称',
  `prize_grade` varchar(32) NOT NULL DEFAULT '' COMMENT '赏别/分类（SP/SSR/Last 等，展示用）',
  `prize_type` tinyint NOT NULL DEFAULT '1' COMMENT '奖品类型：1=虚拟 2=实物SKU 3=箱子',
  `sku_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '实物 SKU ID',
  `child_pool_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '箱中箱子赏池ID（预留）',
  `cash_fen` bigint NOT NULL DEFAULT '0' COMMENT '现金金额（分）',
  `points_amount` bigint NOT NULL DEFAULT '0' COMMENT '积分数量',
  `coupon_template_no` varchar(64) NOT NULL DEFAULT '' COMMENT '优惠券模板号',
  `weight` bigint NOT NULL DEFAULT '1' COMMENT '权重',
  `quantity` int NOT NULL DEFAULT '0' COMMENT '总库存（-1=不限量）',
  `is_prize` tinyint NOT NULL DEFAULT '1' COMMENT '是否奖品：1=有奖 0=谢谢参与',
  `image_url` varchar(512) NOT NULL DEFAULT '' COMMENT '奖档图片',
  `cost_fen` bigint NOT NULL DEFAULT '0' COMMENT '成本价（分）',
  `special_kind` varchar(32) NOT NULL DEFAULT '' COMMENT 'LUCKY/LUCKY_CHILD；空=普通',
  `target_value` int NOT NULL DEFAULT '-1' COMMENT 'LUCKY_CHILD 指向的父奖品位序',
  `parent_index` int NOT NULL DEFAULT '-1' COMMENT '父奖品位序；-1=无',
  `total_pair_count` int NOT NULL DEFAULT '-1' COMMENT '对对碰总碰数档位；其它玩法忽略',
  `extra_json` json NOT NULL DEFAULT (JSON_OBJECT()) COMMENT '模式扩展 JSON',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_pool_index` (`pool_id`,`prize_index`),
  KEY `idx_pool_target` (`pool_id`,`target_value`),
  KEY `idx_pool_pairs` (`pool_id`,`total_pair_count`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='拼团赏当前奖档';

-- ── 旧模型清理：新代码已切换到 play_pool_catalog + play_<mode>_pools/prizes ──
-- 旧表在切换前已清空；这里直接删除，避免两套模型并存。
DROP TABLE IF EXISTS `play_pool_versions`;
DROP TABLE IF EXISTS `play_pool_prizes`;
DROP TABLE IF EXISTS `play_pool_drafts`;
DROP TABLE IF EXISTS `play_pools`;
