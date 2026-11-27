-- ============================================================
-- 02_gacha_identity.sql - 账号域（用户/手机号/第三方身份 + dtm_barrier）（gacha_identity 库终态快照）
-- 表结构 = 本机 MySQL 终态导出；种子 = 历史里程碑脚本按序归并（幂等，可重复执行）。
-- 本文件由 script/split_sql.go 从旧仓 script/split/ 拆库生成。
-- 执行方式：mysql -u root < script/split/02_gacha_identity.sql
-- ============================================================

SET NAMES utf8mb4;

USE `gacha_identity`;

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


CREATE TABLE IF NOT EXISTS `user_phone_grams` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `tenant_id` bigint NOT NULL DEFAULT '1' COMMENT '租户ID',
  `index_version` smallint NOT NULL COMMENT '索引版本',
  `gram_token` binary(32) NOT NULL COMMENT '4-gram HMAC 索引 token',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  PRIMARY KEY (`id`),
  KEY `idx_gram` (`tenant_id`,`index_version`,`gram_token`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户手机号 4-gram 搜索索引表';


CREATE TABLE IF NOT EXISTS `user_phones` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `tenant_id` bigint NOT NULL DEFAULT '1' COMMENT '租户ID（本仓单租户恒 1）',
  `user_id` bigint NOT NULL COMMENT '用户ID（关联 user_users.id）',
  `ciphertext` varbinary(64) NOT NULL COMMENT 'AES-256-GCM 手机号密文（AAD 绑定 tenant+user）',
  `nonce` varbinary(12) NOT NULL COMMENT 'GCM nonce',
  `enc_key_id` varchar(64) NOT NULL COMMENT '加密密钥ID（轮换用）',
  `index_version` smallint NOT NULL COMMENT '索引版本（盲索引密钥轮换用）',
  `exact_token` binary(32) NOT NULL COMMENT 'HMAC-SHA256 精确索引 token（唯一仲裁并发绑定）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_user` (`tenant_id`,`user_id`),
  UNIQUE KEY `uk_exact_token` (`exact_token`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户手机号表（盲索引加密存储）';


CREATE TABLE IF NOT EXISTS `user_third_parties` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `tenant_id` bigint NOT NULL DEFAULT '1' COMMENT '租户ID',
  `kind` tinyint NOT NULL COMMENT '第三方类型：1 微信小程序 / 2 微信 App',
  `open_id` varchar(128) NOT NULL COMMENT '第三方 openid',
  `union_id` varchar(128) NOT NULL DEFAULT '' COMMENT 'unionid（空时以 openid 回填）',
  `user_id` bigint NOT NULL COMMENT '用户ID（关联 user_users.id）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_kind_open` (`tenant_id`,`kind`,`open_id`),
  KEY `idx_tenant_kind_user` (`tenant_id`,`kind`,`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户第三方身份绑定表（微信登录）';


CREATE TABLE IF NOT EXISTS `user_users` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `public_id` varchar(32) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL COMMENT '对外字符串用户ID（u+Base62，唯一；NULL=待回填）',
  `username` varchar(32) NOT NULL COMMENT '用户名（登录账号）',
  `password` varchar(100) NOT NULL COMMENT 'bcrypt 密码哈希',
  `nickname` varchar(32) NOT NULL DEFAULT '' COMMENT '昵称（展示用）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`),
  UNIQUE KEY `uk_public_id` (`public_id`)
) ENGINE=InnoDB AUTO_INCREMENT=1000001 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户表';

-- 增量升级（已有库）：补 public_id 列与唯一索引；历史值由 identity-rpc 启动时回填
DROP PROCEDURE IF EXISTS `__upgrade_user_public_id`;

DELIMITER $$
CREATE PROCEDURE `__upgrade_user_public_id`()
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='gacha_identity' AND TABLE_NAME='user_users' AND COLUMN_NAME='public_id') THEN
    ALTER TABLE `user_users`
      ADD COLUMN `public_id` varchar(32) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL COMMENT '对外字符串用户ID（u+Base62，唯一；NULL=待回填）' AFTER `id`;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='gacha_identity' AND TABLE_NAME='user_users' AND INDEX_NAME='uk_public_id') THEN
    ALTER TABLE `user_users` ADD UNIQUE KEY `uk_public_id` (`public_id`);
  END IF;
END$$
DELIMITER ;

CALL `__upgrade_user_public_id`();
DROP PROCEDURE `__upgrade_user_public_id`;


-- ============================================================
-- 种子数据（幂等；按里程碑序归并）
-- ============================================================

-- 种子数据：演示用户 demo / 123456，初始余额 100000 分（1000 元）
-- 用户数据库 ID 从 1000001 开始（demo 占首号，后续注册从 1000002 递增）
-- ON DUPLICATE KEY UPDATE id=id 实现幂等：已存在则不动
INSERT INTO `user_users` (`id`, `public_id`, `username`, `password`, `nickname`) VALUES
    (1000001, 'uDENLRy4c5', 'demo', '$2a$10$K/388oYK.gV1IwQ3zK3YeOIQzUhCTH97QWOzHyXiBy2n7ISiTm.JO', '演示用户')
ON DUPLICATE KEY UPDATE
    `id` = `id`,
    `public_id` = COALESCE(NULLIF(`public_id`, ''), VALUES(`public_id`));

-- 已有库升级时把自增值推到 1000001 以上；新库在插入 demo 后自动变为 1000002
ALTER TABLE `user_users` AUTO_INCREMENT = 1000001;

SET FOREIGN_KEY_CHECKS = 1;


CREATE TABLE IF NOT EXISTS `user_profiles` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `user_id` bigint NOT NULL COMMENT '用户ID（关联 user_users.id）',
  `avatar_url` varchar(512) NOT NULL DEFAULT '' COMMENT '头像URL',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态：1正常 2禁用',
  `lucky_display` varchar(32) NOT NULL DEFAULT '初始化' COMMENT '集市展示状态',
  `register_ip` varchar(64) NOT NULL DEFAULT '' COMMENT '注册IP',
  `register_device` varchar(128) NOT NULL DEFAULT '' COMMENT '注册设备码',
  `rebate_rate` int NOT NULL DEFAULT '10' COMMENT '返佣比例',
  `identity` varchar(32) NOT NULL DEFAULT '用户' COMMENT '身份',
  `parent_user_id` bigint NOT NULL DEFAULT '0' COMMENT '上级用户ID',
  `reissue_luck_points` tinyint NOT NULL DEFAULT '0' COMMENT '是否补发欧气值',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户后台资料';


-- 用户邀请码（每个用户唯一，首次打开邀请页时生成）
CREATE TABLE IF NOT EXISTS `user_invite_codes` (
  `user_id` bigint NOT NULL COMMENT '用户ID（关联 user_users.id）',
  `code` varchar(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '邀请码（大小写敏感）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`user_id`),
  UNIQUE KEY `uk_invite_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户邀请码表';


-- 邀请关系（被邀请人唯一；奖励由邀请人打开邀请页时结算）
CREATE TABLE IF NOT EXISTS `user_invite_records` (
  `invitee_user_id` bigint NOT NULL COMMENT '被邀请人用户ID（关联 user_users.id）',
  `inviter_user_id` bigint NOT NULL COMMENT '邀请人用户ID（关联 user_users.id）',
  `invite_code` varchar(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '使用的邀请码',
  `reward_points` bigint NOT NULL DEFAULT '0' COMMENT '邀请奖励吉祥值',
  `status` tinyint NOT NULL DEFAULT '0' COMMENT '状态：0=待领取 1=已发放 2=已过期',
  `bound_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '绑定时间',
  `reward_at` datetime DEFAULT NULL COMMENT '奖励发放时间',
  `expires_at` datetime NOT NULL COMMENT '奖励领取截止时间',
  PRIMARY KEY (`invitee_user_id`),
  KEY `idx_inviter_user_id` (`inviter_user_id`),
  KEY `idx_status_expires` (`status`,`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户邀请关系表';


CREATE TABLE IF NOT EXISTS `user_finance_settings` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `user_id` bigint NOT NULL COMMENT '用户ID（关联 user_users.id）',
  `wallet_settings_json` text NOT NULL COMMENT '钱包开关 JSON',
  `refund_limit_fen` bigint NOT NULL DEFAULT '0' COMMENT '管理员设置退款上限（分，0=默认）',
  `refund_limit_count` int NOT NULL DEFAULT '0' COMMENT '管理员设置退款次数（0=默认）',
  `sms_limit_reset_at` datetime DEFAULT NULL COMMENT '短信限制重置时间',
  `realname_limit_reset_at` datetime DEFAULT NULL COMMENT '实名认证次数重置时间',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户财务设置';

CREATE TABLE IF NOT EXISTS `user_payment_bindings` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `name` varchar(64) NOT NULL DEFAULT '' COMMENT '姓名',
  `payment_account` varchar(128) NOT NULL DEFAULT '' COMMENT '支付账号',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户支付绑定信息';

CREATE TABLE IF NOT EXISTS `user_realname_bindings` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `name` varchar(64) NOT NULL DEFAULT '' COMMENT '姓名',
  `id_card` varchar(32) NOT NULL DEFAULT '' COMMENT '身份证',
  `phone` varchar(32) NOT NULL DEFAULT '' COMMENT '手机号',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户实名认证信息';
