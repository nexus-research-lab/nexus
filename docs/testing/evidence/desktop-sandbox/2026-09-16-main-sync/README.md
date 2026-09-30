# main 同步证据（2026-09-16）

本目录记录 main 与沙箱分支的兼容修复，不是完整沙箱发布验收。源码版本、二进制哈希和各检查结果见 `report.json`。原始 Go test JSONL 使用 gzip 压缩；解压后保持原始字节，报告另记录解压前原始 SHA256。

- `identity-before` 是合并 main 后旧 Bridge 丢失真实工具身份的失败证据。
- `nexus-pinned-race` 保留第一次扩展回归，包括两处测试夹具失败；`approval-fixtures-final` 是修正后的两包最终结果。未修改的其他包沿用首次结果。
- `consumer-replace-check` 仅验证中途本地源码组合；最终固定模块由 `nexus-pinned-race`、`approval-fixtures-final`、`real-current`、`real-legacy-settings` 和 `nexus-real-process` 验证。
- `real-current` 核验当前十项能力；`real-legacy-settings` 只核验旧 SDK 缺普通配置能力时的拒绝。其他历史版本组合仍引用上一批记录，未在本批重复验收。
- `settings-write-before_test.go.txt` 与 `settings-write-before` 是下一项配置持久化的红色反例。该测试通过 Go overlay 添加到 SDK `internal/agent/runtime/settings_write_test.go`；SDK 工作树仍干净，四个失败不是修复后验收。

`run-real-process.py` 保留实际使用的路径和环境作为本机复核记录，临时路径不是发布渠道。恢复环境时须从报告列出的 SDK/Bridge 提交重新构建并更新路径，不复用未知 binary。当前 Bridge 模块尚未发布，禁止声称干净机器可直接拉取。
