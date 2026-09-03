# GWM v0.2 行为规范

| 项目 | 内容 |
|---|---|
| 状态 | Implemented，本地发布前规范 |
| 适用版本 | v0.2 |
| 设计基线 | 3.2（薄包装器） |
| 最后更新 | 2026-09-03 |

## 1. 范围

本文是 GWM v0.2 可观察行为的唯一来源，只细化 [DESIGN.md](DESIGN.md) 已确定的产品边界，不负责实施阶段和项目进度。

本文中的“必须”“不得”“只”和“固定”是规范性要求。命令、配置、字段、事件和字面值使用代码格式表示。未在本文定义的行为不属于 v0.2 公共能力。

## 2. 根命令

```text
gwm [-C <repository>] --help
gwm [-C <repository>] --version
gwm [-C <repository>] <command> --help
gwm [-C <repository>] init
gwm [-C <repository>] list
gwm [-C <repository>] add <path>
    [-b <new-branch> | --detach]
    [--from <commit-ish>]
    [--description <text>]
    [--protected]
gwm [-C <repository>] meta <path>
    [--description <text>]
    [--protected <true|false>]
gwm [-C <repository>] remove <path> [--force]
```

规则：

- `-C` 必须位于子命令之前；省略时使用进程启动目录。
- `--help` 和 `--version` 必须分别是 root invocation 中除可选 `-C` 外的唯一参数。
- `<command> --help` 只接受现有五个 command name，并且 `--help` 必须是 command 后的唯一参数。
- Help 输出到 stdout，包括对应 usage；version 固定输出 `gwm v0.2`。两者返回 0，不发现 repository、不运行 Git 或 Hook，`-C` 指向的路径可以不存在。
- 相对 repository path 以进程启动目录为基准；相对 worktree path 以 invocation worktree root 为基准。
- 输入路径和文本必须是有效 UTF-8 且不含 NUL。
- 未知参数、缺少参数和互斥参数返回 usage error。
- v0.2 没有 `--json`、交互式确认或配置文件搜索。

## 3. Repository Context

五个业务命令只支持普通 non-bare repository 的 main 或 linked worktree。`init` 和 `list` 可以在尚未初始化 GWM 的 repository 中运行；`add`、`meta` 和 `remove` 要求 `extensions.worktreeConfig=true`，否则提示先运行 `gwm init`。Help 和 version 不进入 repository context。

GWM 使用系统 Git 发现 invocation root、absolute common directory 和 worktree 清单。后续 Git 调用固定使用该 context，不重新受调用环境中的 `GIT_DIR`、`GIT_WORK_TREE`、`GIT_COMMON_DIR`、`GIT_INDEX_FILE` 或临时 `GIT_CONFIG_*` 注入影响。

所有 Git 命令使用参数数组。读取工作树清单固定使用：

```text
git worktree list --porcelain -z
```

GWM 不直接打开或修改 Git 私有 worktree 管理目录。

## 4. Init

`gwm init` 是幂等操作，只负责启用：

```text
extensions.worktreeConfig=true
```

首次执行前拒绝以下 repository：

- bare repository；
- common config 中存在任意 `core.worktree`；
- common config 中存在 `core.bare=true`；
- extension 尚未启用且 common config 中存在 `core.sparseCheckout` 或 `core.sparseCheckoutCone`。

GWM 不自动迁移这些布局。设置 extension 后必须重读；Git config 返回零且重读为 true 才成功。已经为 true 时返回 success 且不写 config。

Init 不创建 marker、目录、main metadata 或 Hook 文件。

## 5. Metadata

Metadata 位于目标工作树的三个 worktree config 键中：

```text
gwm.worktree.description=修复登录
gwm.worktree.protected=false
gwm.worktree.created-at=2026-09-03T08:30:00Z
```

字段规则：

- `gwm.worktree.description` 缺失或值为空等价于 description 为 null；非空值不得超过 4096 UTF-8 bytes，必须是有效 UTF-8 且不含 NUL。
- `gwm.worktree.protected` 缺失等价于 protected 为 false；存在时必须是 Git 可解析的 boolean。
- `gwm.worktree.created-at` 缺失等价于创建时间未知；存在时必须是精确到秒、以 `Z` 结尾的 UTC RFC 3339 时间。
- 每个键最多有一个值。Description 或 protected 重复、非法时使可编辑 metadata 非法；created-at 重复、非法时只使 created-at 非法。
- 其他 Git config 键不属于 GWM metadata，也不影响读取。
- Add/meta 收到空 description 时删除 `gwm.worktree.description`。
- 只有 `gwm add` 在 Git 成功且重新确认目标已登记后写入 created-at。Main worktree、原生 Git 创建的 linked worktree 和旧工作树不回填；`gwm meta` 不提供 created-at 修改参数。

Description 或 protected 非法时，`list` 将 description 和 protected 都显示为 INVALID；`meta` 修改和 `remove` 拒绝，以免覆盖未知状态。Created-at 非法时仅将 created-at 显示为 INVALID，不阻止 `meta` 或 `remove`。用户可以显式运行以下原生命令清理：

```text
git -C <worktree> config --worktree --unset-all gwm.worktree.description
git -C <worktree> config --worktree --unset-all gwm.worktree.protected
git -C <worktree> config --worktree --unset-all gwm.worktree.created-at
```

`gwm meta` 的写入顺序固定为 description、protected：description 为 null 时删除对应键，否则使用一次 `--replace-all`；protected 使用一次 `--replace-all`。`gwm add` 在两者成功后，以一次 `--replace-all` 写入 created-at。三个键不是一个事务，后一步失败不回滚已完成的前一步，错误必须说明 metadata 可能部分更新。每组写入后重读，只有 Git config 写入步骤成功且重读等于 intended value 才成功；删除一个原本缺失的 description 键视为成功的 no-op。v0.2 不提供 CAS 或 metadata lock。

## 6. List

`gwm list` 读取原生 worktree 清单，并在 repository 已启用 worktree config 时读取各目标 metadata。

Human-readable 输出固定为：

```text
PATH  BRANCH  DESCRIPTION  PROTECTED  CREATED_AT
```

- 保持原生 porcelain 顺序。
- Detached、bare 或无法读取 branch 时显示 `-`。
- Metadata 键缺失时 description 显示 `-`，protected 显示 `false`，created-at 显示 `-`。
- Description 或 protected 非法时 description 和 protected 都显示 `INVALID`；created-at 非法时只有 created-at 显示 `INVALID`。
- 路径和 description 中的换行、回车、制表符及不可打印字符必须转义，保证每个工作树占一行。

List 不计算 dirty、ignored、submodule、upstream 或 ahead/behind，也不修改 repository。

## 7. Add

Add mode 与原生 Git 的映射固定为：

| GWM 参数 | Git 调用 |
|---|---|
| 无 mode | `git worktree add <path> [<from>]` |
| `-b <name>` | `git worktree add -b <name> <path> [<from>]` |
| `--detach` | `git worktree add --detach <path> [<from>]` |

`-b` 与 `--detach` 互斥；`--from` 省略时不向 Git 注入 start point，保留原生推断行为。v0.2 不包装其他 add option；需要其他原生能力时，用户先运行 `git worktree add`，再运行 `gwm meta`。

执行顺序：

1. 要求 repository 已 init，规范化目标 path，验证目标尚不存在。
2. 构造 requested metadata；description 默认 null，protected 默认 false。
3. 运行 `pre-add`；非零时停止，不调用 Git。
4. 调用一次上述 `git worktree add`。
5. 无论 Git 退出码如何，重新读取 worktree 清单。
6. Git 非零或目标未登记时停止，不写 metadata、不运行 `post-add`，并展示 Git 当前状态。
7. Git 成功且目标已登记时捕获当前 UTC 时间，依次写入 description、protected 和 created-at。
8. 无论 metadata 写入是否成功，都运行 `post-add`；payload 使用重读后可观察到的 metadata，无法读取时为 null。
9. Metadata 或 post-hook 失败时返回 partial；不删除 path 或 branch。

## 8. Meta

`gwm meta <path>` 只接受当前登记且路径可读的 worktree。

- 不提供修改 flag 时显示当前 metadata；字段键缺失时显示默认值，created-at 只读显示。
- `--description ""` 把 description 写为 null。
- `--protected true|false` 显式设置保护状态。
- 两个 flag 都可以同时提供；未提供的字段保持原值。
- Intended value 与当前值相同返回 success，不执行写入。

Meta 不触发生命周期 Hook，也不写 created-at。每个可编辑字段采用最后写入者获胜；同时修改两个字段可能部分完成或与其他进程交错。命令只保证全部写入返回成功时，自己随后重读到 intended value。非法 created-at 不影响上述行为。

## 9. Remove

`gwm remove <path>` 只接受当前登记的 linked worktree。

执行顺序：

1. 读取 metadata；非法 description 或 protected 时拒绝，非法 created-at 不阻止删除。
2. `protected=true` 时拒绝，用户必须先运行 `gwm meta <path> --protected false`。
3. 运行 `pre-remove`；非零时停止，不调用 Git。
4. 调用一次 `git worktree remove <path>`；提供 `--force` 时调用一次 `git worktree remove --force <path>`。
5. 无论 Git 退出码如何，重新读取 worktree 清单。
6. Git 非零或目标仍登记时停止，不运行 `post-remove`，并展示 Git 当前状态。
7. Git 成功且目标不再登记时运行 `post-remove`；payload 使用删除前捕获的 metadata。
8. Post-hook 失败时返回 partial；不恢复工作树。

Main worktree、bare worktree 和 locked worktree 不由 GWM remove。Dirty、untracked、ignored 和 submodule 行为交给原生 Git；`--force` 只原样表达用户要求 Git 强制删除。GWM 不递归删除目录，也不删除 branch。

## 10. Hook 配置

Hook key 固定为：

```text
gwm.hooks.pre-add
gwm.hooks.post-add
gwm.hooks.pre-remove
gwm.hooks.post-remove
```

配置要求：

- 只读取 common repository 的 local Git config。
- 每个 key 只能有一个值；重复值使对应操作拒绝执行。
- 值是一个 non-empty path。绝对路径原样使用；相对路径以 main worktree 根目录解析。
- 解析后的路径必须指向可执行的普通文件。
- Executable 可以是 tracked 文件，但必须由用户在当前 clone 的 local config 中显式启用；文件存在不等于授权执行。
- 直接执行该文件且不附加参数，不经过 shell。
- Hook path 的相对解析基准固定为 main worktree；Hook cwd 固定为 invocation worktree root。两者互不影响。
- Hook stdin 是一份 JSON payload；stdout/stderr 直接继承调用终端。
- 未配置对应 key 时跳过该 Hook。
- v0.2 没有 timeout、retry、并行或 Hook 链。

Hook payload 固定为：

```json
{
  "schema_version": 2,
  "event": "pre-add",
  "common_dir": "/workspace/repo/.git",
  "invocation_root": "/workspace/repo",
  "worktree_path": "/workspace/repo-fix",
  "head": null,
  "branch": null,
  "metadata": {
    "description": "修复登录",
    "protected": false,
    "created_at": null
  },
  "options": {
    "new_branch": "fix/login",
    "from": "main",
    "detach": false,
    "force": false
  }
}
```

- `event`：`pre-add`、`post-add`、`pre-remove` 或 `post-remove`。
- `head` 是完整 object ID；目标尚不存在或无法读取时为 null。
- `branch` 是完整 `refs/heads/...`；detached、目标尚不存在或无法读取时为 null。
- `metadata` 无法读取或 description/protected 非法时为 null。`pre-add` 的 `created_at` 为 null；其他事件使用可观察值，created-at 缺失或非法时该字段为 null。
- `options` 始终包含四个字段；不适用的 string 为 null，boolean 为 false。

Pre-hook 返回 0 才继续。Post-hook 返回非零不会改变已经完成的 Git 操作。

## 11. 输出与退出码

GWM 只提供简洁 human output。Git 和 Hook stderr 可以直接显示，但不得输出完整环境、credential、token 或 private key。

| 退出码 | 含义 |
|---|---|
| `0` | 操作成功，或幂等地无需修改 |
| `1` | usage、repository、pre-hook、Git 或 metadata 操作失败，且没有已确认的 Git 成功需要报告 |
| `2` | Git add/remove 已成功，但 metadata 或 post-hook 失败 |

Partial 输出必须明确指出 Git 操作已经完成，避免用户误以为可以安全重试整个命令。

## 12. 兼容边界

- `git clone` 不执行 GWM Hook，clone 也不继承源仓库的 local Hook 配置。
- 原生 Git 操作不会触发 GWM Hook。
- GWM 不包装 move、lock、unlock 或 prune；这些操作后 `gwm list` 直接展示 Git 新状态。
- 原生 remove 可能随工作树删除 metadata；GWM 不保存历史副本。
- GWM 不执行 fetch、push、clone 或远程探测。
- Hook 和系统 Git 可能执行用户本地配置的外部程序；这是明确的本地信任边界。
