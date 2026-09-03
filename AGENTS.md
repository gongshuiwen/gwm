# AGENTS.md

> Repository-local instructions for automated development agents. These rules apply to the entire GWM workspace and do not define user-facing product behavior.

## Scope

这些规则适用于整个 GWM 项目。产品文档使用中文；代码、标识符、命令和固定协议字段保持其原始英文形式。

## Required Reading

规划或实施前完整阅读：

1. `README.md`
2. `DESIGN.md`
3. `SPEC.md`
4. `PLAN.md`

权威顺序为：DESIGN（产品边界）→ SPEC（可观察行为）→ PLAN（阶段与验收）→ 本文件（实施动作）。README 只用于概览和导航。发现冲突时停止受影响工作并先修正文档。

## Current State

当前状态只从 `PLAN.md` 和实时命令获取，不在本文件复制日期、Git 初始化或工具链状态。编码前必须重新运行 `git --version` 和 `go version`。

## Implementation Scope

- 严格按 `PLAN.md` 的当前阶段实施；一次阶段请求不授权后续阶段。
- v0.1 只实现 `init`、`list`、`add`、`meta`、`remove`。
- v0.1 只实现 `pre-add`、`post-add`、`pre-remove`、`post-remove`。
- 不提前增加 DESIGN 明确延期的命令、状态系统或扩展接口。
- 可观察行为变化必须先更新 `SPEC.md`，产品边界变化必须先更新 `DESIGN.md`。

## Git And Metadata Safety

- Git 必须使用参数数组调用，不能构造 shell command string。
- 固定一次命令的 repository context，并清除 Git 重定位和临时 config 注入环境变量。
- 使用 `git worktree list --porcelain -z` 读取工作树清单。
- 只能通过 `git config --worktree` 读写 `gwm.metadata`。
- 不得直接读取、修改或删除 `<git-common-dir>/worktrees`。
- Git 返回后必须重新读取 worktree 清单并输出当前状态。
- 不得为回滚 add 递归删除路径或分支；不得在 remove 后补充递归删除。
- 不实现 GWM repository lock，也不得宣称消除了原生 Git 并发。

## Hook Safety

- GWM Hook 只从 common repository 的 local Git config 读取。
- Hook 必须由用户在 local config 中明确配置；绝对路径原样使用，相对路径以 main worktree 根目录解析，结果必须是可执行的普通文件。
- Hook 使用参数数组直接执行，不经过 shell，不附加隐式参数。
- 不从 tracked 文件、remote 或工作树内容自动发现 Hook。
- Local config 可以显式指向用户已审查的 tracked executable；tracked 文件存在本身不授权执行。
- Pre-hook 非零时不得启动对应修改型 Git。
- Post-hook 失败不得回滚已经完成的 Git 操作。
- 不记录 Hook stdin 中可能出现的敏感 metadata，也不输出完整环境。

## Test Safety

- 所有修改型测试必须创建并拥有独立临时 repository。
- 不得修改用户已有 repository，不得访问网络或创建 remote。
- 测试清理必须验证精确目标，拒绝 `/`、home、workspace root、空路径和未解析变量。
- Hook fixture 必须位于测试临时目录，不得执行用户机器上的真实 Hook。

## External State And Dependencies

- v0.1 运行时只依赖 Go 标准库和系统 Git。
- 未经明确授权，不得安装工具链、提交代码、创建 remote、配置 CI 或发布制品。
- 新增依赖、网络行为或自动发现/启用 tracked Hook 前必须先更新 `DESIGN.md` 并说明信任与供应链影响。
- 未确定 canonical module path 和许可证前，不得猜测并公开发布。

## Validation

执行 `PLAN.md` 当前阶段列出的验证。文档变更还必须检查链接、围栏、JSON 示例，以及五个命令、两个 metadata 字段、四个 Hook 和两阶段计划的一致性。

无法运行的验证必须报告原因、风险和替代检查，不能表述为通过。
