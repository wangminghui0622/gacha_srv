-- ============================================================
-- 04_gacha_dtm.sql - DTM Server 存储（kv/trans_branch_op/trans_global）（gacha_dtm 库终态快照）
-- 表结构 = 本机 MySQL 终态导出（mysqldump --no-data，2026-10-26）；
-- 种子 = 历史里程碑编号脚本按序归并（幂等，可重复执行）。
-- 执行方式：mysql -u root < script/split/04_gacha_dtm.sql
-- ============================================================

SET NAMES utf8mb4;

USE `gacha_dtm`;

SET FOREIGN_KEY_CHECKS = 0;

CREATE TABLE IF NOT EXISTS `kv` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `cat` varchar(45) NOT NULL COMMENT '配置分类：标识这组数据的用途类别',
  `k` varchar(128) NOT NULL COMMENT '配置键名：在同一分类下全局唯一',
  `v` text COMMENT '配置值：支持长文本，用于存储JSON等复杂数据',
  `version` bigint DEFAULT '1' COMMENT '版本号：用于乐观并发控制，每次更新时+1',
  `create_time` datetime DEFAULT NULL COMMENT '记录创建时间',
  `update_time` datetime DEFAULT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `uniq_k` (`cat`,`k`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC;

CREATE TABLE IF NOT EXISTS `trans_branch_op` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `gid` varchar(128) NOT NULL COMMENT '全局事务ID：关联 trans_global.gid',
  `url` varchar(1024) NOT NULL COMMENT '回调URL：DTM调用此URL执行子事务',
  `data` text COMMENT '请求体（已弃用）：旧版HTTP协议使用的JSON数据',
  `bin_data` blob COMMENT '请求体（protobuf二进制）：SAGA子事务的请求参数',
  `branch_id` varchar(128) NOT NULL COMMENT '分支事务ID：01=扣款, 02=开奖',
  `op` varchar(45) NOT NULL COMMENT '操作类型：action=正向操作, compensate=补偿操作',
  `status` varchar(45) NOT NULL COMMENT '执行状态：prepared/succeed/failed',
  `finish_time` datetime DEFAULT NULL COMMENT '完成时间',
  `rollback_time` datetime DEFAULT NULL COMMENT '回滚时间',
  `create_time` datetime DEFAULT NULL COMMENT '记录创建时间',
  `update_time` datetime DEFAULT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `gid_uniq` (`gid`,`branch_id`,`op`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC;

CREATE TABLE IF NOT EXISTS `trans_global` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `gid` varchar(128) NOT NULL COMMENT '全局事务ID（即抽盒请求ID），SAGA唯一标识',
  `trans_type` varchar(45) NOT NULL COMMENT '事务类型：saga/xa/tcc/msg',
  `status` varchar(12) NOT NULL COMMENT '事务状态：prepared/submitted/aborting/succeed/failed',
  `query_prepared` varchar(1024) NOT NULL COMMENT '查询URL：msg/workflow模式用',
  `protocol` varchar(45) NOT NULL COMMENT '通信协议：grpc=本项目使用',
  `create_time` datetime DEFAULT NULL COMMENT '事务创建时间',
  `update_time` datetime DEFAULT NULL COMMENT '记录最后更新时间',
  `finish_time` datetime DEFAULT NULL COMMENT '事务完成时间',
  `rollback_time` datetime DEFAULT NULL COMMENT '回滚完成时间',
  `options` varchar(1024) DEFAULT '' COMMENT '事务选项（JSON）',
  `custom_data` varchar(1024) DEFAULT '' COMMENT '自定义数据',
  `next_cron_interval` int DEFAULT NULL COMMENT '下次轮询间隔（秒）',
  `next_cron_time` datetime DEFAULT NULL COMMENT '下次轮询时间',
  `owner` varchar(128) NOT NULL DEFAULT '' COMMENT '锁定者：DTM实例ID，分布式锁',
  `ext_data` text COMMENT '扩展数据',
  `result` varchar(1024) DEFAULT '' COMMENT '执行结果',
  `rollback_reason` varchar(1024) DEFAULT '' COMMENT '回滚原因',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `gid` (`gid`) USING BTREE,
  KEY `owner` (`owner`) USING BTREE,
  KEY `status_next_cron_time` (`status`,`next_cron_time`) USING BTREE COMMENT 'cron job will use this index to query trans'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC;

SET FOREIGN_KEY_CHECKS = 1;

