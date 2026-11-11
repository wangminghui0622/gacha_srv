#!/bin/sh
# 通用 entrypoint：运行时用 envsubst 把配置里的 ${VAR} 替换为环境变量值，再启动服务。
# 地址/密码等环境相关值通过 compose 的 environment 注入，不打进镜像。
# 容器环境变量天然可被 envsubst 读取，无需额外 export。
set -e

CONF="${APP_CONFIG:-/app/conf/config.yaml}"

if [ -f "${CONF}" ]; then
  tmp="$(mktemp)"
  envsubst < "${CONF}" > "${tmp}"
  mv "${tmp}" "${CONF}"
fi

# 启动标志：go-zero 服务用 -f，DTM 用 -c（由 APP_FLAG 传入，默认 -f）
exec /app/server "${APP_FLAG:--f}" "${CONF}"
