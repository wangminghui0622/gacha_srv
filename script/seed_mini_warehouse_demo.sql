-- 小程序吉仓演示数据（仅 demo 用户；已有吉仓数据时不重复写入）
-- 说明：用于本地/演示环境展示吉仓三列赏品、数量角标与发货选择交互。
USE `gacha_draw`;

SET @demo_user_id := 1000001;
SET @demo_existing := (SELECT COUNT(*) FROM `draw_warehouse_items` WHERE `user_id` = @demo_user_id);

INSERT INTO `draw_warehouse_items` (`user_id`, `record_id`, `box_id`, `item_name`, `status`, `created_at`)
SELECT @demo_user_id, 0, t.`box_id`, t.`item_name`, 1,
       DATE_SUB(NOW(), INTERVAL t.`day_offset` DAY)
FROM (
  SELECT 1 AS `box_id`, '金币×100' AS `item_name`, 1 AS `day_offset`
  UNION ALL SELECT 1, '金币×100', 2
  UNION ALL SELECT 1, '金币×100', 3
  UNION ALL SELECT 1, '金币×100', 4
  UNION ALL SELECT 1, '金币×100', 5
  UNION ALL SELECT 1, '金币×100', 6
  UNION ALL SELECT 1, '限定头像框', 7
  UNION ALL SELECT 1, '限定头像框', 8
  UNION ALL SELECT 1, '限定皮肤', 9
  UNION ALL SELECT 2, '金币×500', 10
  UNION ALL SELECT 2, '金币×500', 11
  UNION ALL SELECT 2, '金币×500', 12
  UNION ALL SELECT 2, '夏日皮肤·泳装', 13
  UNION ALL SELECT 2, '夏日皮肤·泳装', 14
  UNION ALL SELECT 2, '夏日皮肤·冲浪', 15
  UNION ALL SELECT 2, '典藏皮肤·海神', 16
  UNION ALL SELECT 3, '金币×5000', 17
  UNION ALL SELECT 3, '金币×5000', 18
  UNION ALL SELECT 3, '典藏时装', 19
  UNION ALL SELECT 3, '传说武器', 20
  UNION ALL SELECT 3, '传说坐骑', 21
) AS t
WHERE @demo_existing = 0;
