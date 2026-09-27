# macOS 图片网络与升级配套检查（2026-09-28）

SDK `593b1fa64c5d14ccd1a9433a1c63ea1ca2edf5d1`、Bridge
`b0402649d44b50401f8f2876650f8fa6143dd85d` 已推送统一沙箱分支。
Nexus 使用远程规范模块 `v0.1.34-0.20260927155900-b0402649d44b`，
checksum `h1:E3JGaj2x4m7+AaK0VCaVWriTrKFj45Jv+VjL9x4VSBs=`；无本地 replace。

- 完整原生 macOS 基线 40 项检查、527 个必测名称通过，无缺失/必测 skip；
  SDK 从上述提交的 Git archive 构建。运行二进制摘要见 baseline-report.json。
- 图片 HTTP client、逐请求/跳转准入、权限代次撤销、取消未结束响应体及真实
  ViewImage/文件 helper 的 SDK race 测试通过；Bridge 握手与能力拒绝通过。
- Nexus runtime/clientopts/server 的 race 测试、定向 vet、架构依赖门禁通过。
- 新固定 Bridge/候选 nxs 下，nxs 与 Claude 的真实第三方 Provider 10 项基础检查再次通过；
  同步 Windows 机器的 Nexus 提交 `bc74365dd` 后，本机 runtime/clientopts/confinedfs 回归通过。
- 安装包 sidecar 的真实检查对候选 nxs 完成 workspace-write、read-only、
  Full Access 三种握手/关闭；上一批缺少媒体网络能力的 nxs 在打包检查中失败。
- 上一批开发内核 `9956def1` 创建会话 → 当前内核受限续用 → 原内核续用回退
  通过；固定会话 ID、实际 Provider 历史、transcript 原字节前缀及配置/记忆/
  workspace 内容保留。此项是开发基线，不能冒充已发布版本升级证据。
- Nexus/nxs 按整包配套发布。正常用户没有独立内核升级步骤；本批没有改变
  App runtime 的路径选择或开发覆盖行为，重点为历史数据及既有功能兼容。

本批 Nexus 运行来自未提交工作树，报告记录全部差异。实际已发布 nxs、干净固定
App、Developer ID/公证、干净机器、Intel、完整 DM/Room/后台审批、外部 MCP/helper、
秘密/句柄/完整后代监督仍独立验收；`releaseAccepted=false`。不运行 Windows 测试。
