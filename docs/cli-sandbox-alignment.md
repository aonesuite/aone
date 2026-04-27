# aone CLI Sandbox 与 e2b CLI 对齐调研

> 更新时间：2026-04-27
>
> 对照基线：
> `aone`: `/Users/miclle/github/aonesuite/aone`
> `e2b`: `/Users/miclle/github/e2b-dev/e2b/packages/cli`

## 1. 结论先看

当前 `aone cli` 实际只暴露 `sandbox` 这一组能力，因此这份文档应当聚焦：

- `aone sandbox`
- `aone sandbox template`
- `aone sandbox volume`

这份文档只聚焦 `sandbox` 相关能力，不再混入 `auth`、`infra/specs`、未来顶层命令规划等内容。

整体结论：

- `aone sandbox` 主命令面已经覆盖了 `e2b sandbox` 的核心子命令：`list/create/connect/info/kill/pause/resume/exec/logs/metrics`。
- `aone sandbox template` 也已经覆盖了 `e2b template` 的主体能力，并额外保留了 `get`、`builds` 这类 aone 特有能力。
- 主要差异不在“有没有这个命令”，而在参数细节、默认值、交互语义，以及 `template create/init/migrate/build` 的行为设计。
- `aone sandbox volume` 在 `e2b cli` 中没有对应模块，应视为 aone 扩展能力，保留即可，不应为了“对齐”而削弱。

本文中的结论标签统一只有两类：

- `已对齐`：与 `e2b cli` 的命令、参数或行为一致，或没有影响使用方式的实质差异。
- `有差异`：存在任何命令面、参数名、默认值、回退策略、交互流程或输出语义差异；差异点必须明确写出。

## 2. 当前基线澄清

有几条常见判断需要先澄清：

- `aone --version` 缺失：不成立。当前 [cmd/root.go](/Users/miclle/github/aonesuite/aone/cmd/root.go) 已通过 `cobra.Version` 和 `debug.ReadBuildInfo()` 提供版本输出。
- `aone --debug` 缺失：不成立。当前根命令已提供全局 `--debug`，并写入 `AONE_DEBUG`。
- `aone auth` 属于本轮 CLI 对齐范围：不适合。当前产品面只有 `sandbox` 模块，这份文档不应再把 `auth` 作为主范围。
- `template migrate` 还没有 `--config`：不成立。当前 `aone sandbox template migrate` 已支持 `--config`，并会读取项目配置中的 `dockerfile` 与 `template_name` 作为迁移输入默认值。
- `template create` 仍强制要求显式 `-d/--dockerfile`：不成立。当前 `aone` 已支持默认发现 `aone.Dockerfile` / `Dockerfile`，也会按项目根解析相对 Dockerfile 路径。

## 3. 范围定义

### 3.1 纳入本次对齐

- `aone sandbox`
- `aone sandbox template`
- `aone sandbox volume`
- 与上述命令直接相关的项目配置文件：`aone.sandbox.toml`

### 3.2 明确不纳入本次主文档

- `e2b auth` 系列命令
- 团队维度 `--team`
- `infra/specs` 目录重命名与同步脚本

说明：

- `e2b` 的 `auth`、`team`、用户配置体系会影响 CLI 风格，但不属于当前 `aone sandbox` 产品面的直接对齐对象。
- 可以在后续单独起一份 `cli-auth-alignment.md`，避免污染这份 sandbox 专项文档。

## 4. 顶层命令对齐

| 项 | e2b | aone 现状 | 结论 |
|---|---|---|---|
| 顶层 sandbox 入口 | `sandbox` / alias `sbx` | `sandbox` / alias `sbx` | 已对齐 |
| `--version` | 有 | 有 | 已对齐 |
| `--debug` | e2b 以配置/API 层为主，无完全同名全局 flag 依赖 | `aone --debug` 已存在 | aone 已具备 |
| 顶层 `auth` | 有 | 当前未提供 | 当前不纳入 sandbox 对齐 |

## 5. `aone sandbox` 与 `e2b sandbox` 对照

### 5.1 子命令与参数总表

| 命令 | e2b | aone | 结论 |
|---|---|---|---|
| `sandbox list` | `ls`; `--state`; `--metadata`; `--limit`; `--format`; 默认只看 `running` | `ls`; `--state`; `--metadata`; `--limit`; `--format`; 默认也只看 `running`; `--limit 0` 使用服务端默认值 | 有差异：`e2b` 默认 `limit=1000` 且 `0` 表示不限；`aone` 将 `0` 明确设计为使用服务端默认值 |
| `sandbox create` | `cr`; `[template]`; `--detach`; `--path`; `--config`; 未传 template 时回退 `config.template_id`，否则 `base` | `cr`; `[template]`; `--detach`; `--path`; `--config`; 另有 `--timeout` `--metadata` `--env-var` `--auto-pause`; 未传 template 时回退 `config.template_id`，否则 `base` | 有差异：主回退逻辑已对齐，但 aone 仍保留自有扩展参数 |
| `sandbox connect` | `cn`; `<sandboxID>`；连接现有 PTY，不关闭 sandbox | `cn`; `<sandboxID>`；连接现有 PTY，不关闭 sandbox | 已对齐 |
| `sandbox info` | `in`; `<sandboxID>`; `--format` | `in`; `<sandboxID>`; `--format` | 已对齐 |
| `sandbox kill` | `kl`; `[sandboxIDs...]`; `--all`; `--state`; `--metadata` | `kl`; `[sandboxIDs...]`; `--all`; `--state`; `--metadata` | 已对齐 |
| `sandbox pause` | `ps`; `<sandboxID>` | `ps`; `[sandboxIDs...]`; `--all`; `--state`; `--metadata` | 有差异：aone 支持批量与筛选参数 |
| `sandbox resume` | `rs`; `<sandboxID>` | `rs`; `[sandboxIDs...]`; `--all`; `--metadata` | 有差异：aone 支持批量与筛选参数 |
| `sandbox exec` | `ex`; `<sandboxID> <command...>`; `--background`; `--cwd`; `--user`; `--env`; 支持 stdin/EOF/退出码/信号转发 | `ex`; `<sandboxID> -- <command...>`，也兼容不写 `--`; `--background`; `--cwd`; `--user`; `--env`; 支持 stdin/EOF/退出码/信号转发 | 已对齐 |
| `sandbox logs` | `lg`; `<sandboxID>`; `--level`; `--follow`; `--format`; `--loggers`; 400ms 轮询 | `lg`; `<sandboxID>`; `--level`; `--follow`; `--format`; `--loggers`; 另有 `--limit`; 400ms 轮询 | 有差异：aone 额外提供 `--limit` |
| `sandbox metrics` | `mt`; `<sandboxID>`; `--follow`; `--format` | `mt`; `<sandboxID>`; `--follow`; `--format` | 已对齐 |
| `sandbox spawn` | 隐藏兼容命令 `sp` | 无 | 有差异：aone 未提供该隐藏兼容命令 |

差异补充：

- `sandbox connect` 当前实现位于 [internal/sandbox/instance/connect.go](/Users/miclle/github/aonesuite/aone/internal/sandbox/instance/connect.go)，终端连接方向与 e2b 一致。
- `sandbox exec` 已补齐 shell-safe quoting，命令参数中包含空格、引号、特殊字符时的行为已与 e2b 收敛。
- `sandbox create` 已补齐回退到 `base` 的主路径逻辑，当前剩余差异主要是 aone 额外暴露的扩展参数。

## 6. `aone sandbox template` 与 `e2b template` 对照

### 6.1 子命令与参数总表

| 命令 | e2b | aone | 结论 |
|---|---|---|---|
| `template list` | `ls`; `--format`; `--team` | `ls`; `--format` | 有差异：aone 未提供 `--team` |
| `template create` | `ct`; `<template-name>`; `--path`; `--dockerfile` 可缺省自动发现; `--cmd`; `--ready-cmd`; `--cpu-count`; `--memory-mb`; `--no-cache` | `ct`; `<template-name>`; `--path`; `--config`; `--dockerfile` 可缺省自动发现; `--cmd`; `--ready-cmd`; `--cpu-count`; `--memory-mb`; `--no-cache` | 有差异：Dockerfile 自动发现主路径已对齐，aone 仍额外提供 `--config` |
| `template build`（隐藏命令） | 隐藏命令; 旧版 Docker build/push 工作流; `[template]`; `-n/--name`; `--dockerfile`; `--cmd`; `--ready-cmd`; `--cpu-count`; `--memory-mb`; `--build-arg`; `--no-cache`; `--config`; `--path` | 隐藏命令; 内部低层构建入口; 无 `[template]`; `--name`; `--template-id`; `--from-image`; `--from-template`; `--dockerfile`; `--start-cmd`; `--ready-cmd`; `--cpu`; `--memory`; 无 `--build-arg`; `--no-cache`; `--config`; `--path`; `--wait` | 有差异：定位、参数名、参数集合、等待策略都不同 |
| `template delete` | `dl`; `[template]`; `--path`; `--config`; `--select`; `--yes`; `--team`; 默认可从配置读取 template id；删除后会删 `e2b.toml` | `dl`; `[templateIDs...]`; `--path`; `--config`; `--select`; `--yes`; 默认可从配置读取 template id；不删 `aone.sandbox.toml` | 有差异：aone 未提供 `--team`，支持多 ID，且不会删除本地配置 |
| `template publish` / `unpublish` | `pb` / `upb`; `[template]`; `--path`; `--config`; `--select`; `--yes`; `--team`; 默认可从配置读取 template id | `pb` / `upb`; `[template]`; `--path`; `--config`; `--select`; `--yes`; 默认可从配置读取 template id | 有差异：aone 未提供 `--team` |
| `template init` | `it`; `--path`; `-n/--name`; `-l/--language`; 语言为 `typescript` / `python-sync` / `python-async`; 输出目录是 `root/<name>/...` | `it`; `--path`; `-n/--name`; `-l/--language`; 语言为 `go` / `typescript` / `python-sync` / `python-async`；`python` 归一到 `python-sync`；输出目录是 `root/<name>/...` | 有差异：主路径已对齐，但 aone 仍额外保留 `go` 语言 |
| `template migrate` | `--dockerfile`; `--config`; `--path`; `--language`; 输入依赖 Dockerfile + `e2b.toml`; 输出成套 SDK 文件并把旧文件改名 `.old` | `--dockerfile`; `--config`; `--path`; `--language`; `--name`; 会读取 `aone.sandbox.toml` 中的 `dockerfile` 与 `template_name`；仍只生成单个 `template.go/ts/py` 文件 | 有差异：配置输入主路径已对齐，但 aone 仍额外提供 `--name`，且输出范围不同 |
| `template get` | 无 | `gt`; `<templateID>` | 有差异：aone 额外提供该子命令 |
| `template builds` | 无 | `bds`; `<templateID> <buildID>` | 有差异：aone 额外提供该子命令 |

差异补充：

- `template create` 已支持按项目根自动发现 `aone.Dockerfile` / `Dockerfile`，并会按 `--path` 解析配置中的相对 Dockerfile 路径；当前剩余差异主要是 `aone` 额外提供 `--config`。
- `template build` 已经是隐藏命令，因此不一定需要逐项追平，但如果目标是“完全对齐 e2b”，这里仍然需要记为真实差异。
- `template init` 已补齐 `-n`、`--path` 父目录语义，以及 `python-sync` / `python-async` 的目录结构、README、build 入口；剩余主要差异是保留 `go` 语言这一 aone 扩展。
- `template migrate` 已补齐 `--config`，并会读取 `aone.sandbox.toml` 中的 `dockerfile` 与 `template_name`；当前剩余差异主要是输出范围仍只覆盖单个模板源码文件。

## 7. `aone sandbox volume`

`e2b cli` 没有对应的 volume 模块。当前 `aone` 已提供：

- `list`
- `create`
- `info`
- `delete`
- `ls`
- `cat`
- `cp`
- `rm`
- `mkdir`

结论：

- `volume` 不属于“与 e2b 对齐的缺口”，而是 `aone sandbox` 的差异化能力。
- 在这份文档里只需要注明“无 e2b 对照项，完整保留”，不应把它写成待裁剪对象。

## 8. 当前最值得跟进的对齐项

按优先级排序，我建议先做这些：

1. `template migrate`
   配置输入主路径已经对齐，当前主要还需要扩展输出范围，不再只生成单个模板源码文件。

2. `sandbox spawn`
   这是低优先级兼容项。如果目标是命令面完全对齐，可以在最后补成隐藏兼容命令；它不应阻塞主路径命令的对齐。

3. `template init`
   主路径参数和目录结构已经基本对齐，剩余需要决定的是是否继续保留 `go` 作为 aone 的有意扩展，以及是否要把 README / 生成内容进一步做到和 e2b 更接近。

## 9. `sandbox spawn` 与 `template migrate`

### 9.1 `sandbox spawn`

用途：

- `e2b` 中的 `sandbox spawn` 本质上是 `sandbox create` 的旧兼容入口。
- 它是隐藏命令，主要服务于历史脚本、旧习惯或兼容性场景，不是面向普通用户的主路径命令。

建议：

- 建议最终对齐，但优先级低。
- 如果目标是和 `e2b cli` 做完整命令面对齐，最后可以补一个隐藏兼容命令 `spawn`。
- 在 `template migrate` 这类关键主路径还没有对齐前，不建议优先投入它。
### 9.2 `template migrate`

用途：

- `template migrate` 用于把旧的 Dockerfile 模板工程迁移到 SDK 模板定义工作流。
- 典型场景包括从 Dockerfile + 配置文件工作流迁到 SDK 工作流。
- 典型场景包括老模板项目升级。
- 典型场景包括批量把历史模板工程转换成新的模板代码结构。

建议：

- 建议对齐，但排在 `sandbox create`、`sandbox exec`、`template init`、`template create` 这些已完成主路径之后。
- 它不是日常最高频命令，但代表模板迁移能力是否完整。
- 当前 `aone` 与 `e2b` 的主要差异已经收敛到迁移输出范围；如果目标是“完全对齐 e2b”，这里最终还需要补齐成套 SDK 文件生成和旧文件改名策略。

## 10. 当前明确存在的有意差异

这些我不建议为了对齐而硬改：

- 不实现 `--team`
  当前 `aone` 已明确不引入 team 维度，这应是产品边界，不是技术欠账。

- `pause/resume` 的批量能力
  这是 aone 的增强，不必退回到 e2b 的单 ID 形式。

- `sandbox list --limit 0`
  当前 `0` 明确表示使用服务端默认值，而不是 unlimited。全量查询如果未来需要，建议单独增加显式 `--all`。

- `template init` 的 `go` 语言
  当前可以视为 aone 的有意扩展；如果继续保留，应在文档和 help 中明确它不是 e2b 的同构语言集合。

- `volume` 模块
  属于 aone 特性，应该保留。

- `template build` 的内部化定位
  既然 `create` 是主入口，`build` 保持隐藏命令没问题。

## 11. 建议的后续文档拆分

当前我建议保留这份文件名：

- `docs/cli-sandbox-alignment.md`

如果后续还要继续做完整 CLI 规划，再新增：

- `docs/cli-auth-alignment.md`
- `docs/cli-top-level-roadmap.md`

这样会比继续把所有模块塞进一份“总计划”里更清楚。
