# 初始实施记录

> 历史归档：记录截至 2026-09-03 的初始实施与本地验证，原设计基线为 3.2。本文不再维护，不定义当前产品行为，也不代表当前验证结果或发布授权。

以下保留当时四个阶段的完成记录和验收表述。其中阶段 1 的两字段 metadata 是阶段 2 增加只读创建时间之前的历史状态。

现行产品边界见 [DESIGN](../DESIGN.md)，公开行为见 [SPEC](../SPEC.md)，开发验证见 [贡献指南](../../CONTRIBUTING.md)，发布要求见 [发布指南](../RELEASING.md)。首次发布剩余事项见 [首次发布计划](../plans/first-release.md)。

## 已完成里程碑

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
- [x] 实现 `gwm --version`；普通源码构建输出 `gwm unreleased`，发布构建由 tag 注入版本。
- [x] 保证 help/version 不发现 repository、不运行 Git 或 Hook。
- [x] 覆盖 stdout、退出码、无 repository 和非法组合。

退出条件已满足：help/version 符合 SPEC，五个业务命令行为保持不变，完整验证通过。

## 当时的本地验证

2026-09-03 在 Linux amd64、Git 2.55.0、Go 1.26.8 环境完成：

- [x] 格式化检查
- [x] `go test -count=1 ./...`
- [x] `go test -race -count=1 ./...`
- [x] `go vet ./...`
- [x] 临时 CLI 构建
- [x] Help/version 在 non-repository 目录运行
- [x] 文档链接、JSON、围栏、术语和范围一致性检查
- [x] Linux/macOS 的 amd64/arm64 发布压缩包与 SHA-256 校验文件本地构建

macOS 13+ 和最低支持版本组合尚未完成发布级验证。
