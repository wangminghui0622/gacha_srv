#!/usr/bin/env bash
# 基础域模型生成（缓存版）：M14d 起 eligibility 三表（head/versions/rules）改走
# internal/repo 原始 gorm 层（M14b pool.go 先例，主键点查 3 次毫秒级，无缓存需求），
# 本脚本不再生成模型；保留占位说明。
# 用法：bash script/model/base_cache.sh
set -eu

echo "√ 基础域 cache 模型：无（eligibility 走 repo 层，见 base_nocache.sh 说明）"
