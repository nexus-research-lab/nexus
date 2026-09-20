# 2026-09-20 SDK 跨物理根 settings journal 基线

这份证据由 Nexus `75970164d67cdab0e1558df4a6d0a1900bd1d0bf` 的固定门禁生成，SDK 使用本地提交 `9956def130da33af47accf799a9c27c16a551104` 的 clean archive，Bridge 使用 go.mod 中精确 pin 的本地模块 `v0.1.34-0.20260920062457-436346420c29`。门禁命令为：

```text
GOWORK=off GOPROXY=off node scripts/desktop/check-sandbox-baseline.mjs \
  --sdk-source /Users/berhand/program/Work/Nexus/worktrees/nexus-agent-sdk-go/desktop-sandbox \
  --sdk-ref 9956def1
```

最终退出码为 0，全部检查通过且没有必测 skip。`settings-writers` 现在包含 47 个通过事件，其中新增 `TestSettingsJournalCrossRootMixedStateFailsClosed` 验证用户与项目物理根共享 transaction ID 时，一根旧一根新会保留两份 journal 并失败关闭。固定 archive 构建的 nxs SHA-256 为 `e004c631ec555df466c14e13fc091e53f29a0ca4e113e1cd81031c0a2bab0f84`。

该证据只覆盖本机 macOS arm64 的开发门禁、SDK journal 恢复分类和既有 nxs 能力回归；它不证明跨根掉电写入本身 all-or-nothing、持久领域 receipt/reconcile、Provider 秘密文件/句柄/网络、Claude 原生 OS 沙箱、Windows/Linux 原生、clean-host、签名安装包或生产发布。`releaseAccepted=false`，所有提交仅本地、未推送。

原始报告、命令日志与 `manifest.sha256` 在本目录中。
