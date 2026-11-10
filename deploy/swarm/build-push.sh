#!/usr/bin/env bash
# 构建镜像并推送到本地 registry，记录每个镜像的 sha256 digest（版本更新第 2 步，在 128 上执行）。
# 前置：已把交叉编译好的二进制放到 deploy/bin/（第 1 步在本机完成，见 deploy.md）。
# 用法：bash build-push.sh <version> [service ...]
#       例：bash build-push.sh v1.0.1                      # 全部 13 服务 + dtm
#            bash build-push.sh v1.0.1 money-rpc draw-rpc  # 只构建指定服务
# 产物：registry 里的 <version> 与 prod 两个 tag，以及 deploy/swarm/releases/<version>.digest
#       （记录「服务名 sha256 digest」，供 release.sh 按 digest 锁版本，回滚时可靠）。
set -euo pipefail

VERSION="${1:?用法: build-push.sh <version> [service ...]}"
shift || true
REG="192.168.197.128:5000"
cd "$(dirname "$0")/../.."   # 仓库根（/home/master/gacha）

MANIFEST="deploy/swarm/releases/$VERSION.digest"
mkdir -p deploy/swarm/releases
: > "$MANIFEST"   # 清空（若重复构建同一版本，覆盖）

# 服务清单：二进制名 镜像名 [配置文件名]
ALL=(
  "identity-rpc identity-rpc"
  "money-rpc money-rpc"
  "draw-rpc draw-rpc"
  "base-rpc base-rpc"
  "marketing-rpc marketing-rpc"
  "play-rpc play-rpc"
  "task-rpc task-rpc"
  "catalog-rpc catalog-rpc"
  "thirdparty-rpc thirdparty-rpc"
  "ops-rpc ops-rpc"
  "gateway gateway gateway-api.yaml"
  "admin-gateway admin-gateway admin-gateway-api.yaml"
)

# 构建 + 推送（<version> 与 prod 两个 tag）+ 记录 digest
push_and_record() { # $1 二进制 $2 镜像名 $3 配置文件名（默认 $1.yaml）
  local conf="${3:-$1.yaml}"
  echo ">> 构建 gacha/$2:$VERSION"
  docker build -q -f deploy/common/images/go.Dockerfile \
    --build-arg BINARY="$1" --build-arg CONFIG="conf/$conf" \
    -t "gacha/$2:$VERSION" . >/dev/null
  docker tag "gacha/$2:$VERSION" "$REG/gacha/$2:$VERSION"
  docker push "$REG/gacha/$2:$VERSION" >/dev/null
  docker tag "gacha/$2:$VERSION" "$REG/gacha/$2:prod"
  docker push "$REG/gacha/$2:prod" >/dev/null
  # 记录 digest（内容寻址，不可变）
  local digest
  digest=$(docker image inspect --format '{{index .RepoDigests 0}}' "$REG/gacha/$2:$VERSION" | sed 's/.*@//')
  echo "$2 $digest" >> "$MANIFEST"
  echo "   OK $2 @${digest:0:19}…"
}

if [ "$#" -gt 0 ]; then
  # 只构建指定服务（按镜像名匹配）
  for want in "$@"; do
    found=0
    for entry in "${ALL[@]}"; do
      read -r bin img conf <<< "$entry"
      [ "$img" = "$want" ] || continue
      push_and_record "$bin" "$img" "$conf"
      found=1
    done
    [ "$found" -eq 1 ] || { echo "未知服务: $want"; exit 2; }
  done
else
  for entry in "${ALL[@]}"; do
    read -r bin img conf <<< "$entry"
    push_and_record "$bin" "$img" "$conf"
  done
  # DTM（配置格式不同）
  echo ">> 构建 gacha/dtm:$VERSION"
  docker build -q -f deploy/common/images/dtm.Dockerfile \
    --build-arg CONFIG="conf/dtm.yml" -t "gacha/dtm:$VERSION" . >/dev/null
  docker tag "gacha/dtm:$VERSION" "$REG/gacha/dtm:$VERSION"
  docker push "$REG/gacha/dtm:$VERSION" >/dev/null
  docker tag "gacha/dtm:$VERSION" "$REG/gacha/dtm:prod"
  docker push "$REG/gacha/dtm:prod" >/dev/null
  dtm_digest=$(docker image inspect --format '{{index .RepoDigests 0}}' "$REG/gacha/dtm:$VERSION" | sed 's/.*@//')
  echo "dtm $dtm_digest" >> "$MANIFEST"
  echo "   OK dtm @${dtm_digest:0:19}…"
fi

echo ""
echo ">> 完成，digest 清单 $MANIFEST："
cat "$MANIFEST"
echo ">> registry 目录："
curl -s -m 5 "http://$REG/v2/_catalog"
echo ""
