# dev 环境部署说明

**dev 环境 = 内置基础设施的完整参考实现**：`bootstrap.sh` 会自动部署 MySQL/Redis/Etcd 并执行迁移，服务用服务名（`mysql`/`redis`/`etcd`）互连。

> 本目录下的 `swarm/` 脚本是**四个环境（dev/test/staging/product）共用**的部署脚本，test/staging/product 通过各自的 `deploy.sh` + `env.sh`（`INFRA_BUILTIN=0`，外部基础设施）复用同一套脚本。完整部署报告见仓库根目录 `deploy.md`。

## 目录结构

```
dev/
├── swarm/                  # 部署脚本 + 编排（四环境共用）
│   ├── env.sh              # ★ 环境配置中心（地址/用户/密码/基础设施模式）
│   ├── bootstrap.sh        # 一键从零部署（swarm→registry→基础设施→迁移→构建→部署）
│   ├── build-binaries.sh   # 开发机：交叉编译 13 个二进制 + 生成配置模板
│   ├── build-push.sh       # 构建镜像 + 推 registry + 记录 sha256 digest
│   ├── release.sh          # 滚动更新 / 回滚
│   ├── teardown.sh         # 清理（--full 彻底清空）
│   ├── infra-stack.yaml    # 基础设施编排（MySQL/Redis/Etcd，仅 dev 用）
│   └── services-stack.yaml # 业务服务编排（13 服务 × 3 副本）
├── common/images/          # Dockerfile（go.Dockerfile / dtm.Dockerfile）+ entrypoint
├── bin/                    # 编译产物（build-binaries.sh 生成，gitignore）
└── conf/                   # 配置模板（build-binaries.sh 生成，占位符运行时替换）
```

## env.sh 配置（换环境只改这一个文件）

每台机器的设置独立（三台密码可各不相同）：

```bash
# 集群拓扑（3 台机器）
MASTER_IP / MASTER_USER / MASTER_NODE / MASTER_SUDO_PASS
WORKER1_IP / WORKER1_USER / WORKER1_SUDO_PASS
WORKER2_IP / WORKER2_USER / WORKER2_SUDO_PASS

# 基础设施模式
INFRA_BUILTIN="1"            # dev=1（内置）；test/staging/product=0（外部）

# 基础设施连接（内置时保持服务名，外部时填真实地址+账户密码）
MYSQL_HOST / MYSQL_PORT / MYSQL_USER / MYSQL_PASSWORD
REDIS_HOST / REDIS_PORT / REDIS_PASSWORD
ETCD_ENDPOINTS

# 其他
NETWORK_NAME                 # overlay 网络名
MIRROR                       # 镜像源（Docker Hub 被墙时用 docker.1ms.run）
```

## 快速开始（3 步）

```bash
# ① 开发机（装有 Go 的 Linux/WSL）：交叉编译 + 生成配置
bash deploy/dev/swarm/build-binaries.sh

# ② 上传到主节点（二进制/配置/Dockerfile/脚本/SQL）
scp -r deploy/dev/bin deploy/dev/conf deploy/dev/common/images deploy/dev/swarm script/split \
    master@<主节点IP>:/home/master/gacha/

# ③ 主节点：一键从零部署（含基础设施 + 迁移）
ssh master@<主节点IP> 'cd /home/master/gacha && bash deploy/dev/swarm/bootstrap.sh v1.0.0'
```

`bootstrap.sh` 幂等，自动按序完成：swarm 初始化 → 三节点加入 → registry → 基础镜像 → 部署 MySQL/Redis/Etcd → 迁移 → 构建推送镜像 → 部署服务 → 等收敛。全程约 40 分钟（构建镜像占大头）。

## 脚本说明

| 脚本 | 位置 | 作用 |
|------|------|------|
| `bootstrap.sh [version]` | 主节点 | 一键从零部署（幂等，7 步） |
| `build-binaries.sh [svc...]` | 开发机 | 交叉编译 + 生成配置（传服务名只编这些） |
| `build-push.sh <version> [svc...]` | 主节点 | 构建镜像 + 推 registry + 记录 digest |
| `release.sh <version> [svc...]` | 主节点 | 按 digest 滚动更新 |
| `release.sh rollback [svc]` | 主节点 | 回滚到上一个版本 |
| `teardown.sh [--full]` | 主节点 | 清理（--full 清镜像/卷/swarm/registry） |

## 版本更新与回滚

```bash
# 更新（sha256 digest 锁版本，零停机滚动）
bash deploy/dev/swarm/build-push.sh v1.0.1 money-rpc     # 构建推送
bash deploy/dev/swarm/release.sh v1.0.1 money-rpc        # 滚动更新

# 回滚
bash deploy/dev/swarm/release.sh rollback money-rpc      # 回滚上一个
bash deploy/dev/swarm/release.sh v1.0.0 money-rpc        # 回滚到指定版本
```

## 运维

```bash
docker stack services gacha            # 业务服务（应全 3/3）
docker stack services gacha-infra      # 基础设施（1/1）
docker service logs gacha_money-rpc    # 日志
docker service scale gacha_money-rpc=5 # 扩容
curl http://<主节点>:9310/gacha/api/v1/boxes   # 网关测试
```

## 注意事项

- `daemon.json` 只配 `insecure-registries`，**不要加 `live-restore`**（与 Swarm 模式不兼容，会导致 dockerd 启动失败）。
- 外网 Docker Hub 被墙，基础镜像（alpine/mysql/redis/registry）走镜像源 `docker.1ms.run`（bootstrap 自动处理）。
- 换服务器/改密码只改 `env.sh`；详细版本更新、异常处理、schema 变更见 `../deploy.md`。
