# 配置受控写入证据（2026-09-17）

本目录验证配置写入的进程内文件边界、后续请求准入和独立能力集成。完整沙箱与发布验收仍未完成。`report.json` 保存源码版本、模块校验和、命令、最终退出码、范围和 skip；`baseline-report.json` 保存固定 SDK 导出构建后的必测项。压缩日志解压后保持原始字节，原始 SHA256 记录于汇总报告；目录文件校验和见 `manifest.json`。

- SDK 固定为 `00b72d227d8adf108bcdcacaebfa84bf9c34361b`，包含配置写入提交 `65e65b86` 和两条 WebFetch 摘要请求的补充准入。`web-summary-before` 保留修复前环境端点/宿主 adapter 的失败记录；对应测试源另存 `.go.txt`，修复前是在 `65e65b86` 上添加该文件后运行同名测试。
- Bridge 固定为 `a2316d747b092b5e1fd300f6ed26efe7eb61ce12`；Nexus 使用 `v0.1.34-0.20260917020413-a2316d747b09`。模块由 `golang.org/x/mod/zip.CreateFromVCS` 从本地提交生成，使用 `GONOPROXY=none` 的临时 file proxy 下载；未发布，不能声称干净机器可取得。
- Nexus 测试时的基线提交与精确代码差异见 `nexus-source.patch`，关键文件哈希记录于报告。归档文档与证据不影响这些代码。
- 固定基线显式要求 128 个顶层和 285 个具名子场景通过；缺失、失败或必测 skip 都使脚本失败。宽范围 SDK/Nexus 套件中的可选 skip 单独记录，不作为原生通过证据。
- `bridge-real-current` 覆盖当前十一项能力；`bridge-real-legacy` 仅重复验证 `c90c7f7c` 缺写能力时拒绝。其他历史能力缺失组合沿用前批证据。`nexus-tool-identity` 在实际 Nexus 包中验证主分支新增的工具调用身份；Bridge 竞态命令中的同名 pattern 不算该测试证据。
- Windows/Linux 的 `-exec=/usr/bin/true` 只证明交叉编译；没有原生行为或安装包验收。

复核时先恢复报告列出的精确本地模块与 SDK 提交，再运行基线脚本和报告中的检查命令；临时路径必须更新为本机已核验构建。不得把本机缓存、局部原生检查或进程内 unknown 栅栏当成发布、跨进程事务或重启恢复证明。
