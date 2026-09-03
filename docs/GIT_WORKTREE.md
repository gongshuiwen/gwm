# Git worktree 功能与版本历史

| 项目 | 内容 |
|---|---|
| 文档性质 | 原生 Git 背景资料，不定义 GWM 产品行为 |
| 覆盖版本 | Git 2.5–2.55 |
| 最后核对 | 2026-09-03 |

本文总结原生 `git worktree` 的数据模型、当前功能、自动化接口和主要版本演进。GWM 的产品边界以 [DESIGN.md](DESIGN.md) 为准，可观察行为以 [SPEC.md](SPEC.md) 为准；本文中的历史信息不扩大 GWM 当前待发布版本的范围。完整文档导航见 [README.md](README.md)。

## 1. 核心模型

一个 Git repository 可以拥有一个 main worktree 和零个或多个 linked worktree，从而同时检出多个分支。它们共享 object database 和大部分 refs，但保留各自的检出状态。

| 默认共享 | 每个 worktree 独立 |
|---|---|
| objects、普通 `refs/heads/*`、common config、`.git/info/exclude` | `HEAD`、index、工作目录、部分 pseudo/per-worktree refs、可选 `config.worktree` |

在 linked worktree 中，顶层 `.git` 是一个指向私有 Git directory 的文件。该私有目录位于 common repository 的 worktree 管理区中；应用不应依赖其内部布局，而应使用 `git rev-parse`、`git config --worktree` 和 `git worktree` 等公开命令。

相关路径概念：

- `$GIT_COMMON_DIR` 指向所有 worktree 共享的 repository 数据。
- `$GIT_DIR` 在 main worktree 中通常等于 common directory，在 linked worktree 中指向该 worktree 的私有 Git directory。
- 普通 refs 通常共享；`HEAD`、`refs/bisect/*`、`refs/worktree/*` 和 `refs/rewritten/*` 等状态按 worktree 隔离。
- 可通过 `main-worktree/HEAD` 或 `worktrees/<id>/HEAD` 等特殊 ref 名称，从一个 worktree 访问另一个 worktree 的 per-worktree refs。

完整规则见 Git 官方 [`git-worktree` 文档](https://git-scm.com/docs/git-worktree)。

## 2. 当前命令与语义

截至 Git 2.55，`git worktree` 提供八个子命令：

| 子命令 | 作用 | 关键约束 |
|---|---|---|
| `add` | 创建 linked worktree 并检出 commit 或 branch | 默认拒绝检出已被其他 worktree 使用的 branch |
| `list` | 列出 main 和 linked worktree | 脚本应使用 `--porcelain -z` |
| `lock` | 防止 worktree 被 prune、move 或普通 remove | 可用 `--reason` 记录原因 |
| `move` | 移动 linked worktree | 不能移动 main worktree；包含已初始化 submodule 时仍有限制 |
| `prune` | 清除工作目录已经不存在的陈旧管理信息 | `--expire` 只筛选 missing worktree |
| `remove` | 删除 linked worktree 和对应管理信息 | 默认要求 clean；不能删除 main worktree |
| `repair` | 修复因手工移动、复制或 Git directory 变化而断开的链接 | 可以同时传入多个新路径 |
| `unlock` | 解除 worktree 锁定 | 解除后才能正常 prune、move 或 remove |

常用创建方式：

```bash
# 以目录 basename 创建同名 branch
git worktree add ../topic

# 创建新 branch
git worktree add -b topic ../topic main

# 检出已有 branch
git worktree add ../hotfix hotfix

# 创建 detached worktree
git worktree add --detach ../experiment HEAD

# 创建 orphan branch
git worktree add --orphan -b pages ../pages

# 创建后立即锁定
git worktree add --lock --reason "external disk" ../portable topic
```

同一个本地 branch 原则上只允许被一个 worktree 检出。`--force` 可以覆盖部分保护，但调用方应把它视为用户明确要求突破 Git 默认安全检查，而不是普通重试选项。

## 3. 稳定的机器接口

自动化程序应读取：

```bash
git worktree list --porcelain -z
```

`--porcelain` 提供独立于用户配置的稳定字段，`-z` 使用 NUL 分隔字段和记录，可以无歧义地处理换行、引号、反斜杠及其他特殊 path 字节。可能出现的字段包括：

```text
worktree <path>
HEAD <object-id>
branch <full-ref>
bare
detached
locked [<reason>]
prunable [<reason>]
```

规则：

- main worktree 首先输出，其后是 linked worktree。
- `branch` 使用完整的 `refs/heads/...`；detached worktree 输出 `detached`。
- bare、locked 和 prunable 是属性行，不应按固定行数解析记录。
- 不带 `--porcelain` 的人类可读输出允许调整引用、对齐和转义方式，不适合作为协议。

## 4. Worktree-scope 配置

默认情况下，repository config 在所有 worktree 之间共享。Git 2.20 起可以启用：

```bash
git config extensions.worktreeConfig true
```

然后通过公开接口读写当前 worktree 的配置：

```bash
git config --worktree <key> <value>
git config --worktree --get-all <key>
```

worktree config 存放在当前 worktree 对应的 `config.worktree` 中，并覆盖 common config 中的同名值。启用扩展前应特别检查 `core.worktree`、`core.bare` 和 sparse-checkout 相关配置是否需要迁移。旧版 Git 不认识该 repository extension 时会拒绝访问仓库。

这项能力是 GWM 保存 `gwm.worktree.description`、`gwm.worktree.protected` 和 `gwm.worktree.created-at` 的原生基础；具体初始化和失败语义仍以 [SPEC.md](SPEC.md) 为准。

## 5. Absolute 与 relative worktree linkage

Git 默认使用绝对路径维护 main repository 与 linked worktree 之间的双向链接。Git 2.48 起可以选择相对路径：

```bash
git worktree add --relative-paths ../topic topic
git worktree move --relative-paths ../topic ../topic-new
git worktree repair --relative-paths
```

也可以配置默认值：

```bash
git config worktree.useRelativePaths true
```

相对链接适用于 repository 和 worktree 保持相对位置并被整体移动的场景，例如容器、可移动目录或以 bare repository 为中心的布局。它不会解决 repository 与 worktree 被分别移动到不同相对位置的情况。

首次创建或修复 relative worktree 时，Git 会启用 `extensions.relativeWorktrees`。这会使仓库与 Git 2.47 及更早版本不兼容，因此启用前必须确认所有访问该仓库的 Git 和第三方工具都支持这种格式。详见 [`worktree.useRelativePaths`](https://git-scm.com/docs/git-config#Documentation/git-config.txt-worktreeuseRelativePaths)。

## 6. 版本历史

### 2.5–2.10：从实验入口到可管理对象

| 版本 | 主要变化 |
|---|---|
| 2.5 | `git worktree add` 取代实验性的 `git checkout --to`；已有 `prune`，但尚无 list/remove/move |
| 2.7 | 增加 `list` 和 `list --porcelain`；bisect 状态改为 per-worktree，可在不同 worktree 独立运行 |
| 2.9 | 增加 `add --no-checkout`；加强对其他 worktree 正在检出、rebase 或使用中的 branch 的保护 |
| 2.10 | 增加 `lock`、`unlock`；`add -` 可表示 `@{-1}`，即上一个 branch |

来源：[最初的 `worktree add` 实现](https://github.com/git/git/commit/fc56361f58a2)、[Git 2.7 release notes](https://github.com/git/git/blob/v2.7.0/Documentation/RelNotes/2.7.0.txt)、[Git 2.9 release notes](https://github.com/git/git/blob/v2.9.0/Documentation/RelNotes/2.9.0.txt)、[Git 2.10 release notes](https://github.com/git/git/blob/v2.10.0/Documentation/RelNotes/2.10.0.txt)。

### 2.13–2.19：生命周期命令成熟

| 版本 | 主要变化 |
|---|---|
| 2.13 | `add --lock` 在创建过程中直接加锁，避免 add 与后续 lock 之间的 prune 竞争窗口 |
| 2.16 | 改进 remote branch 推断，引入 `--guess-remote`/`worktree.guessRemote`；add 后运行 `post-checkout` Hook |
| 2.17 | 增加 `move`、`remove`，不再要求用户手工移动或删除目录 |
| 2.18 | `add` 可以直接检出已有本地 branch；`remove -f` 与其他命令的 force 简写一致 |
| 2.19 | 支持 `checkout.defaultRemote`，并增加 `--quiet` |

来源：[Git 2.13 release notes](https://github.com/git/git/blob/v2.13.0/Documentation/RelNotes/2.13.0.txt)、[Git 2.16 release notes](https://github.com/git/git/blob/v2.16.0/Documentation/RelNotes/2.16.0.txt)、[Git 2.17 release notes](https://github.com/git/git/blob/v2.17.0/Documentation/RelNotes/2.17.0.txt)、[Git 2.18 release notes](https://github.com/git/git/blob/v2.18.0/Documentation/RelNotes/2.18.0.txt)、[Git 2.19 release notes](https://github.com/git/git/blob/v2.19.0/Documentation/RelNotes/2.19.0.txt)。

### 2.20–2.28：配置隔离与安全加固

| 版本 | 主要变化 |
|---|---|
| 2.20 | 引入 `extensions.worktreeConfig`、`config.worktree` 和 `git config --worktree`；GC/reachability 更完整地考虑其他 worktree 的 refs |
| 2.21 | `move/remove` 放宽为可处理未初始化的 submodule |
| 2.22 | worktree 管理目录改为原子创建；改进 `refs/rewritten/*` 等 per-worktree refs |
| 2.23 | 清洗 worktree 内部名称以适配 refname；损坏的 sibling worktree 不再阻塞新的 `add` |
| 2.25–2.26 | 修复 linked worktree 的 `rev-parse --git-path`、submodule reset，以及 receive-side branch 占用检查 |
| 2.28 | 阻止 `move` 产生两个登记记录指向同一路径 |

来源：[per-worktree config 实现](https://github.com/git/git/commit/58b284a2e912)、[Git 2.21 release notes](https://github.com/git/git/blob/v2.21.0/Documentation/RelNotes/2.21.0.txt)、[Git 2.22 release notes](https://github.com/git/git/blob/v2.22.0/Documentation/RelNotes/2.22.0.txt)、[Git 2.23 release notes](https://github.com/git/git/blob/v2.23.0/Documentation/RelNotes/2.23.0.txt)、[Git 2.26 release notes](https://github.com/git/git/blob/v2.26.0/Documentation/RelNotes/2.26.0.txt)、[Git 2.28 release notes](https://github.com/git/git/blob/v2.28.0/Documentation/RelNotes/2.28.0.txt)。

### 2.29–2.36：修复能力与自动化协议

| 版本 | 主要变化 |
|---|---|
| 2.29 | 增加 `repair`；`add -d` 成为 `--detach` 简写 |
| 2.30 | 人类可读的 `list` 开始显示 locked 状态 |
| 2.31 | `list` 增加 prunable 标注和 `--verbose`；porcelain 增加 locked/prunable 属性 |
| 2.33 | `add --lock --reason` 可以在创建时记录锁定原因 |
| 2.35 | fetch 对已检出 branch 的保护覆盖所有 worktree，而不只是当前 worktree |
| 2.36 | `list --porcelain -z` 提供 NUL 分隔输出；修复 secondary worktree 中的 rebase/stash 与 sparse-checkout 配置问题 |

来源：[Git 2.29 release notes](https://github.com/git/git/blob/v2.29.0/Documentation/RelNotes/2.29.0.txt)、[Git 2.30 release notes](https://github.com/git/git/blob/v2.30.0/Documentation/RelNotes/2.30.0.txt)、[Git 2.31 release notes](https://github.com/git/git/blob/v2.31.0/Documentation/RelNotes/2.31.0.txt)、[Git 2.33 release notes](https://github.com/git/git/blob/v2.33.0/Documentation/RelNotes/2.33.0.txt)、[Git 2.35 release notes](https://github.com/git/git/blob/v2.35.0/Documentation/RelNotes/2.35.0.txt)、[Git 2.36 release notes](https://github.com/git/git/blob/v2.36.0/Documentation/RelNotes/2.36.0.txt)。

### 2.42–2.48：特殊仓库形态与可移动性

| 版本 | 主要变化 |
|---|---|
| 2.42 | `add --orphan`；改善 sparse-index 集成 |
| 2.44 | 通过 refs API 初始化 secondary worktree，为 files 之外的 ref backend 做准备 |
| 2.45 | 改善 `safe.bareRepository=explicit` 下的 secondary worktree 发现和 `git -C` 补全 |
| 2.48 | 增加 relative linkage、`--relative-paths`、`worktree.useRelativePaths` 和格式保护扩展；`repair` 能安全处理成组复制的 repository/worktree |

来源：[Git 2.42 release notes](https://github.com/git/git/blob/v2.42.0/Documentation/RelNotes/2.42.0.txt)、[Git 2.44 release notes](https://github.com/git/git/blob/v2.44.0/Documentation/RelNotes/2.44.0.txt)、[Git 2.45 release notes](https://github.com/git/git/blob/v2.45.0/Documentation/RelNotes/2.45.0.txt)、[Git 2.48 release notes](https://github.com/git/git/blob/v2.48.0/Documentation/RelNotes/2.48.0.txt)、[复制后 repair 修复](https://github.com/git/git/commit/992f7a4fdbad)。

### 2.49–2.55：兼容性、显示与文档修正

| 版本 | 主要变化 |
|---|---|
| 2.49 | 修复启用 per-worktree config 时，从 secondary worktree 判断 main worktree 是否 bare 的错误 |
| 2.50 | `git verify-refs` 可以处理由 Git 2.43 及更早版本创建的 linked worktree |
| 2.51–2.52 | 没有显著的 `git worktree` 用户功能变化 |
| 2.53 | 修复人类可读 `list` 对非 ASCII path 的列宽计算，并安全引用控制字符 |
| 2.54 | 明确 `list/prune --expire` 只作用于 missing worktree；修复从 secondary worktree 启动 `for-each-repo` 等问题 |
| 2.55 | 没有新增 worktree 子命令或选项；明确记录 `.git/info/exclude` 在 linked worktree 之间共享 |

来源：[Git 2.49 release notes](https://github.com/git/git/blob/v2.49.0/Documentation/RelNotes/2.49.0.adoc)、[Git 2.50 release notes](https://github.com/git/git/blob/v2.50.0/Documentation/RelNotes/2.50.0.adoc)、[Git 2.53 release notes](https://github.com/git/git/blob/v2.53.0/Documentation/RelNotes/2.53.0.adoc)、[Git 2.54 release notes](https://github.com/git/git/blob/v2.54.0/Documentation/RelNotes/2.54.0.adoc)、[Git 2.55 release notes](https://github.com/git/git/blob/v2.55.0/Documentation/RelNotes/2.55.0.adoc)。

## 7. GWM 兼容性结论

| 原生能力 | 首个 Git 版本 | 与 GWM 当前待发布版本的关系 |
|---|---:|---|
| `git worktree remove` | 2.17 | `gwm remove` 的原生基础 |
| `git config --worktree` | 2.20 | 三个 metadata 字段的存储基础 |
| `git worktree repair` | 2.29 | GWM 不包装；用户可直接调用 Git |
| `git worktree list --porcelain -z` | 2.36 | GWM 固定使用的清单协议 |
| `git worktree add --orphan` | 2.42 | GWM 当前待发布版本不包装 |
| relative worktree linkage | 2.48 | GWM 不提供开关，但不能假设内部链接是绝对路径 |

GWM 当前声明 Git 2.39+，覆盖当前实现实际使用的 `remove`、worktree config 和 NUL-delimited list 能力。Git 2.48 的 relative worktree 是额外的 repository 格式边界：使用该格式的仓库必须由 Git 2.48+ 访问，无论上层是否使用 GWM。

实现或测试 GWM 时应保持以下原则：

- 使用 `git worktree list --porcelain -z`，不解析人类可读输出。
- 使用 `git config --worktree`，不直接打开 `config.worktree`。
- 不读取、修改或删除 `$GIT_COMMON_DIR/worktrees` 的内部内容。
- 不假设 linked worktree `.git` 中的路径为绝对路径。
- 修改型 Git 返回后重新读取 worktree 清单，以 Git 的最终状态为准。

## 8. 长期限制

即使在当前版本中，多 worktree 与 submodule 的组合仍不完整。尤其是包含已初始化 submodule 的 linked worktree，`move` 等操作继续受到限制。工具不应绕过 Git 的拒绝后自行移动或递归删除目录，而应保留现场并向用户报告原生 Git 状态。

原生 Git 只管理当前状态，不提供 worktree 创建历史、事务日志、跨命令回滚或远端协作模型。这也是 GWM 保持薄包装器边界、只增加少量本地 metadata 和生命周期 Hook 的原因。
