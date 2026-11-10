#!/usr/bin/env bash
# 一键从零部署（在 128 上执行）。前置：本机已交叉编译并上传到 /home/master/gacha。
# 覆盖：Swarm 初始化 → 三节点加入 → 本地 registry → 基础设施 → 迁移 → 构建推送 → 部署服务。
# 用法：bash deploy/swarm/bootstrap.sh [version]    # version 默认 v1.0.0
#
# 前置（开发机，见 deploy.md 10.1）：
#   bash deploy/swarm/build-binaries.sh
#   scp -r deploy/bin deploy/conf deploy/common/images script/split \
#       master@192.168.197.128:/home/master/gacha/
set -euo pipefail

VERSION="${1:-v1.0.0}"
REG="192.168.197.128:5000"
REPO="/home/master/gacha"
WORKERS=(slaver1@192.168.197.129 slaver2@192.168.197.131)
MIRROR="docker.1ms.run"
cd "$REPO"

log() { echo -e "\n\033[1;32m==>\033[0m $*"; }

# ── 0. 前置检查 ──
log "0/7 前置检查"
[ -d deploy/bin ] && [ "$(ls deploy/bin 2>/dev/null | wc -l)" -ge 13 ] || { echo "缺少 deploy/bin（13 个二进制），请先本地交叉编译并上传" >&2; exit 1; }
[ -d deploy/conf ] || { echo "缺少 deploy/conf" >&2; exit 1; }
[ -d deploy/common/images ] || { echo "缺少 deploy/common/images（Dockerfile）" >&2; exit 1; }
[ -d script/split ] || { echo "缺少 script/split（SQL）" >&2; exit 1; }
echo "   二进制 $(ls deploy/bin | wc -l) 个、配置 $(ls deploy/conf | wc -l) 个、SQL $(ls script/split | wc -l) 个 ✓"

# ── 1. Swarm 初始化 + 三节点加入 ──
log "1/7 初始化 Swarm 集群"
docker swarm init --advertise-addr 192.168.197.128 2>/dev/null && echo "   master 已初始化为 Leader" || echo "   (已初始化，跳过)"
TOKEN=$(docker swarm join-token -q worker 2>/dev/null)
for w in "${WORKERS[@]}"; do
  if ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "$w" "docker info 2>/dev/null | grep -q 'Swarm: active'" 2>/dev/null; then
    echo "   $w 已在集群中，跳过"
  else
    ssh -o StrictHostKeyChecking=no "$w" "docker swarm join --token $TOKEN 192.168.197.128:2377" 2>&1 | tail -1
  fi
done
docker node ls

# ── 2. 本地 registry + insecure-registries 配置 ──
log "2/7 配置本地 registry（三节点 insecure-registries）"
DAEMON='{"insecure-registries":["192.168.197.128:5000"]}'
# 写入各节点 daemon.json（仅当内容不同才重启 dockerd，避免不必要中断）
for node in "master@192.168.197.128" "${WORKERS[@]}"; do
  if [ "$node" = "master@192.168.197.128" ]; then
    cur=$(cat /etc/docker/daemon.json 2>/dev/null || echo "")
    if [ "$cur" != "$DAEMON" ]; then
      printf '%s' "$DAEMON" > /tmp/daemon.json
      echo '1' | sudo -S cp /tmp/daemon.json /etc/docker/daemon.json 2>/dev/null
      echo '1' | sudo -S systemctl reset-failed docker 2>/dev/null
      echo '1' | sudo -S systemctl restart docker 2>/dev/null
      echo "   master 已配置并重启 dockerd"
    else
      echo "   master 配置已就绪"
    fi
  else
    cur=$(ssh -o StrictHostKeyChecking=no "$node" 'cat /etc/docker/daemon.json 2>/dev/null || echo ""' 2>/dev/null)
    if [ "$cur" != "$DAEMON" ]; then
      printf '%s' "$DAEMON" | ssh -o StrictHostKeyChecking=no "$node" "cat > /tmp/daemon.json; echo 1 | sudo -S -p '' cp /tmp/daemon.json /etc/docker/daemon.json 2>/dev/null; echo 1 | sudo -S -p '' systemctl reset-failed docker 2>/dev/null; echo 1 | sudo -S -p '' systemctl restart docker 2>/dev/null" < /dev/null
      echo "   $node 已配置并重启 dockerd"
    else
      echo "   $node 配置已就绪"
    fi
  fi
done
sleep 5
# 启动 registry（若未运行）
if ! docker ps --format '{{.Names}}' | grep -q '^registry$'; then
  # 拉 registry:2（Docker Hub 被墙走镜像源）
  if ! docker image inspect registry:2 >/dev/null 2>&1; then
    echo "   拉取 registry:2（镜像源 $MIRROR）"
    docker pull "$MIRROR/library/registry:2"
    docker tag "$MIRROR/library/registry:2" registry:2
  fi
  docker run -d --name registry -p 5000:5000 --restart unless-stopped registry:2
  echo "   registry 已启动"
fi
sleep 3
curl -s -m 5 "http://$REG/v2/_catalog" && echo " ✓"

# ── 2b. 准备基础镜像（Docker Hub 被墙：alpine/mysql/redis 走镜像源拉取 + 重打 tag）──
# 清空镜像后 Docker Hub 无法重拉，必须在此预拉；etcd 来自 quay.io 可达，无需处理。
prepare_base() { # $1=镜像源完整名 $2=本地 tag
  if ! docker image inspect "$2" >/dev/null 2>&1; then
    echo "   拉取 $2（源 $1）"
    docker pull "$1" || { echo "   拉取 $1 失败（镜像源不可用？）" >&2; exit 1; }
    docker tag "$1" "$2"
  else
    echo "   $2 已存在，跳过"
  fi
}
echo "   准备基础镜像（构建用 alpine + 基础设施 mysql/redis）"
prepare_base "$MIRROR/library/alpine:3.20" "alpine:3.20"
prepare_base "$MIRROR/library/mysql:8" "mysql:8"
prepare_base "$MIRROR/library/redis:7-alpine" "redis:7-alpine"

# ── 3. 基础设施 ──
log "3/7 部署基础设施（MySQL/Redis/Etcd）"
docker stack deploy -c deploy/swarm/infra-stack.yaml gacha-infra 2>&1 | tail -4

# ── 4. 等待 MySQL 就绪 + 迁移 ──
log "4/7 等待 MySQL 就绪并执行迁移"
MYSQL_CID=""
for i in $(seq 1 40); do
  MYSQL_CID=$(docker ps -q -f name=gacha-infra_mysql | head -1)
  [ -n "$MYSQL_CID" ] && docker exec "$MYSQL_CID" mysql -h127.0.0.1 -uroot -pwe7root -e 'SELECT 1' >/dev/null 2>&1 && break
  sleep 5
done
[ -n "$MYSQL_CID" ] || { echo "MySQL 未就绪" >&2; exit 1; }
echo "   MySQL 就绪 ($MYSQL_CID)"
for f in script/split/*.sql; do
  echo "   执行 $(basename "$f")"
  docker exec -i "$MYSQL_CID" mysql -uroot -pwe7root --default-character-set=utf8mb4 < "$f" 2>/dev/null || {
    echo "   迁移失败: $f" >&2; exit 1;
  }
done
echo "   迁移完成：$(docker exec "$MYSQL_CID" mysql -uroot -pwe7root -N -e 'SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name LIKE "gacha_%" OR schema_name="dtm_barrier";' 2>/dev/null) 个库"

# ── 5. 构建镜像 + 推送（版本 tag + prod 别名）──
log "5/7 构建并推送镜像（$VERSION + prod）"
bash deploy/swarm/build-push.sh "$VERSION"

# ── 6. 部署业务服务 ──
log "6/7 部署业务服务"
docker stack deploy -c deploy/swarm/services-stack.yaml gacha 2>&1 | tail -4

# ── 7. 等待收敛 ──
log "7/7 等待服务收敛"
for i in $(seq 1 36); do
  ready=$(docker service ls --format '{{.Replicas}}' | grep -cE '^(3/3|1/1)$' || true)
  if [ "$ready" -ge 16 ]; then
    echo "   全部 16 个服务已收敛（第 $i 次检查）"
    break
  fi
  sleep 5
done
docker service ls --format '{{.Name}} {{.Replicas}}' | sort
echo ""
echo "================ 部署完成 ================"
echo "网关:   http://192.168.197.128:9310/gacha/api/v1"
echo "后台:   http://192.168.197.128:9321"
echo "registry: http://192.168.197.128:5000/v2/_catalog"
echo "版本:   $VERSION"
