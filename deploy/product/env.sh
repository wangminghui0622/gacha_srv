#!/usr/bin/env bash
# ════════════════════════════════════════════════════════════════════
# product 环境配置（外部基础设施：MySQL/Redis/Etcd 均外部托管，用账户密码访问）
# 改这里：集群地址 + 外部基础设施地址 + 账户密码。
# ════════════════════════════════════════════════════════════════════

# ── 主节点（Swarm Manager + registry）──
export MASTER_IP="<生产主节点IP>"
export MASTER_USER="<SSH用户>"
export MASTER_NODE="<hostname>"
export MASTER_SUDO_PASS="<sudo密码>"

# ── 从机 1（Swarm Worker）──
export WORKER1_IP="<从机1IP>"
export WORKER1_USER="<SSH用户>"
export WORKER1_SUDO_PASS="<sudo密码>"

# ── 从机 2（Swarm Worker）──
export WORKER2_IP="<从机2IP>"
export WORKER2_USER="<SSH用户>"
export WORKER2_SUDO_PASS="<sudo密码>"

# ── 基础设施模式：0 = 外部（不部署 MySQL/Redis/Etcd，不跑迁移）──
export INFRA_BUILTIN="0"

# ── 外部基础设施连接（账户密码）──
export MYSQL_HOST="<外部MySQL地址>"
export MYSQL_PORT="3306"
export MYSQL_USER="<MySQL账户>"           # 例如 luckybox（非 root）
export MYSQL_PASSWORD="<MySQL密码>"

export REDIS_HOST="<外部Redis地址>"
export REDIS_PORT="6379"
export REDIS_PASSWORD="<Redis密码>"

export ETCD_ENDPOINTS="<外部Etcd地址>:2379"

# ── 业务服务 overlay 网络名（外部时 bootstrap 自建）──
export NETWORK_NAME="gacha_backend"

# ── 镜像仓库（外部）──
export REGISTRY_BUILTIN="0"
export REGISTRY="<外部仓库地址>"       # 例如 harbor.example.com

# ── 镜像源 ──
export MIRROR="docker.1ms.run"

SSH_MASTER="${MASTER_USER}@${MASTER_IP}"
WORKERS_SSH=("${WORKER1_USER}@${WORKER1_IP}" "${WORKER2_USER}@${WORKER2_IP}")
WORKERS_SUDO=("${WORKER1_SUDO_PASS}" "${WORKER2_SUDO_PASS}")
