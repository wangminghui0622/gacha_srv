#!/bin/bash
# ============================================================
# 场景注册表：run_all.sh 与 menu.sh 的单一事实源
# 新增场景只需在这里加一行（SCENARIOS 与 NAMES 平行数组）。
#
# RUN_ALL_SKIP：一键回归排除项（场景 5 概率核对 1000 抽约 4-6 分钟、
# 场景 6 故障注入依赖时序、场景 10 捕获故障注入，菜单可单独执行）。
# ============================================================

SCENARIOS=(
    "scenario_01_success"
    "scenario_02_insufficient_balance"
    "scenario_03_insufficient_stock"
    "scenario_04_idempotent"
    "scenario_05_probability"
    "scenario_06_fault_injection"
    "scenario_07_wallet_ledger"
    "scenario_08_eligibility_deny"
    "scenario_09_risk_reject"
    "scenario_10_capture_fault"
    "scenario_11_coupon_loyalty"
    "scenario_12_coupon_release"
    "scenario_13_signin"
    "scenario_14_redeem"
    "scenario_15_wheel"
    "scenario_16_campaign_box"
    "scenario_17_dig_box"
    "scenario_18_rolling_draw"
    "scenario_19_full_car"
    "scenario_20_gashapon"
    "scenario_21_lucky_assign"
    "scenario_22_combo_chain"
    "scenario_23_group_buy"
    "scenario_24_admin_console"
    "scenario_25_pool_publication"
    "scenario_26_catalog"
    "scenario_27_console_m14d"
    "scenario_28_task_reporting"
    "scenario_29_phone_wechat_login"
    "scenario_30_memory_match"
    "scenario_31_task_player"
    "scenario_32_integration_channels"
)

NAMES=(
    "正常购买开奖（success）"
    "余额不足（insufficient_balance）"
    "奖池售罄退款（insufficient_stock）"
    "同 Key 幂等（idempotent，带 5 折券）"
    "概率公示核对（probability，1000 抽）"
    "故障注入（fault_injection，DTM 重试）"
    "钱包流水断言（wallet_ledger）"
    "资格策略拒绝（eligibility_deny）"
    "风控拒绝（risk_reject）"
    "捕获故障注入（capture_fault，两段锁接力）"
    "券+积分抵扣（coupon_loyalty）"
    "售罄补偿释放券（coupon_release）"
    "签到活动（sign_in）"
    "兑换码活动（redeem_code）"
    "幸运转盘（prize_wheel）"
    "活动宝箱零付开箱（campaign_box）"
    "挖宝箱（dig_box）"
    "滚动中奖（rolling_draw）"
    "满车（full_car）"
    "扭蛋（gashapon）"
    "幸运指派（lucky_assign）"
    "连击链（combo_chain）"
    "拼团抽（group_buy）"
    "管理控制台（admin_console）"
    "赏池草稿→发布（pool_publication）"
    "商品目录控制台（catalog）"
    "控制台五域全链路（console_m14d）"
    "任务计划+报表聚合（task_reporting）"
    "手机号/微信登录（phone_wechat_login）"
    "记忆翻牌（memory_match）"
    "任务玩家体系（task_player）"
    "M15d 集成渠道（integration_channels）"
)

# 一键回归排除项（run_all.sh 用；menu.sh 全量可跑）
RUN_ALL_SKIP=(
    "scenario_05_probability"
    "scenario_06_fault_injection"
    "scenario_10_capture_fault"
)
