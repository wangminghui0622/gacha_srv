# gacha_srv 部署报告

> 抽奖/盲盒系统（go-zero + gRPC + MySQL + Redis + Etcd + DTM SAGA）在 3 台虚拟机上的 Docker Swarm 集群部署，每服务 3 副本（含 DTM）。

## 1. 部署拓扑

| 节点 | 内网 IP | 用户名 | 角色 | CPU | 内存 | 磁盘 |
|------|---------|--------|------|-----|------|------|
| master | 192.168.197.128 | master | Swarm Manager + 基础设施 | 4 | 10Gi | 18G |
| slaver1 | 192.168.197.129 | slaver1 | Swarm Worker | 4 | 3.8Gi | 9.8G |
| slaver2 | 192.168.197.131 | slaver2 | Swarm Worker | 4 | 3.8Gi | 9.8G |

- **Docker** 29.9.2 × 3，Swarm 集群：`master` = Leader，`slaver1`/`slaver2` = Worker。
- **基础设施**（MySQL 8.4 / Redis 7 / Etcd 3.5.16）以 Swarm 服务方式约束到 master 节点，挂 attachable overlay 网络 `gacha-infra_backend`，业务服务通过服务名 `mysql`/`redis`/`etcd` 互连（不依赖 host IP）。
- **业务服务**：13 个服务（DTM + 10 RPC + 2 网关）各 3 副本 = 39 容器，均分布到 3 台节点（每节点各 1 副本）。

## 2. 服务清单

| 服务 | 端口 | 副本 | 说明 |
|------|------|------|------|
| gateway | 9310 | 3 | 玩家 HTTP 网关（VIP + 发布端口） |
| admin-gateway | 9321 | 3 | 运营后台网关（VIP + 发布端口） |
| identity-rpc | 9311 | 3 | 身份（账号/会话） |
| money-rpc | 9312 | 3 | 资金（钱包/资产/支付/退款） |
| draw-rpc | 9313 | 3 | 抽奖引擎 |
| base-rpc | 9314 | 3 | 基础（资格/风控） |
| marketing-rpc | 9315 | 3 | 营销（报价/忠诚度/活动/券） |
| play-rpc | 9316 | 3 | 玩法（满车/扭蛋/连击链/拼团/挖宝/翻牌） |
| task-rpc | 9317 | 3 | 任务（签到/通行证/会员） |
| catalog-rpc | 9318 | 3 | 商品目录 |
| thirdparty-rpc | 9319 | 3 | 第三方（短信/微信/支付宝/广告） |
| ops-rpc | 9320 | 3 | 运营（权限/任务计划/看板） |
| **dtm** | 36789/36790 | **3** | **DTM SAGA 协调器（重点）** |
| mysql / redis / etcd | 3306 / 6379 / 2379 | 1 | 基础设施（约束 master） |

**DTM 多副本机制**：3 副本共享 MySQL `gacha_dtm` 库存储全局事务，存储层锁 `LockOneGlobalTransaction` 协调（同一事务只被一个副本处理，避免重复）；gRPC 客户端默认 `round_robin`（`config.go` `GrpcServiceConfig` 默认值 + `GRPC_SERVICE_CONFIG` 环境变量），回调地址用服务名 + DNSRR 端点模式解析到多副本 IP。

## 3. 部署流程

### 3.1 构建（本机交叉编译 → 128 构建镜像）

```bash
# 本机（Windows，Go 1.26）交叉编译 13 个 Linux 二进制
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" \
  -o deploy/bin/<svc> ./app/<svc>     # dtm/identity/money/draw/base/marketing/
                                      # play/task/catalog/thirdparty/ops/gateway/admin-gateway
```

`-trimpath -s -w` 将二进制从 ~100MB 压到 ~65–73MB（总 854MB → 镜像 ~65MB/个）。

### 3.2 镜像构建与分发

```bash
# 128 上（构建上下文 = 仓库根，含 deploy/bin、deploy/conf、script/split）
docker build -f deploy/common/images/go.Dockerfile \
  --build-arg BINARY=draw-rpc --build-arg CONFIG=conf/draw-rpc.yaml -t gacha/draw-rpc:prod .
docker build -f deploy/common/images/dtm.Dockerfile \
  --build-arg CONFIG=conf/dtm.yml -t gacha/dtm:prod .

# 分发到 worker（无 registry，用 save/load）
docker save $(docker images --format '{{.Repository}}:{{.Tag}}' | grep '^gacha/') -o images.tar
scp images.tar slaver1@129:/tmp/ && ssh slaver1 "docker load -i /tmp/images.tar"
scp images.tar slaver2@131:/tmp/ && ssh slaver2 "docker load -i /tmp/images.tar"
```

### 3.3 Swarm 集群 + 部署

```bash
docker swarm init --advertise-addr 192.168.197.128          # master
docker swarm join --token <worker-token> 192.168.197.128:2377  # slaver1/slaver2

docker stack deploy -c deploy/swarm/infra-stack.yaml gacha-infra   # 基础设施
# 等 MySQL 就绪后跑迁移（script/split/*.sql，建库建表 + 种子）
docker stack deploy -c deploy/swarm/services-stack.yaml gacha      # 业务服务
```

编排文件：`deploy/swarm/infra-stack.yaml`、`deploy/swarm/services-stack.yaml`（本次新增，Swarm 多机多副本专用）。

## 4. 部署中发现的 Bug 及修复

| # | 问题 | 根因 | 修复 |
|---|------|------|------|
| 1 | **镜像启动报 `exit 126` / `Permission denied`，`/app/server` 成了目录** | `go.Dockerfile`/`dtm.Dockerfile` 的 `ARG BINARY`/`ARG CONFIG` 声明在 `FROM` 之前，`FROM` 后未重新声明，导致 `${BINARY}` 为空，`COPY deploy/bin/${BINARY}` 变成 `COPY deploy/bin/`（把全部 13 个二进制拷成目录 `/app/server`） | 在 `FROM alpine` 之后重新声明 `ARG BINARY`、`ARG CONFIG` |
| 2 | **迁移报 `DELIMITER must be followed by a delimiter`** | `script/split/04_gacha_draw.sql` 第 181 行 `DELIMITER `（丢失 `$$`），且存储过程结尾 `END` 缺 `$$` | 改为 `DELIMITER $$` … `END$$` … `DELIMITER ;`（基表已含 `seed_hex`/`audit_hash`，fresh 迁移不受影响） |
| 3 | **MySQL 启动失败 `unknown variable 'default-authentication-plugin'`** | `mysql:8` 已指向 8.4.x，该旗标在 8.4 被移除 | 去掉 `--default-authentication-plugin=mysql_native_password` |

> 注：本次部署按用户要求账号实际为 `slaver1@129`/`slaver2@131`（初始提供的 `slaver` 用户实测密码认证失败，已更正）。

## 5. 测试结果（全通过）

### 5.1 核心链路

| 用例 | 结果 |
|------|------|
| 注册 → 登录 → 查余额 → 个人信息 | ✅ `userId`/JWT/`balance` 正确 |
| 充值 100 → 余额 100.00 | ✅ |
| 充值负数 | ✅ `400 金额必须大于0` |
| 重复注册 / 错误密码 / 非法用户名 | ✅ `1003 用户名已存在` / `1002 密码错误` / `400` |

### 5.2 抽奖主链路（DTM SAGA）

| 用例 | 结果 |
|------|------|
| 报价（`/box/quote`）→ 购买（`/box/buy`）→ 轮询记录 | ✅ 报价 payable 精确、购买提交 SAGA、3 秒后落库（"限定头像框"）、余额 100→99 精确扣款 |
| 余额不足购买（box3=100 元，余额 99） | ✅ DTM `saga failed` 并补偿，余额不变、无抽奖记录 |
| 无效 boxId | ✅ `3001 宝箱不存在` |

### 5.3 玩法（6 种 + 记忆翻牌）

满车购座 / 扭蛋 / 连击链购票 / 拼团下注 / 挖宝 / 记忆翻牌：全部 ✅（提交 SAGA、记录落库、余额精确扣减）。

### 5.4 营销与积分

| 用例 | 结果 |
|------|------|
| 领券 → 我的券 → 用券报价 | ✅ 券抵扣 5 元（gross 10 → payable 5） |
| 签到活动（带 Idempotency-Key） | ✅ streak=1 + 10 积分 |
| 转盘活动预览 | ✅ 槽位/权重完整 |
| 手机验证码（DryRun）+ 错误码登录 | ✅ `9002 频控` / `9001 验证码无效` |

### 5.5 健壮性

| 用例 | 结果 |
|------|------|
| **并发 10 次购买** | ✅ 余额 95→85 精确扣 10 元、11 条记录，无超卖/重复扣款/丢更新 |
| **幂等（Idempotency-Key）** | ✅ 同 key 两次购买返回同 session、只扣一次款 |
| **副本故障切换（kill 1 个 DTM 副本）** | ✅ 事务仍成功（余额 85→75 精确扣 10 元），副本 `on-failure` 自动重启恢复 3/3 |

### 5.6 异常场景测试（分布式事务中途失败 / 服务宕机）

对抽盒 SAGA（7 分支：01 base 资格 → 02 base 风控 → 03 marketing 锁券 → 04 money 冻结 → 05 draw 开奖 → 06 money 发资产 → 07 money 核销）逐一模拟服务宕机：

| 场景 | 操作 | 结果 | 数据一致性 |
|------|------|------|-----------|
| 提交前杀 draw-rpc | `scale gacha_draw-rpc=0` 后购买 | 网关 `500 produced zero addresses`，SAGA 未提交 | ✅ 余额不变、无记录 |
| 中途杀 money-rpc（冻结分支 04） | `scale gacha_money-rpc=0` 后购买 | SAGA 提交成功，分支 04 失败 → DTM 退避重试 10 次 → 转 `aborting` | ✅ 余额未扣、无记录、无脏数据 |
| 首分支杀 base-rpc（01/02） | `scale gacha_base-rpc=0` 后购买 | 分支 01 失败 → `aborting` | ✅ 报价未锁定（无锁残留） |
| DTM 全宕 | `scale gacha_dtm=0` 后购买 | 网关 `630 开奖请求提交失败`（干净报错，不挂起） | ✅ 余额不变；恢复后立即正常 |
| 余额不足（业务失败） | 余额 99 买 100 元 box | 分支 04 返回 Aborted → `saga failed` + 补偿 | ✅ 余额不变、无记录 |

**结论**：所有异常场景下 **资金/记录数据一致性始终正确**（无重复扣款、无部分扣款、无脏状态）；补偿机制（CancelPayment/ReleaseQuote/SettleDrawRevert）在业务失败（Aborted）时即时生效。

## 6. 遗留问题与发现

1. **`GET /tasks` 匿名访问返回 `7003 任务参数无效`**：网关路由标记为「公开（JwtAuthOptional）」，但 task 服务 `ListTasks` 要求 `userId>0`（`userID<=0 → CodeTaskInvalid`，注释标注「老项目语义」）。带 token 正常。属网关「公开」声明与 task 服务「必须登录」的不一致，建议二者对齐（或改公开语义、或改路由为强制鉴权）。
2. **余额不足异步失败**：购买接口返回 `pending`（异步 SAGA），余额不足在事务执行期才失败（`saga failed`），客户端只能靠轮询记录无结果感知。建议在 `quote` 阶段做同步余额预检，提升体验。
3. **微信登录返回 500 原始错误**：`thirdparty-rpc` 配置 `Wechat.Mini.APIBase=http://127.0.0.1:9410`（本地 mock server，未部署），调用时 `connection refused` 以原始 500 透出，而非映射为 `6005 微信接口失败`。生产替换真实微信 API 后应恢复；建议对 mock 不可达做错误码归一。
4. **slaver1 磁盘偏紧（90%，可用 ~990M）**：Ubuntu 24.04 + `swap.img` 2G 占用大；9.8G 盘装 OS + 14 镜像后余量小。如需长期运行建议扩容磁盘或关 swap。
5. **服务间 gRPC 客户端未做 round_robin**：go-zero `zrpc` 客户端默认 `pick_first`，跨服务调用（如 gateway→identity）只连 1 个副本（其余副本仅作故障切换，不参与负载均衡）。DTM 侧已用 round_robin。如需真正负载均衡，可在各 `MustNewClient` 加 `grpc.WithDefaultServiceConfig(round_robin)`。
6. **DTM 分支宕机时补偿被指数退避推迟**（本次异常测试发现）：当 SAGA 中间分支服务（如 money 冻结）宕机（网络错误，非业务 Aborted），DTM 按 `RetryLimit=10`（`saga/builder.go` 默认）+ `RetryInterval` 指数退避重试，退避间隔翻倍至 ~11 小时，导致事务长时间停在 `aborting`、`ReleaseQuote` 补偿迟迟不触发。实际影响被两处缓解：①报价锁 `quote_locks.expires_at` 有 15 分钟 TTL，券/资格锁约 15 分钟自动过期；②资金/记录数据本身一致（冻结未确认不扣款）。建议优化：收窄 `RetryLimit`（如 3）或给退避设上限，使补偿在分钟级内收敛。

## 7. 与旧项目 server_v2 的差异

- **已对齐**：抽奖主链路（开奖确定性 + audit hash 字节级对齐）、满车/扭蛋/连击链/拼团/挖宝/翻牌、资金（钱包/支付/退款）、身份、营销（报价/忠诚度/活动/券）、商品、任务、运营。
- **裁剪/替代**：
  - `delivery`（交付）/ `trade`（交易）/ `engagement`（互动）三个服务未移植；
  - Kafka 领域事件 → MySQL outbox + DTM；
  - `inventory` 合并进 money/play；
  - 服务发现 Consul → Etcd；SAGA 内嵌 → 独立 DTM 协调器。

## 8. 运维手册

```bash
# 查看状态
docker stack services gacha            # 业务服务（应全 3/3）
docker stack services gacha-infra      # 基础设施（1/1）

# 查看日志
docker service logs gacha_money-rpc
docker service logs gacha_dtm

# 扩容/缩容（如 money-rpc 到 5 副本）
docker service scale gacha_money-rpc=5

# 滚动重启某服务
docker service update --force gacha_draw-rpc

# 网关 HTTP 测试入口
curl http://<任意节点>:9310/gacha/api/v1/boxes

# 停止
docker stack rm gacha
docker stack rm gacha-infra
```

**环境变量**（services-stack.yaml 注入）：`MYSQL_HOST=mysql`、`MYSQL_PASS=we7root`、`REDIS_HOST=redis`、`REDIS_PASS=we7redis`、`ETCD_ENDPOINTS=etcd:2379`、`GRPC_SERVICE_CONFIG`（round_robin）。密码通过 entrypoint `envsubst` 运行时替换，不打进镜像。

## 9. 版本更新（发布）流程

### 9.1 发布基础设施

- **本地镜像仓库**：128 上运行 `registry:2`（端口 5000，HTTP，`--restart unless-stopped`），作为三节点共享的镜像源。
- **insecure-registries**：三节点 `/etc/docker/daemon.json` 已配置 `{"insecure-registries":["192.168.197.128:5000"]}`。
- **镜像源**：外网 Docker Hub 被墙，拉基础镜像用 `docker.1ms.run` 镜像源（`docker pull docker.1ms.run/library/alpine:3.20` 再 `docker tag` 回原 tag）。

### 9.2 发布步骤（4 步）

```bash
# ① 开发机（装有 Go 的 Linux/WSL）交叉编译 → deploy/bin/
bash deploy/swarm/build-binaries.sh [service ...]   # 不传=全部，传部分=只编这些

# ② 上传二进制到 128
scp deploy/bin/* master@192.168.197.128:/home/master/gacha/deploy/bin/

# ③ 128 上构建镜像 + 推送到 registry（tag 用不可变版本号，如 v1.0.1）
ssh master@192.168.197.128 'cd /home/master/gacha && bash deploy/swarm/build-push.sh v1.0.1 [service ...]'

# ④ 128 上滚动更新（start-first，新副本就绪才停旧副本，无停机）
ssh master@192.168.197.128 'cd /home/master/gacha && bash deploy/swarm/release.sh v1.0.1 [service ...]'
```

关键点：
- **只更新部分服务**：①②③④ 都支持传服务名（如 `money-rpc draw-rpc`），只重建/更新改动的服务，加快迭代。
- **用 sha256 digest 锁版本**：`build-push.sh` 构建后会记录每个镜像的 digest 到 `deploy/swarm/releases/<version>.digest`，`release.sh` 按 `@sha256:digest` 更新（内容寻址、不可变，比 tag 更严格——tag 可被重新指向，digest 永远唯一）。
- **滚动更新零停机**：Swarm `update_config` 已配 `order: start-first` + `parallelism: 1`，逐副本替换，`3/3` 全程保持。

### 9.3 回滚

```bash
# 回滚单个服务到上一个版本（Swarm 保留上一次配置）
ssh master@192.168.197.128 'cd /home/master/gacha && bash deploy/swarm/release.sh rollback money-rpc'

# 回滚全部服务
ssh master@192.168.197.128 'cd /home/master/gacha && bash deploy/swarm/release.sh rollback'

# 或回滚到指定历史版本（release.sh 会按该版本的 digest 清单锁版本）
ssh master@192.168.197.128 'cd /home/master/gacha && bash deploy/swarm/release.sh v1.0.0 money-rpc'
```

### 9.4 版本更新验证清单

| 检查项 | 命令 |
|--------|------|
| 服务收敛到 3/3 | `docker service ls --format '{{.Name}} {{.Replicas}}'` |
| 更新到目标镜像 | `docker service ls --format '{{.Name}} {{.Image}}'` |
| 功能正常 | `curl http://<节点>:9310/gacha/api/v1/boxes`（或注册/登录/购买） |
| 无启动报错 | `docker service logs gacha_<svc> --since 1m` |
| 清理旧镜像（防磁盘满） | `docker image prune -a -f`（注意：会删掉未在用的历史版本镜像，回滚需重新 build） |

### 9.5 注意事项

1. **schema 变更**：新版本若改表结构，先手动执行新的 `script/split/*.sql`（`docker exec -i <mysql> mysql -uroot -pwe7root < xxx.sql`），再发布。
2. **配置变更**：改 `pkg/etcs/*.yaml` 后，需重新生成 `deploy/conf/*.yaml`（占位符替换）并 build-push（配置打进镜像）。
3. **磁盘**：每次发布新增 ~65MB×N 镜像层，slaver1 磁盘偏紧（90%），发布后记得 `docker image prune` 或扩容。
4. **外网镜像源**：拉取新基础镜像走 `docker.1ms.run`；若该镜像源失效，换其它可用源。

## 10. 从零重来（清理 + 一键部署）

全部脚本均在 128 上（`/home/master/gacha/deploy/swarm/`），本地仓库 `deploy/swarm/` 同步保存。

### 10.1 一键部署（新环境 / 清理后重来）

**前置条件**（一次性环境准备，脚本不覆盖）：

1. 三节点已装 Docker（29.x）且可免密 SSH：`master@128`、`slaver1@129`、`slaver2@131`（密码均为 `1`）；**128 → 129/131 需配好免密**（`ssh-copy-id` 或把 128 公钥加到 129/131 的 `authorized_keys`），否则 `bootstrap.sh` 的 join/配置步骤会失败。
2. 开发机装有 Go（1.25+）。

```bash
# ① 开发机：交叉编译 + 生成配置
bash deploy/swarm/build-binaries.sh

# ② 上传到 128（二进制/配置/Dockerfile/SQL 四类产物）
scp -r deploy/bin deploy/conf deploy/common/images script/split \
    master@192.168.197.128:/home/master/gacha/

# ③ 128 上：一键从零部署（自动完成 swarm 初始化→三节点加入→registry→
#    基础设施→迁移→构建推送→部署服务→收敛检查）
ssh master@192.168.197.128 'cd /home/master/gacha && bash deploy/swarm/bootstrap.sh v1.0.0'
```

`bootstrap.sh` 是幂等的（已初始化/已配置/已迁移的步骤会自动跳过），既可用于全新环境，也可用于中断后重跑。`build-binaries.sh` 会同时生成 `deploy/bin`（二进制）和 `deploy/conf`（配置模板，从 `pkg/etcs` + `app/dtm/conf.yml` sed 生成），无需预先手工生成。

### 10.2 清理

```bash
# 软清理：只卸载 stack，保留镜像/卷/文件（可快速重启）
ssh master@192.168.197.128 'cd /home/master/gacha && bash deploy/swarm/teardown.sh'

# 彻底清理：镜像/卷/swarm/registry 全清（二进制/配置/SQL 文件保留）
ssh master@192.168.197.128 'cd /home/master/gacha && bash deploy/swarm/teardown.sh --full'
```

### 10.3 脚本清单（全部 bash，Linux 侧）

| 脚本 | 位置 | 作用 |
|------|------|------|
| **`env.sh`** | 开发机/128 | **环境配置中心**：集群地址、SSH 用户、密码、镜像源（换服务器只改这一个文件） |
| `build-binaries.sh` | 开发机 | 交叉编译 13 个二进制 + 生成配置模板（从 pkg/etcs sed） |
| `bootstrap.sh` | 128 | 从零一键部署（7 步） |
| `build-push.sh` | 128 | 构建镜像 + 推 registry + 记录 sha256 digest（`releases/<version>.digest`） |
| `release.sh` | 128 | 滚动更新 / 回滚 / 收敛检查 |
| `teardown.sh` | 128 | 清理（软清理 / --full 彻底） |
| `infra-stack.yaml` / `services-stack.yaml` | 128 | Swarm 编排（infra / 业务服务，3 副本） |

### 10.4 换环境只需改 env.sh

所有脚本和编排 yaml 里**不再硬编码地址/密码**，统一从 `deploy/swarm/env.sh` 读取。换 3 台服务器只需改 `env.sh` 开头几个变量：

```bash
# 集群拓扑
MASTER_IP / MASTER_USER / MASTER_NODE     # 主节点
WORKER1_IP / WORKER1_USER / WORKER1_NODE  # 从机 1
WORKER2_IP / WORKER2_USER / WORKER2_NODE  # 从机 2
# 密码
MYSQL_PASSWORD / REDIS_PASSWORD / SUDO_PASSWORD
# 镜像源（Docker Hub 被墙时）
MIRROR
```

yaml 里的 `${MYSQL_PASSWORD}`、`${REDIS_PASSWORD}`、`${REGISTRY}`、`${MASTER_NODE}` 由 `docker stack deploy` 从 shell 环境自动插值（bootstrap 已 export）。

### 10.5 部署说明（为什么这样设计）

- **镜像全走本地 registry**（`${REGISTRY}/gacha/<svc>`，默认 `192.168.197.128:5000`），三节点自动拉取，不依赖手工 `save/load`；`services-stack.yaml` 已改为 registry 地址。
- **sha256 digest 锁版本**（对齐 server_v2）：`build-push.sh` 推送 `<version>` + `prod` 两个 tag，并记录每个镜像的 digest 到 `releases/<version>.digest`；`release.sh` 用 `@sha256:digest` 更新，回滚时 Swarm 恢复上一个 digest。`prod` 别名仅作 `stack deploy` 的初始指针。
- **幂等 + 自愈**：bootstrap 各步可重复执行；服务 `on-failure` 自动重启；DTM 多副本靠 MySQL 共享 + 存储锁协调。
- **注意**：`daemon.json` 只配 `insecure-registries`，**不要加 `live-restore`**（与 Swarm 模式不兼容，会导致 dockerd 启动失败）。
