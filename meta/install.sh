#!/usr/bin/env bash
# 工程标准包安装脚本（Git Bash / Linux / macOS 通用）
# 用法：bash install.sh -t <目标项目路径> [-f] [-s] [-n <项目名>] [-g]
#   -t  目标项目目录（必须；不存在会自动创建；支持 F:/... 与 "F:\..." 两种写法，反斜杠写法必须加引号）
#   -n  项目名：把规范中的 <项目名>/<项目> 占位符自动替换为该值；省略时自动取目标目录最后一段（如 F:/.../shop -> shop）
#   -g  若目标目录还不是 git 仓库则执行 git init
#   -f  覆盖已存在的文件（默认跳过）
#   -s  跳过规范文档 specs
set -euo pipefail

KIT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET=""
FORCE=0
SKIP_SPECS=0
NAME=""
GIT_INIT=0

GREEN='\033[32m'; YELLOW='\033[33m'; CYAN='\033[36m'; DIM='\033[2m'; NC='\033[0m'

usage() {
    echo "用法：bash install.sh -t <目标项目路径> [-f] [-s] [-n <项目名>] [-g]"
    echo "      -t 目标项目目录（不存在会自动创建）"
    echo "      -n 项目名（省略时自动取目标目录最后一段；一般不用传）"
    echo "      -g 目标目录还不是 git 仓库时执行 git init"
    echo "示例：bash install.sh -t \"F:/workspace/src/shop\" -g   （项目名自动取 shop）"
    echo "      bash install.sh -t \"F:\\workspace\\src\\shop\" -g   （反斜杠路径必须加引号）"
}

while getopts "t:fsn:g" opt; do
    case "$opt" in
        t) TARGET="$OPTARG" ;;
        f) FORCE=1 ;;
        s) SKIP_SPECS=1 ;;
        n) NAME="$OPTARG" ;;
        g) GIT_INIT=1 ;;
        *) usage; exit 1 ;;
    esac
done

if [ -n "$NAME" ] && [[ "$NAME" =~ [^A-Za-z0-9_-] ]]; then
    echo "警告：项目名 $NAME 含特殊字符，占位符替换可能异常；建议只用字母/数字/-/_"
fi

if [ -z "$TARGET" ]; then
    echo "错误：必须用 -t 指定目标项目目录"
    usage
    exit 1
fi

# Windows 反斜杠路径转正斜杠（Git Bash 下两种写法都接受）；去掉末尾斜杠
TARGET="${TARGET//\\//}"
TARGET="${TARGET%/}"

# 防呆：盘符后无斜杠，疑似反斜杠被 shell 转义吞掉（如 F:workspacesrcxxx）
if [[ "$TARGET" =~ ^[A-Za-z]:[^/] ]]; then
    echo "错误：路径疑似被 shell 转义吞掉了反斜杠：$TARGET"
    echo "提示：反斜杠路径必须加引号：-t \"F:\\workspace\\src\\xxx\"，或用正斜杠：-t F:/workspace/src/xxx"
    exit 1
fi

if [ ! -d "$TARGET" ]; then
    mkdir -p "$TARGET"
    echo -e "${GREEN}√ 创建目标目录：${TARGET}${NC}"
fi

# -n 未指定时，自动取目标目录最后一段作为项目名（-t 与 -n 本就是同一信息，不必传两遍）
if [ -z "$NAME" ]; then
    AUTO_NAME="$(basename "$TARGET")"
    if [[ "$AUTO_NAME" =~ ^[A-Za-z0-9_-]+$ ]]; then
        NAME="$AUTO_NAME"
        echo "- 未指定 -n，项目名自动取目录名：$NAME"
    else
        echo -e "${YELLOW}提示：目录名「$AUTO_NAME」含特殊字符，不自动替换占位符；如需替换请显式加 -n <项目名>${NC}"
    fi
fi

# 可选：初始化 git 仓库
if [ "$GIT_INIT" -eq 1 ]; then
    if git -C "$TARGET" rev-parse --git-dir >/dev/null 2>&1; then
        echo "- 已是 git 仓库，跳过 init"
    else
        git init "$TARGET" >/dev/null 2>&1 && echo -e "${GREEN}√ git init：${TARGET}${NC}"
    fi
fi

# 用 -n 指定的项目名替换复制产物中的占位符（只处理本包复制过去的文件）
apply_name() {
    if [ -n "$NAME" ]; then
        if [ -d "$1" ]; then
            find "$1" -type f \( -name '*.md' -o -name '*.json' \) -exec sed -i "s/<项目名>/$NAME/g; s/<项目>/$NAME/g" {} +
        else
            sed -i "s/<项目名>/$NAME/g; s/<项目>/$NAME/g" "$1"
        fi
    fi
}

copy_safe() {
    local from="$1" to="$2" what="$3"
    mkdir -p "$(dirname "$to")"
    if [ -e "$to" ]; then
        if [ "$FORCE" -eq 1 ]; then
            rm -rf "$to"
            cp -r "$from" "$to"
            apply_name "$to"
            echo -e "${YELLOW}√ 覆盖：${what} -> ${to}${NC}"
        else
            echo -e "${DIM}× 跳过（已存在，加 -f 覆盖）：${to}${NC}"
        fi
    else
        cp -r "$from" "$to"
        apply_name "$to"
        echo -e "${GREEN}√ 复制：${what} -> ${to}${NC}"
    fi
}

echo ""
echo -e "${CYAN}===== 安装工程标准包 =====${NC}"
echo "目标项目：$TARGET"
echo "标准包：$KIT_ROOT"
echo ""

# 1. 根 CLAUDE.md
copy_safe "$KIT_ROOT/CLAUDE.md" "$TARGET/CLAUDE.md" "CLAUDE.md 总纲"

# 2. .claude/rules/ 路径作用域规则
RULES_DEST="$TARGET/.claude/rules"
mkdir -p "$RULES_DEST"
copy_safe "$KIT_ROOT/rules/go.md"  "$RULES_DEST/go.md"  "规则 go.md"
copy_safe "$KIT_ROOT/rules/web.md" "$RULES_DEST/web.md" "规则 web.md"

# 3. .claude/skills/ 技能
SKILLS_DEST="$TARGET/.claude/skills"
mkdir -p "$SKILLS_DEST"
for skill_dir in "$KIT_ROOT"/skills/*/; do
    [ -d "$skill_dir" ] || continue
    name="$(basename "$skill_dir")"
    copy_safe "$skill_dir" "$SKILLS_DEST/$name" "技能 $name"
done

# 4. .claude/settings.json（只在不存时创建，不覆盖用户已有权限配置）
SETTINGS_DEST="$TARGET/.claude/settings.json"
if [ -e "$SETTINGS_DEST" ]; then
    echo -e "${DIM}× 跳过：${SETTINGS_DEST} 已存在，权限配置请手工合并${NC}"
else
    cp "$KIT_ROOT/settings.example.json" "$SETTINGS_DEST"
    echo -e "${GREEN}√ 复制：settings 模板 -> ${SETTINGS_DEST}（请按项目调整权限）${NC}"
fi

# 5. 项目 .gitignore（来自 templates/）
copy_safe "$KIT_ROOT/templates/.gitignore" "$TARGET/.gitignore" "项目 .gitignore"

# 6. 项目 README 骨架（仅当目标还没有 README.md 时创建，绝不覆盖已有内容）
if [ -e "$TARGET/README.md" ]; then
    if grep -q "<项目名>" "$TARGET/README.md" 2>/dev/null; then
        # 已存在但含占位符（说明是模板装出来的），只替换占位符不覆盖内容
        apply_name "$TARGET/README.md"
        echo -e "${YELLOW}√ README.md 已存在，仅替换占位符${NC}"
    else
        echo -e "${DIM}× 跳过：${TARGET}/README.md 已存在${NC}"
    fi
else
    cp "$KIT_ROOT/templates/project-README.md" "$TARGET/README.md"
    apply_name "$TARGET/README.md"
    echo -e "${GREEN}√ 复制：README 骨架 -> ${TARGET}/README.md（请按项目补充）${NC}"
fi

# 7. specs -> docs/standards/
if [ "$SKIP_SPECS" -eq 1 ]; then
    echo "- 跳过规范文档（-s）"
else
    copy_safe "$KIT_ROOT/specs" "$TARGET/docs/standards" "规范文档 specs"
fi

# 8. 写入安装版本记录（日后判断该项目装的是哪版标准、是否需要升级）
mkdir -p "$TARGET/docs/standards"
KIT_VERSION=""
if git -C "$KIT_ROOT" rev-parse --short HEAD >/dev/null 2>&1; then
    KIT_VERSION="$(git -C "$KIT_ROOT" rev-parse --short HEAD)"
    git -C "$KIT_ROOT" diff --quiet 2>/dev/null || KIT_DIRTY="（另有未提交改动）"
fi
{
    echo "标准包安装记录"
    echo "安装时间：$(date '+%Y-%m-%d %H:%M:%S')"
    echo "标准包版本：${KIT_VERSION:-未知（标准包目录不是 git 仓库）}${KIT_DIRTY:-}"
    [ -n "$NAME" ] && echo "项目名：$NAME"
} > "$TARGET/docs/standards/VERSION"
echo -e "${GREEN}√ 写入版本记录 -> ${TARGET}/docs/standards/VERSION${NC}"

echo ""
echo -e "${CYAN}===== 安装完成 =====${NC}"
echo "后续检查："
echo "1. CLAUDE.md 中「项目概览」按实际项目填写（技术栈、领域）"
if [ -n "$NAME" ]; then
    echo "2. 占位符已自动替换为 $NAME（如需换名，重新执行并带 -n 新名）"
else
    echo "2. 占位符未自动替换，请手工替换（grep '<项目名>' 检查）"
fi
echo "3. .claude/settings.json 按项目合并权限"
echo "4. 补充 README.md 与 CLAUDE.md 的项目信息后，提交首次 commit"
