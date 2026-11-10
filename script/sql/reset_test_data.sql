-- ════════════════════════════════════════════════════════════════
-- reset_test_data.sql - 测试数据重置脚本
--
-- 功能：把 draw_pool_entries.remaining 恢复为 03_gacha_draw.sql 种子初值。
-- 场景测试会消耗库存（开奖扣 remaining / 人为清零制造售罄），
-- run_all.sh 开头自动执行本脚本，避免上次运行消耗导致断言误报。
--
-- 执行方式：mysql -u root --default-character-set=utf8mb4 gacha_draw < reset_test_data.sql
-- 说明：按 (box_id, name) 定位条目，幂等可重复执行。
-- ⚠ 仅限测试环境执行！误跑在生产库会把真实剩余库存回滚到种子值。
-- ════════════════════════════════════════════════════════════════

USE gacha_draw;

-- 新手体验盒（box 1）：谢谢参与不限量，其余按种子数量回填
UPDATE draw_pool_entries SET remaining = -1    WHERE box_id = 1 AND `name` = '谢谢参与';
UPDATE draw_pool_entries SET remaining = 10000 WHERE box_id = 1 AND `name` = '金币×100';
UPDATE draw_pool_entries SET remaining = 100   WHERE box_id = 1 AND `name` = '限定头像框';
UPDATE draw_pool_entries SET remaining = 10    WHERE box_id = 1 AND `name` = '限定皮肤';

-- 夏日限定盒（box 2）
UPDATE draw_pool_entries SET remaining = -1    WHERE box_id = 2 AND `name` = '谢谢参与';
UPDATE draw_pool_entries SET remaining = 5000  WHERE box_id = 2 AND `name` = '金币×500';
UPDATE draw_pool_entries SET remaining = 50    WHERE box_id = 2 AND `name` = '夏日皮肤·泳装';
UPDATE draw_pool_entries SET remaining = 20    WHERE box_id = 2 AND `name` = '夏日皮肤·冲浪';
UPDATE draw_pool_entries SET remaining = 5     WHERE box_id = 2 AND `name` = '典藏皮肤·海神';

-- 传说宝箱（box 3）
UPDATE draw_pool_entries SET remaining = -1    WHERE box_id = 3 AND `name` = '谢谢参与';
UPDATE draw_pool_entries SET remaining = 1000  WHERE box_id = 3 AND `name` = '金币×5000';
UPDATE draw_pool_entries SET remaining = 15    WHERE box_id = 3 AND `name` = '典藏时装';
UPDATE draw_pool_entries SET remaining = 10    WHERE box_id = 3 AND `name` = '传说武器';
UPDATE draw_pool_entries SET remaining = 5     WHERE box_id = 3 AND `name` = '传说坐骑';
