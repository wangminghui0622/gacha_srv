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
  `username` varchar(32) NOT NULL COMMENT '用户名（登录账号）',
  `password` varchar(100) NOT NULL COMMENT 'bcrypt 密码哈希',
  `nickname` varchar(32) NOT NULL DEFAULT '' COMMENT '昵称（展示用）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户表';


-- ============================================================
-- 种子数据（幂等；按里程碑序归并）
-- ============================================================

-- 种子数据：演示用户 demo / 123456，初始余额 100000 分（1000 元）
-- ON DUPLICATE KEY UPDATE id=id 实现幂等：已存在则不动
INSERT INTO `user_users` (`id`, `username`, `password`, `nickname`) VALUES
    (1, 'demo', '$2a$10$K/388oYK.gV1IwQ3zK3YeOIQzUhCTH97QWOzHyXiBy2n7ISiTm.JO', '演示用户')
ON DUPLICATE KEY UPDATE `id` = `id`;

SET FOREIGN_KEY_CHECKS = 1;
