# AGENTS.md

本文件约束整个 GWM 仓库中的自动化开发动作。

## 通用约束

- 只执行用户已授权的任务；相关计划细化范围与验收，一次阶段请求不授权后续阶段。
- 未经明确授权，不得安装工具链、提交代码、创建 remote、配置 CI 或发布制品。
- 产品边界以 DESIGN 为准，可观察行为以 SPEC 为准；变更前先更新对应文档。不提前实现非目标或为其预留空接口。
- 发现规范冲突时，停止受影响工作并先修正文档。

## 按需读取

开始操作前，按任务匹配下表，必须阅读适用的章节；一项任务可以匹配多行。已读且未变化的内容可复用，无需沿链接展开全部文档。

| 涉及的任务 | 必读内容 |
|---|---|
| 修改代码 | [DESIGN](docs/DESIGN.md) 的产品边界与相关设计、[SPEC](docs/SPEC.md) 的相关行为；[开发环境](CONTRIBUTING.md#开发环境)与 [Go 变更验证](CONTRIBUTING.md#go-变更) |
| 修改 Git 集成或 metadata 逻辑 | [Git 集成约束](docs/DESIGN.md#5-git-集成)、[安全边界](docs/DESIGN.md#8-安全与信任边界)，以及 SPEC 中对应命令、repository context 和 metadata 章节 |
| 修改或调试 Hook | [Hook 信任与执行边界](docs/DESIGN.md#6-生命周期-hook)、[敏感信息限制](docs/DESIGN.md#8-安全与信任边界)、[Hook 协议](docs/SPEC.md#10-hook-配置) |
| 运行或修改测试、清理测试数据 | [测试安全](CONTRIBUTING.md#测试安全)及对应验证要求 |
| 修改文档 | 目标文档及相关规范来源、[文档维护](CONTRIBUTING.md#文档维护)与[文档验证](CONTRIBUTING.md#文档变更) |
| 变更依赖、运行时网络、Hook 信任或仓库标识 | [依赖与仓库标识](CONTRIBUTING.md#依赖与仓库标识) |
| 发布 | [发布指南](docs/RELEASING.md)及对应活动计划 |

需要了解项目用途时查阅 [README](README.md)；需要定位文档、活动计划或历史背景时查阅 [文档索引](docs/README.md)。任务状态结合相关计划、Issue/PR 和实时检查确认，归档不代表当前状态或授权。

## 验证与交付

按修改类型完成表中对应验证及相关计划的验收。交付时说明修改与验证结果；无法执行的验证须报告原因、风险及替代检查，不得表述为通过。
