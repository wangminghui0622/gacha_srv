#!/usr/bin/env bash
# ════════════════════════════════════════════════════════════════════
# 部署环境配置中心：换服务器/改密码/改镜像源，只需改这一个文件。
# 用法：各脚本 `source "$(dirname "$0")/env.sh"`；编排 yaml 用 ${VAR} 插值。
# ════════════════════════════════════════════════════════════════════

# ── 集群拓扑（3 台服务器）──
export MASTER_IP="192.168.197.128"        # 主节点：Swarm Manager + 基础设施 + registry
export MASTER_USER="master"               # 主节点 SSH 用户名
export MASTER_NODE="master"               # 主节点 hostname（infra 部署约束用）

export WORKER1_IP="192.168.197.129"
export WORKER1_USER="slaver1"
export WORKER1_NODE="slaver1"

export WORKER2_IP="192.168.197.131"
export WORKER2_USER="slaver2"
export WORKER2_NODE="slaver2"

# ── 基础设施密码（生产建议改掉，可改由外部注入）──
export MYSQL_PASSWORD="we7root"
export REDIS_PASSWORD="we7redis"

# ── 各节点 sudo 密码（bootstrap 配 daemon.json、teardown 清理时用）──
export SUDO_PASSWORD="1"

# ── 镜像源（Docker Hub 被墙时，拉基础镜像用；若 Docker Hub 可达可留空）──
export MIRROR="docker.1ms.run"

# ── 派生值（一般不用改）──
export REGISTRY="${MASTER_IP}:5000"       # 本地镜像仓库地址

# SSH 目标（数组不能 export，脚本内部用）
SSH_MASTER="${MASTER_USER}@${MASTER_IP}"
SSH_WORKER1="${WORKER1_USER}@${WORKER1_IP}"
SSH_WORKER2="${WORKER2_USER}@${WORKER2_IP}"
WORKERS=("$SSH_WORKER1" "$SSH_WORKER2")
