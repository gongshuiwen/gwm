# GWM 项目状态与发布计划

| 项目 | 当前值 |
|---|---|
| 设计基线 | 3.2（薄包装器） |
| 实现版本 | v0.2 |
| 项目状态 | 本地实现与验证完成，尚未发布 |
| 最后验证 | 2026-09-03 |
| Git 状态 | GitHub `origin/main`；无 CI |
| 本地工具 | Git 2.55.0；Go 1.26.8 |

本文件是项目进度、验收状态和发布门槛的唯一来源。产品边界见 [DESIGN.md](DESIGN.md)，可观察行为见 [SPEC.md](SPEC.md)。

## 1. 当前产物

- 五个命令：`init`、`list`、`add`、`meta`、`remove`。
- 三字段 metadata：`description`、`protected`、只读 `created-at`。
- 四个生命周期 Hook：`pre-add`、`post-add`、`pre-remove`、`post-remove`。
- Root help、五个子命令 help 和 version 信息入口。
- 一个可提交但必须由 local config 显式启用的 `.githooks/` 示例。
- 仅依赖 Go 标准库和系统 Git 的本地 CLI。
- 单元测试和使用独立临时 repository 的集成测试。
- README、设计、行为规范和自动化开发约束。
- MIT `LICENSE`，Copyright (c) 2026 gongshuiwen。

Canonical repository 是 `https://github.com/gongshuiwen/gwm`，Go module path 是 `github.com/gongshuiwen/gwm`。当前没有 CI、版本标签、发布渠道或发布制品。

## 2. 已完成里程碑

### 阶段 0：原生包装与 Metadata

- [x] 初始化本地 Git repository，不创建 remote 或 commit。
- [x] 创建 `go.mod`、CLI 入口和最小 package 结构。
- [x] 实现 `-C`、固定 repository context、Git 参数数组 runner 和环境隔离。
- [x] 实现 `git worktree list --porcelain -z` 解析。
- [x] 实现幂等 `init` 和 `extensions.worktreeConfig` preflight。
- [x] 实现 `gwm.worktree.description`、`gwm.worktree.protected` 的校验、独立写入和重读验证。
- [x] 实现 `list`、`add`、`meta`、`remove` 非 Hook 路径。
- [x] 覆盖 main、linked、detached、locked、特殊 UTF-8 path、metadata missing/invalid 和原生 Git 失败。
- [x] 验证失败时不猜测性回滚、remove 不递归补删目录且不删除 branch。

退出条件已满足：五个命令的非 Hook 行为符合 SPEC，所有修改型测试只操作测试创建的临时 repository。

### 阶段 1：Lifecycle Hook 与本地完成

- [x] 实现四个 repository-local Hook key 的读取和唯一值验证。
- [x] 支持绝对 Hook path 和相对于 main worktree 的 Hook path，验证普通可执行文件，并直接启动且不经过 shell。
- [x] 实现固定 JSON stdin payload 和 invocation-root cwd。
- [x] 接入 pre-add、post-add、pre-remove、post-remove 的规范顺序。
- [x] 覆盖未配置、重复配置、不可执行、pre 非零和 post 非零。
- [x] 验证 pre 失败不调用修改型 Git，post 失败不回滚已完成的 Git。
- [x] 验证原生 `git worktree` 不触发 GWM Hook。
- [x] 完成 README 用法、usage text 和退出码测试。

退出条件已满足：五个命令、两字段 metadata 和四个 Hook 均符合 SPEC；partial 路径具有明确输出；实现中没有延期功能或空扩展接口。

### 阶段 2：只读创建时间

- [x] 增加可缺失的 `gwm.worktree.created-at`，由 `gwm add` 在 Git 成功后写入 UTC RFC 3339 时间。
- [x] 在 `list` 和 `meta` 中展示 created-at，但不增加编辑参数、回填或推断。
- [x] 保证非法 created-at 不使 description/protected 无效，也不阻止 `meta` 或 `remove`。
- [x] 将 Hook payload 升级到 schema 2，并覆盖 pre/post add/remove 的 created_at 语义。
- [x] 覆盖原生 Git 创建、缺失、非法、重复和 metadata 部分失败路径。

退出条件已满足：三个 metadata 字段符合 SPEC，现有五个命令与四个 Hook 没有新增扩展接口，完整验证通过。

### 阶段 3：CLI 自描述

- [x] 实现 `gwm --help` 和五个 `gwm <command> --help`。
- [x] 实现固定输出 `gwm v0.2` 的 `gwm --version`。
- [x] 保证 help/version 不发现 repository、不运行 Git 或 Hook。
- [x] 覆盖 stdout、退出码、无 repository 和非法组合。

退出条件已满足：help/version 符合 SPEC，五个业务命令行为保持不变，完整验证通过。

## 3. 质量门槛

Go 变更必须通过：

```bash
gofmt -w <changed-go-files>
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/gwm
```

文档变更必须验证：

- 没有尾随空白，代码围栏成对。
- 所有相对链接有效，所有 fenced JSON 可解析。
- 五个业务命令、两个信息 flag、三个 metadata 字段、四个 Hook 和四个实施阶段一致。
- README、DESIGN、SPEC、PLAN、AGENTS 之间没有规范循环定义。

修改型集成测试必须创建并独占临时 repository，不访问网络、不创建 remote，也不执行用户机器上的真实 GWM Hook。具体约束见 [AGENTS.md](AGENTS.md)。

### 最近一次本地验证

2026-09-03 在 Linux amd64、Git 2.55.0、Go 1.26.8 环境完成：

- [x] 格式化检查
- [x] `go test -count=1 ./...`
- [x] `go test -race -count=1 ./...`
- [x] `go vet ./...`
- [x] 临时 CLI 构建
- [x] Help/version 在 non-repository 目录运行
- [x] 文档链接、JSON、围栏、术语和范围一致性检查

macOS 13+ 和最低支持版本组合尚未完成发布级验证。

## 4. 发布门槛

- [x] 确认 canonical repository URL：`https://github.com/gongshuiwen/gwm`。
- [x] 确认最终 Go module path：`github.com/gongshuiwen/gwm`。
- [x] 选择并添加 MIT 许可证。
- [ ] 在 Linux 和 macOS 13+ 上验证最低与当前 Git/Go 组合。
- [ ] 确认 CI、发布渠道、版本标记和发布授权。

这些事项不阻塞本地开发，但在全部完成前不得将 v0.2 表述为正式发布，也不授权创建 remote、CI 或发布制品。

## 5. 变更流程

1. 产品范围变化先更新 DESIGN。
2. 可观察行为变化先更新 SPEC。
3. 实现行为并同步增加或调整测试。
4. 更新 README 的用户入口和 PLAN 的验收状态。
5. 运行本文件定义的质量门槛。

README 只提供概览和使用入口；AGENTS 只约束自动化开发动作，两者不创建产品规范。
