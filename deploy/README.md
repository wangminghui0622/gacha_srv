# deploy 目录说明

部署采用 **Docker Swarm 三副本** 方案，全部脚本在 `deploy/swarm/`。完整部署报告见仓库根目录 `deploy.md`。

## 目录结构

```
deploy/
├── swarm/                 # 部署脚本 + 编排（唯一部署入口）
│   ├── env.sh             # ★ 环境配置中心：地址/用户/密码/镜像源（换环境只改这里）
│   ├── bootstrap.sh       # 一键从零部署（swarm→registry→基础设施→迁移→构建→部署）
│   ├── build-binaries.sh  # 开发机：交叉编译二进制 + 生成配置模板
│   ├── build-push.sh      # 构建镜像 + 推 registry + 记录 sha256 digest
│   ├── release.sh         # 滚动更新 / 回滚
│   ├── teardown.sh        # 清理（--full 彻底清空）
│   ├── infra-stack.yaml   # 基础设施编排（MySQL/Redis/Etcd）
│   └── services-stack.yaml# 业务服务编排（13 服务 × 3 副本）
├── common/
│   └── images/            # Dockerfile（go.Dockerfile / dtm.Dockerfile）+ entrypoint
├── bin/                   # 编译产物（build-binaries.sh 生成，gitignore）
└── conf/                  # 配置模板（build-binaries.sh 生成，占位符运行时替换）
```

## 快速开始（3 步）

```bash
# ① 开发机：编译 + 生成配置
bash deploy/swarm/build-binaries.sh

# ② 上传到主节点
scp -r deploy/bin deploy/conf deploy/common/images deploy/swarm script/split \
    master@<主节点IP>:/home/master/gacha/

# ③ 主节点：一键部署
ssh master@<主节点IP> 'cd /home/master/gacha && bash deploy/swarm/bootstrap.sh v1.0.0'
```

## 换服务器

只需改 `deploy/swarm/env.sh` 一个文件（每台机器的 IP/用户/sudo 密码各管各的）。

详细说明、版本更新、回滚、运维手册见 `../deploy.md`。
