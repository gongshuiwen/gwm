# GWM 文档

## 使用 GWM

| 需求 | 文档 |
|---|---|
| 了解功能、构建并开始使用 | [项目 README](../README.md) |
| 查询命令参数与路径规则 | [命令参考](SPEC.md#2-根命令) |
| 配置工作树描述、保护与查看创建时间 | [Metadata 参考](SPEC.md#5-metadata) |
| 启用生命周期 Hook、编写接收程序 | [启用示例](../README.md#生命周期-hook)、[Hook 配置与 schema 2](SPEC.md#10-hook-配置) |
| 处理失败和部分成功 | [输出与退出码](SPEC.md#11-输出与退出码) |
| 了解 Git worktree 模型与兼容限制 | [Git worktree 背景](GIT_WORKTREE.md) |

[SPEC.md](SPEC.md) 提供完整行为规范，按需要查询即可。

## 参与开发

- [贡献指南](../CONTRIBUTING.md)：开发环境、修改流程、验证和文档维护。
- [设计说明](DESIGN.md)：产品边界、架构、数据所有权和安全原则。
- [AGENTS.md](../AGENTS.md)：自动化开发代理的执行约束。

## 维护者入口

- [发布指南](RELEASING.md)：版本规则、发布前验证、流水线与制品检查。
- [首次发布计划](plans/first-release.md)：尚未完成的验证和发布决策。
- [初始实施归档](archive/initial-implementation.md)：四个已完成阶段和当时的验证记录，不再维护。

## 文档职责

产品边界以 DESIGN 为准，可观察行为以 SPEC 为准。贡献指南负责开发验证，发布指南负责发布流程。`docs/plans/` 记录活动任务的范围与验收，`docs/archive/` 保存历史；两者均不定义当前产品行为。具体维护规则见 [贡献指南](../CONTRIBUTING.md#文档维护)。
