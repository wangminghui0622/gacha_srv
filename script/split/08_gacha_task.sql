-- ============================================================
-- 08_gacha_task.sql - 任务域（任务进度/事件/签到/通行证/会员 + dtm_barrier）（gacha_task 库终态快照）
-- 表结构 = 本机 MySQL 终态导出；种子 = 历史里程碑脚本按序归并（幂等，可重复执行）。
-- 本文件由 script/split_sql.go 从旧仓 script/split/ 拆库生成。
-- 执行方式：mysql -u root < script/split/08_gacha_task.sql
-- ============================================================

SET NAMES utf8mb4;

USE `gacha_task`;

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


CREATE TABLE IF NOT EXISTS `task_claim` (
  `request_id` varchar(64) NOT NULL COMMENT '领取幂等键',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `task_id` bigint NOT NULL COMMENT '任务ID',
  `reward_template_id` varchar(64) NOT NULL COMMENT '奖励模板ID（legacy 字符串，不实物发放）',
  `reward_count` int unsigned NOT NULL COMMENT '奖励数量',
  `claimed_count` int unsigned NOT NULL COMMENT '领取后累计已领次数',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`request_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='任务领取记录表';


CREATE TABLE IF NOT EXISTS `task_event` (
  `event_id` varchar(64) NOT NULL COMMENT '事件ID（支付事件=pay:订单号），唯一键幂等',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`event_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='任务事件表';


CREATE TABLE IF NOT EXISTS `task_member` (
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `spend_minor` bigint NOT NULL DEFAULT '0' COMMENT '累计消费（分，饱和 1<<62）',
  `level` int unsigned NOT NULL DEFAULT '0' COMMENT '当前会员等级（0=未达最低档）',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='会员表';


CREATE TABLE IF NOT EXISTS `task_member_claim` (
  `request_id` varchar(64) NOT NULL COMMENT '领取幂等键',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `level` int unsigned NOT NULL COMMENT '会员档位',
  `reward_template_id` varchar(64) NOT NULL COMMENT '奖励模板ID（legacy 字符串）',
  `reward_count` int unsigned NOT NULL COMMENT '奖励数量',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`request_id`),
  UNIQUE KEY `uk_task_member_claim_level` (`user_id`,`level`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='会员领取记录表';


CREATE TABLE IF NOT EXISTS `task_pass_claim` (
  `request_id` varchar(64) NOT NULL COMMENT '领取幂等键',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `pass_id` bigint NOT NULL COMMENT '通行证ID',
  `level` int unsigned NOT NULL COMMENT '奖励等级',
  `track` tinyint unsigned NOT NULL COMMENT '轨道：1=FREE 2=PREMIUM',
  `reward_template_id` varchar(64) NOT NULL COMMENT '奖励模板ID（legacy 字符串）',
  `reward_count` int unsigned NOT NULL COMMENT '奖励数量',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`request_id`),
  UNIQUE KEY `uk_task_pass_claim_slot` (`user_id`,`pass_id`,`level`,`track`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='通行证领取记录表';


CREATE TABLE IF NOT EXISTS `task_pass_progress` (
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `pass_id` bigint NOT NULL COMMENT '通行证ID',
  `xp` bigint NOT NULL DEFAULT '0' COMMENT '经验值（封顶 XPPerLevel*MaxLevel）',
  `premium` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '高级线已解锁：1=是 0=否',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`user_id`,`pass_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='通行证进度表';


CREATE TABLE IF NOT EXISTS `task_progress` (
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `task_id` bigint NOT NULL COMMENT '任务ID',
  `period_key` varchar(16) NOT NULL COMMENT '周期键：once / 2006-01-02 / 2024-W01',
  `amount` bigint NOT NULL COMMENT '周期内累计进度量',
  `claimed_count` int unsigned NOT NULL DEFAULT '0' COMMENT '已领取次数',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`user_id`,`task_id`,`period_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='任务进度表';


CREATE TABLE IF NOT EXISTS `task_sign_in` (
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `task_id` bigint NOT NULL COMMENT '任务ID',
  `last_date` char(10) NOT NULL COMMENT '最近签到日期（上海日 2006-01-02）',
  `streak` int unsigned NOT NULL DEFAULT '0' COMMENT '连续签到天数',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`user_id`,`task_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='任务签到表';

SET FOREIGN_KEY_CHECKS = 1;
