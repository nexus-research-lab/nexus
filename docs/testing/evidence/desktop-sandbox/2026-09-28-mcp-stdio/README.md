# macOS stdio MCP 验收（2026-09-28）

本批补齐用户保存的命令配置与 Connector stdio 配置的实际执行。Nexus 与 nxs 整包发布，
不增加用户单独升级内核的步骤。当前合同见[沙箱规范](../../../../specs/desktop-sandbox-spec.md)。

## 固定来源

| 项目 | 来源 |
| --- | --- |
| Nexus | `1b1bb88ca12761afad3d2d486d53f893a86cdf8c` |
| SDK | `b84b7b6c7c3970d4a948501eaed2874563730cc2` |
| Bridge | `v0.1.34-0.20260927182153-4b2972f09143`，远程规范模块，无本地 replace |
| Bridge checksum | `h1:61JYZ5QUkuhaEb1Y0Bxe17ETd6CYolEQVQgZdb+bNRc=` |
| nxs SHA-256 | `bee303f173601780412ff544dcd5c1653dbc959156849817a7b4122a429951fa` |
| 平台 | 本机 macOS arm64，Go go1.27.1，完整信息见报告 |

验收启动时 Nexus 与 SDK 工作树均干净；nxs 从固定 SDK Git 归档构建。
证据文档是后续提交，不改变已验证代码。

## 结果

- [完整基线](baseline-report.json)：**45 项检查、640 个必测名称全部通过**，无必测缺失或 skip。
  [原始输出](baseline-logs.tar.gz)包含新增 stdio 和已有文件、网络、配置、恢复等全部基线。
- Nexus options builder → 固定 Bridge → 真实 nxs → stdio 进程 → 工具结果 → 后续模型请求：
  持久命令配置（省略 type、按 command 推断）和 typed Connector 两条路径通过。
  原有 HTTP/SSE 及各自 helper 四条往返一并通过。
- 原生 stdio 覆盖允许读、禁止读、只读拒写、scratch 写入、命令网络拒绝和显式允许、
  继承 Provider 凭据过滤、显式服务凭据保留、保留环境拒绝、原样 argv、取消、权限变化、
  关闭、同名替换及普通同组后代清理。请求前后重验配置和最初权限代次。
- 协议覆盖并发独立 wire ID、乱序及迟到回复、初始化寿命、异常和超限 JSONL、管道背压取消、
  禁用/移除/替换/关闭、服务端 ping 和不支持方法。JSON、HTTP SSE 和旧式 SSE 的聚合限额通过。
- SDK client/MCP/executor/nxs 目标包及 runtime 包回归通过；新增 stdio、既有 helper、协议限额的
  定向 race 通过。Bridge client/protocol race、Nexus runtime/clientopts、真实 MCP 往返的定向 race
  和架构依赖检查通过。不据此宣称全 SDK runtime race 或全产品测试通过。
- [配套自检](package-runtime-check.json)：固定 nxs 与源码 sidecar 在 workspace-write、read-only、
  full-access 三种配置下完成握手和关闭。本项未创建新 App/DMG。

## 回归修正

扩展运行 SDK runtime 包时，两条历史记忆测试把新增的 `.nexus-settings.lock` 当作记忆产物。
同样失败在上一版 SDK `5dc1eb8b` 的干净归档中复现；来源是 settings journal 恢复的跨进程锁，
不属于记忆存储。本批断言仅排除这个名称的普通锁文件，仍拒绝其他额外产物，保留 MEMORY.md、
memory/ 与禁用持久记忆的断言；修正后整个 runtime 包回归通过。

## 复现与范围

```sh
GOWORK=off node scripts/desktop/check-sandbox-baseline.mjs \
  --sdk-source /absolute/path/to/nexus-agent-sdk-go \
  --sdk-ref b84b7b6c7c3970d4a948501eaed2874563730cc2
```

原始报告目录：`/var/folders/jk/9xhnwgrx6cj4fj76wffmxy0w0000gn/T/nexus-sandbox-baseline-ZfCyah`。
日志归档 SHA-256：`c263b062c9cbb6eb3bd423ca2856b7deb51f5b440803e68d5b7159f64f9dfd6d`。

模型、HTTP/SSE 以及 stdio 服务均为本机确定性协议夹具；真实 nxs 与真实 OS 进程不等于实际第三方
MCP 服务或 App UI。取消会结束整个 stdio 服务及其他在途调用，用户核对结果后才显式重连，不自动重放。
普通进程组收口不证明独立 setsid 后代终态。可信 MCP 代理、后台/秘密/Provider 边界、DM/Room/后台 App
场景及正式签名/公证/安装升级仍需继续；本批未运行 Windows，`releaseAccepted=false`，Goal 保持进行中。
