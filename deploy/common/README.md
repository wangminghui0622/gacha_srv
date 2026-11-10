# deploy 部署

gacha_srv 的 docker-compose 编排：四个环境（dev / test / staging / product），本地编译二进制 + 配置打包进镜像（不在镜像内编译），按依赖顺序启动。

## 目录结构

```
deploy/
├── dev.sh / test.sh / staging.sh / product.sh   # 各环境入口脚本
├── common/                 # 公共资源
│   ├── images/             # 镜像构建
│   │   ├── go.Dockerfile   # go-zero 服务通用镜像（COPY 二进制 + 配置 + entrypoint）
│   │   ├── dtm.Dockerfile  # DTM 镜像（-c 标志）
│   │   ├── migrate.Dockerfile # 数据库迁移镜像（执行 script/split/*.sql）
│   │   ├── entrypoint.sh   # 运行时 envsubst 替换配置占位符
│   │   └── migrate.sh      # 按序执行 SQL 迁移
│   ├── infra/compose.yaml  # 基础设施：MySQL / Redis / Etcd（dev、test 引用）
│   ├── services.yaml       # 业务服务公共定义（13 服务 + 迁移 + depends_on 顺序）
│   ├── scripts/build-images.sh # 本地编译 + 生成配置模板 + 构建镜像
│   └── README.md
├── dev/                    # 本地开发：内置基础设施 + 端口映射宿主机
├── test/                   # CI/集成测试：内置基础设施，隔离端口
├── staging/                # 预发：外部托管基础设施
├── product/                # 生产：外部基础设施 + 多副本/资源限制
└── conf/                   # 生成的配置模板（build-images.sh 产出，占位符）
```

## 四个环境

| 环境 | 基础设施 | 端口映射 | 用途 |
|------|---------|---------|------|
| dev | 内置（MySQL/Redis/Etcd） | 全部映射到 127.0.0.1 | 本地开发调试 |
| test | 内置（端口与 dev 错开） | 不映射业务服务 | CI/集成测试 |
| staging | 外部托管 | 不映射 | 预发验证 |
| product | 外部托管 | 不映射，多副本 | 生产 |

## 快速开始

```bash
# 1. 构建镜像（本地编译 Linux 二进制 + 生成配置模板 + docker build）
bash deploy/common/scripts/build-images.sh dev

# 2. 准备环境变量
cp deploy/dev/.env.example deploy/dev/.env   # 按需修改密码

# 3. 启动 dev 环境
bash deploy/dev.sh up

# 查看状态 / 日志 / 停止
bash deploy/dev.sh ps
bash deploy/dev.sh logs
bash deploy/dev.sh down
```

## 镜像构建策略

「本地二进制 + 配置打包进镜像」：`build-images.sh` 在本机交叉编译 Linux 二进制（`CGO_ENABLED=0 GOOS=linux`）到 `deploy/bin/`，生成配置模板到 `deploy/conf/`，再 `docker build` 把二进制 + 配置 COPY 进 alpine 镜像。**镜像内不做任何编译**，构建快、可复现。

## 配置占位符机制

go-zero 的 `conf.MustLoad` 不支持环境变量，故配置模板里用 `${VAR}` 占位，镜像 entrypoint 用 `envsubst` 在容器启动时替换（密码只经 compose environment 注入，不打进镜像）：

| 占位符 | 含义 | dev/test 默认值 | staging/product |
|--------|------|----------------|----------------|
| `${MYSQL_HOST}` | MySQL 地址 | `mysql` | 外部地址（.env） |
| `${MYSQL_PASS}` | MySQL 密码 | `we7root`（.env） | 外部密码（.env） |
| `${REDIS_HOST}` | Redis 地址 | `redis` | 外部地址（.env） |
| `${REDIS_PASS}` | Redis 密码 | `we7redis`（.env） | 外部密码（.env） |
| `${ETCD_ENDPOINTS}` | Etcd 地址 | `etcd:2379` | 外部地址（.env） |

服务间 RPC 直连（DTM 回调，DTM 不接 Etcd）在配置模板里替换为容器服务名（如 `draw-rpc:9313`）；服务监听地址替换为 `0.0.0.0`（容器内互通）。

## 服务启动顺序

```
L0 基础设施   MySQL / Redis / Etcd            (healthcheck 就绪)
L1 迁移       migrate                          (mysql healthy 后执行建库建表)
L2 事务       dtm                              (mysql/etcd healthy + migrate 完成)
L3 基础 RPC   base / catalog / draw / task / ops
L3.5          thirdparty → ops、identity → thirdparty
L4 循环组     money ↔ marketing ↔ play         (Etcd 发现 + go-zero RPC 重试自愈)
L5 网关       gateway / admin-gateway          (聚合所有 RPC，最后启动)
```

- 所有业务服务 `depends_on`：etcd + mysql（healthy）、migrate（completed）、dtm（started）。
- `money`/`marketing`/`play` 存在循环调用，不写 `depends_on`（避免 compose 循环依赖报错），靠 Etcd 服务发现 + go-zero RPC 客户端重试在启动后自愈。
- `gateway` 聚合 7 个 RPC，`admin-gateway` 聚合 7 个 RPC，`depends_on` 全部就绪后启动。

## 端口一览

| 服务 | 端口 | 服务 | 端口 |
|------|------|------|------|
| gateway | 9310 | identity-rpc | 9311 |
| money-rpc | 9312 | draw-rpc | 9313 |
| base-rpc | 9314 | marketing-rpc | 9315 |
| play-rpc | 9316 | task-rpc | 9317 |
| catalog-rpc | 9318 | thirdparty-rpc | 9319 |
| ops-rpc | 9320 | admin-gateway | 9321 |
| dtm | 36789/36790 | MySQL/Redis/Etcd | 3306/6379/2379 |

## 多机部署（多副本）

product 环境所有服务（DTM + 10 RPC + 2 网关）都配置了 `replicas: 2`，多副本的正确性由以下机制保证：

| 链路 | 机制 |
|------|------|
| DTM 多副本 | MySQL(`gacha_dtm`) 共享全局事务 + 存储层锁 `LockOneGlobalTransaction`（`FOR UPDATE`）协调，避免多副本重复处理同一事务 |
| RPC 多副本 | go-zero + Etcd 服务发现，gateway / 服务间客户端自动负载均衡 |
| DTM → RPC 回调 | 回调地址为服务名（`base-rpc:9314`）+ DTM 默认 round_robin（`GrpcServiceConfig` 默认值） |
| gateway → DTM | `DtmServer.Target` 为服务名 + `GRPC_SERVICE_CONFIG=round_robin`（services.yaml 注入） |
| ID 不冲突 | 无 Snowflake（MySQL 自增 + UUID requestId），多副本天然安全 |

**Swarm 部署注意**：

- `deploy.replicas` 是 Swarm 语法，多机部署用 `docker stack deploy`：
  ```bash
  docker swarm init
  docker stack deploy -c deploy/product/compose.yaml gacha
  ```
- 已配置 `endpoint_mode: dnsrr`：gRPC 的 round_robin 需要解析到多个副本 IP，而 Swarm 默认 VIP 模式只返回单个 VIP（round_robin 失效），DNSRR 让每个副本暴露独立 IP。
- 单机多副本（不建 Swarm）用 `docker compose --compatibility up`（把 `deploy.replicas` 映射为 `--scale`），compose 的 DNS 天然返回所有副本 IP，无 VIP 问题。

## 说明与限制

- **未处理**：thirdparty 的微信/alipay mock 地址（`127.0.0.1:9410/9411`）、前端（web/console、web/mini-app）暂不纳入编排。
- 旧的 `deploy/dev/` 本地 shell 脚本（build.sh/start.sh/stop.sh）已由本 docker-compose 方案取代。
