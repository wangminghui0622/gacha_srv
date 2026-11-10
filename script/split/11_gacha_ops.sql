-- ============================================================
-- 09_gacha_ops.sql - 运营域（账号/角色/任务/看板/报表）（gacha_ops 库终态快照）
-- 表结构 = 本机 MySQL 终态导出（mysqldump --no-data，2026-10-26）；
-- 种子 = 历史里程碑编号脚本按序归并（幂等，可重复执行）。
-- 执行方式：mysql -u root < script/split/09_gacha_ops.sql
-- ============================================================

SET NAMES utf8mb4;

USE `gacha_ops`;

SET FOREIGN_KEY_CHECKS = 0;

CREATE TABLE IF NOT EXISTS `console_accounts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL DEFAULT '1',
  `username` varchar(64) NOT NULL,
  `password_hash` varchar(255) NOT NULL,
  `display_name` varchar(128) NOT NULL DEFAULT '',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '1 鍚?敤 / 2 鍋滅敤',
  `role_version` bigint unsigned NOT NULL DEFAULT '1' COMMENT '主体角色乐观锁版本（PutPrincipalRoles 用）',
  `totp_secret` varchar(64) NOT NULL DEFAULT '' COMMENT '绌?鏈?粦瀹?TOTP',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_username` (`tenant_id`,`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='鎺у埗鍙拌处鍙';

CREATE TABLE IF NOT EXISTS `console_sessions` (
  `jti` varchar(64) NOT NULL,
  `family_id` varchar(64) NOT NULL COMMENT 'refresh 鏃嬭浆瀹舵棌锛屾硠闇叉椂鏁存棌鍚婇攢',
  `account_id` bigint unsigned NOT NULL,
  `tenant_id` bigint unsigned NOT NULL,
  `refresh_hash` char(64) NOT NULL COMMENT 'SHA256(refresh_token) 鍗佸叚杩涘埗',
  `previous_refresh_hash` char(64) NOT NULL DEFAULT '' COMMENT '上一枚 refresh 哈希（重放旧令牌 → 家族吊销）',
  `fingerprint_hash` char(64) NOT NULL COMMENT 'SHA256(client_fingerprint) 鍗佸叚杩涘埗',
  `expires_at` datetime NOT NULL,
  `revoked_at` datetime DEFAULT NULL,
  `compromised` tinyint NOT NULL DEFAULT '0',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`jti`),
  UNIQUE KEY `uk_refresh_hash` (`refresh_hash`),
  KEY `idx_account` (`account_id`),
  KEY `idx_family` (`family_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='鎺у埗鍙颁細璇濓紙access=JWT jti / refresh=SHA256 瀛樺偍锛';

CREATE TABLE IF NOT EXISTS `ops_audit_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL DEFAULT '1',
  `account_id` bigint unsigned NOT NULL DEFAULT '0',
  `operation_id` varchar(128) NOT NULL,
  `resource` varchar(64) NOT NULL,
  `action` varchar(64) NOT NULL,
  `target_id` varchar(64) NOT NULL DEFAULT '',
  `request_id` varchar(64) NOT NULL DEFAULT '',
  `detail_json` text NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_created` (`created_at`),
  KEY `idx_account` (`account_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='绠＄悊鎿嶄綔瀹¤?鏃ュ織';

CREATE TABLE IF NOT EXISTS `principal_roles` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `account_id` bigint unsigned NOT NULL,
  `role_id` bigint unsigned NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_account_role` (`account_id`,`role_id`),
  KEY `idx_role` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='主体角色绑定';

CREATE TABLE IF NOT EXISTS `reporting_dashboard` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '看板 id',
  `tenant_id` bigint unsigned NOT NULL DEFAULT '1' COMMENT '租户 id',
  `dashboard_code` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL COMMENT '看板编码（租户内唯一）',
  `title` varchar(128) NOT NULL COMMENT '标题',
  `status` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '1 DRAFT / 2 PUBLISHED / 3 ARCHIVED',
  `version` bigint unsigned NOT NULL DEFAULT '1' COMMENT '乐观锁版本（保存/发布均 +1）',
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  `published_at` datetime(3) DEFAULT NULL COMMENT '发布时间（未发布 NULL）',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_reporting_dashboard_code` (`tenant_id`,`dashboard_code`),
  KEY `idx_reporting_dashboard_tenant_status` (`tenant_id`,`status`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='报表看板（保存全量替换 widgets）';

CREATE TABLE IF NOT EXISTS `reporting_dashboard_widget` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'widget id',
  `dashboard_id` bigint unsigned NOT NULL COMMENT '看板 id',
  `widget_code` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL COMMENT 'widget 编码（看板内唯一）',
  `title` varchar(128) NOT NULL COMMENT '标题',
  `kind` tinyint unsigned NOT NULL COMMENT '1 NUMBER / 2 LINE / 3 BAR / 4 TABLE',
  `metric_code` varchar(96) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL COMMENT '指标编码',
  `grain` tinyint unsigned NOT NULL COMMENT '1 HOUR / 2 DAY',
  `dimension_code` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL COMMENT '维度编码',
  `grid_x` int unsigned NOT NULL COMMENT '网格 x（24 列）',
  `grid_y` int unsigned NOT NULL,
  `grid_width` int unsigned NOT NULL,
  `grid_height` int unsigned NOT NULL,
  `display_options` json NOT NULL COMMENT '展示选项（合法 JSON object，≤8192 字节）',
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_reporting_dashboard_widget_code` (`dashboard_id`,`widget_code`),
  KEY `idx_reporting_dashboard_widget_metric` (`metric_code`,`dimension_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='报表看板 widget';

CREATE TABLE IF NOT EXISTS `reporting_dimension_definition` (
  `dimension_code` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL COMMENT '维度编码',
  `title` varchar(128) NOT NULL COMMENT '标题',
  `status` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '1 启用 / 0 停用',
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`dimension_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='报表维度定义';

CREATE TABLE IF NOT EXISTS `reporting_metric_bucket` (
  `tenant_id` bigint unsigned NOT NULL,
  `provider_code` varchar(64) NOT NULL,
  `external_account_id` varchar(128) NOT NULL,
  `standard_metric` tinyint unsigned NOT NULL,
  `bucket_seconds` int unsigned NOT NULL,
  `bucket_start` datetime(3) NOT NULL,
  `dimension_key` varchar(256) NOT NULL,
  `currency` char(3) NOT NULL,
  `value` bigint NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`tenant_id`,`provider_code`,`external_account_id`,`standard_metric`,`bucket_seconds`,`bucket_start`,`dimension_key`,`currency`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='指标桶聚合（看板查询数据源）';

CREATE TABLE IF NOT EXISTS `reporting_metric_definition` (
  `metric_code` varchar(96) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL COMMENT '指标编码',
  `title` varchar(128) NOT NULL COMMENT '标题',
  `standard_metric` tinyint unsigned NOT NULL COMMENT '标准指标 1-5',
  `value_kind` tinyint unsigned NOT NULL COMMENT '1 COUNT / 2 MONEY',
  `supported_grains` set('hour','day') NOT NULL COMMENT '支持粒度',
  `status` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '1 启用 / 0 停用',
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`metric_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='报表指标定义（全局目录）';

CREATE TABLE IF NOT EXISTS `reporting_metric_dimension` (
  `metric_code` varchar(96) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  `dimension_code` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  PRIMARY KEY (`metric_code`,`dimension_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='指标×维度能力表';

CREATE TABLE IF NOT EXISTS `reporting_metric_inbox` (
  `event_id` bigint unsigned NOT NULL COMMENT '事件ID（outbox 行 id）',
  `tenant_id` bigint unsigned NOT NULL,
  `event_type` varchar(128) NOT NULL COMMENT 'integration.metric.snapshot.changed.v1',
  `source_revision` bigint unsigned NOT NULL,
  `received_at` datetime(3) NOT NULL,
  PRIMARY KEY (`event_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='指标事件 inbox（幂等去重）';

CREATE TABLE IF NOT EXISTS `reporting_metric_snapshot` (
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
  `event_id` bigint unsigned NOT NULL COMMENT '最新应用的事件ID',
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`tenant_id`,`provider_code`,`external_account_id`,`external_object_id`,`standard_metric`,`bucket_start`,`bucket_seconds`,`dimension_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='指标快照（槽位唯一，修订号守卫）';

CREATE TABLE IF NOT EXISTS `role_permissions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `role_id` bigint unsigned NOT NULL,
  `resource` varchar(64) NOT NULL,
  `action` varchar(64) NOT NULL,
  `data_scope` tinyint NOT NULL DEFAULT '6' COMMENT '1 SELF/2 TEAM/3 DEPARTMENT/4 ORGANIZATION/5 CUSTOM/6 ALL',
  `approval_mode` tinyint NOT NULL DEFAULT '1' COMMENT '1 NONE/2 SINGLE/3 DUAL_CONTROL',
  `condition_expr` varchar(512) NOT NULL DEFAULT '' COMMENT 'CEL 琛ㄨ揪寮忥紙subject/resource/context 鍙橀噺锛夛紝绌?鏃犳潯浠',
  `allowed_fields` text NOT NULL COMMENT 'JSON 瀛楃?涓叉暟缁勶細瀛楁?鐧藉悕鍗曪紙浜わ級',
  `denied_fields` text NOT NULL COMMENT 'JSON 瀛楃?涓叉暟缁勶細瀛楁?榛戝悕鍗曪紙骞讹級',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_resource_action` (`role_id`,`resource`,`action`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='瑙掕壊鏉冮檺锛堟巿鏉冭?鍒欙細rbac.<role_code>.<resource>.<action>锛';

CREATE TABLE IF NOT EXISTS `roles` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL DEFAULT '1',
  `code` varchar(64) NOT NULL,
  `name` varchar(128) NOT NULL,
  `enabled` tinyint NOT NULL DEFAULT '1' COMMENT '1 鍚?敤 / 0 鍋滅敤',
  `version` bigint unsigned NOT NULL DEFAULT '1' COMMENT '涔愯?閿佺増鏈',
  `created_by` bigint unsigned NOT NULL DEFAULT '0' COMMENT '鍒涘缓鑰?account_id锛圤WNER 鏁版嵁鑼冨洿鐢?級',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_code` (`tenant_id`,`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='鎺у埗鍙拌?鑹';

CREATE TABLE IF NOT EXISTS `task_plan_head` (
  `singleton_id` tinyint unsigned NOT NULL COMMENT '恒为 1',
  `current_version` bigint unsigned NOT NULL DEFAULT '0' COMMENT '当前发布版本',
  `latest_version` bigint unsigned NOT NULL DEFAULT '0' COMMENT '最新版本（含草稿）',
  `updated_at` datetime(3) NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`singleton_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='任务计划版本头（单行）';

CREATE TABLE IF NOT EXISTS `task_plan_publication_approval` (
  `plan_version` bigint unsigned NOT NULL COMMENT '计划版本号',
  `config_hash` binary(32) NOT NULL COMMENT '审批对象配置哈希',
  `requester_id` bigint unsigned NOT NULL COMMENT '申请人（控制台主体 id）',
  `approver_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '审批人（0=待审批）',
  `status` tinyint unsigned NOT NULL COMMENT '1 PENDING / 2 APPROVED',
  `requested_at` datetime(3) NOT NULL COMMENT '提请时间',
  `approved_at` datetime(3) DEFAULT NULL COMMENT '批准时间',
  PRIMARY KEY (`plan_version`),
  KEY `idx_task_plan_approval_queue` (`status`,`requested_at`,`plan_version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='任务计划发布审批表（单审批，审批人≠申请人）';

CREATE TABLE IF NOT EXISTS `task_plan_version` (
  `version` bigint unsigned NOT NULL COMMENT '版本号（head.latest+1 递增）',
  `status` tinyint unsigned NOT NULL COMMENT '1 DRAFT / 2 PUBLISHED / 3 RETIRED',
  `config_hash` binary(32) NOT NULL COMMENT 'SHA256(deterministic canonical plan)',
  `config_blob` mediumblob NOT NULL COMMENT '确定性 proto 字节（canonical：version/status 清零）',
  `created_at` datetime(3) NOT NULL COMMENT '创建时间',
  `published_at` datetime(3) DEFAULT NULL COMMENT '发布时间（未发布 NULL）',
  PRIMARY KEY (`version`),
  UNIQUE KEY `uk_task_plan_config_hash` (`config_hash`),
  KEY `idx_task_plan_status_version` (`status`,`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='任务计划版本表';

SET FOREIGN_KEY_CHECKS = 1;

-- ============================================================
-- 种子数据（幂等；按里程碑序归并）
-- ============================================================

-- ── 种子：内置超管角色（admin 账号由 ops-rpc 启动时自动开通，密码 env 覆盖）──
INSERT INTO roles (tenant_id, code, name, enabled, version, created_by)
VALUES (1, 'super_admin', '超级管理员', 1, 1, 0)
ON DUPLICATE KEY UPDATE name = VALUES(name);

-- 超管全量权限：resource/action 通配（授权器对 * 通配匹配，见 core/authorize.go）
INSERT INTO role_permissions (role_id, resource, action, data_scope, approval_mode, condition_expr, allowed_fields, denied_fields)
SELECT id, '*', '*', 6, 1, '', '[]', '[]'
FROM roles
WHERE tenant_id = 1 AND code = 'super_admin'
ON DUPLICATE KEY UPDATE data_scope = 6;

-- 促销管理员角色 + 权限（照抄 server_v2 平台策略种子 v4：
--   promotion.coupon_template read/publish、promotion.tenant_loyalty read/write、
--   promotion.user_coupon grant、identity.console_account create；OpenConsoleAccount 回复的
--   role_code 即此角色）
INSERT INTO roles (tenant_id, code, name, enabled, version, created_by)
VALUES (1, 'promotion-admin', '促销管理员', 1, 1, 0)
ON DUPLICATE KEY UPDATE name = VALUES(name);

INSERT INTO role_permissions (role_id, resource, action, data_scope, approval_mode, condition_expr, allowed_fields, denied_fields)
SELECT r.id, v.resource, v.action, 6, 1, '', '[]', '[]'
FROM roles r
JOIN (
  SELECT 'promotion.coupon_template' AS resource, 'read' AS action
  UNION ALL SELECT 'promotion.coupon_template', 'publish'
  UNION ALL SELECT 'promotion.tenant_loyalty', 'read'
  UNION ALL SELECT 'promotion.tenant_loyalty', 'write'
  UNION ALL SELECT 'promotion.user_coupon', 'grant'
  UNION ALL SELECT 'identity.console_account', 'create'
) v
WHERE r.tenant_id = 1 AND r.code = 'promotion-admin'
ON DUPLICATE KEY UPDATE data_scope = 6;

-- ── M14b 赏池控制台：promotion-admin 补 gacha.pool/assign_lucky（照抄 server_v2
-- 平台策略种子 v5：促销管理员只持有池域「指定幸运用户」，其余 pool 权限
-- create/read/publish/request_publication/approve_publication 由测试角色按场景授予）──
INSERT INTO role_permissions (role_id, resource, action, data_scope, approval_mode, condition_expr, allowed_fields, denied_fields)
SELECT r.id, 'gacha.pool', 'assign_lucky', 6, 1, '', '[]', '[]'
FROM roles r
WHERE r.tenant_id = 1 AND r.code = 'promotion-admin'
ON DUPLICATE KEY UPDATE data_scope = 6;

-- ── 种子：维度 'all' + 5 个 provider 指标（照抄 000026）──
INSERT INTO reporting_dimension_definition (dimension_code, title, status, created_at, updated_at)
VALUES ('all', '全部', 1, NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE title = VALUES(title);

INSERT INTO reporting_metric_definition (metric_code, title, standard_metric, value_kind, supported_grains, status, created_at, updated_at)
VALUES ('provider.impressions', '展示量', 1, 1, 'hour,day', 1, NOW(3), NOW(3)),
       ('provider.clicks',      '点击量', 2, 1, 'hour,day', 1, NOW(3), NOW(3)),
       ('provider.conversions', '转化量', 3, 1, 'hour,day', 1, NOW(3), NOW(3)),
       ('provider.spend',       '投放花费', 4, 2, 'hour,day', 1, NOW(3), NOW(3)),
       ('provider.revenue',     '投放营收', 5, 2, 'hour,day', 1, NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE title = VALUES(title), standard_metric = VALUES(standard_metric),
    value_kind = VALUES(value_kind), supported_grains = VALUES(supported_grains);

INSERT INTO reporting_metric_dimension (metric_code, dimension_code)
SELECT metric_code, 'all' FROM reporting_metric_definition
ON DUPLICATE KEY UPDATE dimension_code = VALUES(dimension_code);

-- ── 种子：bootstrap 任务计划 v1（PUBLISHED；1 个 legacy 任务「每日登录」。
--    config_hash = SHA256(deterministic canonical plan)，blob 同为 canonical 字节
--    （version/status 清零，decode 时由行字段回填——server_v2 taskPlanPayload 同款）。
--    由 .smoke_risk/hashgen/main.go 生成）──
INSERT INTO task_plan_version (version, status, config_hash, config_blob, created_at, published_at)
VALUES (1, 2,
    UNHEX('c4ea9a722bcccc403081206cf2e0ad6c9525f8452f2891da3d6bbecaa4d7fe06'),
    UNHEX('1a22080110011a0ce6af8fe697a5e799bbe5bd952002280330014206542d424f4f544801'),
    NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE version = version;

INSERT INTO task_plan_head (singleton_id, current_version, latest_version, updated_at)
VALUES (1, 1, 1, NOW(3))
ON DUPLICATE KEY UPDATE updated_at = updated_at;

-- ── 种子：promotion-admin 任务计划/报表权限（照抄 000120 语义 + permission_catalog：
--    task.plan publish=DUAL_CONTROL(3) 其余 NONE(1)；task.progress read；
--    reporting.metric read；reporting.dashboard read/write/publish/query）──
INSERT INTO role_permissions (role_id, resource, action, data_scope, approval_mode, condition_expr, allowed_fields, denied_fields)
SELECT r.id, v.resource, v.action, 6, v.mode, '', '[]', '[]'
FROM roles r
JOIN (
  SELECT 'task.plan' AS resource, 'read' AS action, 1 AS mode
  UNION ALL SELECT 'task.plan', 'write', 1
  UNION ALL SELECT 'task.plan', 'request_publication', 1
  UNION ALL SELECT 'task.plan', 'approve_publication', 1
  UNION ALL SELECT 'task.plan', 'publish', 3
  UNION ALL SELECT 'task.progress', 'read', 1
  UNION ALL SELECT 'reporting.metric', 'read', 1
  UNION ALL SELECT 'reporting.dashboard', 'read', 1
  UNION ALL SELECT 'reporting.dashboard', 'write', 1
  UNION ALL SELECT 'reporting.dashboard', 'publish', 1
  UNION ALL SELECT 'reporting.dashboard', 'query', 1
) v
WHERE r.tenant_id = 1 AND r.code = 'promotion-admin'
ON DUPLICATE KEY UPDATE data_scope = 6, approval_mode = VALUES(approval_mode);

