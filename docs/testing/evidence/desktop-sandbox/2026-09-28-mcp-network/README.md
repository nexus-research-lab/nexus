# macOS 远端 MCP 与配套发布验收

本批次修复默认桌面沙箱令已配置 HTTP/SSE MCP 阻止整个 Agent 启动的问题。
Nexus 和内置 nxs 作为同一产品整包发布；历史 nxs 仅用于生成升级前数据，
正常用户无需分别管理应用和内核版本。包内自检与用户历史数据兼容分别验收。

## 固定来源

- Nexus 代码：`626f11d5c`（包含 `de6c8899b`）。
- SDK：`b487ef247f4057f327896d4a2eea6779d06811ea`（包含 `7e200b34`，以及 Windows 机器的 `38221838`）。
- Bridge：`c2b5eaf37bf3`，使用已发布到远程的 canonical Go module；没有本地 replace。
- 平台：macOS arm64。没有执行 Windows 测试。
- 固定源码 nxs SHA-256：`20f7159be5296aab776df24f2e126ca46ae187890e0aea66cc74fcec7958065f`。

## 覆盖内容

1. Nexus 的持久化 HTTP 配置、typed Connector SSE 配置，分别经固定 Bridge 启动真实 nxs；协议模型夹具生成一次工具调用，真实 MCP 服务收到它，返回值实际进入下一次模型请求。
2. MCP 端点只获得本服务 origin 的网络权限；跨主机/端口跳转、其他工具借用授权、显式禁止、托管域名限制均有反例。SSE 服务不能另行指定跨 origin POST 地址。
3. 每次请求/响应读取支持取消；禁用、删除、替换和关闭撤销连接。迟到 discovery 不复活旧配置，测试记录确认工具没有自动重发。
4. runtime 逐请求检查配置并绑定权限代次；变化取消响应读取。失败的服务不阻断整个 runtime；认证 helper 在执行前拒绝。
5. MCP 凭据不采用任务 settings 的 HTTP/SOCKS/MITM 路由，也不静默绕过已要求的代理；专门的宿主代理合同仍是独立工作。
6. Nexus 的进程指纹、有效策略回执和包内握手要求携带独立 `sandbox_mcp_network_v1`。源代码 sidecar 与固定 nxs 的 workspace-write/read-only/full-access 自检通过，见 `runtime-selfcheck.json`；这不是本次重新签名的 App。

## 固定源码基线

最终 `report.json`：42 项检查、569 个指定测试名全部通过，无必测跳过或缺失。运行开始时三仓代码和 SDK 导出均为固定提交；Nexus 与 SDK 工作树清洁。新 MCP 两组完整逐测试日志保存在同目录。

## 补充验证及未通过尝试

- `bridge-race.log`：Bridge client/protocol 竞态回归通过。
- `product-race.log`、`architecture.log`：Nexus runtime/clientopts 竞态与架构门禁通过；目标包 vet 同样通过。
- `sdk-transport-race.log`、`sdk-proxy-race.log`：真实 HTTP/SSE 与代理边界竞态测试通过。
- `native-runtime.log`：macOS 原生 runtime 显式/非显式 MCP 装配通过。
- `sdk-broader-attempt.log`：扩展 SDK 回归中 client、nxs、MCP、executor、sandboxexec 通过；runtime 的 `TestNewInitializesAutoMemoryInConfiguredWorkspace` 与 `TestSessionPersistenceOffDisablesPersistentMemory` 失败。
- 上述两个失败在改动前 `593b1fa6` 的独立源码导出上同样复现，见 `sdk-original-memory-failures.log`。只排除这两个已复现的基线失败后，runtime/MCP 竞态回归通过，见 `sdk-registry-runtime-race.log`；不把完整 runtime 包记为全绿。
- `native-runtime-race-attempt.log`：在原生文件 worker 同时采用 race 插桩的尝试中，runtime 构造阶段超时；调整 race 退出等待后仍超时，原因尚未完成诊断。该尝试不计作通过。正常原生门禁和 transport/registry 竞态证据分别记录，不能互相冒充。
- 最初的门禁脚本要求了 Go 并不会单独产生的 `http`/`sse` 中间父测试名；已修正为真实存在的各场景全名。最终报告仍要求全部指定场景通过，不接受 skip 或缺失。

## 证据边界

这是本机原生和 Nexus→Bridge→真实 nxs 的协议集成验证，模型与 MCP 均使用本机确定性服务，未访问生产账号或真实第三方 MCP。
`releaseAccepted=false`：不代表完整 App UI 的 DM/Room/后台审批、签名公证、干净机器安装或 App 数据库升级验收。
认证 helper/stdio 进程、OAuth 发现/令牌、Provider 出口、秘密文件/句柄及脱离 session 后代监督仍需独立完成；保持 macOS Goal active。

## 合并另一台机器的后续提交

归档后远程新增 Windows `eb1af51b0`、`8b17601fd`，已保留双方历史合入同一分支，不改写上述已验收提交。传入的运行代码只涉及 Windows 进程存活判断；合并后使用同一固定 nxs 重跑 macOS runtime/clientopts 竞态门禁（包含真实 MCP 往返）通过，见 `postmerge-macos-race.log`。没有在本机执行 Windows 验证。
