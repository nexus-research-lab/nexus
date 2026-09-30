# Provider 环境所有权证据（2026-09-17）

本批验证 settings/环境/插值的输入边界与宿主进程换代，不代表整个 SDK 或外部 MCP 已隔离。所有提交和固定 Bridge 模块均仅本地，`releaseAccepted=false`。

- SDK `101f34fa` 固定 Provider 输入并清理命令/hook 环境；`460c0f1c` 补齐 HTTP hook 与 MCP 配置插值。Bridge 保持 `a2316d7`，模块版本、checksum、nxs SHA256 和平台见 `report.json`。
- `baseline-report.json` 是从 `460c0f1c` 导出构建后的完整开发基线：143 个顶层/309 个指定子场景均通过，无必测 skip。其全部原始命令日志以 `baseline-*.log.gz` 保存。
- 基线的宿主测试执行后新增了进程所有权指纹修复，因此最终 Nexus 状态另外执行了 runtime/clientopts 全包 race；`final-host.jsonl.gz` 和 `nexus-provider-final-host-required.json` 分别保存原始结果与 2 个顶层/4 个指定子场景的严格检查。最终脚本总要求为 145/313，不能把这 6 项误写为初次基线已经运行。
- Nexus 源码为 `nexusBaseRevision` 加 `nexus-source.patch.gz`；精确文件 SHA256 见报告。文档/证据提交自身不影响代码身份。
- `sdk-before` 在旧 SDK `00b72d22` 的 git archive 中仅加入 `before-fixture-*.source` 对应测试后执行，26 个失败事件包含父子项。`sdk-body-before` 使用后续同名测试，生产代码相当于 `101f34fa` 中将 `internal/config/env/provider.go` 替换为本目录 `provider-body-before.go.source`；该中间状态仍允许附加正文替换模型。
- `interpolation-before` 是 `101f34fa` 加 `460c0f1c` 的两个新增测试文件。`host-before` 是 Nexus 基线加启动标记测试；`host-replace-before` 保持基线 `process_policy.go` 并加入后续同名替换测试。这些失败是预期反例，不是最终门禁失败。
- SDK 目标包、下游调用方、插值补充、最终宿主分别记录，不能将各 suite 的事件数相加当作互不重复测试数。可选原生 skip 列在报告中，不冒充通过。Windows/Linux 的 `-exec=/usr/bin/true` 只证明编译。

复核：先验证 `manifest.json` 的 SHA256，再恢复报告中的提交/补丁和本地 Bridge 模块，按报告命令运行；环境始终使用 `GOWORK=off`。固定源码完整入口为：

```sh
GOWORK=off GOPROXY=off node scripts/desktop/check-sandbox-baseline.mjs --sdk-source /absolute/nexus-agent-sdk-go --sdk-ref 460c0f1c
```

该命令需要原生 macOS 与本地依赖缓存；不能证明干净机器可下载本地未发布模块。秘密文件、进程/句柄、MCP helper 执行、后台 IO、网络、完整后代监督、scratch、持久恢复、默认策略、Claude 和平台安装包门禁继续保留。
