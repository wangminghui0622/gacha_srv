#!/usr/bin/env bash
# test 环境部署入口（外部基础设施，无内置 MySQL/Redis/Etcd）。
# 用法：bash deploy/test/deploy.sh [version]    # version 默认 v1.0.0
# 前置：先在 deploy/dev/swarm/ 里跑 build-binaries.sh 生成 bin/conf，并上传到主节点。
set -euo pipefail

ENV_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export ENV_DIR

bash "$ENV_DIR/../dev/swarm/bootstrap.sh" "${1:-v1.0.0}"
