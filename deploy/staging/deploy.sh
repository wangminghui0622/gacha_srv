#!/usr/bin/env bash
# staging 环境部署入口（外部基础设施，无内置 MySQL/Redis/Etcd）。
# 用法：bash deploy/staging/deploy.sh [version]
set -euo pipefail

ENV_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export ENV_DIR

bash "$ENV_DIR/../dev/swarm/bootstrap.sh" "${1:-v1.0.0}"
