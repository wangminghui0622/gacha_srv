#!/usr/bin/env bash
# 滚动更新服务到指定版本（按 sha256 digest 锁版本，版本更新第 3 步，在 128 上执行）。
# 依赖：镜像已推到本地 registry，且 build-push.sh 已生成 deploy/swarm/releases/<version>.digest。
# 用法：bash release.sh <version> [service ...]
#       例：bash release.sh v1.0.1                      # 全部服务
#            bash release.sh v1.0.1 money-rpc           # 只更新 money-rpc
# 回滚：bash release.sh rollback [service]              # 回滚到上一个（Swarm 保留上次 image+digest）
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/env.sh"

REG="$REGISTRY"
STACK="${STACK:-gacha}"

# 服务清单（stack 内服务名）
ALL_SERVICES=(
  base-rpc catalog-rpc draw-rpc task-rpc ops-rpc thirdparty-rpc
  identity-rpc money-rpc marketing-rpc play-rpc dtm
  gateway admin-gateway
)

update_one() { # $1 服务名 $2 版本
  local svc="$1" ver="$2"
  local digest="" image
  # 优先用 digest（不可变）；找不到 digest 清单时降级用 tag
  local manifest="deploy/swarm/releases/$ver.digest"
  if [ -f "$manifest" ]; then
    digest=$(awk -v s="$svc" '$1==s{print $2}' "$manifest")
  fi
  if [ -n "$digest" ]; then
    image="$REG/gacha/$svc@$digest"
  else
    echo "   !! 未找到 $svc 在 $ver 的 digest，降级用 tag :$ver" >&2
    image="$REG/gacha/$svc:$ver"
  fi
  echo ">> 更新 $STACK"_"$svc -> $image"
  docker service update --image "$image" --with-registry-auth \
    "$STACK"_"$svc" 2>&1 | tail -1
}

wait_converge() { # $1 服务名
  local svc="$1" i
  for i in $(seq 1 30); do
    local repl
    repl=$(docker service ls --format '{{.Replicas}}' --filter "name=$STACK"_"$svc" 2>/dev/null)
    # 收敛判定：期望副本全 running（形如 N/N，且不含 starting/running 中间态）
    if [[ "$repl" =~ ^([0-9]+)/([0-9]+)$ ]] && [ "${BASH_REMATCH[1]}" = "${BASH_REMATCH[2]}" ]; then
      echo "   $svc 收敛: $repl"
      return 0
    fi
    sleep 3
  done
  echo "   !! $svc 未在 90s 内收敛，请检查: docker service ps $STACK"_"$svc" >&2
  return 1
}

# ── 回滚分支 ──
if [ "${1:-}" = "rollback" ]; then
  shift
  if [ "$#" -eq 0 ]; then set -- "${ALL_SERVICES[@]}"; fi
  for svc in "$@"; do
    echo ">> 回滚 $STACK"_"$svc"
    docker service rollback "$STACK"_"$svc" 2>&1 | tail -1
  done
  echo ">> 回滚完成"
  exit 0
fi

# ── 正常更新分支 ──
VERSION="${1:?用法: release.sh <version> [service ...] 或 release.sh rollback [service]}"
shift || true

if [ "$#" -gt 0 ]; then
  SERVICES=("$@")
else
  SERVICES=("${ALL_SERVICES[@]}")
fi

for svc in "${SERVICES[@]}"; do
  update_one "$svc" "$VERSION"
done

echo ">> 等待收敛..."
for svc in "${SERVICES[@]}"; do
  wait_converge "$svc" || true
done

echo ">> 更新完成，当前服务状态："
docker service ls --format '{{.Name}}  {{.Image}}  {{.Replicas}}' | grep "$STACK"_ | sort
