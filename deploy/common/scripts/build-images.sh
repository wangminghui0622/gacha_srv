#!/usr/bin/env bash
# 构建脚本：本地编译 13 个二进制 → 生成配置模板（占位符）→ docker build 镜像。
# 「本地二进制 + 配置打包进镜像」，不在镜像内编译。
# 用法：bash deploy/scripts/build-images.sh [tag]    # tag 默认 dev
set -euo pipefail

DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REPO_ROOT="$(cd "$DEPLOY_DIR/.." && pwd)"
BIN_DIR="$DEPLOY_DIR/bin"
CONF_DIR="$DEPLOY_DIR/conf"
TAG="${1:-dev}"

cd "$REPO_ROOT"
mkdir -p "$BIN_DIR" "$CONF_DIR"

# ── 1. 交叉编译 Linux 二进制（docker 镜像为 Linux；纯 Go，CGO 关闭）──
echo ">> 编译 Linux 二进制（GOOS=linux）"
export CGO_ENABLED=0 GOOS=linux GOARCH=amd64
(cd app/dtm && go build -o "$BIN_DIR/dtm" .)
for svc in identity money draw base marketing play task catalog thirdparty ops; do
  (cd "app/$svc" && go build -o "$BIN_DIR/$svc-rpc" .)
  echo "   $svc-rpc"
done
(cd app/gateway && go build -o "$BIN_DIR/gateway" .)
(cd app/admin-gateway && go build -o "$BIN_DIR/admin-gateway" .)
echo "   gateway / admin-gateway / dtm"

# ── 2. 生成配置模板：pkg/etcs 复制到 deploy/conf，地址/密码替换为占位符 ──
echo ">> 生成配置模板（占位符 ${MYSQL_HOST}/${MYSQL_PASS}/${REDIS_HOST}/${REDIS_PASS}/${ETCD_ENDPOINTS}）"
cp -f pkg/etcs/*.yaml "$CONF_DIR/"
cp -f app/dtm/conf.yml "$CONF_DIR/dtm.yml"

for conf in "$CONF_DIR"/*.yaml; do
  sed -i \
    -e 's|ListenOn: 127.0.0.1:|ListenOn: 0.0.0.0:|g' \
    -e 's|root:we7root@tcp(127.0.0.1:3306)|root:${MYSQL_PASS}@tcp(${MYSQL_HOST}:3306)|g' \
    -e 's|127.0.0.1:6379|${REDIS_HOST}:6379|g' \
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

# dtm.yml 单独替换（字段格式不同：Host/Password 分离）
sed -i \
  -e "s|Host: '127.0.0.1'|Host: '\${MYSQL_HOST}'|g" \
  -e "s|Password: 'we7root'|Password: '\${MYSQL_PASS}'|g" \
  "$CONF_DIR/dtm.yml"

# ── 3. docker build 镜像 ──
echo ">> 构建镜像（tag=$TAG）"
build_go() { # $1 二进制名 $2 镜像名 $3 配置文件名（默认 $1.yaml）
  local conf="${3:-$1.yaml}"
  docker build -f deploy/common/images/go.Dockerfile \
    --build-arg BINARY="$1" --build-arg CONFIG="conf/$conf" \
    -t "gacha/$2:$TAG" .
}
build_go identity-rpc identity-rpc
build_go money-rpc money-rpc
build_go draw-rpc draw-rpc
build_go base-rpc base-rpc
build_go marketing-rpc marketing-rpc
build_go play-rpc play-rpc
build_go task-rpc task-rpc
build_go catalog-rpc catalog-rpc
build_go thirdparty-rpc thirdparty-rpc
build_go ops-rpc ops-rpc
build_go gateway gateway gateway-api.yaml
build_go admin-gateway admin-gateway admin-gateway-api.yaml

docker build -f deploy/common/images/dtm.Dockerfile --build-arg CONFIG="conf/dtm.yml" -t "gacha/dtm:$TAG" .
docker build -f deploy/common/images/migrate.Dockerfile -t "gacha/migrate:$TAG" .

echo ">> 完成：bin/ $(ls "$BIN_DIR" | wc -l) 个二进制 + conf/ $(ls "$CONF_DIR" | wc -l) 个配置 + 14 个镜像（tag=$TAG）"
