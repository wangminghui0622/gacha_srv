#!/usr/bin/env bash
# 商城域模型生成（无缓存）：catalog 全量走 internal/repo/catalog.go 原始 gorm 层
# （M14c 激活：products/skus/修订/发布 7 表，主键点查+有限列表，无缓存需求，
#   M14b pool.go 先例），pkg/model 与 pkg/model_nocache 均无 gacha_catalog。
# 表结构变更改 09_gacha_catalog.sql + repo 层手写，不跑本脚本。
# 用法：bash script/model/catalog_nocache.sh
set -eu

echo "√ 商城域 nocache 模型：无（catalog 走 internal/repo/catalog.go 手写 gorm 层）"
