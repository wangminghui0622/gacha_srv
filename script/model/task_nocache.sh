#!/usr/bin/env bash
# 任务域模型生成（无缓存）：task 8 表联合主键 goctl 不支持 → 走 internal/repo 手写
# gorm 结构体（旧仓 M15c 惯例），pkg/model 与 pkg/model_nocache 均无 gacha_task。
# 表结构变更改 08_gacha_task.sql + repo 层手写，不跑本脚本。
# 用法：bash script/model/task_nocache.sh
set -eu

echo "√ 任务域 nocache 模型：无（task 走 internal/repo 手写 gorm 层）"
