#!/usr/bin/env bash
# 代码生成脚本：参数 <all|identity|money|draw|base|marketing|play|task|catalog|thirdparty|ops|gateway|admin-gateway>
# 用法：在项目根目录执行 bash script/goctl.sh <all|identity|money|...>
# 说明：
#   - identity/money/draw/base/marketing/play/task/catalog/thirdparty/ops：
#     goctl rpc protoc 从 pkg/proto/<svc>.proto 生成到 app/<svc>（--style goZero）
#   - 10 个 proto 统一在 pkg/proto/（旧仓 proto_admin 已并入；ops 生成到 app/ops，管理后台服务）
#   - gateway/admin-gateway：goctl api go 从 pkg/api/gateway.api、pkg/api/admin-gateway.api 生成
#     ⚠️ 已加硬闸：两个网关路由手写维护（routes.go 是事实源，.api 仅登记部分路由），默认拒绝重生成
#   - 生成后自动清理 app/<svc>/etc，配置统一放 pkg/etcs/（启动参数 -f ../../pkg/etcs/<svc>-rpc.yaml）
#   - 模型代码生成见 script/model/*.sh（缓存/无缓存两套）
set -eu

PKG_DIR="pkg"
API_FILE="${PKG_DIR}/api/gateway.api"
API_ADMIN_FILE="${PKG_DIR}/api_admin/gateway.api"
RPC_SERVICES="identity money draw base marketing play task catalog thirdparty ops"

gen_gateway() {
    # 硬闸：玩家网关路由手写维护，routes.go 是事实源（03-api-proto.md §现状与事实源）。
    # .api 仅登记 M7 面 10 条，重生成会覆盖 46 个手写路径（M8-M15d 全部玩法端点）。
    if [ "${FORCE_GATEWAY_REGENERATE:-0}" != "1" ]; then
        echo "✗ 拒绝执行：玩家网关路由手写维护，禁跑 goctl gateway（会覆盖 routes.go 手写路由）"
        echo "  确需重生成：先备份 app/gateway/internal/handler/，再 FORCE_GATEWAY_REGENERATE=1 bash script/goctl.sh gateway"
        exit 1
    fi
    echo "=== 生成网关代码 ==="
    goctl api go -api "${API_FILE}" -dir app/gateway --style goZero
    # 清理 goctl 生成的默认 etc，统一走 pkg/etcs/（见 08-deploy-config 规范）
    rm -rf "app/gateway/etc"
    echo "√ 已清理 app/gateway/etc（配置统一在 pkg/etcs/gateway-api.yaml）"
}

gen_admin_gateway() {
    # 硬闸：task 计划/reporting 控制台 14 条路由手写在 routes.go 无 .api 登记（03-api-proto.md §现状与事实源），
    # 重生成会覆盖丢失（fail-closed 校验能拦住启动，但文件已毁）。
    if [ "${FORCE_GATEWAY_REGENERATE:-0}" != "1" ]; then
        echo "✗ 拒绝执行：管理网关 14 条路由（task 计划/reporting）手写无 .api 登记，禁跑 goctl admin-gateway"
        echo "  确需重生成：先备份 app/admin-gateway/internal/handler/，再 FORCE_GATEWAY_REGENERATE=1 bash script/goctl.sh admin-gateway"
        exit 1
    fi
    if [ ! -f "${API_ADMIN_FILE}" ]; then
        echo "错误：找不到 ${API_ADMIN_FILE}"
        exit 1
    fi
    echo "=== 生成管理网关代码 ==="
    goctl api go -api "${API_ADMIN_FILE}" -dir app/admin-gateway --style goZero
    # 清理 goctl 生成的默认 etc，统一走 pkg/etcs/（见 08-deploy-config 规范）
    rm -rf "app/admin-gateway/etc"
    echo "√ 已清理 app/admin-gateway/etc（配置统一在 pkg/etcs/admin-gateway-api.yaml）"
}

gen_rpc() {
    local svc="$1"
    local proto="${PKG_DIR}/proto/${svc}.proto"
    if [ ! -f "${proto}" ]; then
        echo "错误：找不到 ${proto}"
        exit 1
    fi
    echo "=== 生成 ${svc} RPC 代码 ==="
    # --zrpc_out 指定目录，protobuf 产物（.pb.go）与 zrpc 产物同目录
    # --proto_path 覆盖 proto 所在目录与内置 well-known types 目录（本机无 protoc include 目录；
    # thirdparty/ops.proto 引入 google/protobuf/timestamp.proto）
    goctl rpc protoc "${proto}" --proto_path "${PKG_DIR}/proto" --proto_path "${PKG_DIR}/proto/include" --go_out="app/${svc}" --go-grpc_out="app/${svc}" --zrpc_out="app/${svc}" --style goZero
    # 清理 goctl 生成的默认 etc，统一走 pkg/etcs/（见 08-deploy-config 规范）
    rm -rf "app/${svc}/etc"
    echo "√ 已清理 app/${svc}/etc（配置统一在 pkg/etcs/${svc}-rpc.yaml）"
}

case "${1:-}" in
    all)
        # gateway/admin-gateway 不重生成（路由手写维护，见 03-api-proto.md §现状与事实源），只生成 RPC 代码
        echo "注意：跳过 gateway / admin-gateway（路由手写维护，禁重生成）"
        for svc in ${RPC_SERVICES}; do
            gen_rpc "${svc}"
        done
        ;;
    gateway) gen_gateway ;;
    admin-gateway) gen_admin_gateway ;;
    identity|money|draw|base|marketing|play|task|catalog|thirdparty|ops) gen_rpc "$1" ;;
    *)
        echo "用法：bash script/goctl.sh <all|identity|money|draw|base|marketing|play|task|catalog|thirdparty|ops|gateway|admin-gateway>"
        exit 1
        ;;
esac

echo "√ 代码生成完成；生成文件禁止手改（handler/logic 除外）。RPC 接口变更改 pkg/proto/*.proto 后重新生成；网关路由变更直接手写 routes.go（不写 .api）"
