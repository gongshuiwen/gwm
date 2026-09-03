# GWM v0.2 设计

| 项目 | 内容 |
|---|---|
| 状态 | Accepted，已完成本地实现 |
| 设计基线 | 3.2（薄包装器） |
| 适用版本 | v0.2 |
| 最后更新 | 2026-09-03 |

## 1. 背景与目标

Git 已经拥有 worktree、branch、HEAD、lock 和删除语义。GWM 的目标不是建立第二套工作树管理系统，而是在原生 `git worktree` 之上提供一个较短的命令入口，并补充两个 Git 未直接表达的本地能力：

1. 每个工作树的 `description`、`protected` 和可缺失的 `created-at` metadata。
2. GWM add/remove 前后的 repository-local 生命周期 Hook。

v0.2 提供 `init`、`list`、`add`、`meta`、`remove` 五个命令和 `pre-add`、`post-add`、`pre-remove`、`post-remove` 四个 Hook。`created-at` 只记录 `gwm add` 成功确认工作树已登记的时间，不是 Git 或文件系统提供的通用创建时间。

CLI 另外提供 `--help`、五个子命令的 `--help` 和 `--version`。这些入口只描述本地程序，不依赖 repository context，也不运行 Git 或 Hook；它们不是新的工作树管理命令。

## 2. 非目标

以下能力没有当前需求，不进入 v0.2：

- UUID、接管时间、最后使用时间和历史记录
- doctor、adopt、edit、show 等独立命令
- JSON CLI、稳定错误码和跨语言 schema
- repository lock、transaction、自动回滚和崩溃恢复日志
- Hook 链、重试、并行、全局 Hook，以及 tracked Hook 的自动发现或自动启用
- move、lock、unlock、prune 的重复包装
- fetch、push、远端发布判断、GUI 和后台服务

这些能力不能通过预留空接口进入实现；只有出现明确需求并更新本设计后才能增加。

## 3. 架构

实现保持五个职责明确的 package：

```text
cmd/gwm/          进程入口和退出码
internal/app/     参数解析、repository context 和命令编排
internal/gitcli/  Git 参数数组调用与 config 读取
internal/meta/    gwm.worktree.* 校验和读写
internal/hooks/   Hook 配置、payload 和直接执行
```

依赖方向是单向的：

```text
cmd/gwm → app → gitcli
              → meta → gitcli
              → hooks → meta
                      → gitcli
```

`gitcli.Runner` 和 `hooks.Executor` 是仅有的运行边界接口，用于隔离子进程并在测试中注入失败。v0.2 不增加通用 repository、event、clock、transaction 或 plugin 抽象。

Help 和 version 在参数解析阶段直接输出；只有五个业务命令进入 repository discovery 和命令编排。版本值作为 v0.2 源码常量维护，发布前不增加 linker 注入或版本探测机制。

## 4. 数据所有权

Git 是以下状态的唯一事实源：

- worktree path 和存在性
- HEAD 和 branch
- main、linked、detached、bare、locked 状态
- 工作树目录和 branch 的创建、删除行为

GWM 不保存这些字段的副本，只拥有三个 worktree-scope 配置键：

```text
gwm.worktree.description=修复登录
gwm.worktree.protected=false
gwm.worktree.created-at=2026-09-03T08:30:00Z
```

`gwm.worktree.description` 缺失等价于 description 为 null，`gwm.worktree.protected` 缺失等价于 protected 为 false。`gwm.worktree.created-at` 是可缺失的 UTC RFC 3339 时间，只由 `gwm add` 在 Git 成功后写入；main worktree、原生 Git 创建的 linked worktree 和旧工作树可以没有该键。GWM 不回填或编辑它，不定义 managed/unmanaged 生命周期，也不生成稳定 UUID。

`created-at` 是展示信息，不参与删除保护。其值非法或重复时只标记时间字段无效，不使 description/protected 无效，也不阻止 `meta` 或 `remove`。

## 5. Git 集成

GWM 通过参数数组调用系统 Git，不构造 shell command string，也不直接读写 `<git-common-dir>/worktrees`。

`gwm init` 只启用 `extensions.worktreeConfig`。Metadata 通过目标工作树中的公开命令读写：

```text
git config --worktree --get-all gwm.worktree.description
git config --worktree --bool --get-all gwm.worktree.protected
git config --worktree --get-all gwm.worktree.created-at
git config --worktree --replace-all gwm.worktree.description <text>
git config --worktree --replace-all gwm.worktree.protected <boolean>
git config --worktree --replace-all gwm.worktree.created-at <timestamp>
```

Description 为 null 时删除 `gwm.worktree.description`。`gwm meta` 只写 description 和 protected；`gwm add` 在 Git 成功并重新确认目标已登记后捕获当前 UTC 时间，按 description、protected、created-at 的顺序写入。三个键是独立的 Git config 写入，不构成多键事务；写入失败时 GWM 不回滚已经完成的字段。

工作树创建和删除只调用原生 `git worktree add/remove`。修改型 Git 返回后，GWM 重新读取原生 worktree 清单，不根据目录残留猜测结果。

Git 已完成的副作用不会因后续 metadata 或 Hook 失败而回滚。GWM 不递归清理工作树目录，也不删除 branch。

## 6. 生命周期 Hook

v0.2 的执行顺序固定为：

```text
pre-add → git worktree add → metadata write → post-add
pre-remove → git worktree remove → post-remove
```

Hook 只从 common repository 的 local Git config 读取，每个事件最多配置一个 executable path：

```text
gwm.hooks.pre-add
gwm.hooks.post-add
gwm.hooks.pre-remove
gwm.hooks.post-remove
```

绝对路径原样使用；相对路径固定以 main worktree 根目录解析，与命令从 main 或 linked worktree 调用无关。解析后的路径必须指向普通可执行文件。

GWM 直接执行 Hook，不经过 shell，不从 tracked 文件发现 Hook，也不自动信任仓库内容。Hook executable 可以作为普通文件提交到 `.githooks/`，但文件存在本身不授予执行权限。每个 clone 必须由用户审查后，在该仓库的 local Git config 中显式配置路径；相对路径只是减少机器相关的绝对路径，不改变显式授权要求。Clone 不复制源仓库的 local config，GWM 也不在 clone 时执行生命周期 Hook。

系统 Git 仍可能按用户已有配置运行原生 Hook、filter 或 fsmonitor；它们不属于 GWM Hook。

Pre-hook 非零时不调用修改型 Git。Git 成功后，metadata 或 post-hook 失败只报告部分成功，不回滚工作树。

Hook 只覆盖经 GWM 发起的操作。用户直接运行原生 Git 时，不会触发 GWM Hook。

## 7. 失败与并发

每次修改遵循同一个最小模型：

```text
读取当前状态
→ 运行 pre-hook
→ 调用一次修改型 Git
→ 重新读取 Git 当前状态
→ 完成 metadata/post-hook
→ 输出最终结果
```

结果只区分三类：

- success：Git 和 GWM 后续步骤全部成功。
- failure：Git 尚未确认成功，或 pre-hook 阻止操作。
- partial：Git 已成功，但 metadata 或 post-hook 失败。

GWM 不增加 repository lock。并发 Git 操作由 Git 自身锁处理；每个 metadata 键采用最后写入者获胜，多字段更新可能部分完成或与其他进程交错。命令只报告结束时可重新读取到的状态，不承诺跨进程强一致性。

## 8. 安全与信任边界

- 清除调用环境中的 Git repository 重定位和临时 config 注入变量。
- 不因 tracked 文件存在而自动执行它，也不执行 remote 内容或网络返回内容。
- Hook 必须由用户在 local Git config 中显式配置。
- 不输出完整环境、credential、token、private key 或 Hook stdin。
- 修改型测试只操作测试创建并独占的临时 repository。

Hook 和系统 Git 可能执行用户本地配置的程序，这是明确的本地信任边界，不属于远端代码信任。

## 9. 兼容性与依赖

| 项目 | 要求 |
|---|---|
| Git | 2.39+ |
| Go | 1.26+，仅构建需要 |
| 平台 | Linux、macOS 13+ |
| Go 依赖 | 仅标准库 |
| 网络 | v0.2 不访问网络 |
| 许可证 | MIT |
| Canonical repository | `https://github.com/gongshuiwen/gwm` |
| Go module | `github.com/gongshuiwen/gwm` |

Canonical repository 和 module path 已固定。版本标签、module version 和预编译制品仍须满足 [PLAN.md](PLAN.md) 的发布门槛后才能发布。

## 10. 规范关系

本文拥有产品边界、数据所有权和跨模块设计原则。[SPEC.md](SPEC.md) 可以细化公开行为，但不能扩大产品范围；[PLAN.md](PLAN.md) 只记录实施状态、验收和发布门槛。

README 用于用户导航，AGENTS 用于约束自动化开发动作，两者都不创建新的产品行为。
