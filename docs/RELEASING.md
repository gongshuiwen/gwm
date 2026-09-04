# 发布指南

本文面向维护者，说明版本规则、发布前验证和发布操作。产品与供应链边界见 [DESIGN](DESIGN.md#10-发布自动化)，制品格式见 [SPEC](SPEC.md#13-发布制品)。活动发布任务从 [维护者入口](README.md#维护者入口) 查阅。

## 版本与渠道

- Canonical repository 为 `https://github.com/gongshuiwen/gwm`，Go module path 为 `github.com/gongshuiwen/gwm`。
- 发布渠道为 GitHub Releases，许可证为 MIT。
- 稳定 SemVer tag `vMAJOR.MINOR.PATCH` 是唯一发布版本，源码不预设产品版本号。
- 普通源码构建的 `gwm --version` 输出 `gwm unreleased`；发布构建通过 Go linker 注入 tag，输出必须为 `gwm <release-tag>`。

## 发布前验证

发布前必须完成 [贡献指南](../CONTRIBUTING.md#验证) 中的质量检查，并满足以下要求：

1. 在 Linux 和 macOS 13+ 上验证最低与当前 Git/Go 组合。最低要求为 Git 2.39、Go 1.26；记录实际使用的系统、架构和工具版本。
2. 在对应环境验证五个业务命令、三个 metadata 字段、四个生命周期 Hook、Hook schema 2，以及 help/version 不依赖 repository 的行为。
3. 验证 Linux/macOS 的 amd64/arm64 发布压缩包、版本输出和 SHA-256 校验文件符合 SPEC。
4. 确认活动发布计划中的未完成事项已处理，README 的安装说明与发布状态准确，发布说明描述用户可见的变化。
5. 确定发布版本号和目标 commit，并由维护者完成人工发布决策。

在对应发布 PR 或任务中记录验证证据和未覆盖项。交叉编译成功不代表目标平台运行验证已完成；流水线的 Linux 检查也不替代 macOS 与最低版本组合验证。未完成这些要求时不得创建并 push release tag。

## 发布操作

1. 完成上述验证和人工决策后，在确认的 commit 上创建所选版本的稳定 SemVer tag。
2. 显式向 canonical GitHub repository push 该 tag。Push release tag 是发布授权，会触发自动发布。
3. 查看 [发布流水线](../.github/workflows/release.yml) 的测试、构建和发布结果。
4. 确认 GitHub Release 对应预期 tag，四个平台的压缩包和 `checksums.txt` 齐全，发布二进制的版本与 tag 一致。
5. 核对自动生成的 release notes，补充必要的用户可见变化与兼容说明，完成对应发布任务的记录。

配置流水线本身不创建 tag 或 Release。Branch、pull request 和手动 dispatch 均不提供发布入口。

## 流水线行为

[`.github/workflows/release.yml`](../.github/workflows/release.yml) 监听 `v*.*.*` tag push，并在发布前拒绝不符合稳定 SemVer 的 tag。其执行顺序为：

1. 检出源码，使用 `go.mod` 声明的 Go 版本。
2. 运行 `go test -count=1 ./...`、`go test -race -count=1 ./...` 和 `go vet ./...`。
3. 使用 `CGO_ENABLED=0`、`-trimpath` 和注入 tag 的 linker flag，交叉构建 Linux/macOS 的 amd64/arm64 二进制。
4. 为每个平台生成包含 `gwm`、`README.md` 和 `LICENSE` 的 `tar.gz`。
5. 在 Linux amd64 runner 上验证对应二进制的 `gwm --version` 精确输出 `gwm <release-tag>`；不一致时停止发布。
6. 生成记录四个压缩包 SHA-256 的 `checksums.txt`。
7. 仅在前述步骤全部成功后，使用 `gh release create --verify-tag --generate-notes` 为已存在的 tag 创建 GitHub Release 并上传制品。

Action 来源、固定 SHA、权限与 credential 的约束由 [发布自动化设计](DESIGN.md#10-发布自动化) 定义。流水线检查失败时不应将该次发布记录为成功；应保留日志并在发布任务中处理失败原因。
