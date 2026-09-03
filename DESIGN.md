# GWM v0.1 设计

| 项目 | 内容 |
|---|---|
| 状态 | Accepted，已完成本地实现 |
| 设计基线 | 3.0（薄包装器） |
| 适用版本 | v0.1 |
| 最后更新 | 2026-09-03 |

## 1. 背景与目标

Git 已经拥有 worktree、branch、HEAD、lock 和删除语义。GWM 的目标不是建立第二套工作树管理系统，而是在原生 `git worktree` 之上提供一个较短的命令入口，并补充两个 Git 未直接表达的本地能力：

1. 每个工作树的 `description` 和 `protected` metadata。
2. GWM add/remove 前后的 repository-local 生命周期 Hook。

v0.1 提供 `init`、`list`、`add`、`meta`、`remove` 五个命令和 `pre-add`、`post-add`、`pre-remove`、`post-remove` 四个 Hook。

## 2. 非目标

以下能力没有当前需求，不进入 v0.1：

- UUID、创建时间、接管时间和历史记录
- doctor、adopt、edit、show 等独立命令
- JSON CLI、稳定错误码和跨语言 schema
- repository lock、transaction、自动回滚和崩溃恢复日志
- Hook 链、重试、并行、全局 Hook 和 tracked Hook
- move、lock、unlock、prune 的重复包装
- fetch、push、远端发布判断、GUI 和后台服务

这些能力不能通过预留空接口进入实现；只有出现明确需求并更新本设计后才能增加。

## 3. 架构

实现保持五个职责明确的 package：

```text
cmd/gwm/          进程入口和退出码
internal/app/     参数解析、repository context 和命令编排
internal/gitcli/  Git 参数数组调用与 config 读取
internal/meta/    gwm.metadata 编解码和读写
internal/hooks/   Hook 配置、payload 和直接执行
```

依赖方向是单向的：

```text
cmd/gwm → app → gitcli
              → meta → gitcli
              → hooks → meta
                      → gitcli
```

`gitcli.Runner` 和 `hooks.Executor` 是仅有的运行边界接口，用于隔离子进程并在测试中注入失败。v0.1 不增加通用 repository、event、transaction 或 plugin 抽象。

## 4. 数据所有权

Git 是以下状态的唯一事实源：

- worktree path 和存在性
- HEAD 和 branch
- main、linked、detached、bare、locked 状态
- 工作树目录和 branch 的创建、删除行为

GWM 不保存这些字段的副本，只拥有一个 worktree-scope 配置值：

```text
gwm.metadata={"description":"修复登录","protected":false}
```

Metadata 缺失是正常状态，等价于 description 为 null、protected 为 false。GWM 不定义 managed/unmanaged 生命周期，也不生成稳定 UUID。

## 5. Git 集成

GWM 通过参数数组调用系统 Git，不构造 shell command string，也不直接读写 `<git-common-dir>/worktrees`。

`gwm init` 只启用 `extensions.worktreeConfig`。Metadata 通过目标工作树中的公开命令读写：

```text
git config --worktree --get-all gwm.metadata
git config --worktree --replace-all gwm.metadata <json>
```

工作树创建和删除只调用原生 `git worktree add/remove`。修改型 Git 返回后，GWM 重新读取原生 worktree 清单，不根据目录残留猜测结果。

Git 已完成的副作用不会因后续 metadata 或 Hook 失败而回滚。GWM 不递归清理工作树目录，也不删除 branch。

## 6. 生命周期 Hook

v0.1 的执行顺序固定为：

```text
pre-add → git worktree add → metadata write → post-add
pre-remove → git worktree remove → post-remove
```

Hook 只从 common repository 的 local Git config 读取，每个事件最多配置一个绝对可执行文件：

```text
gwm.hook.pre-add
gwm.hook.post-add
gwm.hook.pre-remove
gwm.hook.post-remove
```

GWM 直接执行 Hook，不经过 shell，不从 tracked 文件发现 Hook，也不自动信任仓库内容。系统 Git 仍可能按用户已有配置运行原生 Hook、filter 或 fsmonitor；它们不属于 GWM Hook。

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

GWM 不增加 repository lock。并发 Git 操作由 Git 自身锁处理；metadata edit 采用最后写入者获胜。命令只报告结束时可重新读取到的状态，不承诺跨进程强一致性。

## 8. 安全与信任边界

- 清除调用环境中的 Git repository 重定位和临时 config 注入变量。
- 不自动执行 tracked 文件、remote 内容或网络返回内容。
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
| 网络 | v0.1 不访问网络 |

在 canonical repository URL 和许可证确定前，只进行本地构建与验证，不发布 module 或制品。

## 10. 规范关系

本文拥有产品边界、数据所有权和跨模块设计原则。[SPEC.md](SPEC.md) 可以细化公开行为，但不能扩大产品范围；[PLAN.md](PLAN.md) 只记录实施状态、验收和发布门槛。

README 用于用户导航，AGENTS 用于约束自动化开发动作，两者都不创建新的产品行为。
