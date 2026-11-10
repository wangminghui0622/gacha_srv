#!/usr/bin/env bash
# product 环境部署入口：启动/停止 product（外部基础设施 + 多副本）。
# 用法：bash deploy/product.sh [up|down|ps|logs|config]
set -euo pipefail

cd "$(dirname "$0")/.."

ENV=product
action="${1:-up}"

env_file="deploy/$ENV/.env"
if [ ! -f "$env_file" ]; then
  echo "缺少 $env_file：先 cp deploy/$ENV/.env.example deploy/$ENV/.env 并填入密钥" >&2
  exit 2
fi

compose=(docker compose --env-file "$env_file" -f "deploy/$ENV/compose.yaml")

case "$action" in
  up)     "${compose[@]}" up -d --remove-orphans ;;
  down)   "${compose[@]}" down ;;
  ps)     "${compose[@]}" ps ;;
  logs)   "${compose[@]}" logs -f ;;
  config) "${compose[@]}" config --quiet ;;
  *) echo "usage: $0 [up|down|ps|logs|config]" >&2; exit 2 ;;
esac