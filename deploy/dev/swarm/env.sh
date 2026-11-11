#!/usr/bin/env bash
# ════════════════════════════════════════════════════════════════════
# 部署环境配置中心：换服务器/改密码/改镜像源，只需改这一个文件。
# 每台机器的设置独立（IP、SSH 用户、sudo 密码各管各的），三台密码不同也没关系。
# 用法：各脚本 `source "$(dirname "$0")/env.sh"`；编排 yaml 用 ${VAR} 插值。
# ════════════════════════════════════════════════════════════════════

# ── 主节点（Swarm Manager + registry）──
export MASTER_IP="192.168.197.128"
export MASTER_USER="master"
export MASTER_NODE="master"          # hostname（内置 infra 部署约束用）
export MASTER_SUDO_PASS="1"          # 主节点 sudo 密码

# ── 从机 1（Swarm Worker）──
export WORKER1_IP="192.168.197.129"
export WORKER1_USER="slaver1"
export WORKER1_SUDO_PASS="1"         # 从机 1 sudo 密码（可与主节点不同）

# ── 从机 2（Swarm Worker）──
export WORKER2_IP="192.168.197.131"
export WORKER2_USER="slaver2"
export WORKER2_SUDO_PASS="1"         # 从机 2 sudo 密码（可不同）

# ── 基础设施模式 ──
#   1 = 内置（dev）：bootstrap 会部署 MySQL/Redis/Etcd 并跑迁移
#   0 = 外部（test/staging/product）：用下面的外部地址+账户密码，不部署基础设施
export INFRA_BUILTIN="1"

# 业务服务 overlay 网络名（内置时=infra 栈网络，外部时=bootstrap 自建）
export NETWORK_NAME="gacha-infra_backend"

# ── 基础设施连接（外部时填真实地址+账户密码；内置时保持服务名即可）──
export MYSQL_HOST="mysql"            # 内置=服务名 mysql；外部=IP/域名
export MYSQL_PORT="3306"
export MYSQL_USER="root"             # 账户（外部时改成真实账号，如 luckybox）
export MYSQL_PASSWORD="we7root"      # 密码

export REDIS_HOST="redis"            # 内置=服务名 redis；外部=IP/域名
export REDIS_PORT="6379"
export REDIS_PASSWORD="we7redis"

export ETCD_ENDPOINTS="etcd:2379"    # 内置=服务名 etcd:2379；外部=IP:2379（多节点逗号分隔）

# ── 镜像源（Docker Hub 被墙时拉基础镜像用）──
export MIRROR="docker.1ms.run"

# ── 派生值（脚本遍历用，一般不用改）──
export REGISTRY="${MASTER_IP}:5000"  # 本地镜像仓库地址

SSH_MASTER="${MASTER_USER}@${MASTER_IP}"
# 从机 SSH 目标 与 sudo 密码，两个数组下标一一对应（增删从机需同时改两处）
WORKERS_SSH=("${WORKER1_USER}@${WORKER1_IP}" "${WORKER2_USER}@${WORKER2_IP}")
WORKERS_SUDO=("${WORKER1_SUDO_PASS}" "${WORKER2_SUDO_PASS}")
