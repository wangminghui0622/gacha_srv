#!/usr/bin/env bash
# 本地编译 + 生成配置（在装有 Go 的开发机 / Linux / WSL 上执行）。
# 产物：deploy/dev/bin（13 个二进制）+ deploy/conf（配置模板，占位符待运行时替换）。
# 供 128 上 build-push.sh 构建镜像 / bootstrap.sh 一键部署。
# 用法：bash deploy/dev/swarm/build-binaries.sh [服务名 ...]
#       不传服务名 = 全部；只传部分 = 只编译这些（加快迭代；配置始终全量重生成）。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/env.sh"

REPO="$(cd "$SCRIPT_DIR/../../.." && pwd)"   # 仓库根
BIN="$SCRIPT_DIR/../bin"                     # deploy/dev/bin
CONF="$SCRIPT_DIR/../conf"                   # deploy/dev/conf
mkdir -p "$BIN" "$CONF"

# ── 1. 交叉编译 Linux 二进制 ──
export CGO_ENABLED=0 GOOS=linux GOARCH=amd64
LD='-s -w'

# 服务清单：目录名 -> 二进制名
declare -A ALL=(
  [dtm]=dtm
  [identity]=identity-rpc
  [money]=money-rpc
  [draw]=draw-rpc
  [base]=base-rpc
  [marketing]=marketing-rpc
  [play]=play-rpc
  [task]=task-rpc
  [catalog]=catalog-rpc
  [thirdparty]=thirdparty-rpc
  [ops]=ops-rpc
  [gateway]=gateway
  [admin-gateway]=admin-gateway
)

if [ "$#" -gt 0 ]; then
  TARGETS=("$@")
else
  TARGETS=("${!ALL[@]}")
fi

cd "$REPO"
for svc in "${TARGETS[@]}"; do
  [ -n "${ALL[$svc]:-}" ] || { echo "未知服务目录: $svc" >&2; exit 1; }
  out="$BIN/${ALL[$svc]}"
  echo ">> 编译 $svc -> ${ALL[$svc]}"
  go build -trimpath -ldflags "$LD" -o "$out" "./app/$svc"
done

# ── 2. 生成配置模板（pkg/etcs + dtm conf → deploy/dev/conf，地址/密码替换为占位符）──
echo ">> 生成配置模板（占位符 \${MYSQL_HOST}/\${MYSQL_PASS}/\${REDIS_HOST}/\${REDIS_PASS}/\${ETCD_ENDPOINTS}）"
cp -f "$REPO/pkg/etcs/"*.yaml "$CONF/"
cp -f "$REPO/app/dtm/conf.yml" "$CONF/dtm.yml"

for conf in "$CONF"/*.yaml; do
  sed -i \
    -e 's|ListenOn: 127.0.0.1:|ListenOn: 0.0.0.0:|g' \
    -e 's|root:we7root@tcp(127.0.0.1:3306)|${MYSQL_USER}:${MYSQL_PASS}@tcp(${MYSQL_HOST}:${MYSQL_PORT})|g' \
    -e 's|127.0.0.1:6379|${REDIS_HOST}:${REDIS_PORT}|g' \
    -e 's|Pass: "we7redis"|Pass: "${REDIS_PASS}"|g' \
    -e 's|127.0.0.1:2379|${ETCD_ENDPOINTS}|g' \
    -e 's|127.0.0.1:36790|dtm:36790|g' \
    -e 's|127.0.0.1:9311|identity-rpc:9311|g' \
    -e 's|127.0.0.1:9312|money-rpc:9312|g' \
    -e 's|127.0.0.1:9313|draw-rpc:9313|g' \
    -e 's|127.0.0.1:9314|base-rpc:9314|g' \
    -e 's|127.0.0.1:9315|marketing-rpc:9315|g' \
    -e 's|127.0.0.1:9316|play-rpc:9316|g' \
    -e 's|127.0.0.1:9317|task-rpc:9317|g' \
    -e 's|127.0.0.1:9318|catalog-rpc:9318|g' \
    -e 's|127.0.0.1:9319|thirdparty-rpc:9319|g' \
    -e 's|127.0.0.1:9320|ops-rpc:9320|g' \
    "$conf"
done

# dtm.yml 单独（字段格式不同：Host/User/Password 分离）
sed -i \
  -e "s|Host: '127.0.0.1'|Host: '\${MYSQL_HOST}'|g" \
  -e "s|User: 'root'|User: '\${MYSQL_USER}'|g" \
  -e "s|Password: 'we7root'|Password: '\${MYSQL_PASS}'|g" \
  "$CONF/dtm.yml"

echo ""
echo ">> 完成：二进制 $(ls "$BIN" | wc -l) 个 + 配置 $(ls "$CONF" | wc -l) 个"
ls -lh "$BIN" | awk 'NR>1 { printf "   %-16s %s\n", $9, $5 }'
echo ""
echo ">> 下一步：上传到主节点并一键部署（地址见 env.sh）"
echo "   scp -r deploy/dev/bin deploy/dev/conf deploy/dev/common/images deploy/dev/swarm script/split ${SSH_MASTER}:/home/master/gacha/"
echo "   ssh ${SSH_MASTER} 'cd /home/master/gacha && bash deploy/dev/swarm/bootstrap.sh <version>'"
