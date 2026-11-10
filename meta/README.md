# meta/ — 项目元文件总目录

一个目录装下所有「让 AI 认识这个项目」的东西（标准包源 + 安装器 + 规范模板），
拷贝项目时**整个目录一起拷**即可，不散落在各处。

## 目录清单

| 路径 | 作用 | 备注 |
|------|------|------|
| `CLAUDE.md` | 工程规范总纲（标准包副本） | 与项目根 `CLAUDE.md` 一致，两份必须同步修改 |
| `install.sh` | 标准包安装器 | `bash meta/install.sh -t <目标项目>` |
| `rules/` | 路径作用域规则源（go/web） | 安装到目标项目 `.claude/rules/` |
| `skills/` | 技能源（go-zero-service / standards-check / vue-page） | 安装到目标项目 `.claude/skills/` |
| `specs/` | 规范文档源 | 安装到目标项目 `docs/standards/` |
| `templates/` | 项目模板（.gitignore / README 骨架） | 安装到目标项目根 |
| `settings.example.json` | Claude Code 权限模板 | 安装为目标项目 `.claude/settings.json`（已存在则跳过） |

## 换新机器 / 拷贝到另一台计算机

AI（Claude Code）识别项目靠 4 个锚点，全部**已入库**、随整目录拷贝走：

1. 项目根 `CLAUDE.md` —— 打开项目自动加载
2. `.claude/rules/*.md` —— 按文件路径自动加载的语言规则
3. `.claude/skills/*/SKILL.md` —— 自动发现的技能
4. `.claude/settings.json` —— 项目权限

**整目录拷贝（Explorer / robocopy / zip）**：什么都不用做，识别即生效。

**git 传输（clone）**：`install.sh` 与 `meta/` 内源文件按团队约定不入库；clone 后如需恢复标准包：
```bash
bash meta/install.sh -t . -f -n Gacha
```
> 注意：`.claude/settings.json` 已存在时 install.sh 不覆盖（权限配置手工合并）。

## 给其他项目装标准包

```bash
bash meta/install.sh -t "F:/workspace/src/新项目" -g
```
`-g` 顺带 git init；`-s` 跳过 specs；`-f` 强制覆盖；`-n <项目名>` 替换 `<项目名>` 占位符。

## 维护约定

- 改规范：改 `meta/CLAUDE.md`（或根 CLAUDE.md）**必须同步另一份**；`rules/ skills/ specs/` 改完执行 `bash meta/install.sh -t . -f` 刷入 `.claude/` 与 `docs/standards/`。
- `docs/` 留在项目根：它是 specs 的安装产物 + 被 CLAUDE.md/rules/skills 按路径引用，移动会断引用。
