# Notebook 文件能力证据（2026-09-18）

本目录记录独立 Notebook 文件能力批次。SDK 固定提交 `7bc597ea3c9db479b561d6203fb2a8d03698c982`，Bridge 固定提交 `8a4576ba97ece60e0485f2bfbb0bce53e5b89502`，Nexus 使用本地模块 `v0.1.34-0.20260918053632-8a4576ba97ec`，checksum 为 `h1:nYthJgS+xL7KZayRvMhy086O/xXtJ2KcqGjB/xMPwyo=`。模块来自本机 `file://` proxy，未发布。

Nexus 桌面 nxs 现在独立要求 `required_sandbox_notebook_files=true`，并把 `sandbox_notebook_files_v1` 纳入能力协商和进程策略指纹。Bridge 在首条任务前验证该能力；旧 nxs、只有命令/普通文件能力的 binary、缺文件能力或不受支持平台都失败关闭。Notebook 内容与 cell output 的读取复用受限文件执行器。

验证命令和最终退出码保存在 `report.json`；原始 stdout/stderr 保留为本目录的 `.log` 文件。固定 nxs SHA-256 为 `b1aebef92731ab9a1397136b2e1656407d8c2b59818b71d9ca4c71025f0c52b5`。本批次没有发送模型请求。

当前范围仍有限：只证明本地 Notebook 读取准入和 macOS 文件执行路径，不证明 Notebook 执行、远程网络、完整 SDK IO、Provider/秘密文件/句柄隔离、崩溃恢复、Windows/Linux 原生限制、Claude 或签名安装包。`releaseAccepted=false`，所有提交仅本地、未推送。
