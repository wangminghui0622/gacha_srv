#!/usr/bin/env bash
# 清理脚本（在 128 上执行）。默认只卸载 stack，保留镜像和文件；
# --full 则彻底清理（含镜像、卷、swarm、registry），用于「从零重来」。
# 用法：bash deploy/dev/swarm/teardown.sh [--full]
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_DIR="${ENV_DIR:-$SCRIPT_DIR}"
source "$ENV_DIR/env.sh"

FULL=0
[ "${1:-}" = "--full" ] && FULL=1

echo "==> 卸载业务服务 stack (gacha)"
docker stack rm gacha 2>&1 | tail -2 || true
echo "==> 卸载基础设施 stack (gacha-infra)"
docker stack rm gacha-infra 2>&1 | tail -2 || true

sleep 10

if [ "$FULL" -eq 1 ]; then
  echo "==> 彻底清理：镜像/卷/容器/registry/swarm"
  docker rm -f registry 2>/dev/null || true
  # 从机先离开，管理器最后离开（否则从机会失去管理节点）
  for w in "${WORKERS_SSH[@]}"; do
    ssh -o StrictHostKeyChecking=no "$w" "docker swarm leave --force 2>/dev/null" < /dev/null || true
  done
  docker swarm leave --force 2>/dev/null || true
  # 清理三节点镜像 + 卷
  docker system prune -a -f --volumes 2>&1 | tail -1 || true
  for w in "${WORKERS_SSH[@]}"; do
    ssh -o StrictHostKeyChecking=no "$w" "docker system prune -a -f --volumes 2>&1 | tail -1" < /dev/null || true
  done
  echo "==> 彻底清理完成（二进制/config/SQL 文件保留在 /home/master/gacha）"
else
  echo "==> 清理完成（镜像/卷/文件保留，可再次 bootstrap 快速重启）"
  echo "    如需彻底清空再加 --full"
fi
