# 贡献指南

本文说明 GWM 的开发、验证和文档维护流程。开始使用 GWM 请阅读根目录 [README](README.md)；产品边界和公开行为分别由 [DESIGN](docs/DESIGN.md) 和 [SPEC](docs/SPEC.md) 定义。

## 开发环境

- Linux 或 macOS 13+。
- Git 2.39+、Go 1.26+；Go module path 为 `github.com/gongshuiwen/gwm`。
- GWM 仅依赖 Go 标准库和系统 Git。

开始编码前确认实际工具版本：

```bash
git --version
go version
```

## 代码组织

```text
cmd/gwm/          CLI 入口和退出码
internal/app/     参数解析、repository context 和命令编排
internal/gitcli/  Git 参数数组 runner 与 config 读取
internal/meta/    gwm.worktree.* 校验和读写
internal/hooks/   Hook 配置、payload 和直接执行
```

模块依赖与设计取舍见 [架构说明](docs/DESIGN.md#3-架构)。

## 修改流程

1. 根据修改范围阅读 DESIGN 和 SPEC 的相关章节；有相关活动计划时，阅读文档索引中的对应计划。
2. 产品边界变化先更新 DESIGN，可观察行为变化先更新 SPEC。
3. 实现修改，并为行为变化增加或调整必要测试。
4. 同步受影响的使用说明和参考链接；任务进度记录在对应计划、Issue 或 PR 中。
5. 运行下述与修改类型对应的验证，并在 PR 中说明结果和未覆盖项。

自动化开发代理还须遵循 [AGENTS.md](AGENTS.md) 的范围、权限与安全约束。

## 依赖与仓库标识

新增依赖、运行时网络行为或自动发现/启用 tracked Hook 前，必须先更新 [DESIGN](docs/DESIGN.md#8-安全与信任边界)，说明信任与供应链影响。

Canonical repository 和 module path 固定为 [DESIGN 中的值](docs/DESIGN.md#9-兼容性与依赖)；变更前同步更新 DESIGN、[发布指南](docs/RELEASING.md)及相关活动计划。

## 验证

### Go 变更

Go 变更必须完成格式化、测试、race test、静态检查和构建：

```bash
gofmt -w <changed-go-files>
go test ./...
go test -race ./...
go vet ./...
go build -o bin/gwm ./cmd/gwm
```

### 文档变更

文档变更必须检查：

- 无尾随空白，代码围栏成对。
- 相对链接与章节锚点有效，fenced JSON 示例可解析。
- 五个业务命令 `init`、`list`、`add`、`meta`、`remove`，两个信息 flag `--help`、`--version`，三个 metadata 字段 `description`、`protected`、`created-at`，四个 Hook `pre-add`、`post-add`、`pre-remove`、`post-remove` 和 Hook schema 2 的说明一致。
- 文档职责清晰；摘要与规范一致，计划和归档不定义当前产品行为。
- 移动或归档文档时，检查旧路径引用，保留尚未完成的要求，并同步文档索引和 AGENTS 的阅读入口。

仅修改文档时执行文档检查；涉及命令示例或工作流说明时，还须核对对应 CLI 或 workflow。不能运行的验证应说明原因、风险和替代检查，不得记为通过。

### 测试安全

所有修改型测试必须创建并独占临时 repository，不操作已有用户 repository，不访问网络或创建 remote。Hook fixture 必须位于测试临时目录，不执行用户机器上的真实 Hook。清理前必须验证精确目标，拒绝 `/`、home、workspace root、空路径和未解析变量。

## 文档维护

产品文档使用中文；代码、标识符、命令和固定协议字段保持英文。每类信息有明确归属，其他页面使用简述和链接：

| 内容 | 位置 |
|---|---|
| 项目介绍、构建、快速开始 | [README](README.md) |
| 产品边界、架构与信任边界 | [DESIGN](docs/DESIGN.md) |
| 命令、metadata、Hook 协议和制品格式 | [SPEC](docs/SPEC.md) |
| 开发与验证流程 | 本文 |
| 版本规则、发布步骤与验证要求 | [发布指南](docs/RELEASING.md) |
| 正在执行的方案、验收与待办 | `docs/plans/` 或对应 Issue |
| 值得保留的历史方案、调研和实施记录 | `docs/archive/` |
| 单次测试日志、环境信息与讨论 | 对应 PR、CI 日志或附件 |

活动计划说明任务范围、未完成事项和验收条件。任务结束后，把仍需长期维护的内容移入相应指南或规范；有回顾价值的记录再归档。归档文档须注明记录日期或版本、停止维护及不定义当前行为，并保留指向现行文档的入口。普通完成清单可通过 Git 历史查阅，无需全部归档。

用户文档直接描述当前能力，不重复设计基线、阶段完成状态或某次本地验证结果。发布记录描述用户可见的新增、修复和兼容变化，不复制开发任务清单。

## 发布与维护入口

发布操作见 [发布指南](docs/RELEASING.md)。活动计划和历史记录统一从 [文档索引的维护者入口](docs/README.md#维护者入口) 查阅；归档记录不代表当前验证结果，也不授予后续实施或发布权限。
