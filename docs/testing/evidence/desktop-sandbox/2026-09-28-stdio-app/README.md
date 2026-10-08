# 当前 stdio MCP 内核的 macOS App 与真实模型验收（2026-09-28）

## 固定来源与产物

- Nexus `d92fdb7e94b74538a48e6677a10a73b79a75b2f6`，SDK `b84b7b6c7c3970d4a948501eaed2874563730cc2`；构建开始时工作树干净，
  [包元数据](package-metadata.json)的 `source.dirty=false`。
- Bridge 规范模块 `v0.1.34-0.20260927182153-4b2972f09143`，与本批
  [45 项/640 个必测名称的基线](../2026-09-28-mcp-stdio/README.md)一致。
- nxs 签名前 SHA-256：`bee303f173601780412ff544dcd5c1653dbc959156849817a7b4122a429951fa`。
- 包内 nxs 签名后 SHA-256：`478078af4a6ae42c1bd5574b1309479fcb6c02ebfa4d38277d03e1607e4fa912`。
  开发签名改变 Mach-O 内容；两个摘要对应签名前后，不是换用另一内核。
- DMG 位于 `/var/folders/jk/9xhnwgrx6cj4fj76wffmxy0w0000gn/T/nexus-macos-stdio-package-dn8qqrlq/artifacts/Nexus-macos-arm64-0.2.0-2477.dmg`，
  摘要见 [package.sha256](package.sha256)。包本体未提交源码仓库。
- [执行回执](receipt.json)、[日志](acceptance-logs.tar.gz)归档；日志 SHA-256：`006fd107e13392b1eb60bf4b7fc6ce881f15f764efcee495663b6a87c3f2180d`。

元数据中的 `runtime.nxs.release=nxs-stable` 是原打包脚本对自定义路径保留的默认标签，不能据此
把本次输入当成已发布稳定内核；本次实际来源由上述固定 SDK、二进制摘要和配套握手证明。
后续签名专用入口显式标记 `source-b84b7b6c7c3970d4a948501eaed2874563730cc2`。

## 已通过

1. 当前 Web、Swift 宿主、Go sidecar、nexusctl/nexuscfg 与 nxs/rg 完整构建，ad-hoc 签名 DMG 生成成功。
2. 真实 App 在新状态根与独立 preferences suite 启动；主窗口、Launcher、web.ready、退出及 sidecar
   清理通过，未放开 fallback reveal。未使用用户主数据库。
3. 校验 DMG 摘要、只读挂载、校验 App 签名；从 DMG 内 App 再次执行同样 smoke，配套 nxs 在
   workspace-write/read-only/full-access 三种模式下握手与关闭通过，见[自检](mounted-runtime-check.json)。
   最终卸载成功，未保留本次挂载。
4. 使用 App 内已签名 nxs，经 Nexus options builder 与规范 Bridge 调用真实第三方
   Anthropic-compatible 模型，五项检查通过：原生 Write 与 Bash 读取、原生文件拒写、
   命令拒写、命令网络拒绝、中断及关闭。只读取主工作目录 `.env` 的 Provider 字段；日志已核验无对应凭据。

首轮 DMG 操作在 smoke 与 runtime 自检成功后，由临时验证器错误的 lsregister 路径导致清理命令失败。
随后显式卸载成功，修正临时验证器后重跑挂载、smoke、自检和卸载全流程，最终 exit 0；没有把首次
清理失败记成整体通过。仓库原有 smoke 脚本的系统工具路径正确，无需改动。

## 尚未完成

这是本机 arm64、开发签名的独立测试包。真实 Provider 验证经过宿主 options builder，
并非在 App UI 中完成 DM/Room/后台任务的全部交互。Developer ID、公证、Intel 原生、正常
Gatekeeper 的新机器、用户 App 数据库升级/回退仍待验收，`releaseAccepted=false`。
签名专用 CI 入口已提交并通过静态检查；私有 SDK 禁止 Deploy Key，等待其 CI 只读授权后运行。
本批未执行 Windows 验证，官方 Claude 账号/OAuth 继续暂缓，Goal 保持 active。
