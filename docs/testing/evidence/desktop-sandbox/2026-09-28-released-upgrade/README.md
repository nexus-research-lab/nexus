# 配套版本读取旧用户数据验收（2026-09-28）

Nexus 和 nxs 正式配套发布；用户没有独立升级内核的步骤。本批验证新配套版本继续
使用已有数据，不把开发环境版本混用作为普通用户场景，也没有改动原有 runtime
路径选择或启动行为。

## 已验证

- 已发布 `nxs-v0.1.34` 的 macOS arm64 归档来自 Bridge GitHub Release，下载内容
  SHA-256 `63bfecf8e510bd1ef459aceb7c7c6f5f283bed4a3f03fe1ad6fea0917eb15868` 与
  发布 manifest 完全一致；可执行文件输出 `0.1.34 (nxs)`。
- 该版本创建持久会话 → SDK `593b1fa6` 当前内核以桌面受限策略续用 → 原发布内核
  回退续用三阶段全部通过。实际 Provider 请求保留前序用户/助手历史，会话 ID 不变，
  transcript 原字节保持前缀；settings/config、MEMORY.md 和工作文件逐字节保持。
- 对最终 App 捆绑并 ad-hoc 签名后的 nxs 再次运行同一验证，通过。
- Nexus 干净提交 `223849cb8bdb98f47d868e6b756e791d54fe1625` 构建 macOS arm64
  App/DMG，通过实际 sidecar/nxs 三种权限配置发布自检和 App 启动/窗口/路由/退出
  smoke。测试使用独立 `NEXUS_DESKTOP_STATE_ROOT` 和 preferences suite，未读取用户
  的实际数据库或配置。
- Bridge 固定 `v0.1.34-0.20260927155900-b0402649d44b`，无本地替换。

## 证据边界

旧会话验证使用本地确定性 Provider 协议夹具，不是旧完整 Nexus App 的数据库升级/
降级测试。App/DMG 为本机 arm64 ad-hoc 产物，未提供 Developer ID/公证、clean-host、
Intel、所有历史版本或完整产品功能升级证据。既有远程 MCP 配置与新增沙箱默认策略的
准入兼容仍需修复，其他 macOS 剩余项继续见统一开发计划；`releaseAccepted=false`。

实际 DMG 留在本机 `/private/tmp/nexus-macos-compatibility-package/`，没有发布正式版本。
