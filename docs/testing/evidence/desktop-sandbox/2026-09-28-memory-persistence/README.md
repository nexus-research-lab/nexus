# macOS 记忆初始化与会话摘要文件边界

2026-09-28，macOS arm64 本机验证。**52 项检查、767 个必测名称全部通过**，无必测缺失或跳过；`releaseAccepted=false`。

## 固定来源

- Nexus：`50db89fa1f1024a7c8bc34024b8c2daa46842ecc`，运行开始时工作树干净。
- SDK：`0392af36c6ff52e9f27ed960b8e73ecfb3fe5e00`，由固定 Git 归档构建，运行开始时工作树干净。
- Bridge：`v0.1.34-0.20260927182153-4b2972f09143`，checksum `h1:61JYZ5QUkuhaEb1Y0Bxe17ETd6CYolEQVQgZdb+bNRc=`，`GOWORK=off`，没有 replace。
- nxs SHA-256：`1ad05c78aa151326d47ba1e5e3997203754c304a9dc7c1ef54d7abd0d29f1054`。

```sh
node scripts/desktop/check-sandbox-baseline.mjs \
  --sdk-source /absolute/nexus-agent-sdk-go \
  --sdk-ref 0392af36c6ff52e9f27ed960b8e73ecfb3fe5e00
```

## 修复与验证

修复前，在 SDK `5281d803` 的导出源码上加入归档的原生夹具，复现 10 个拒绝场景失效：记忆目录、索引和根链接初始化越界；只读会话创建布局；compact 读取禁止的摘要；Summary 准备直接读取禁止的正文、模板、提示词和链接目标，以及创建禁止写入的摘要。两个允许对照通过。完整原始失败日志和当时的夹具都保留；最终夹具另加了正常初始化、新摘要与自定义模板三个允许场景，并非同一份测试文件。

| 验证 | 结果 |
| --- | --- |
| 原生初始化、只读启动、compact 与 Summary 准备 | 18 个测试名称通过；禁止内容未进入模型，禁止位置未创建文件，既有索引和摘要内容保留 |
| 独占创建 | 已有普通文件及链接目标不覆盖；16 个并发创建者只有一个成功创建；无效请求不触碰文件 |
| 已执行但 helper 异常退出 | 返回 unknown，保留原文件，未重试或按路径删除；即使已收到成功响应也核对进程终态 |
| 摘要创建竞争和读取错误 | 竞争者内容保留；拒绝不是缺失；未知创建不重读、不重放 |
| 相关包与静态检查 | 相关包竞态检查和最终 vet 通过，见下述中间失败说明 |
| 完整固定基线 | 52 项/767 个必测名称通过，包括现有宿主审批、关闭、恢复、MCP、文件、媒体、Skill、配置和原生命令检查 |

最初竞态检查中，worker 测试误用了高于模块 Go 1.24 要求的 `WaitGroup.Go`；其他相关包通过，但该次整体结果为失败。改为兼容语法后，worker 及最终 worker/sandboxfs 竞态检查通过。初次编译的未使用 import、初次 vet 的版本错误均保留原日志，不能算作通过。原生拒绝场景使用普通构建验证，不能把独立包竞态检查表述为所有原生场景均通过 race。

首次完整基线在两个宿主启动夹具上失败：它们没有创建产品 `Agent.EnsureReady` 会准备的用户 runtime 临时目录。修正夹具，复用 `EnsureUserRuntimeLayoutAt` 创建隔离目录后，两个真实 nxs 启动用例及最终完整基线通过；没有放宽沙箱或关闭记忆来通过测试。

## 归档与边界

`baseline-report.json` 保存最终完整报告；`baseline-logs.tar.gz` 保存其 104 份命令日志；`focused-logs.tar.gz` 保存 16 份修复前后、竞态/静态检查、首次失败基线及原始夹具。`manifest.json` 保存来源及每个归档/条目的 SHA-256。归档只包含所列日志、报告和测试夹具，不包含数据库、运行时二进制、完整源码包或 Provider 凭据。

本批初始文件使用独占创建，不能据此宣称内容原子发布或多文件掉电事务。AutoDream 锁/完成标记写入、历史会话辅助 IO、任意脱离后代监督、一般 unknown 恢复、秘密文件/句柄、Provider 网络出口与完整安装发布仍未闭合。没有重做真实 App 窗口、第三方模型、Intel 或 Windows 验收；此前 App 证据保持各自原固定版本，不归入本批 SDK。
