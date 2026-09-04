# Git worktree 背景与兼容性

本文解释 GWM 所依赖的 Git worktree 模型及兼容限制。GWM 的产品边界和可观察行为分别以 [DESIGN](DESIGN.md) 和 [SPEC](SPEC.md) 为准。

## 工作树模型

一个 Git repository 可以拥有一个 main worktree 和零个或多个 linked worktree，从而同时检出多个分支。

| 默认共享 | 每个 worktree 独立 |
|---|---|
| objects、普通 `refs/heads/*`、common config、`.git/info/exclude` | `HEAD`、index、工作目录、部分 pseudo/per-worktree refs、可选 `config.worktree` |

Linked worktree 顶层的 `.git` 是指向该 worktree 私有 Git directory 的文件。应用应通过 `git rev-parse`、`git config --worktree` 和 `git worktree` 等公开命令取得信息，不依赖 Git 私有管理目录的布局。

同一个本地 branch 通常只能由一个 worktree 检出；需要共享同一 commit 而不占用 branch 时，可以使用 detached worktree。完整模型与原生命令见 Git 官方 [`git-worktree` 文档](https://git-scm.com/docs/git-worktree)。

## 工作树清单

GWM 固定使用：

```bash
git worktree list --porcelain -z
```

Porcelain 提供稳定字段，`-z` 使用 NUL 分隔字段和记录，以处理换行、引号、反斜杠等特殊路径字节。解析时应识别 `worktree`、`HEAD`、`branch`、`bare`、`detached`、`locked` 和 `prunable` 等字段，不依赖固定行数或人类可读输出的列宽。

GWM 按 Git 原生顺序展示工作树，修改型 Git 返回后重新读取清单，以 Git 的可观察状态为准。

## Worktree-scope 配置

Repository config 默认在工作树间共享。启用 `extensions.worktreeConfig` 后，可以通过 `git config --worktree` 为每个工作树分别存储配置。这是 GWM 保存 `gwm.worktree.description`、`gwm.worktree.protected` 和 `gwm.worktree.created-at` 的基础。

GWM 用户通过 `gwm init` 启用扩展。它会检查 `core.worktree`、`core.bare` 和 sparse-checkout 相关配置，不自动迁移不支持的布局；具体拒绝条件见 [Init 规范](SPEC.md#4-init)。

## 版本与路径兼容性

GWM 要求 Git 2.39+，覆盖实现所需的原生能力：

| 原生能力 | 首个 Git 版本 | 用途 |
|---|---|---|
| `git worktree remove` | 2.17 | 删除 linked worktree |
| `git config --worktree` | 2.20 | 按工作树存储 metadata |
| `git worktree list --porcelain -z` | 2.36 | 读取可无歧义解析的工作树清单 |

Git 2.48 引入 relative worktree linkage。启用该格式的 repository 要求 Git 2.48+，即使 GWM 自身的最低版本要求是 2.39。GWM 不提供相对链接的配置开关，也不假设 Git 内部链接使用绝对路径。配置说明见 [`worktree.useRelativePaths`](https://git-scm.com/docs/git-config#Documentation/git-config.txt-worktreeuseRelativePaths)。

原生能力的演进可查阅 Git 官方 [release notes](https://github.com/git/git/tree/master/Documentation/RelNotes)，包括 [2.17](https://github.com/git/git/blob/v2.17.0/Documentation/RelNotes/2.17.0.txt)、[2.20](https://github.com/git/git/blob/v2.20.0/Documentation/RelNotes/2.20.0.txt)、[2.36](https://github.com/git/git/blob/v2.36.0/Documentation/RelNotes/2.36.0.txt) 和 [2.48](https://github.com/git/git/blob/v2.48.0/Documentation/RelNotes/2.48.0.txt)。

## 原生操作与限制

GWM 不包装 move、lock、unlock、prune 或 repair。需要这些能力时直接使用 Git，之后 `gwm list` 展示 Git 的当前状态。原生 Git 操作不触发 GWM Hook，也不会自动写入 GWM metadata；原生 remove 可能随工作树删除 metadata。

包含 submodule 的多 worktree 布局仍存在限制，例如 `move` 对已初始化 submodule 的限制。应保留 Git 的拒绝结果，不通过自行移动或递归删除目录绕过检查。

Git 不提供 worktree 创建历史、事务日志或跨命令回滚。GWM 的只读创建时间仅记录 `gwm add` 确认成功的时刻，不能用来推断其他工作树的真实创建时间。
