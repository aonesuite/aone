# CLI TUI 管理控制台分析

本文用于分析是否需要在 `aone` CLI 中增加一个 TUI 管理模式，例如通过
`aone console` 进入交互式管理控制台。

结论先行：建议预留并逐步实现 TUI，但不要让 TUI 替代现有 CLI。CLI 仍然
应该保持可脚本化、可组合、可自动化；TUI 更适合作为面向人工管理场景的
控制台入口。

## 背景

当前 `aone` 主要面向以下能力：

- 账号和凭据管理：`aone auth`
- Sandbox 生命周期管理：`aone sandbox`
- Template 初始化、构建、发布和日志查看：`aone sandbox template`
- Text To Speech：`aone tts`

这些能力天然存在两种使用方式：

- 命令式使用：用户明确知道要执行什么命令，适合脚本、CI、本地自动化和高级用户。
- 管理式使用：用户需要先观察状态，再选择对象，然后执行动作，适合日常运维和探索。

TUI 的价值主要来自第二种场景。

## 推荐判断

建议增加一个可选的 TUI 子命令：

```sh
aone console
```

也可以考虑以下命名：

| 命令 | 适用感受 | 备注 |
|---|---|---|
| `aone console` | 产品化、自然 | 推荐。表达“进入控制台”而不是强调技术形态。 |
| `aone admin` | 管理属性强 | 适合后续能力偏运维或组织管理时使用。 |
| `aone manage` | 行为中性 | 可读性不错，但命令感略弱。 |
| `aone tui` | 技术直白 | 适合实验期，不太适合长期产品入口。 |
| `aone labs console` | 实验入口 | 适合未稳定前隐藏或低调发布。 |

推荐先使用 `aone console`。如果还处于实验阶段，可以先挂在
`aone labs console`，稳定后迁移到正式入口。

## 为什么 TUI 有价值

### 状态密集

Sandbox、template build、logs、metrics 都不是单次命令能完整表达的场景。
用户往往需要反复执行：

```sh
aone sandbox list
aone sandbox info <sandboxID>
aone sandbox logs <sandboxID>
aone sandbox metrics <sandboxID>
```

TUI 可以把这些状态聚合到一个界面里，减少用户在多个命令之间来回切换。

### 操作依赖上下文

很多操作需要先选择资源，再选择动作：

- 选择 sandbox 后 connect、kill、exec、logs、metrics
- 选择 template 后 get、publish、unpublish、delete，以及钻取到具体 build
- 选择 build 后查看状态和日志（CLI 中对应 `sandbox template logs <templateID> <buildID>`，必须有 buildID）

这类流程用纯 CLI 表达时，用户需要记住 ID、参数和命令层级。TUI 可以通过
列表、详情、快捷键和确认弹窗降低认知负担。

### 人工管理场景更多

CLI 对自动化友好，但人类用户的真实工作流常常是：

1. 我现在有哪些 sandbox？
2. 哪个还在跑？
3. 哪个最近失败？
4. 我能不能直接看日志？
5. 如果没用了，能不能安全删掉？

这些问题更接近“管理控制台”，而不是“一条命令完成一个动作”。

### 危险操作需要保护

例如 kill、delete、unpublish 这类操作，在 TUI 中可以提供更明确的预览和确认：

- 显示资源名称、ID、状态、创建时间
- 显示会影响的对象数量
- 二次确认危险动作
- 对批量操作给出清晰反馈

这比要求用户在命令行中精确输入一长串 ID 更安全。

## 为什么不能过度依赖 TUI

TUI 也会带来额外复杂度：

- 需要维护交互状态、焦点、键盘快捷键、刷新逻辑和错误展示。
- 需要处理终端尺寸、颜色、无 TTY 环境、CI 环境等兼容问题。
- 容易和 CLI 命令实现出两套行为。
- 容易把低频功能全部塞进界面，导致界面变复杂。

因此，TUI 不应该成为第二套业务系统。它应该是 CLI 能力的交互式编排层。

## 产品边界

### TUI 应该做什么

第一阶段建议严格收敛到 sandbox 管理这一个最有价值的场景：

- 查看当前认证状态和 API endpoint。
- 浏览 sandbox 列表（含状态过滤）。
- 查看 sandbox 详情、日志、metrics。
- 对 sandbox 执行 `kill`（带二次确认）和 `refresh`。

`connect` / `exec` 需要让渡终端（见 Phase 3），`template` 运维（含 build 日志、
publish / unpublish / delete）放到 Phase 2，详见"实现策略"。

### TUI 不应该做什么

第一阶段不建议覆盖所有 CLI 能力：

- 不做完整配置编辑器。
- 不做 TTS 的复杂交互界面。
- 不替代所有 `--format json` 输出。
- 不把每个 flag 都映射成表单项。
- 不把脚本化工作流迁移进 TUI。

这能保持 TUI 聚焦，也能避免一开始就背上过大的维护成本。

## 建议的信息架构

TUI 可以按照管理对象组织，而不是按照技术实现组织。以下是**长期目标**信息架构；
**Phase 1 只启用 Overview 和 Sandboxes**，其余在 Phase 2/3 逐步引入。

```text
Overview
Sandboxes
Templates
Builds
Settings
```

不单独列 `Logs` 顶级条目：sandbox logs 和 build logs 都作为对应资源详情下的面板存在；
Overview Phase 2 的"最近失败事件"已经承担了全局视角的入口。等真有 aggregated
events / logs 的需求时再独立成视图。

### Overview

显示全局状态。

Phase 1：

- 当前登录状态
- API endpoint
- sandbox 数量和状态分布

Phase 2 及以后：

- template 数量和最近构建状态
- 最近失败事件

### Sandboxes

核心视图：

- sandbox 列表
- 状态过滤
- 详情面板
- 日志面板
- metrics 面板

常用动作：

- logs
- metrics
- kill
- refresh
- connect、exec（Phase 3：需要让渡终端 —— TUI 必须 suspend alt-screen，
  子进程退出后再 resume；实现复杂度高，不进 MVP / Phase 2）

### Templates

核心视图：

- template 列表
- template 详情
- 已知 buildID 的 build 状态与日志（当前 CLI 是 `template builds <templateID> <buildID>`
  和 `template logs <templateID> <buildID>`，均要求显式 buildID；
  是否支持"按 template 列出全部 build"取决于后端 API 能力，未确认前不承诺该视图）

常用动作（仅覆盖已有 template 的运维动作）：

- publish
- unpublish
- delete
- view build logs（基于已知 buildID；若后端支持 build history，再支持列表钻取）

`template init`、`template create <name>`、`template build` 三者都依赖本地项目目录、
Dockerfile 和 `aone.sandbox.toml`，不适合纯 TUI 表达，应继续走 CLI。TUI 中可以展示
等价命令引导用户切回 CLI 执行。

### Settings

Phase 1 不做独立的 Settings 视图：auth 状态和 endpoint 作为 Overview 的只读子区域
展示即可，避免提前引入导航复杂度。独立 Settings 视图延后到有实际配置编辑需求时
再评估。

未来独立成视图后，只放高频、低风险配置：

- 查看当前配置来源
- 查看 masked API key
- 查看 endpoint
- 跳转到 `aone auth configure`

不建议第一阶段在 TUI 中直接编辑所有配置。

## 与 CLI 的关系

TUI 最好复用现有 CLI 下面的业务层，而不是自己重新实现 API 调用逻辑。

理想分层：

```text
Command Layer
  - cobra / CLI commands
  - TUI entry command

Application Service Layer
  - auth service
  - sandbox service
  - template service
  - tts service

Client Layer
  - API client
  - config loader
  - credential store
```

CLI 和 TUI 都调用同一套 service。这样可以避免：

- CLI 和 TUI 行为不一致
- 错误处理不一致
- 输出字段不一致
- 测试重复

TUI 中还可以展示等价命令，帮助用户学习 CLI：

```sh
aone sandbox logs sbx_xxx --follow
aone sandbox kill sbx_xxx
aone sandbox template publish tpl_xxx
```

这会让 TUI 成为 CLI 的学习入口，而不是 CLI 的替代品。

## MVP 建议

MVP 等价于"实现策略 / Phase 1"：Overview、sandbox 列表 / 详情、logs、metrics、
`kill` 二次确认、`refresh`，加上错误提示和退出快捷键。详细范围见 Phase 1 节。

下面只补充 Phase 1 中**明确暂缓**的项，避免范围蔓延：

- 完整配置编辑。
- 复杂表单。
- 批量操作。
- TTS 交互。
- 自定义快捷键。
- 多主题。
- Template 视图（推到 Phase 2）。
- connect / exec（推到 Phase 3）。

## 是否值得做的判断标准

可以用以下两组标准决定是否进入实现。

**正向信号（命中越多越值得做）：**

| 问题 | 命中含义 |
|---|---|
| 用户是否经常需要连续执行 3 条以上相关命令？ | TUI 能显著减少重复输入。 |
| 用户是否经常需要先看状态再决定动作？ | TUI 适合"列表 → 详情 → 动作"流程。 |
| 是否存在 kill、delete、publish 等危险操作？ | TUI 的确认面板比命令行更安全。 |

**反向信号（命中说明需要先做前置工作或暂缓）：**

| 问题 | 处理方式 |
|---|---|
| 是否需要支持脚本、CI、管道输出？ | CLI 仍是一等接口，TUI 只作补充。 |
| 是否只是为了让工具看起来更高级？ | 暂缓，没有真实使用场景就不要做。 |
| 业务逻辑是否还耦合在 cobra command 里？ | 先按"实现策略 / Phase 0"拆出 query / action 层，再启动 TUI；否则必然出现双实现。 |

## 实现策略

分四个阶段推进，前一阶段是后一阶段的前置条件。

### Phase 0：拆出可复用的 service / query / action 层（前置，必须先做）

当前 CLI 业务函数把 API 调用、stdout/stderr 输出、表格 / JSON 格式化、错误打印
混在一起，无法被 TUI 直接调用。典型反例：

```go
// internal/sandbox/instance/list.go
func List(info ListInfo) {
    client, err := sbClient.NewSandboxClient()
    if err != nil { sbClient.PrintError("%v", err); return }
    // ...
    if info.Format == sbClient.FormatJSON {
        sbClient.PrintJSON(sandboxes); return
    }
    tw := sbClient.NewTable(os.Stdout)
    // 直接写 stdout
}
```

这个函数返回 `void`，没法在 TUI 里用。Phase 0 的目标就是把这类函数拆成三层：

- **query 层**：返回结构化数据，`func ListSandboxes(ctx, params) ([]Sandbox, error)`，
  不打印、不格式化、不直接读写 stdout/stderr。返回类型优先使用 SDK / API 的原始结构，
  必要时可用薄 view model 适配，但**不要返回已格式化的字符串**，也不要为 TUI
  过早发明一套 UI-only 数据模型。
- **action 层**：副作用动作，`func KillSandbox(ctx, id) error`、`func PublishTemplate(ctx, ids) error`，
  返回 error 不打印。
- **renderer / command 层**：负责 cobra flag 解析、table / JSON 输出、错误打印、
  退出码。具体位置不强制 —— cobra command 自然落在 `cmd/`，共享的 table / JSON
  renderer 可以继续放在 `internal/sandbox` 下被 CLI 与 TUI 复用。关键约束：
  **需要被 TUI 复用的业务逻辑必须位于 query / action 层，且不能主动写 stdout/stderr**。

涉及目录至少包括 `internal/sandbox/instance/`、`internal/sandbox/template/`、
`internal/sandbox/`（client、config、credential）。这一阶段不写任何 TUI 代码，
但 CLI 行为必须保持完全等价（可通过 `cmd_test.go` 验证）。

### Phase 1：MVP `aone console`（仅 sandbox）

入口：

```sh
aone console
```

如果在稳定前希望低调发布，可以先挂在：

```sh
aone labs console
```

第一版只包含：

- Overview：登录状态、API endpoint、sandbox 数量与状态分布。
- Sandboxes：列表 + 状态过滤 + 详情面板 + 日志面板 + metrics 面板。
- 动作：`kill`（二次确认）、`refresh`。
- 基础设施：错误提示、退出快捷键、非 TTY 环境检测并降级到提示。

不包含 template，不包含 connect / exec。

### Phase 2：Template 运维视图

Sandbox 视图稳定后再加入：

- Template 列表、详情。
- 已知 buildID 的 build 状态与日志（不承诺 build history 列表，依赖后端能力）。
- 动作：`publish`、`unpublish`、`delete`（均带二次确认）。

`template init` / `create` / `build` 不进 TUI，TUI 中只展示等价 CLI 命令引导用户。

### Phase 3：终端让渡的交互动作

当上面两阶段都稳定后，再考虑 `connect`、`exec`。需要解决：

- TUI suspend alt-screen，把 stdin / stdout / stderr 让给子进程。
- 子进程退出后 resume TUI 并刷新状态。
- 信号处理（Ctrl-C 必须送给子进程，不能误退 TUI）。
- 窗口尺寸变化的级联传递。

这一阶段如果体验做不好，宁可继续让用户用 CLI 执行 connect / exec。

## 风险与控制

| 风险 | 控制方式 |
|---|---|
| TUI 变成第二套业务实现 | 启动 TUI 前必须先完成 Phase 0 的 query / action 层拆分，CLI 与 TUI 共用同一套底层逻辑。 |
| 界面功能膨胀 | 第一版只做 sandbox 管理。 |
| 终端兼容性问题 | 无 TTY 时直接报错并提示使用 CLI。 |
| 自动化能力被弱化 | CLI 输出和 `--format json` 继续作为一等能力。 |
| 维护成本过高 | 控制依赖，避免过早支持主题、插件和复杂表单。 |

## 建议结论

建议做，但要小步做。

更准确地说，不是“为 CLI 增加一个 TUI 使用方式”，而是“为 CLI 增加一个
交互式管理控制台”。这个定位更清晰，也更不容易和已有命令体系冲突。

推荐路线：

- **Phase 0**：把 CLI 业务函数拆成 query / action / renderer 三层，
  让 `cmd/` 之外的代码能返回结构化数据。CLI 行为保持完全等价。
- **Phase 1**：`aone console`，仅 sandbox 列表 / 详情 / logs / metrics / kill / refresh。
- **Phase 2**：扩展到 template 列表 / 详情 / 已知 buildID 的 build 状态与日志，
  以及 publish / unpublish / delete（带确认）。
- **Phase 3**：评估是否要做 connect / exec 的终端让渡，做不好就继续让用户走 CLI。

CLI 始终是核心接口，TUI 不替代 CLI、也不分叉业务逻辑。Phase 0 没完成前不应启动
Phase 1 —— 否则必然出现双实现，把 TUI 变成长期维护负担。
