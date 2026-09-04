# GWM

GWM（Git Worktree Manager）是原生 `git worktree` 的本地薄包装器，提供简短的命令入口、每个工作树的 metadata，以及 add/remove 前后的生命周期 Hook。

- 使用 `init`、`list`、`add`、`meta`、`remove` 管理常用工作树操作。
- 为工作树添加描述、删除保护和只读创建时间。
- 通过 repository-local 配置启用 `pre-add`、`post-add`、`pre-remove`、`post-remove` Hook。
- 保持 Git 为工作树路径、HEAD、branch、locked 和存在性的唯一事实源。

GWM 不提供工作树历史、事务、自动回滚、后台服务或远端集成。

## 安装与构建

GWM 当前为 Unreleased，尚无版本标签或预编译制品，请从源码构建。

| 项目 | 要求 |
|---|---|
| 平台 | Linux、macOS 13+ |
| 运行依赖 | Git 2.39+ |
| 构建依赖 | Go 1.26+，仅使用标准库 |
| 源码仓库 | [github.com/gongshuiwen/gwm](https://github.com/gongshuiwen/gwm) |

在源码根目录运行：

```bash
go build -o bin/gwm ./cmd/gwm
```

构建后可以直接运行 `bin/gwm`，或将它复制到本机 `PATH` 中的目录。普通源码构建的 `gwm --version` 输出为 `gwm unreleased`，正式发布二进制显示对应 release tag。

## 快速开始

在普通 non-bare Git repository 的 main 或 linked worktree 中运行：

```bash
gwm init
gwm add ../repo-fix -b fix/login --from main --description "修复登录"
gwm meta ../repo-fix --protected true
gwm list
gwm meta ../repo-fix --protected false
gwm remove ../repo-fix
```

`gwm init` 幂等启用 `extensions.worktreeConfig=true`，不创建目录、marker、metadata 或 Hook 文件。可以使用 `gwm -C <repository> <command>` 指定 repository；相对工作树路径以 invocation worktree root 为基准。

## 命令

| 命令 | 作用 |
|---|---|
| `init` | 启用 worktree config extension |
| `list` | 按 Git 原生顺序显示 worktree、branch 和 metadata |
| `add` | 创建工作树，成功后写入 metadata |
| `meta` | 查看或更新已登记 worktree 的 metadata |
| `remove` | 删除 linked worktree，不删除 branch |

```bash
gwm --help
gwm add --help
gwm --version
```

每个子命令都支持 `--help`。`--help` 和 `--version` 不要求当前目录位于 Git repository，也不运行 Git 或 Hook。完整参数与行为见 [命令参考](docs/SPEC.md#2-根命令)。

## Metadata

Metadata 保存在各工作树的 Git worktree config 中：

| 配置键 | 用途 |
|---|---|
| `gwm.worktree.description` | 工作树描述；缺失或为空表示 null |
| `gwm.worktree.protected` | 删除保护；缺失表示 false，为 true 时拒绝 `gwm remove` |
| `gwm.worktree.created-at` | `gwm add` 成功确认工作树已登记时写入的 UTC RFC 3339 时间；缺失表示未知 |

`created-at` 只读，不回填或推断原生 Git 创建的工作树的时间。字段校验和更新规则见 [Metadata 参考](docs/SPEC.md#5-metadata)。

## 生命周期 Hook

用户审查 Hook executable 后，在当前 clone 的 local Git config 中显式启用。例如本项目的 [示例脚本](.githooks/gwm/lifecycle-notify)：

```bash
git config --local gwm.hooks.pre-add .githooks/gwm/lifecycle-notify
git config --local gwm.hooks.post-add .githooks/gwm/lifecycle-notify
git config --local gwm.hooks.pre-remove .githooks/gwm/lifecycle-notify
git config --local gwm.hooks.post-remove .githooks/gwm/lifecycle-notify
```

相对路径以 main worktree 根目录解析，也可以使用绝对路径。目标必须是普通可执行文件；GWM 直接执行它，不经过 shell，不附加参数，事件信息通过 JSON stdin 传入。

Tracked 文件不会自动启用，clone 也不继承源仓库的 local Hook 配置。原生 `git worktree` 操作不会触发 GWM Hook，也不会自动写入 GWM metadata。完整配置与 payload 见 [Hook 参考](docs/SPEC.md#10-hook-配置)。

## 失败处理

退出码 `0` 表示成功，`1` 表示尚未确认 Git 成功时发生错误，`2` 表示 Git add/remove 已成功，但 metadata 或 post-hook 失败。

退出 `2` 时，工作树操作已经完成，应检查当前状态及失败信息再决定后续操作。GWM 不会回滚已完成的 Git 操作。完整说明见 [输出与退出码](docs/SPEC.md#11-输出与退出码)。

## 文档与贡献

- [文档索引](docs/README.md)：按使用、行为查询和维护需求查找文档。
- [行为规范](docs/SPEC.md)：命令、metadata、Hook、输出与退出码。
- [Git worktree 背景与兼容性](docs/GIT_WORKTREE.md)：底层模型与使用限制。
- [贡献指南](CONTRIBUTING.md)：开发、验证和文档维护。

## 许可证

本项目采用 [MIT License](LICENSE)，Copyright (c) 2026 gongshuiwen。
