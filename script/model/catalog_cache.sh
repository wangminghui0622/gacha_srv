#!/usr/bin/env bash
# 商城域模型生成（缓存版）：catalog 全量走 internal/repo/catalog.go 原始 gorm 层
# （M14c 激活即如此：7 表主键点查+有限列表，毫秒级，无缓存需求，M14b pool.go 先例），
# 本脚本不生成模型；保留占位说明。
# 用法：bash script/model/catalog_cache.sh
set -eu

echo "√ 商城域 cache 模型：无（catalog 走 repo 层，见 catalog_nocache.sh 说明）"
