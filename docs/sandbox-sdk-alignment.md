# Sandbox SDK 三方对齐分析报告

> 对比对象：
> - **aone Go SDK** — `aonesuite/aone/packages/go/sandbox/`
> - **七牛 Go SDK** — `miclle/qiniu-go-sdk/sandbox/`
> - **E2B SDK（标杆）** — `e2b-dev/e2b/packages/js-sdk/`（并交叉验证 `python-sdk/`）
>
> 本报告基于 commit 时点的源码静态分析，不依赖运行行为。所有源代码事实均可在引用的文件路径处直接核对。

---

## 1. 概览

| 维度 | aone Go SDK | 七牛 Go SDK | E2B SDK |
|------|-------------|-------------|---------|
| 包路径 | `packages/go/sandbox/` | `sandbox/` | `packages/js-sdk/src/`（含 `python-sdk`） |
| 代码量（不含测试） | ~5970 行（24 个 .go 文件 + `dockerfile/` 子包 940 行） | ~3460 行（13 个 .go 文件，不含 `internal/`） | TS 主仓 ~6000 行 src + 自动生成 schema |
| 默认 Endpoint | `https://sandbox.aonesuite.com` | `https://cn-yangzhou-1-sandbox.qiniuapi.com` | E2B SaaS（`e2b.app`/`e2b.dev`） |
| envd 端口 | `49983` | `49983` | `49983` |
| MCP 端口 | `50005` | — | `50005` |
| 默认 user | `user` | `user` | `user` |
| RPC 协议 | ConnectRPC + protobuf | ConnectRPC + protobuf | ConnectRPC（connect-web） + protobuf |

三个 SDK 共享同一份 envd 协议（process / filesystem proto）和同一份控制面 OpenAPI 派生物，因此**最底层的传输与协议是一致的**；分歧主要发生在 SDK 表层 API、辅助子模块与错误类型。

aone Go SDK 已经把 E2B 作为对齐基准（`doc.go`、最近多次 commit message 中的 "align with E2B" 措辞、同名的 sentinel error 列表、TemplateBuilder/ReadyCmd 全套）；七牛 Go SDK 体量更小、定位偏于"控制面 + envd 直连"的最小可用面。

---

## 2. 顶层结构与子模块

### 2.1 文件组织

| 子模块 | aone | 七牛 | E2B JS |
|--------|------|------|--------|
| 入口/客户端 | `client.go`, `sandbox.go` | `client.go`, `sandbox.go` | `sandbox/index.ts`, `sandbox/sandboxApi.ts` |
| 类型 | `types.go`, `types_template.go` | `types.go`, `types_template.go`, `types_injection_rule.go` | 内联 + `template/types.ts` + `volume/types.ts` |
| 进程命令 | `commands.go` | `commands.go` | `sandbox/commands/` |
| 伪终端 | `pty.go` | `pty.go` | `sandbox/commands/pty.ts` |
| 文件系统 | `filesystem.go` | `filesystem.go` | `sandbox/filesystem/` |
| Git | `git.go` ✅ | — ❌ | `sandbox/git/` ✅ |
| Volume | `volume.go` ✅（包级） | — ❌ | `volume/` ✅ |
| Snapshot | `snapshot.go` ✅ | — ❌ | 内嵌在 `sandboxApi.ts` ✅ |
| 模板 CRUD | `template.go` | `template.go` | `template/index.ts`（静态方法） |
| 模板 DSL | `template_builder.go` ✅ | — ❌ | `template/index.ts`（fluent class） ✅ |
| Dockerfile 解析 | `dockerfile_convert.go` + `dockerfile/` 子包 ✅ | — ❌ | `template/dockerfileParser.ts` ✅ |
| ReadyCmd 助手 | `template_builder.go: WaitForPort/URL/Process/File/Timeout` ✅ | — ❌ | `template/readycmd.ts` ✅ |
| 错误 | `errors.go`（含 sentinel） ✅ | `errors.go`（仅 APIError） ⚠️ | `errors.ts`（typed classes） ✅ |
| 轮询 | `poll.go` | `poll.go` | 内嵌于 `buildApi.ts`、`sandboxApi.ts` |
| 分页器 | `paginator.go` ✅ | — ❌ | `sandboxApi.ts: SandboxPaginator/SnapshotPaginator` ✅ |
| Multipart | `multipart.go` | `multipart.go` | 浏览器 fetch 内置 |
| MCP 注入 | `sandbox.go: GetMCPURL/GetMCPToken` ✅ | — ❌ | `sandbox/mcp.d.ts` + `index.ts` ✅ |
| 注入规则（HTTPS 代理） | — | `injection_rule.go` + `types_injection_rule.go` ✅ | — |

**结论**：aone 与 E2B 在子模块数量上**几乎一一对应**；七牛 SDK 在"工程化辅助"维度上大量缺失（Git、Volume、Snapshot、TemplateBuilder、Dockerfile、Paginator、MCP），但**独家保留了 InjectionRule 模块**（出站 HTTPS 注入规则），这是 E2B 与 aone 都没有的 Qiniu-specific 能力。

### 2.2 子模块懒初始化

aone 与七牛在 `Sandbox` 结构体上使用相同的 `sync.Once + xxxOnce`/`xxx` 字段对：`filesOnce/files`、`commandsOnce/commands`、`ptyOnce/pty`、`processRPCOnce/processRPC`，进程 RPC 在 Commands 和 Pty 之间共享。E2B JS 在构造函数里直接 `new Filesystem/Commands/Pty/Git`，因为 JS 没有 Once 习惯，但语义一致。

aone 多出 `mcpToken *string` 字段并从 `/etc/mcp-gateway/.token` 懒加载。

---

## 3. 客户端配置（Config）

| 字段 | aone | 七牛 | E2B |
|------|------|------|------|
| `APIKey` | ✅ | ✅ | ✅（`apiKey`） |
| `Endpoint` | ✅ + `EnvEndpoint=AONE_SANDBOX_API_URL` | ✅（仅字段） | ✅ + `E2B_DOMAIN` |
| 环境变量回退 | ✅ `AONE_API_KEY` / `AONE_SANDBOX_API_URL` / `AONE_DEBUG` | ❌ 全部需显式赋值 | ✅ `E2B_API_KEY` / `E2B_DOMAIN` / `E2B_DEBUG` |
| `HTTPClient` | ✅ | ✅ | ✅（fetch 注入） |
| `RequestTimeout` | ✅（控制面专用，envd 流不受影响） | ❌ | ✅（`requestTimeoutMs`） |
| `Debug` | ✅ | ❌ | ✅ |
| `Credentials` | 仅占位（`any`，"intentionally not implemented"） | ✅ `auth.Credentials`（用于 InjectionRule 的 Qiniu V2 签名） | — |
| 双 header 兼容（`X-API-Key` + `Authorization: Bearer`） | ✅（`apiKeyEditor` 同时设置两个头，调用方预设 `Authorization` 时让位） | ✅（与 E2B 行为一致） | ✅ |

**对齐缺口**
- aone 已经做了 env-var fallback 与 `RequestTimeout` 注入器；七牛**完全没有 env-var 路径**，CLI/容器化场景需要包装一层。
- aone 的 `apiKeyEditor` 已对齐 E2B/七牛行为，**同时设置 `X-API-Key` 和 `Authorization: Bearer <APIKey>`**，覆盖只接受其一的旧网关；调用方若已预设 `Authorization`（例如 Qiniu V2 签名），则两个头都不写入，让位给调用方。见 `client.go:130-145`。
- aone 的 `Config.Credentials` 字段写成 `any` 并保留注释 *"Injection-rule APIs are intentionally not implemented in this package"*，等于明确把七牛的 InjectionRule 走向标记为 out-of-scope。

---

## 4. Sandbox 生命周期

`Client.Create / Connect / List / CreateAndWait / GetSandboxesMetrics`：三方语义对齐良好，差异点如下。

| 能力 | aone | 七牛 | E2B JS |
|------|------|------|--------|
| `Create` 默认模板 | `base`，启用 MCP 时切换到 `mcp-gateway` | `base`，需显式给 `TemplateID` | 同 aone |
| `CreateParams.Lifecycle.OnTimeout` (kill/pause) | ✅ | ❌ 仅有 `AutoPause *bool` | ✅ `lifecycle.onTimeout` |
| `CreateParams.Lifecycle.AutoResume` | ✅ | ❌ | ✅ |
| `CreateParams.VolumeMounts` | ✅ | ❌ | ✅ |
| `CreateParams.MCP` | ✅（`MCPConfig map[string]any`，调用 `mcp-gateway --config ...`） | ❌ | ✅（`mcp` 选项 + `mcp-gateway`） |
| `CreateParams.Injections` | ❌ | ✅（`SandboxInjectionSpec` 列表，可 by-ID 或 inline） | ❌ |
| `Sandbox.Pause` / `Refresh` / `SetTimeout` | ✅ | ✅ | ✅ |
| `Sandbox.Kill` | ✅ | ✅ | ✅ |
| `Sandbox.GetInfo` / `GetMetrics` / `GetLogs` | ✅ | ✅ | ✅ |
| `Sandbox.IsRunning`（`/health` 探测） | ✅ | ✅ | ✅ |
| `Sandbox.WaitForReady` (轮询) | ✅ | ✅ | 通过 `await create()` 隐式完成 |
| `SandboxInfo` 字段 | 包含 `Lifecycle / Network / VolumeMounts / AllowInternetAccess` | 仅基础字段（缺 `Lifecycle / Network / VolumeMounts`） | 全 |
| `ListPage` 返回 `nextToken` | ✅ | ❌（只有 `List`，没有暴露 next token） | ✅ Paginator |
| `SandboxPaginator` | ✅ | ❌ | ✅ |

**MCP 流程一致性**：aone `sandbox.go:107-153` 和 E2B `sandbox/index.ts:294-318` 都生成 `crypto.randomUUID` 风格的 token、通过 `/bin/bash` 启动 `mcp-gateway --config ...`、把 token 注入到 `GATEWAY_ACCESS_TOKEN` 环境变量；aone 的 token 通过 `crypto/rand` 16 字节十六进制（实际上是 32 hex 字符），与 E2B 的 UUID v4 格式略有差异但语义等价。

**Snapshot**：aone（`snapshot.go`）与 E2B 同名同形（`CreateSnapshot/ListSnapshots/DeleteSnapshot/SnapshotPaginator`）；七牛**完全没有快照能力**。

---

## 5. Commands（命令执行）

三方都通过 envd 的 `process.proto`（`Start/Connect/List/SendInput/CloseStdin/SendSignal/Update`）构建。

| 能力 | aone | 七牛 | E2B |
|------|------|------|------|
| `Run` 同步执行 | ✅ | ✅ | ✅（`commands.run`） |
| `Start` 异步返回 handle | ✅ | ✅ | ✅ |
| `CommandHandle.Wait` | ✅ | ✅ | ✅ |
| `CommandHandle.WaitOK`（非零退出抛 `CommandExitError`） | ✅ | ❌ | ✅ |
| `CommandHandle.Disconnect`（停接收不杀进程） | ✅ | ❌ | ✅ |
| `CommandHandle.Stdout()/Stderr()/ExitCode()/ErrorMessage()`（中途读取） | ✅ | ❌ | ✅ |
| `CommandHandle.WaitPID` | ✅ | ✅ | （E2B 同步返回 PID） |
| `Connect`（连接已有进程） | ✅ | ✅ | ✅ |
| `List` | ✅ | ✅ | ✅ |
| `SendStdin` | ✅ | ✅ | ✅ |
| `CloseStdin` | ✅ | ✅ | ✅ |
| `Kill`（SIGKILL） | ✅ | ✅ | ✅ |
| 选项：`WithEnvs/WithCwd/WithCommandUser/WithTag/WithTimeout/WithStdin/WithOnStdout/WithOnStderr/WithOnPtyData` | ✅ 全 | ✅ 全 | ✅ 全 |
| 启动 shell | `/bin/bash -l -c <cmd>` | `/bin/bash -l -c <cmd>` | `/bin/bash -l -c <cmd>`（envd 行为） |
| keepalive header | ✅ `Keepalive-Ping-Interval: 50` | ✅ 同 | ✅ 同 |

**关键差异**：aone 的 `CommandHandle` 暴露了`WaitOK / Disconnect / Stdout() / Stderr() / ExitCode() / ErrorMessage()` 这套与 E2B `CommandExitError` 对齐的 API，七牛只保留了 `Wait / WaitPID / Kill / PID`，行为更接近"最小同步等待"。

---

## 6. Pty（伪终端）

完全对齐：三方都提供 `Create(size, opts) -> CommandHandle / Connect(pid) / SendInput / Resize / Kill`，默认环境注入 `TERM=xterm`、`LANG=C.UTF-8`、`LC_ALL=C.UTF-8`，启动 `/bin/bash -i -l`。aone 与七牛代码几乎逐行一致。

---

## 7. Filesystem

| 能力 | aone | 七牛 | E2B |
|------|------|------|------|
| `Read / ReadText / ReadStream` | ✅ | ✅ | ✅ |
| `Write([]byte)` | ✅ | ✅ | ✅ |
| `WriteStream(io.Reader)`（流式上传，可边读边 gzip） | ✅ | ❌ | ✅（Blob/ReadableStream） |
| `WriteFiles`（批量 multipart） | ✅ | ✅ | ✅ |
| `List / Exists / GetInfo / MakeDir / Remove / Rename` | ✅ | ✅ | ✅ |
| `WatchDir`（含 `WithRecursive`） | ✅ | ✅ | ✅ |
| `WithGzip` 上传/下载 | ✅（请求 `Accept-Encoding: gzip` 或对上传体压缩 + `Content-Encoding`） | ❌ | ✅ |
| `WithUser` | ✅ | ✅ | ✅ |
| `WithDepth`（List 递归深度） | ✅ | ✅ | ✅ |
| 签名 URL（`DownloadURL/UploadURL`） | ✅ | ✅ | ✅ |
| 签名算法 | `v1_` + SHA256(path:operation:user:token:exp) | 同 aone（**逐字对齐**） | 同（`getSignature`） |
| `EntryInfo` 字段（含 ModifiedTime/SymlinkTarget） | ✅ | ✅ | ✅ |
| `FilesystemEvent` 类型枚举 | create/write/remove/rename/chmod | 同 | 同 |

**aone 独有**：`WriteStream` + 流式 gzip 编码（`filesystem.go:394-456`），适合大文件场景；七牛在写入侧只支持一次性 `[]byte`。

---

## 8. Git

| 能力 | aone | 七牛 | E2B |
|------|------|------|------|
| 是否有独立 Git 模块 | ✅ `git.go`（329 行） | ❌ 完全缺失 | ✅ `sandbox/git/`（千行级，含 utils） |
| `Clone`（含 username/password、`DangerouslyStoreCredentials`、自动剥离凭据） | ✅ | — | ✅ |
| `Init / RemoteAdd / RemoteGet` | ✅（`overwrite=true` 时 `add \|\| set-url`） | — | ✅ |
| `Status / Branches / CreateBranch / CheckoutBranch / DeleteBranch` | ✅ | — | ✅ |
| `Add / Commit / Reset / Restore / Push / Pull` | ✅（Commit 支持 `authorName/authorEmail/allowEmpty`） | — | ✅ |
| `SetConfig / GetConfig / ConfigureUser` | ✅ | — | ✅ |
| `DangerouslyAuthenticate`（凭据写入 git credential.helper） | ✅ | — | ✅ |
| 选项 `WithGitCommandOptions` | ✅ | — | `GitRequestOpts` |

**对齐缺口（七牛 vs E2B/aone）**：七牛用户必须直接写 `Commands().Run("git ...")`，没有任何 Go-side 的 git 包装，凭据剥离、shell-quote、上游错误识别（`GitAuthError/GitUpstreamError`）都需要使用方自己实现。

aone 的 git 接口签名比 E2B JS 更扁平（每个动作一个方法 + 多位置参数），E2B 用 `GitXxxOpts` 结构体；语义等价。

---

## 9. 模板（CRUD + Builder + Dockerfile）

### 9.1 CRUD API

| 端点 | aone Client | 七牛 Client | E2B Template static |
|------|-------------|-------------|---------------------|
| 列模板 | `ListTemplates` | `ListTemplates`（额外有 `TeamID` 参数，aone 删除） | `Template.list`（不在仓） |
| 创建模板（v3） | `CreateTemplate` | `CreateTemplate` | 内部 `requestBuild` |
| 删除模板 | `DeleteTemplate` | `DeleteTemplate` | — |
| 更新模板（public 切换） | `UpdateTemplate` | `UpdateTemplate` | — |
| 启动构建（v2） | `StartTemplateBuild` | `StartTemplateBuild` | `triggerBuild` |
| 重新构建（POST `/templates/{id}`） | ❌（不暴露） | ✅ `RebuildTemplate`（含 `Dockerfile/StartCmd/ReadyCmd` 等参数） | ❌（用 builder 全套替代） |
| 构建状态 | `GetTemplateBuildStatus` | 同 | `getBuildStatus` |
| 构建日志 | `GetTemplateBuildLogs` | 同 | 通过 `waitForBuildFinish` 内部拉 |
| 上传文件元数据 | `GetTemplateFiles` | 同 | `getFileUploadLink` |
| 别名查询 | `GetTemplateByAlias` | `GetTemplateByAlias` | `aliasExists/exists` |
| `TemplateAliasExists` | ✅（封装 200/403/404） | ❌ | ✅ `Template.exists` |
| `AssignTemplateTags` / `DeleteTemplateTags` | ✅ + 短别名 `AssignTags/RemoveTags/GetTags` | ✅（无 `GetTags`） | ✅ `assignTags/removeTags/getTags` |
| `GetTemplateTags`（列出所有标签） | ✅ | ❌ | ✅ |
| `WaitForBuild` | ✅，含 `WithOnBuildLogs` 流式回调 + cursor | ✅，**但没有 onBuildLogs 回调** | ✅，含 `onBuildLogs` |

**aone 对 E2B 的对齐做了三处显式追加**（见 `template.go` 中的注释 *"Mirrors E2B's …"*）：`TemplateAliasExists`、`GetTemplateTags`、`AssignTags/RemoveTags/GetTags` 短别名。`WaitForBuild` 还实现了"日志游标 + 增量回调"逻辑（`template.go:225-283`），与 E2B `waitForBuildFinish` 中的 `logsRefreshFrequency` 行为对齐。

**七牛独有**：`RebuildTemplate`（POST `/templates/{templateID}` 旧式 v2），按 `doc.go` 注释流程是 `RebuildTemplate → StartTemplateBuild → WaitForBuild` 三步。aone 走 v3 创建路径，没有这个 legacy entry。

### 9.2 TemplateBuilder（DSL）

| 链式方法 | aone | 七牛 | E2B |
|----------|------|------|------|
| `FromDebianImage / FromUbuntuImage / FromPythonImage / FromNodeImage / FromBunImage / FromBaseImage / FromImage / FromTemplate` | ✅ | ❌（无 builder） | ✅ |
| `FromDockerfile`（解析 Dockerfile 并预填） | ✅ | ❌ | ✅ |
| `FromRegistry / FromAWSRegistry / FromGCPRegistry`（私有镜像） | ✅ | ❌ | ✅ |
| `Copy / CopyItems` | ✅（`CopyItem{Src,Dest}`） | ❌ | ✅（额外有 `mode/user/forceUpload/resolveSymlinks`） |
| `Remove / Rename / MakeDir / MakeSymlink` | ✅ | ❌ | ✅ |
| `RunCmd / SetWorkdir / SetUser` | ✅ | ❌ | ✅ |
| `PipInstall / NpmInstall / BunInstall / AptInstall` | ✅ | ❌ | ✅（E2B 多 `g/dev/noInstallRecommends/fixMissing` 选项） |
| `GitClone` | ✅ | ❌ | ✅ |
| `SetEnvs` | ✅ | ❌ | ✅ |
| `AddMcpServer` | ✅（注释明确 "Mirrors E2B addMcpServer"，发出 `MCP_SERVER` 步骤） | ❌ | ✅（强制 `mcp-gateway` 模板） |
| `SkipCache / ForceBuild / ForceNextLayer` | ✅ | ❌ | ✅（E2B 用 `skipCache()` 切换 forceNextLayer） |
| `SetContextPath / IgnorePatterns`（dockerignore） | ✅ | ❌ | ✅ |
| `SetStartCmd(start, ReadyCmd) / SetReadyCmd` | ✅ | ❌ | ✅ |
| `ToDockerfile`（反向渲染） | ✅ | ❌ | ✅ |
| `Build / BuildInBackground` 入口 | ✅（`(*TemplateBuilder).Build(ctx, c, name, opts, pollOpts...)`） | ❌ | ✅（`Template.build/buildInBackground` 静态） |
| 文件 hash 计算 | ✅ `dockerfile.ComputeFilesHash`（SHA256，按 POSIX 路径排序） | ❌ | ✅ `calculateFilesHash` |

**亮点**：aone 的 `template_builder.go` 是逐方法照着 E2B 实现的，每个方法的实参形态、默认值、shell 拼接策略与 E2B JS 同名方法一一对应。

**E2B 独有未对齐**：
- `betaDevContainerPrebuild / betaSetDevContainerStart`（devcontainer 集成）
- `pipInstall(g=false)` 这类细粒度选项（aone 默认全局）
- 完整的 `stackTraces` 链路（用于构建日志中显示出错的源码行）

### 9.3 ReadyCmd 助手

aone 在 `template_builder.go:23-53` 实现了 `WaitForPort / WaitForURL / WaitForProcess / WaitForFile / WaitForTimeout`，命令字符串与 E2B `template/readycmd.ts` **逐字对齐**（`ss -tuln | grep :PORT`、`curl -s -o /dev/null -w "%{http_code}" URL | grep -q "STATUS"`、`pgrep ...`）。七牛**完全缺失**这一族。

### 9.4 Dockerfile 解析

aone 拥有独立子包 `packages/go/sandbox/dockerfile/`（`parser.go` 474 行 + `context.go` 297 行）：
- `dockerfile.Parse` 解析 FROM/RUN/COPY/ADD/WORKDIR/USER/ENV/ARG/CMD/ENTRYPOINT 等指令；
- `ParseEnvValues / StripHeredocMarkers / ParseCommand` 处理 ENV 多键值、heredoc、CMD 字符串/数组形式；
- `ComputeFilesHash` 按"COPY src dest 头部 + 排序后的相对 POSIX 路径 + mode + size + 内容"算 SHA-256；
- `ReadDockerignore` 加载忽略规则。

`dockerfile_convert.go` 把解析结果重放成 `TemplateStep` 流，自动追加缺省的 `USER root / WORKDIR / ` 序章和 `USER user / WORKDIR /home/user` 收尾，与 E2B 模板构建系统行为一致。

七牛**没有任何 Dockerfile 处理**。

---

## 10. Volume（持久卷）

| 能力 | aone | 七牛 | E2B |
|------|------|------|------|
| 是否实现 | ✅ `volume.go`（380 行） | ❌ | ✅ `volume/`（多文件） |
| `CreateVolume / ConnectVolume / GetVolumeInfo / ListVolumes / DestroyVolume` | ✅ | — | ✅ |
| `Volume.{List/MakeDir/GetInfo/Exists/UpdateMetadata/ReadFile/ReadFileText/WriteFile/ReadFileStream/WriteFileFromReader/Remove}` | ✅ | — | ✅ |
| `VolumeWriteOptions` 含 UID/GID/Mode/Force | ✅ | — | ✅ |
| `VolumeMount` 类型并能在 `CreateParams.VolumeMounts` 里挂载 | ✅ | ❌（`CreateParams` 没有 VolumeMounts） | ✅ |
| 通过独立 token 访问 content API | ✅（`volume.Token` + `Authorization: Bearer`） | — | ✅ |

aone 的实现复用了 OpenAPI 派生客户端 `internal/volumeapi`；命名（如 `VolumeFileType`、`VolumeEntryStat`）与 E2B 字段保持一一对应。

---

## 11. 错误模型

| 维度 | aone | 七牛 | E2B |
|------|------|------|------|
| `APIError` 类型 | ✅，含 `StatusCode/Body/Reqid/Code/Message/RetryAfter/sentinel` | ✅，含 `StatusCode/Body/Reqid/Code/Message`（无 sentinel） | ✅，多个 typed Error class |
| Sentinel/typed errors | ✅ `ErrNotFound, ErrSandboxNotFound, ErrFileNotFound, ErrAuthentication, ErrGitAuth, ErrGitUpstream, ErrInvalidArgument, ErrNotEnoughSpace, ErrRateLimited, ErrTimeout, ErrTemplate, ErrBuild, ErrFileUpload, ErrVolume`（与 E2B 1:1） | ❌ 仅 `APIError` 加状态码比较 | ✅ `SandboxError, TimeoutError, InvalidArgumentError, NotEnoughSpaceError, NotFoundError, FileNotFoundError, SandboxNotFoundError, AuthenticationError, GitAuthError, GitUpstreamError, TemplateError, RateLimitError, BuildError, FileUploadError, VolumeError` |
| 资源 hint 升级 404 → SandboxNotFound/FileNotFound | ✅ `resourceHint` + `newAPIErrorFor` | ❌ | ✅（HTTP 层根据 path 推断） |
| `Retry-After` 解析（秒/HTTP-date） | ✅ | ❌ | ✅ |
| 502/503 → ErrTimeout（沙箱超时语义） | ✅ | ❌ | ✅（`formatSandboxTimeoutError`） |
| Git 错误识别（按消息切到 GitAuth/GitUpstream） | ✅ `classifySentinel` 内消息扫描 | ❌ | ✅ `isAuthFailure/isMissingUpstream` |
| `errors.Is(err, sandbox.ErrSandboxNotFound)` 风格 | ✅ | ❌（要走 `*APIError` 类型断言 + 状态码） | `instanceof` 风格等价 |
| `isNotFoundError`（含 `connect.CodeNotFound`） | ✅ | ✅（基础版） | — |

**结论**：aone 的错误模型在 Go 圈层级与 E2B 完全等价，**七牛 SDK 的错误处理颗粒度明显更粗**，调用方几乎只能 switch StatusCode。

---

## 12. 轮询（Poll）

三方都提供 `WithPollInterval / WithBackoff / WithOnPoll`。

aone 多出 `WithOnBuildLogs` 用于 `WaitForBuild` 时增量回送 `[]BuildLogEntry`（`poll.go:50-56`、`template.go:225-283`），与 E2B `waitForBuildFinish(logsRefreshFrequency)` 的"日志流"对齐；七牛的 `WaitForBuild` 只判断 `ready/error` 状态，不暴露日志回调。

---

## 13. 分页

aone：`SandboxPaginator / SnapshotPaginator`（`paginator.go`），与 E2B `SandboxPaginator / SnapshotPaginator` 对应。
七牛：`Client.List` 直接返回 `[]ListedSandbox`（**吞掉了 next token**），调用方只能拿到第一页。

---

## 14. MCP 网关

| 能力 | aone | 七牛 | E2B |
|------|------|------|------|
| `CreateParams.MCP` | ✅ `MCPConfig map[string]any` | ❌ | ✅ `mcp` 选项 |
| 启动时执行 `mcp-gateway --config <json>` | ✅ | ❌ | ✅ |
| 自动生成 token 并设 `GATEWAY_ACCESS_TOKEN` | ✅（16 字节 hex） | — | ✅（UUID） |
| `Sandbox.GetMCPURL()` | ✅（`https://50005-<id>.<domain>/mcp`） | — | ✅（`getMcpUrl`） |
| `Sandbox.GetMCPToken()` | ✅（懒读 `/etc/mcp-gateway/.token`） | — | ✅（`getMcpToken`） |
| `TemplateBuilder.AddMcpServer` | ✅ | — | ✅ |

---

## 15. InjectionRule（七牛独有）

七牛 `injection_rule.go` + `types_injection_rule.go` 提供：

- `ListInjectionRules / CreateInjectionRule / GetInjectionRule / UpdateInjectionRule / DeleteInjectionRule`
- `InjectionSpec`（OpenAI / Anthropic / Gemini / Qiniu / HTTP discriminated union）
- `SandboxInjectionSpec` 可在 `CreateParams.Injections` 中按 ID 引用或 inline，对沙箱出站 HTTPS 流量做请求头注入
- 这些 API 必须使用 Qiniu V2 签名（`Authorization: Qiniu <sig>`），通过 `Client.GetCredentialsOption()` 取得

**aone 与 E2B 都没有这套能力**，aone 的 `Config.Credentials any` 字段就是为了源代码兼容这条 surface 但不实现。

---

## 16. 跨语言一致性（E2B Python 对比）

E2B Python SDK（`packages/python-sdk/e2b/`）拆分了 `sandbox_sync/` 与 `sandbox_async/`，但暴露的方法集合与 JS 一致：`Sandbox.create/connect/list/kill/pause/get_info/get_metrics/set_timeout/create_snapshot/list_snapshots`、`files/commands/pty/git`、`Template.{from_*/copy/run_cmd/.../build}`、`Volume.{create/connect/...}`。

因此 aone Go SDK 在功能维度上**与 E2B 跨 JS/Python 的并集对齐**；七牛 Go SDK 与 E2B Python 的差距与上文描述的 vs JS 差距一致。

---

## 17. 总体差距矩阵

下表标注：✅ 对齐 / ⚠️ 部分对齐 / ❌ 未实现。

| 能力组 | aone vs E2B | 七牛 vs E2B | aone vs 七牛 |
|--------|-------------|-------------|--------------|
| 客户端配置 + env-var | ✅ | ❌ env-var 缺、Bearer 头缺、debug 缺、req-timeout 缺 | aone 多 env-var/Debug/RequestTimeout |
| 创建/连接/列出 + Lifecycle/AutoResume | ✅ | ⚠️（缺 lifecycle、缺 next token） | aone 多分页、Lifecycle、VolumeMounts |
| Pause / Refresh / SetTimeout / Kill | ✅ | ✅ | 等价 |
| Snapshot | ✅ | ❌ | aone 多一整套 |
| MCP（沙箱 + builder） | ✅ | ❌ | aone 多 |
| Filesystem（含 gzip / WriteStream） | ✅ | ⚠️（无 gzip / 无流式 Write） | aone 多 gzip + WriteStream |
| Commands（含 WaitOK/Disconnect） | ✅ | ⚠️（CommandHandle 表面较薄） | aone 多 WaitOK/Disconnect/中途读取 |
| Pty | ✅ | ✅ | 等价 |
| Git | ✅ | ❌ | aone 多一整套 |
| Templates CRUD | ✅ + 短别名 + GetTags | ⚠️（缺 GetTags、缺 alias-exists、含 legacy RebuildTemplate） | 各有侧重 |
| TemplateBuilder DSL | ✅ | ❌ | aone 多一整套 |
| Dockerfile 解析 + filesHash | ✅ | ❌ | aone 多一整套 |
| ReadyCmd 助手 | ✅ | ❌ | aone 多一整套 |
| WaitForBuild + onBuildLogs | ✅ | ⚠️（无 onBuildLogs） | aone 多 onBuildLogs |
| Volume | ✅ | ❌ | aone 多一整套 |
| Sentinel/typed errors + Retry-After | ✅ | ❌ | aone 多 |
| Paginator | ✅ | ❌ | aone 多 |
| InjectionRule | ❌ | ✅ | 七牛独有 |

---

## 18. 建议（按优先级）

### 18.1 七牛 SDK 若要追平 E2B/aone（按"投入小→大"排序）

1. **错误模型升级**：在 `errors.go` 旁补一份 sentinel error 列表 + `classifySentinel`，最少代价拿到 `errors.Is` 的诊断能力。aone 的 `errors.go` 可直接借鉴。
2. **Env-var fallback + Bearer header**：照 aone `client.go:82-120` 改 30 行即可。
3. **`ListPage` + `SandboxPaginator`**：让分页成为可能；目前 `List` 把 `next-token` 丢了。
4. **CommandHandle 表面对齐**：`WaitOK / Disconnect / Stdout()/Stderr()/ExitCode()`，能直接复用 aone `commands.go:46-153`。
5. **`WaitForBuild` 加 `WithOnBuildLogs`**：实时构建日志在 CI 中刚需，aone 已经实现，逻辑可移植。
6. **TemplateAliasExists / GetTemplateTags**：HTTP 层已经支持，套一层 SDK 包装即可。
7. **ReadyCmd 助手**：纯字符串拼接，5 个函数一晚上完成。
8. **TemplateBuilder DSL + Dockerfile 解析 + filesHash**：体量最大但价值最高；aone 的 `template_builder.go` + `dockerfile/` 子包是"开箱即用"的实现参考（依赖 internal apis 的细节需要按七牛包路径替换）。
9. **Git / Volume / Snapshot / MCP**：四个独立模块，按业务诉求决定是否补齐。

### 18.2 aone SDK 已实现但仍可考虑的小补丁

1. ~~**`apiKeyEditor` 双 header**~~ — **已完成**。`client.go: apiKeyEditor` 现同时写入 `X-API-Key` 和 `Authorization: Bearer <APIKey>`，并保留"调用方已预设 Authorization 则让位"的逻辑，与 E2B JS / 七牛 Go 行为一致。
2. **`Config.Credentials` 字段去掉或改成可解释的占位**：当前 `any` + 注释 *"intentionally not implemented"* 容易误导调用方（看起来像支持，实际不支持）。
3. **如果业务确实需要类似七牛 InjectionRule 的"出站请求注入"能力**，可以以独立子模块形式引入；但这是 Qiniu-specific 控制面 API，与 E2B 协议无关，需要服务端先支持。
4. **`StartTemplateBuild` 不接受 `Dockerfile` 字符串**：七牛额外的 `RebuildTemplate` 一步把 Dockerfile 直接发给后端，对"已经把 Dockerfile 当源码"的用户更顺。aone 走 `FromDockerfile` builder 路径，强制了 SDK 端解析。两套都对齐 E2B（E2B 也是 SDK 端解析 + steps 上传），保持现状即可，但文档中可以写明此差异。

---

## 19. 结论

- **aone Go SDK 是三者中"对齐 E2B 最彻底"的**：sentinel errors、TemplateBuilder、Dockerfile 解析、Volume、Snapshot、Git、MCP、ReadyCmd、Paginator、`onBuildLogs`、`WriteStream/gzip`、`WaitOK/Disconnect`、env-var fallback、双 header 兼容 —— 一个也不少。剩余可考虑的小点是 `Config.Credentials` 字段语义。
- **七牛 Go SDK 是"最小可用核心 + 七牛特有 InjectionRule"**：能完成 sandbox/template/files/commands/pty/RPC 闭环，但工程化外围（错误诊断、分页、构建 DSL、Volume、Git、Snapshot、MCP）几乎全部缺失；要追平 aone 大约需要把 aone 的对应文件按内部 API 路径调整后整体移植。
- **InjectionRule 是七牛独家的能力**，与 E2B/aone 在协议层并不冲突；如果该能力要进入 aone，应作为独立的"出站策略"模块设计，而不是塞进 sandbox 包。
- 三者的 envd RPC、控制面 OpenAPI 形态都基于同一族协议；选择 SDK 时**主要差异在于是否需要 E2B 风格的工程化辅助**（构建 DSL、Volume、Git、Snapshot、错误分类）以及**是否依赖七牛侧的 InjectionRule**。

---

## 附录 A：源码对照位置

- aone：`packages/go/sandbox/{client,sandbox,types,types_template,template,template_builder,commands,pty,filesystem,git,volume,snapshot,errors,poll,paginator,multipart,dockerfile_convert}.go`、`dockerfile/{parser,context}.go`
- 七牛：`sandbox/{client,sandbox,types,types_template,types_injection_rule,template,commands,pty,filesystem,injection_rule,errors,poll,multipart}.go`
- E2B JS：`packages/js-sdk/src/{index,errors,connectionConfig,utils,logs}.ts`、`sandbox/{index,sandboxApi,signature,network,mcp.d}.ts`、`sandbox/{filesystem,commands,git}/`、`template/{index,buildApi,dockerfileParser,readycmd,logger,types,utils,consts}.ts`、`volume/{index,client,types}.ts`
- E2B Python：`packages/python-sdk/e2b/{sandbox,sandbox_sync,sandbox_async,template,template_sync,template_async,volume}/...`
