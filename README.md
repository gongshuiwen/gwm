# GWM

GWM（Git Worktree Manager）是原生 `git worktree` 的本地薄包装器。它保留 Git 的工作树模型，只增加每个工作树的少量 metadata，以及围绕 add/remove 的 repository-local 生命周期 Hook。

| 项目 | 状态 |
|---|---|
| 当前版本 | v0.2（本地实现完成，尚未发布） |
| 运行平台 | Linux、macOS 13+ |
| 最低依赖 | Git 2.39、Go 1.26（仅构建需要 Go） |
| 运行时依赖 | Go 标准库、系统 Git |
| 许可证 | MIT |
| 源码仓库 | [github.com/gongshuiwen/gwm](https://github.com/gongshuiwen/gwm) |

> 当前尚无 CI、版本标签或发布制品。发布前事项见 [PLAN.md](PLAN.md)。

## 功能

- 用五个命令覆盖常用 worktree 初始化、查看、创建、metadata 编辑和删除。
- 通过 worktree config 保存 `description`、`protected` 和只读的 `created-at`。
- 在 GWM 发起 add/remove 时执行 `pre-add`、`post-add`、`pre-remove`、`post-remove`。
- 保持 Git 为 path、HEAD、branch、locked 和工作树存在性的唯一事实源。

GWM 不替代 Git，也不提供工作树历史、事务、自动回滚、后台服务或远端集成。完整边界见 [DESIGN.md](DESIGN.md)。

## 构建

```bash
go build -o bin/gwm ./cmd/gwm
```

构建完成后可以直接运行 `bin/gwm`，或将它复制到本机 `PATH` 中的目录。Canonical Go module path 是 `github.com/gongshuiwen/gwm`；项目尚未发布版本标签或预编译制品。

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

`gwm init` 只启用 `extensions.worktreeConfig=true`，不会创建目录、marker、metadata 或 Hook 文件。

## 命令

```text
gwm --help
gwm <command> --help
gwm --version
gwm [-C <repository>] init
gwm [-C <repository>] list
gwm [-C <repository>] add <path> [-b <new-branch> | --detach]
    [--from <commit-ish>] [--description <text>] [--protected]
gwm [-C <repository>] meta <path>
    [--description <text>] [--protected <true|false>]
gwm [-C <repository>] remove <path> [--force]
```

`--help` 和 `--version` 不要求当前目录位于 Git repository；它们分别输出帮助或 `gwm v0.2`，成功时返回 0。

| 命令 | 作用 |
|---|---|
| `init` | 幂等启用 worktree config extension |
| `list` | 按 Git 原生顺序显示 worktree、branch 和 metadata |
| `add` | 调用一次 `git worktree add`，成功后写 metadata |
| `meta` | 查看或更新已登记 worktree 的 metadata |
| `remove` | 调用一次 `git worktree remove`，不删除 branch |

精确参数、输出和失败语义以 [SPEC.md](SPEC.md) 为准。

## Metadata

GWM 只拥有目标工作树中的三个 worktree-scope 配置键：

```text
gwm.worktree.description=修复登录
gwm.worktree.protected=false
gwm.worktree.created-at=2026-09-03T08:30:00Z
```

- `gwm.worktree.description`：缺失或为空表示 null，非空值最多 4096 UTF-8 bytes。
- `gwm.worktree.protected`：缺失表示 false；为 true 时，`gwm remove` 拒绝删除该工作树。
- `gwm.worktree.created-at`：`gwm add` 成功确认工作树已登记时写入的 UTC RFC 3339 时间；缺失表示未知，GWM 不回填或编辑。
- 三个键独立写入，不提供多键事务或回滚；非法 `created-at` 只影响时间展示，不阻止 metadata 编辑或删除。

## 生命周期 Hook

仓库可以提交 Hook executable，例如本项目提供的：

```text
.githooks/gwm/lifecycle-notify
```

Tracked 文件不会自动启用。用户审查脚本后，需要在该 clone 的 local Git config 中显式启用：

```bash
git config --local gwm.hooks.pre-add .githooks/gwm/lifecycle-notify
git config --local gwm.hooks.post-add .githooks/gwm/lifecycle-notify
git config --local gwm.hooks.pre-remove .githooks/gwm/lifecycle-notify
git config --local gwm.hooks.post-remove .githooks/gwm/lifecycle-notify
```

相对 Hook path 始终以 main worktree 根目录解析，即使命令从 linked worktree 调用也是如此；也可以配置仓库外的绝对路径。解析结果必须是普通可执行文件。GWM 直接执行它，不经过 shell，不附加参数，也不从 tracked 文件或 remote 自动发现 Hook。事件信息通过 JSON stdin 传入；完整 payload 见 [SPEC.md 的 Hook 配置](SPEC.md#10-hook-配置)。

Clone 只取得 `.githooks/` 中的文件，不会取得源仓库 `.git/config` 中的启用状态，因此默认不执行。

原生 `git worktree` 命令不会触发 GWM Hook，也不会自动写入 GWM metadata。

## 退出码

| 退出码 | 含义 |
|---|---|
| `0` | 成功，或幂等地无需修改 |
| `1` | Git 尚未确认成功前发生 usage、repository、pre-hook、Git 或 metadata 错误 |
| `2` | Git add/remove 已成功，但 metadata 或 post-hook 失败 |

退出 2 表示部分成功。GWM 不会回滚 Git 已完成的工作树或 branch 操作。

## 开发与验证

```bash
gofmt -w <changed-go-files>
go test ./...
go test -race ./...
go vet ./...
```

所有修改型集成测试只使用测试自己创建的临时 repository，不访问网络或创建 remote。

项目结构：

```text
cmd/gwm/          CLI 入口
internal/app/     命令解析与五个命令流程
internal/gitcli/  Git 参数数组 runner 与 config 读取
internal/meta/    gwm.worktree.* 校验和读写
internal/hooks/   Hook 配置、payload 和直接执行
```

## 文档

| 文档 | 职责 |
|---|---|
| [DESIGN.md](DESIGN.md) | 产品边界、架构选择和安全原则 |
| [SPEC.md](SPEC.md) | 命令、metadata、Hook、输出和退出码的规范 |
| [PLAN.md](PLAN.md) | 当前状态、已完成里程碑和发布门槛 |
| [GIT_WORKTREE.md](GIT_WORKTREE.md) | 原生 Git worktree 的功能、版本历史和 GWM 兼容性背景 |
| [AGENTS.md](AGENTS.md) | 自动化开发代理的仓库约束 |
| [LICENSE](LICENSE) | MIT 许可证全文 |

文档发生冲突时，产品边界以 DESIGN 为准，可观察行为以 SPEC 为准，进度与验收以 PLAN 为准。

## 许可证

本项目采用 [MIT License](LICENSE)，Copyright (c) 2026 gongshuiwen。
