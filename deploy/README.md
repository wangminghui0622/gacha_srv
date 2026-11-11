# deploy 目录说明

部署采用 **Docker Swarm 三副本** 方案，分 4 个环境目录。完整部署报告见仓库根目录 `deploy.md`。

## 环境划分

| 环境 | 目录 | 基础设施 | 说明 |
|------|------|---------|------|
| dev | `dev/` | **内置**（bootstrap 自动部署 MySQL/Redis/Etcd + 迁移） | 完整参考实现 |
| test | `test/` | **外部**（账户密码连接外部 MySQL/Redis/Etcd） | |
| staging | `staging/` | **外部** | |
| product | `product/` | **外部** | |

## 目录结构

```
deploy/
├── dev/                        # 完整参考（内置基础设施）
│   ├── swarm/                  # ★ 部署脚本 + 编排（四个环境共用）
│   │   ├── env.sh              #   环境配置中心（dev：内置基础设施）
│   │   ├── infra-stack.yaml    #   基础设施编排（仅 dev 用）
│   │   ├── services-stack.yaml #   业务服务编排（13 服务 × 3 副本，四环境共用）
│   │   ├── bootstrap.sh        #   一键部署（ENV_DIR 感知）
│   │   ├── build-binaries.sh   #   交叉编译 + 生成配置
│   │   ├── build-push.sh       #   构建镜像 + 推 registry + 记录 digest
│   │   ├── release.sh          #   滚动更新 / 回滚
│   │   └── teardown.sh         #   清理
│   ├── common/images/          # Dockerfile + entrypoint
│   ├── bin/                    # 编译产物（生成，gitignore）
│   ├── conf/                   # 配置模板（生成，占位符运行时替换）
│   └── README.md
├── test/                       # 外部基础设施
│   ├── env.sh                  #   环境配置（外部 MySQL/Redis/Etcd + 账户密码）
│   └── deploy.sh               #   部署入口（调用 dev/swarm/bootstrap.sh）
├── staging/
│   ├── env.sh
│   └── deploy.sh
└── product/
    ├── env.sh
    └── deploy.sh
```

## 快速开始

### dev（内置基础设施）

```bash
# ① 开发机：编译 + 生成配置
bash deploy/dev/swarm/build-binaries.sh

# ② 上传到主节点
scp -r deploy/dev/bin deploy/dev/conf deploy/dev/common/images deploy/dev/swarm script/split \
    master@<主节点IP>:/home/master/gacha/

# ③ 主节点：一键部署（含基础设施 + 迁移）
ssh master@<主节点IP> 'cd /home/master/gacha && bash deploy/dev/swarm/bootstrap.sh v1.0.0'
```

### test / staging / product（外部基础设施）

```bash
# ① 先填好 deploy/<env>/env.sh 的外部地址 + 账户密码（INFRA_BUILTIN=0）

# ② 主节点：一键部署（跳过基础设施，外部 MySQL 需先自行执行 script/split/*.sql）
ssh master@<主节点IP> 'cd /home/master/gacha && bash deploy/<env>/deploy.sh v1.0.0'
```

## 换服务器 / 改密码

- 每台机器的 IP/用户/sudo 密码、外部 MySQL/Redis/Etcd 的账户密码，都在各自的 `env.sh` 里，各环境独立。
- 外部基础设施需在部署前手动执行一次迁移：`mysql -h<host> -P<port> -u<user> -p < script/split/*.sql`。

详细说明、版本更新、回滚、运维手册见 `../deploy.md`。
