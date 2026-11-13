-- ============================================================
-- 07_gacha_play.sql - 玩法域（挖宝/满车/扭蛋/连击链/拼团/记忆翻牌/发奖标记 + dtm_barrier）（gacha_play 库终态快照）
-- 表结构 = 本机 MySQL 终态导出；种子 = 历史里程碑脚本按序归并（幂等，可重复执行）。
-- 本文件由 script/split_sql.go 从旧仓 script/split/ 拆库生成。
-- 执行方式：mysql -u root < script/split/07_gacha_play.sql
-- ============================================================

SET NAMES utf8mb4;

USE `gacha_play`;

SET FOREIGN_KEY_CHECKS = 0;


CREATE TABLE IF NOT EXISTS `combo_chain_configs` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `name` varchar(64) NOT NULL COMMENT '连击链名称',
  `price_fen` bigint NOT NULL COMMENT '单票价格（分）',
  `tickets_per_car` int NOT NULL COMMENT '每车票数（2..1000）',
  `total_cars` int NOT NULL COMMENT '总车数',
  `global_slots_json` text NOT NULL COMMENT '全局奖槽 JSON：[{index,prize_kind,cash_fen,points_amount,coupon_template_no}]（每槽 1 件，按 index%total_cars+1 分车）',
  `shared_prizes_json` text NOT NULL COMMENT '共享奖 JSON：[{kind:COMBO_TIER/EMPTY_STREAK/WIN_STREAK,prize_kind,cash_fen,points_amount,coupon_template_no,quantity,tier_weight,concentrate_threshold,streak_threshold}]',
  `tier_trigger_probability_bps` int NOT NULL DEFAULT '0' COMMENT 'COMBO_TIER 触发概率（万分数 0..10000）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1=上架 0=下架',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='连击链玩法配置表';


CREATE TABLE IF NOT EXISTS `combo_chain_records` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `request_id` varchar(64) NOT NULL COMMENT '请求ID（= DTM 全局事务 gid），唯一键幂等',
  `user_id` bigint NOT NULL COMMENT '购票用户ID',
  `config_id` bigint NOT NULL COMMENT '连击链配置ID',
  `car_index` int NOT NULL COMMENT '购票车号',
  `price_fen` bigint NOT NULL COMMENT '本次实付（分）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 正常 2 已撤销',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '购票时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_request_id` (`request_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='连击链购票记录表';


CREATE TABLE IF NOT EXISTS `combo_chain_state` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `config_id` bigint NOT NULL COMMENT '连击链配置ID（uk）',
  `car_index` int NOT NULL COMMENT '当前车号（1 起）',
  `participants_json` text NOT NULL COMMENT '当前车参与用户 JSON：[userId,...]（每用户每车限 1 票）',
  `awards_json` text NOT NULL COMMENT '结算奖 JSON：[{user_id,kind:combo/global/empty_streak/win_streak,combo_index,prize_kind,cash_fen,points_amount,coupon_template_no,car_index,request_id}]',
  `tier_remaining_json` text NOT NULL COMMENT 'COMBO_TIER 剩余库存 JSON：[remaining per combo tier]',
  `streaks_before_json` text NOT NULL COMMENT '最近结算前连击快照 JSON：{userId:{empty_streak,win_streak}}（05 补偿回滚依据）',
  `remaining_tickets` int NOT NULL COMMENT '当前车剩余票数（每票-1；0=本车已满待结算）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 OPEN 2 SETTLED（全部车次完成）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_config_id` (`config_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='连击链状态表';


CREATE TABLE IF NOT EXISTS `combo_chain_streaks` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `config_id` bigint NOT NULL COMMENT '连击链配置ID',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `empty_streak` int NOT NULL DEFAULT '0' COMMENT '空手连击数（本车无全局奖且非大奖得主 +1，获奖清零）',
  `win_streak` int NOT NULL DEFAULT '0' COMMENT '中奖连击数（本车得全局奖 +1，未得清零）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_config_user` (`config_id`,`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='连击链连击计数表';


CREATE TABLE IF NOT EXISTS `dig_box_cells` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `config_id` bigint NOT NULL COMMENT '挖宝箱配置ID',
  `cell_index` int NOT NULL COMMENT '格子序号（0 起）',
  `prize_kind` tinyint NOT NULL DEFAULT '0' COMMENT '奖品类型：0 无 1 CASH 2 POINTS 3 COUPON 5 GOODS',
  `cash_fen` bigint NOT NULL DEFAULT '0' COMMENT '现金金额（分）',
  `points_amount` bigint NOT NULL DEFAULT '0' COMMENT '积分数',
  `coupon_template_no` varchar(64) NOT NULL DEFAULT '' COMMENT '券模板单号',
  `sku_id` bigint NOT NULL DEFAULT '0' COMMENT 'catalog SKU ID（prize_kind=5）',
  `item_name` varchar(64) NOT NULL DEFAULT '' COMMENT '奖品/格位展示名',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_config_cell` (`config_id`,`cell_index`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='挖宝箱格子表';


CREATE TABLE IF NOT EXISTS `dig_box_configs` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `name` varchar(128) NOT NULL DEFAULT '' COMMENT '挖宝箱名称',
  `price_fen` bigint NOT NULL COMMENT '单格价格（分）',
  `cell_count` int NOT NULL COMMENT '格子数（≤62，BIGINT 位图上限）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 ACTIVE 0 停用',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='挖宝箱配置表';


CREATE TABLE IF NOT EXISTS `dig_box_records` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `request_id` varchar(64) NOT NULL COMMENT '全局事务单号（gid，uk）',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `config_id` bigint NOT NULL COMMENT '挖宝箱配置ID',
  `session_no` varchar(96) NOT NULL DEFAULT '' COMMENT '会话单号',
  `cells_chosen` varchar(255) NOT NULL DEFAULT '' COMMENT '本次所选格子（逗号分隔，0 起）',
  `prizes_json` text NOT NULL COMMENT '揭晓奖品 JSON 数组',
  `total_price_fen` bigint NOT NULL DEFAULT '0' COMMENT '本次实付总额（分）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 正常 2 已撤销',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_request_id` (`request_id`),
  KEY `idx_user_created` (`user_id`,`created_at`),
  KEY `idx_session` (`session_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='挖宝记录表';


CREATE TABLE IF NOT EXISTS `dig_box_sessions` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `session_no` varchar(96) NOT NULL COMMENT '会话单号（"dg_"+userId+"_"+configId，uk）',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `config_id` bigint NOT NULL COMMENT '挖宝箱配置ID',
  `occupied_bitmap` bigint NOT NULL DEFAULT '0' COMMENT '已挖格位图（bit=1 已挖，cell_index 为位号）',
  `total_cells_paid` int NOT NULL DEFAULT '0' COMMENT '累计已购格数',
  `total_price_fen` bigint NOT NULL DEFAULT '0' COMMENT '累计实付（分）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 PLAYING 2 COMPLETED 3 CANCELED',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_session_no` (`session_no`),
  UNIQUE KEY `uk_user_config` (`user_id`,`config_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='挖宝会话表';


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


CREATE TABLE IF NOT EXISTS `full_car_configs` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `name` varchar(64) NOT NULL COMMENT '专列名称',
  `price_fen` bigint NOT NULL COMMENT '单抽（占一座）价格（分）',
  `seat_count` int NOT NULL COMMENT '座位数（2..1000）',
  `settlement_prizes_json` text NOT NULL COMMENT '整车结算奖 JSON：[{tier,weight,prize_kind,cash_fen,points_amount,coupon_template_no}]',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1=上架 0=下架',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='满车玩法配置表';


CREATE TABLE IF NOT EXISTS `full_car_entries` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `config_id` bigint NOT NULL COMMENT '所属满车配置ID（关联 full_car_configs.id）',
  `name` varchar(64) NOT NULL COMMENT '条目名称（奖品名或"谢谢参与"）',
  `weight` int NOT NULL COMMENT '权重（整数）',
  `quantity` int NOT NULL COMMENT '总库存：-1=不限量',
  `remaining` int NOT NULL COMMENT '剩余库存：-1=不限量；0=售罄不参与抽取',
  `is_prize` tinyint NOT NULL DEFAULT '1' COMMENT '是否奖品：1=中奖 0=谢谢参与',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_config_id` (`config_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='满车奖池条目表';


CREATE TABLE IF NOT EXISTS `full_car_records` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `request_id` varchar(64) NOT NULL COMMENT '请求ID（= DTM 全局事务 gid），唯一键幂等',
  `user_id` bigint NOT NULL COMMENT '抽奖用户ID',
  `config_id` bigint NOT NULL COMMENT '满车配置ID',
  `seat_index` int NOT NULL COMMENT '分配座位（0 起）',
  `entry_name` varchar(64) NOT NULL COMMENT 'regular 奖条目名称',
  `is_prize` tinyint NOT NULL COMMENT '是否中奖：1=是 0=谢谢参与',
  `price_fen` bigint NOT NULL COMMENT '本次实付（分）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 正常 2 已撤销',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '开奖时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_request_id` (`request_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='满车抽奖记录表';


CREATE TABLE IF NOT EXISTS `prize_grant_marks` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `request_id` varchar(64) NOT NULL COMMENT '请求ID（= DTM 全局事务 gid），唯一键幂等',
  `user_id` bigint NOT NULL COMMENT '受益用户ID',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '标记时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_request_id` (`request_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='SAGA 06 发奖落库标记表（04 补偿退款前置检查：已发奖则不退钱）';


CREATE TABLE IF NOT EXISTS `full_car_state` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `config_id` bigint NOT NULL COMMENT '满车配置ID（uk）',
  `seats_json` text NOT NULL COMMENT '座位 JSON：[userId,...] 长度=seat_count，0=空位',
  `awards_json` text NOT NULL COMMENT '整车结算奖 JSON：[{seat_index,user_id,prize_kind,cash_fen,points_amount,coupon_template_no,tier,request_id}]',
  `remaining_tickets` int NOT NULL COMMENT '剩余票数（每抽-1；0=满员已出发）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 OPEN 2 DEPARTED（已出发/已结算）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_config_id` (`config_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='满车状态表';


CREATE TABLE IF NOT EXISTS `gashapon_configs` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `name` varchar(64) NOT NULL COMMENT '扭蛋机名称',
  `price_fen` bigint NOT NULL COMMENT '单抽价格（分）',
  `special_prizes_json` text NOT NULL COMMENT '特别奖 JSON：[{prize_index,special_kind,prize_kind,cash_fen,points_amount,coupon_template_no,quantity,stage_order}]',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1=上架 0=下架',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='扭蛋玩法配置表';


CREATE TABLE IF NOT EXISTS `gashapon_entries` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `config_id` bigint NOT NULL COMMENT '所属扭蛋配置ID（关联 gashapon_configs.id）',
  `name` varchar(64) NOT NULL COMMENT '条目名称（奖品名或"谢谢参与"）',
  `weight` int NOT NULL COMMENT '权重（整数）',
  `quantity` int NOT NULL COMMENT '总库存（必须有限量）',
  `remaining` int NOT NULL COMMENT '剩余库存：0=售罄不参与抽取',
  `is_prize` tinyint NOT NULL DEFAULT '1' COMMENT '是否奖品：1=中奖 0=谢谢参与',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_config_id` (`config_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='扭蛋奖池条目表';


CREATE TABLE IF NOT EXISTS `gashapon_lucky_assignments` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `config_id` bigint NOT NULL COMMENT '扭蛋配置ID',
  `prize_index` int NOT NULL COMMENT '特别奖下标（仅 LUCKY 可指派）',
  `user_id` bigint NOT NULL COMMENT '指派用户ID',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_config_prize` (`config_id`,`prize_index`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='扭蛋幸运指派表';


CREATE TABLE IF NOT EXISTS `gashapon_records` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `request_id` varchar(64) NOT NULL COMMENT '请求ID（= DTM 全局事务 gid），唯一键幂等',
  `user_id` bigint NOT NULL COMMENT '抽奖用户ID',
  `config_id` bigint NOT NULL COMMENT '扭蛋配置ID',
  `count` int NOT NULL COMMENT '本次抽数',
  `entries_json` text NOT NULL COMMENT 'regular 奖 JSON：[{name,is_prize}]',
  `price_fen` bigint NOT NULL COMMENT '本次实付（分）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 正常 2 已撤销',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '开奖时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_request_id` (`request_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='扭蛋抽奖记录表';


CREATE TABLE IF NOT EXISTS `gashapon_state` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `config_id` bigint NOT NULL COMMENT '扭蛋配置ID（uk）',
  `participants_json` text NOT NULL COMMENT '参与次数 JSON：{userId: count}',
  `records_json` text NOT NULL COMMENT '参与记录 JSON：[{user_id,request_id,count}]（按序=票据区间）',
  `sent_json` text NOT NULL COMMENT '已发数量 JSON：[sent per prize_index]',
  `awards_json` text NOT NULL COMMENT '特别奖 JSON：[{prize_index,unit,user_id,special_kind,prize_kind,cash_fen,points_amount,coupon_template_no,request_id}]',
  `remaining_tickets` int NOT NULL COMMENT '剩余票数',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 OPEN 2 SETTLED（已结算）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_config_id` (`config_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='扭蛋状态表';


CREATE TABLE IF NOT EXISTS `group_buy_configs` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `name` varchar(64) NOT NULL COMMENT '拼团名称',
  `price_fen` bigint NOT NULL COMMENT '单注价格（分）',
  `target_bet_count` int NOT NULL COMMENT '成团注数（2..1000）',
  `per_bet_award_limit` int NOT NULL DEFAULT '0' COMMENT '每注奖励上限（0=默认 slotCount-target+1）',
  `lucky_prizes_json` text NOT NULL COMMENT '幸运奖 JSON：[{prize_index,prize_kind,cash_fen,points_amount,coupon_template_no,quantity}]',
  `lucky_children_json` text NOT NULL COMMENT '幸运子奖 JSON：[{lucky_index,prize_kind,cash_fen,points_amount,coupon_template_no,quantity}]（随幸运奖逐件发放）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1=上架 0=下架',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='拼团抽玩法配置表';


CREATE TABLE IF NOT EXISTS `group_buy_entries` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `config_id` bigint NOT NULL COMMENT '所属拼团配置ID（关联 group_buy_configs.id）',
  `name` varchar(64) NOT NULL COMMENT '条目名称（奖品名或"谢谢参与"）',
  `weight` int NOT NULL COMMENT '权重（整数）',
  `quantity` int NOT NULL COMMENT '总库存（普通槽展开基准）',
  `remaining` int NOT NULL COMMENT '剩余库存：0=不参与展开',
  `is_prize` tinyint NOT NULL DEFAULT '1' COMMENT '是否奖品：1=中奖 0=谢谢参与',
  `parent_entry_id` bigint NOT NULL DEFAULT '0' COMMENT '父条目ID：0=普通条目；!=0=子奖（随父条目发放）',
  `prize_kind` int NOT NULL DEFAULT '0' COMMENT '子奖资产类型：1 CASH 2 POINTS 3 COUPON（普通条目=0 仅展示）',
  `cash_fen` bigint NOT NULL DEFAULT '0' COMMENT '子奖现金（分）',
  `points_amount` bigint NOT NULL DEFAULT '0' COMMENT '子奖积分',
  `coupon_template_no` varchar(64) NOT NULL DEFAULT '' COMMENT '子奖券模板单号',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_config_id` (`config_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='拼团奖池条目表';


CREATE TABLE IF NOT EXISTS `group_buy_records` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `request_id` varchar(64) NOT NULL COMMENT '请求ID（= DTM 全局事务 gid），唯一键幂等',
  `user_id` bigint NOT NULL COMMENT '下注用户ID',
  `config_id` bigint NOT NULL COMMENT '拼团配置ID',
  `count` int NOT NULL COMMENT '本次下注数',
  `price_fen` bigint NOT NULL COMMENT '本次实付（分）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 正常 2 已撤销',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '下注时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_request_id` (`request_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='拼团下注记录表';


CREATE TABLE IF NOT EXISTS `group_buy_state` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `config_id` bigint NOT NULL COMMENT '拼团配置ID（uk）',
  `bets_json` text NOT NULL COMMENT '下注记录 JSON：[{user_id,draw_id,count}]（按序）',
  `awards_json` text NOT NULL COMMENT '结算奖 JSON：[{user_id,kind:normal/child/lucky,entry_id,lucky_index,prize_kind,cash_fen,points_amount,coupon_template_no,request_id}]',
  `lucky_sent_json` text NOT NULL COMMENT '幸运奖已发 JSON：[sent per lucky prize index]',
  `remaining_bets` int NOT NULL COMMENT '剩余注数（0=成团已结算）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 OPEN 2 SETTLED（已成团结算）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_config_id` (`config_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='拼团抽状态表';


CREATE TABLE IF NOT EXISTS `memory_match_appearance_entitlements` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `config_id` bigint NOT NULL COMMENT '配置ID',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `preset_id` int NOT NULL COMMENT '正面预设ID（背面预设不校验权益，同老项目 face 权益语义）',
  `granted_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '发放时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_config_user_preset` (`config_id`,`user_id`,`preset_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='记忆翻牌外观权益表';


CREATE TABLE IF NOT EXISTS `memory_match_appearance_selections` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `config_id` bigint NOT NULL COMMENT '配置ID',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `face_preset_id` int NOT NULL COMMENT '正面预设ID',
  `back_preset_id` int NOT NULL COMMENT '背面预设ID',
  `version` int NOT NULL DEFAULT '1' COMMENT '乐观锁版本',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_config_user` (`config_id`,`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='记忆翻牌外观选择表';


CREATE TABLE IF NOT EXISTS `memory_match_champion` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `config_id` bigint NOT NULL COMMENT '配置ID（uk）',
  `user_id` bigint NOT NULL COMMENT '现任擂主用户ID',
  `total_pair_count` int NOT NULL COMMENT '现任擂主总配对数',
  `pending_income_minor` bigint NOT NULL DEFAULT '0' COMMENT '待结算分成（分，被换擂时全额打款）',
  `version` int NOT NULL DEFAULT '1' COMMENT '乐观锁版本',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_config_id` (`config_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='记忆翻牌擂主表';


CREATE TABLE IF NOT EXISTS `memory_match_champion_events` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `session_id` bigint NOT NULL COMMENT '会话ID（uk，终局擂主判定幂等）',
  `user_id` bigint NOT NULL COMMENT '玩家用户ID',
  `total_pair_count` int NOT NULL COMMENT '该局总配对数',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_session_id` (`session_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='记忆翻牌擂主判定事件表';


CREATE TABLE IF NOT EXISTS `memory_match_champion_payouts` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `config_id` bigint NOT NULL COMMENT '配置ID',
  `user_id` bigint NOT NULL COMMENT '被换擂的擂主（收款人）',
  `amount_minor` bigint NOT NULL COMMENT '待结算分成（分）',
  `grant_no` varchar(80) NOT NULL COMMENT '幂等发放单号 = mm-champion:<被换擂时的会话ID>',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 PENDING 2 PAID',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_grant_no` (`grant_no`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='记忆翻牌擂主打款流水表';


CREATE TABLE IF NOT EXISTS `memory_match_configs` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `name` varchar(64) NOT NULL COMMENT '玩法名称',
  `rounds_per_purchase` int NOT NULL COMMENT '每次购买回合数（1..100）',
  `max_pair_count` int NOT NULL COMMENT '每回合配对数上限（1..27）',
  `purchase_price_minor` bigint NOT NULL COMMENT '购买价格（分）',
  `wallet_kind` tinyint NOT NULL DEFAULT '1' COMMENT '支付钱包种类：1=现金（本仓仅现金）',
  `second_destiny_enabled` tinyint NOT NULL DEFAULT '0' COMMENT '第二命运开关：1=开 0=关',
  `deck_strategy` tinyint NOT NULL DEFAULT '1' COMMENT '发牌策略：0 UNSPECIFIED 1 DYNAMIC 2 EXACT',
  `pattern_weights_json` text NOT NULL COMMENT 'DYNAMIC 发牌 9 图案权重 JSON：[w1..w9]（EXACT 忽略）',
  `exact_min_pair_count` int NOT NULL DEFAULT '0' COMMENT 'EXACT 策略目标配对数下限',
  `exact_max_pair_count` int NOT NULL DEFAULT '0' COMMENT 'EXACT 策略目标配对数上限',
  `exact_attempt_limit` int NOT NULL DEFAULT '32' COMMENT 'EXACT 试发次数上限（默认 32，上限 64）',
  `face_presets_json` text NOT NULL COMMENT '正面预设 JSON：[{presetId,name,faces:[{patternId,name,imageUrl}]×9}]',
  `back_presets_json` text NOT NULL COMMENT '背面预设 JSON：[{presetId,name,backImageUrl,faceBackgroundImageUrl}]',
  `default_face_preset_id` int NOT NULL DEFAULT '1' COMMENT '默认正面预设',
  `default_back_preset_id` int NOT NULL DEFAULT '1' COMMENT '默认背面预设',
  `champion_min_total_pairs` int NOT NULL DEFAULT '0' COMMENT '擂主门槛：总配对数（0=无擂主玩法）',
  `champion_share_bps` int NOT NULL DEFAULT '0' COMMENT '擂主分成 bps（每次购买价×bps/10000 计入现任 pending）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1=上架 0=下架',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='记忆翻牌玩法配置表';


CREATE TABLE IF NOT EXISTS `memory_match_entries` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `config_id` bigint NOT NULL COMMENT '所属配置ID（关联 memory_match_configs.id）',
  `name` varchar(64) NOT NULL COMMENT '奖档名称（奖品名或"谢谢参与"）',
  `total_pair_count` int NOT NULL COMMENT '终局总配对数档位（精确匹配）',
  `weight` int NOT NULL DEFAULT '1' COMMENT '权重（老项目 FIXED_SKU 恒 1）',
  `prize_kind` tinyint NOT NULL DEFAULT '0' COMMENT '奖励种类：1=现金 2=积分 3=券 0=无（谢谢参与）',
  `cash_fen` bigint NOT NULL DEFAULT '0' COMMENT '现金奖励（分，prize_kind=1）',
  `points_amount` bigint NOT NULL DEFAULT '0' COMMENT '积分奖励（prize_kind=2）',
  `coupon_template_no` varchar(64) NOT NULL DEFAULT '' COMMENT '券模板号（prize_kind=3）',
  `quantity` int NOT NULL COMMENT '总库存：-1=不限量',
  `remaining` int NOT NULL COMMENT '剩余库存：-1=不限量；0=售罄不发放',
  `is_prize` tinyint NOT NULL DEFAULT '1' COMMENT '是否奖品：1=中奖 0=谢谢参与',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_config_total` (`config_id`,`total_pair_count`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='记忆翻牌奖档表';


CREATE TABLE IF NOT EXISTS `memory_match_rounds` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `session_id` bigint NOT NULL COMMENT '会话ID',
  `round_no` int NOT NULL COMMENT '回合号（1 起）',
  `state` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 READY 2 PLAYING 3 RESOLVED 4 SKIPPED',
  `state_json` text NOT NULL COMMENT '回合进度 JSON（deck/initial_hand/seed_commitment/draw_position/available_destiny/destiny_draws/pair_count/actions）',
  `pair_count` int NOT NULL DEFAULT '0' COMMENT '本回合配对数',
  `result_json` text COMMENT '回合结果 JSON（RESOLVED/SKIPPED 时）',
  `result_hash` char(64) DEFAULT NULL COMMENT '回合结果哈希 hex（SHA256(result_json 不含 hash)）',
  `version` int NOT NULL DEFAULT '1' COMMENT '乐观锁版本',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_session_round` (`session_id`,`round_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='记忆翻牌回合表';


CREATE TABLE IF NOT EXISTS `memory_match_sessions` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '会话ID（自增主键）',
  `user_id` bigint NOT NULL COMMENT '购买用户ID',
  `config_id` bigint NOT NULL COMMENT '配置ID',
  `request_id` varchar(64) NOT NULL COMMENT '请求ID（= DTM gid），唯一键幂等',
  `engine_seed` char(64) DEFAULT NULL COMMENT '回合随机引擎种子 hex（购买时 crypto/rand 32B，仅服务端）',
  `engine_version` smallint NOT NULL DEFAULT '1' COMMENT '引擎版本（恒 1）',
  `purchased_rounds` int NOT NULL COMMENT '购买回合数（1..100）',
  `purchase_price_minor` bigint NOT NULL COMMENT '购买价格（分，快照）',
  `rules_json` text NOT NULL COMMENT '规则快照 JSON（购买时配置原样）',
  `appearance_json` text NOT NULL COMMENT '外观快照 JSON（face/back preset）',
  `destiny_pattern_id` int NOT NULL DEFAULT '0' COMMENT '命运图案（1..9；0=未选）',
  `second_destiny_enabled` tinyint NOT NULL DEFAULT '0' COMMENT '第二命运开关（快照）',
  `second_destiny_pattern_id` int NOT NULL DEFAULT '0' COMMENT '第二命运图案（0=未选）',
  `completed_rounds` int NOT NULL DEFAULT '0' COMMENT '已完成回合数',
  `current_round` int NOT NULL DEFAULT '1' COMMENT '当前回合号',
  `skip_requested` tinyint NOT NULL DEFAULT '0' COMMENT '跳过请求标记',
  `final_result_hash` char(64) DEFAULT NULL COMMENT '终局结果哈希 hex（COMPLETED 后）',
  `champion_credit_user_id` bigint NOT NULL DEFAULT '0' COMMENT '购买时计入分成的擂主用户ID（0=当时无擂主；05 Revert 回扣校验用）',
  `champion_share_minor` bigint NOT NULL DEFAULT '0' COMMENT '本次购买计入擂主的分成（分；05 Revert 等额回扣用）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 PENDING 2 AWAITING_DESTINY 3 AWAITING_SECOND_DESTINY 4 PLAYING 5 SETTLING 6 COMPLETED 7 EXPIRED',
  `version` int NOT NULL DEFAULT '1' COMMENT '乐观锁版本',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_request_id` (`request_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='记忆翻牌会话表';


CREATE TABLE IF NOT EXISTS `memory_match_settlements` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `session_id` bigint NOT NULL COMMENT '会话ID（uk）',
  `result_hash` char(64) NOT NULL COMMENT '终局结果哈希 hex（final_hash）',
  `reward_json` text NOT NULL COMMENT '奖励 JSON（{totalPairCount, entry 快照}）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1 待发奖 2 已发奖',
  `version` int NOT NULL DEFAULT '1' COMMENT '乐观锁版本',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_session_id` (`session_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='记忆翻牌终局结算表';


-- 挖宝箱 ×1：9 格 100 分/格；格 0 现金100、格 1 积分50、格 2 券 T-FIX-500、格 3 空、
-- 格 4 现金500、格 5 积分100、格 6 空、格 7 券 T-PCT-5000、格 8 现金1000
-- （全开 9 格 = 实付 900 分，奖品 1600 分现金 + 150 积分 + 2 券）
INSERT INTO `dig_box_configs` (`id`, `name`, `price_fen`, `cell_count`, `status`)
VALUES (1, '挖宝箱·初级', 100, 9, 1)
ON DUPLICATE KEY UPDATE
    name=VALUES(name), price_fen=VALUES(price_fen), cell_count=VALUES(cell_count), status=VALUES(status);


INSERT INTO `dig_box_cells` (`config_id`, `cell_index`, `prize_kind`, `cash_fen`, `points_amount`, `coupon_template_no`)
VALUES (1, 0, 1, 100, 0, ''),
       (1, 1, 2, 0, 50, ''),
       (1, 2, 3, 0, 0, 'T-FIX-500'),
       (1, 3, 0, 0, 0, ''),
       (1, 4, 1, 500, 0, ''),
       (1, 5, 2, 0, 100, ''),
       (1, 6, 0, 0, 0, ''),
       (1, 7, 3, 0, 0, 'T-PCT-5000'),
       (1, 8, 1, 1000, 0, '')
ON DUPLICATE KEY UPDATE
    prize_kind=VALUES(prize_kind), cash_fen=VALUES(cash_fen),
    points_amount=VALUES(points_amount), coupon_template_no=VALUES(coupon_template_no);


-- 满车 1 号：4 座专列；整车结算 LUCKY 层=现金1000+积分500、NORMAL 层=积分100+券T-FIX-500
INSERT INTO `full_car_configs` (`id`, `name`, `price_fen`, `seat_count`, `settlement_prizes_json`, `status`) VALUES
    (1, '满车·福气专列', 100, 4,
     '[{"tier":1,"weight":1,"prize_kind":1,"cash_fen":1000},{"tier":1,"weight":1,"prize_kind":2,"points_amount":500},{"tier":2,"weight":1,"prize_kind":2,"points_amount":100},{"tier":2,"weight":1,"prize_kind":3,"coupon_template_no":"T-FIX-500"}]',
     1)
ON DUPLICATE KEY UPDATE `id` = `id`;


-- 满车条目种子（本表无业务唯一键：DELETE 种子范围后插入，保证幂等）
DELETE FROM `full_car_entries` WHERE `config_id` = 1;

INSERT INTO `full_car_entries` (`config_id`, `name`, `weight`, `quantity`, `remaining`, `is_prize`) VALUES
    (1, '谢谢参与',   30, -1,   -1,   0),
    (1, '金币×50',    30, 5000, 5000, 1),
    (1, '限定皮肤',   25, 100,  100,  1),
    (1, '限定头像框', 15, 100,  100,  1);


-- 满车状态：4 票（每抽一票，最后一抽触发整车结算）；seats 须为 seat_count 长度的 0 数组（Lock 分支强校验长度）
INSERT INTO `full_car_state` (`config_id`, `seats_json`, `awards_json`, `remaining_tickets`, `status`) VALUES
    (1, '[0,0,0,0]', '[]', 4, 1)
ON DUPLICATE KEY UPDATE `config_id` = `config_id`;


-- 扭蛋 1 号：总票数 6。特别奖 LAST 现金5000 q1 / MVP 积分2000 q1 / LUCKY 现金1000 q2 分阶段 stage1（阈值 T1=3、T2=6）
INSERT INTO `gashapon_configs` (`id`, `name`, `price_fen`, `special_prizes_json`, `status`) VALUES
    (1, '扭蛋·欧皇许愿机', 100,
     '[{"prize_index":0,"special_kind":"LAST","prize_kind":1,"cash_fen":5000,"quantity":1,"stage_order":0},{"prize_index":1,"special_kind":"MVP","prize_kind":2,"points_amount":2000,"quantity":1,"stage_order":0},{"prize_index":2,"special_kind":"LUCKY","prize_kind":1,"cash_fen":1000,"quantity":2,"stage_order":1}]',
     1)
ON DUPLICATE KEY UPDATE `id` = `id`;


-- 扭蛋条目种子（本表无业务唯一键：DELETE 种子范围后插入，保证幂等）
DELETE FROM `gashapon_entries` WHERE `config_id` = 1;

INSERT INTO `gashapon_entries` (`config_id`, `name`, `weight`, `quantity`, `remaining`, `is_prize`) VALUES
    (1, '谢谢参与', 40, 2, 2, 0),
    (1, '金币×50',  30, 2, 2, 1),
    (1, '限定皮肤', 20, 1, 1, 1),
    (1, '限定头像框', 10, 1, 1, 1);


-- 扭蛋状态：6 票
INSERT INTO `gashapon_state` (`config_id`, `participants_json`, `records_json`, `sent_json`, `awards_json`, `remaining_tickets`, `status`) VALUES
    (1, '{}', '[]', '[0,0,0]', '[]', 6, 1)
ON DUPLICATE KEY UPDATE `config_id` = `config_id`;


-- 连击链 1 号：3 票/车 × 2 车。全局奖 4 槽（槽 0/1 → 车 1；槽 2/3 → 车 2）；
-- COMBO_TIER 现金 5000 q1（trigger 100% 触发、concentrate_threshold=1）；
-- EMPTY_STREAK 积分 100 阈值 2；WIN_STREAK 现金 300 阈值 1（连击奖无限量，quantity 仅展示）
INSERT INTO `combo_chain_configs` (`id`, `name`, `price_fen`, `tickets_per_car`, `total_cars`, `global_slots_json`, `shared_prizes_json`, `tier_trigger_probability_bps`, `status`) VALUES
    (1, '连击链·福运连车', 100, 3, 2,
     '[{"index":0,"prize_kind":1,"cash_fen":500},{"index":1,"prize_kind":2,"points_amount":100},{"index":2,"prize_kind":1,"cash_fen":500},{"index":3,"prize_kind":3,"coupon_template_no":"T-FIX-500"}]',
     '[{"kind":"COMBO_TIER","prize_kind":1,"cash_fen":5000,"quantity":1,"tier_weight":1,"concentrate_threshold":1},{"kind":"EMPTY_STREAK","prize_kind":2,"points_amount":100,"quantity":-1,"streak_threshold":2},{"kind":"WIN_STREAK","prize_kind":1,"cash_fen":300,"quantity":-1,"streak_threshold":1}]',
     10000, 1)
ON DUPLICATE KEY UPDATE `id` = `id`;


-- 连击链状态：车 1、3 票；COMBO_TIER 剩余 1
INSERT INTO `combo_chain_state` (`config_id`, `car_index`, `participants_json`, `awards_json`, `tier_remaining_json`, `streaks_before_json`, `remaining_tickets`, `status`) VALUES
    (1, 1, '[]', '[]', '[1]', '{}', 3, 1)
ON DUPLICATE KEY UPDATE `config_id` = `config_id`;


-- 拼团 1 号：3 注成团。幸运奖 现金 1000 q1 + 幸运子奖 积分 50 q1；
-- 条目 5 槽（谢谢×2 金币×2 皮肤×1）+ 皮肤子奖 积分 100 q1（父条目抽中即发）
INSERT INTO `group_buy_configs` (`id`, `name`, `price_fen`, `target_bet_count`, `per_bet_award_limit`, `lucky_prizes_json`, `lucky_children_json`, `status`) VALUES
    (1, '拼团·三人成团', 100, 3, 0,
     '[{"prize_index":0,"prize_kind":1,"cash_fen":1000,"quantity":1}]',
     '[{"lucky_index":0,"prize_kind":2,"points_amount":50,"quantity":1}]',
     1)
ON DUPLICATE KEY UPDATE `id` = `id`;


INSERT INTO `group_buy_entries` (`id`, `config_id`, `name`, `weight`, `quantity`, `remaining`, `is_prize`, `parent_entry_id`, `prize_kind`, `cash_fen`, `points_amount`, `coupon_template_no`) VALUES
    (1, 1, '谢谢参与',  30, 2, 2, 0, 0, 0, 0,   0, ''),
    (2, 1, '金币×50',   30, 2, 2, 1, 0, 0, 0,   0, ''),
    (3, 1, '限定皮肤',  20, 1, 1, 1, 0, 0, 0,   0, ''),
    (4, 1, '皮肤碎片',  0,  1, 1, 1, 3, 2, 0, 100, '')
ON DUPLICATE KEY UPDATE `id` = `id`;


-- 拼团状态：3 注成团；幸运奖 sent=[0]
INSERT INTO `group_buy_state` (`config_id`, `bets_json`, `awards_json`, `lucky_sent_json`, `remaining_bets`, `status`) VALUES
    (1, '[]', '[]', '[0]', 3, 1)
ON DUPLICATE KEY UPDATE `config_id` = `config_id`;


-- ============================================================
-- 种子：配置 1「记忆翻牌·命运牌局」（3 轮×3 对、100 分、DYNAMIC、第二命运开、擂主 5 对/5%）
-- ============================================================
INSERT INTO `memory_match_configs`
    (`id`, `name`, `rounds_per_purchase`, `max_pair_count`, `purchase_price_minor`, `wallet_kind`,
     `second_destiny_enabled`, `deck_strategy`, `pattern_weights_json`,
     `exact_min_pair_count`, `exact_max_pair_count`, `exact_attempt_limit`,
     `face_presets_json`, `back_presets_json`, `default_face_preset_id`, `default_back_preset_id`,
     `champion_min_total_pairs`, `champion_share_bps`, `status`)
VALUES
    (1, '记忆翻牌·命运牌局', 3, 3, 100, 1,
     1, 1, '[10,10,10,10,10,10,10,10,10]',
     0, 0, 32,
     '[{"presetId":1,"name":"经典扑克","faces":[{"patternId":1,"name":"樱桃","imageUrl":"/img/mm/face-1-1.svg"},{"patternId":2,"name":"铃铛","imageUrl":"/img/mm/face-1-2.svg"},{"patternId":3,"name":"星星","imageUrl":"/img/mm/face-1-3.svg"},{"patternId":4,"name":"月亮","imageUrl":"/img/mm/face-1-4.svg"},{"patternId":5,"name":"皇冠","imageUrl":"/img/mm/face-1-5.svg"},{"patternId":6,"name":"钻石","imageUrl":"/img/mm/face-1-6.svg"},{"patternId":7,"name":"闪电","imageUrl":"/img/mm/face-1-7.svg"},{"patternId":8,"name":"彩虹","imageUrl":"/img/mm/face-1-8.svg"},{"patternId":9,"name":"幸运草","imageUrl":"/img/mm/face-1-9.svg"}]},{"presetId":2,"name":"霓虹幻彩","faces":[{"patternId":1,"name":"霓虹樱桃","imageUrl":"/img/mm/face-2-1.svg"},{"patternId":2,"name":"霓虹铃铛","imageUrl":"/img/mm/face-2-2.svg"},{"patternId":3,"name":"霓虹星星","imageUrl":"/img/mm/face-2-3.svg"},{"patternId":4,"name":"霓虹月亮","imageUrl":"/img/mm/face-2-4.svg"},{"patternId":5,"name":"霓虹皇冠","imageUrl":"/img/mm/face-2-5.svg"},{"patternId":6,"name":"霓虹钻石","imageUrl":"/img/mm/face-2-6.svg"},{"patternId":7,"name":"霓虹闪电","imageUrl":"/img/mm/face-2-7.svg"},{"patternId":8,"name":"霓虹彩虹","imageUrl":"/img/mm/face-2-8.svg"},{"patternId":9,"name":"霓虹幸运草","imageUrl":"/img/mm/face-2-9.svg"}]}]',
     '[{"presetId":1,"name":"深蓝卡背","backImageUrl":"/img/mm/back-1.svg","faceBackgroundImageUrl":"/img/mm/bg-1.svg"},{"presetId":2,"name":"鎏金卡背","backImageUrl":"/img/mm/back-2.svg","faceBackgroundImageUrl":"/img/mm/bg-2.svg"}]',
     1, 1,
     5, 500, 1)
ON DUPLICATE KEY UPDATE `name` = VALUES(`name`);


-- 奖档：9 对→限定皮肤（现金 5000 分）/ 6 对→头像框（积分 500）/ 3 对→金币（现金 100）/ 0 对→谢谢参与
INSERT INTO `memory_match_entries`
    (`config_id`, `name`, `total_pair_count`, `weight`, `prize_kind`, `cash_fen`, `points_amount`, `coupon_template_no`, `quantity`, `remaining`, `is_prize`)
VALUES
    (1, '限定皮肤·星夜', 9, 1, 1, 5000, 0, '', 10, 10, 1),
    (1, '头像框·霓虹',   6, 1, 2, 0,    500, '', 50, 50, 1),
    (1, '金币×100',     3, 1, 1, 100,  0, '', 100, 100, 1),
    (1, '谢谢参与',      0, 1, 0, 0,    0, '', -1, -1, 0)
ON DUPLICATE KEY UPDATE `name` = VALUES(`name`);

-- ============================================================
-- 统一 play 赏池/奖品模型（所有玩法共用；发布时投影到各玩法运行表）
-- ============================================================

CREATE TABLE IF NOT EXISTS `play_pool_heads` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '赏池ID（从 1000000 开始）',
  `mode` varchar(32) NOT NULL COMMENT '玩法：gashapon/fullcar/combochain/dig/groupbuy/memorymatch',
  `name` varchar(128) NOT NULL DEFAULT '' COMMENT '主题名称',
  `next_version` bigint unsigned NOT NULL DEFAULT '1' COMMENT '下一版本号',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mode_id` (`mode`,`id`)
) ENGINE=InnoDB AUTO_INCREMENT=1000000 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='统一玩赏池头表';

CREATE TABLE IF NOT EXISTS `play_pool_versions` (
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID',
  `version` bigint unsigned NOT NULL COMMENT '版本号（从 1 递增）',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '1=DRAFT 2=PUBLISHED 3=RETIRED',
  `mode` varchar(32) NOT NULL COMMENT '玩法',
  `name` varchar(128) NOT NULL DEFAULT '' COMMENT '主题名称（快照）',
  `price_fen` bigint NOT NULL DEFAULT '0' COMMENT '单次价格（分）',
  `config_json` json NOT NULL COMMENT '玩法规则 JSON（座位数/车数/连击规则等）',
  `config_hash` char(64) NOT NULL DEFAULT '' COMMENT '配置哈希 SHA256',
  `published_at` datetime DEFAULT NULL COMMENT '发布时间',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`pool_id`,`version`),
  KEY `idx_status_published` (`status`,`published_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='统一玩赏池版本表';

CREATE TABLE IF NOT EXISTS `play_pool_prizes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `pool_id` bigint unsigned NOT NULL COMMENT '赏池ID',
  `version` bigint unsigned NOT NULL COMMENT '版本号',
  `prize_index` int NOT NULL COMMENT '奖品位序（0 起）',
  `prize_grade` varchar(32) NOT NULL DEFAULT '' COMMENT '赏别：SP5/A/B/LAST...',
  `special_kind` varchar(16) NOT NULL DEFAULT '' COMMENT '特别赏类型：LAST/MVP/LUCKY（空=普通赏）',
  `prize_type` tinyint NOT NULL DEFAULT '1' COMMENT '1=虚拟 2=实物SKU 3=箱子',
  `name` varchar(128) NOT NULL COMMENT '奖品名（快照）',
  `sku_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '实物 SKU ID',
  `child_pool_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '箱中箱子赏池ID',
  `weight` bigint NOT NULL DEFAULT '1' COMMENT '权重',
  `quantity` int NOT NULL DEFAULT '0' COMMENT '总数量（-1=无限）',
  `remaining` int NOT NULL DEFAULT '0' COMMENT '剩余数量',
  `is_prize` tinyint NOT NULL DEFAULT '1' COMMENT '1=中奖 0=谢谢参与',
  `target_value` int NOT NULL DEFAULT '-1' COMMENT '目标值（记忆翻牌=碰数）',
  `cash_fen` bigint NOT NULL DEFAULT '0' COMMENT '现金（分）',
  `points_amount` bigint NOT NULL DEFAULT '0' COMMENT '积分',
  `coupon_template_no` varchar(64) NOT NULL DEFAULT '' COMMENT '券模板号',
  `cost_fen` bigint NOT NULL DEFAULT '0' COMMENT '成本价（分）',
  `image_url` varchar(255) NOT NULL DEFAULT '' COMMENT '缩略图',
  `parent_index` int NOT NULL DEFAULT '-1' COMMENT '父奖品位序（-1=无；拼团子奖等）',
  `extra_json` json NOT NULL DEFAULT (JSON_OBJECT()) COMMENT '玩法特有参数 JSON（连击阈值等）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_pool_version_index` (`pool_id`,`version`,`prize_index`),
  KEY `idx_pool_version` (`pool_id`,`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='统一玩赏品表';

CREATE TABLE IF NOT EXISTS `play_pool_drafts` (
  `idempotency_key` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  `pool_id` bigint unsigned NOT NULL,
  `version` bigint unsigned NOT NULL,
  `request_hash` char(64) NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`idempotency_key`),
  KEY `idx_pool_version` (`pool_id`,`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='统一玩赏池草稿幂等表';

SET FOREIGN_KEY_CHECKS = 1;
