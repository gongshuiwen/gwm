# GWM 文档

本目录保存 GWM 的产品设计、行为规范、项目状态和背景资料。根目录 [README](../README.md) 是用户入口，[AGENTS.md](../AGENTS.md) 是自动化开发约束，[LICENSE](../LICENSE) 是许可证全文。

| 文档 | 职责 |
|---|---|
| [DESIGN.md](DESIGN.md) | 产品边界、架构选择、数据所有权和安全原则 |
| [SPEC.md](SPEC.md) | 命令、metadata、Hook、输出和退出码的可观察规范 |
| [PLAN.md](PLAN.md) | 当前状态、已完成里程碑、质量门槛和发布计划 |
| [GIT_WORKTREE.md](GIT_WORKTREE.md) | 原生 Git worktree 的背景、版本历史和兼容性资料 |

## 阅读顺序

1. 先阅读 DESIGN，确认产品目标和非目标。
2. 再阅读 SPEC，确认用户可观察行为。
3. 实施或发布前阅读 PLAN，确认当前阶段和验收状态。
4. 需要核对原生 Git 能力时再阅读 GIT_WORKTREE。

文档发生冲突时，产品边界以 DESIGN 为准，可观察行为以 SPEC 为准，进度与验收以 PLAN 为准。GIT_WORKTREE 只提供背景资料，不扩大产品范围。
