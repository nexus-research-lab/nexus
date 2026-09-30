# macOS 记忆召回与抽取清单读取（2026-09-28）

本批修复 nxs 记忆召回绕过文件沙箱的问题。固定版本的宿主和 macOS 原生基线全部通过；
不声明整个后台记忆 IO 或正式安装包已经验收。

## 固定来源

- Nexus `d05a18b53604e4141e72d5f7039c0f500c3a206d`。
- SDK `5281d803376a0300a30becabdc111de863ffe8f9`；修复前源码为 `5a937a18355c94d8766458b705d4b9eef7bd8a23`。
- Bridge `v0.1.34-0.20260927182153-4b2972f09143`，checksum `h1:61JYZ5QUkuhaEb1Y0Bxe17ETd6CYolEQVQgZdb+bNRc=`，无 replace 或 go.work。
- 基线开始时两个源码工作树均干净；nxs 从固定 SDK Git 归档构建，SHA-256 为 `4997df1078566d43a679b4bd7acb89106cb0a520d87bb95370039391def1096b`。
- macOS arm64；[基线报告](baseline-report.json)记录 50 项检查、724 个必测名称全部通过，包含新增的 35 个记忆/前缀读取测试名。必测缺失或跳过均不能通过。

## 缺陷与修复

原记忆扫描直接调用宿主 `WalkDir`、`Stat`、`Open`，选中正文再次直接 `Open`。
在真实 Seatbelt 夹具下，明确禁止读取的普通文件、文件链接和目录仍进入 selector，
选中后把合法文件换成指向禁止目标的链接也把内容带入 attachment。
修复前四个拒绝场景的失败日志保留在 [focused-logs.tar.gz](focused-logs.tar.gz) 的
`recall-before.log`；全部使用隔离临时目录中的标记文本，没有读取真实秘密。

现在候选扫描、元数据、frontmatter、抽取 manifest 和选择后正文显式使用当前文件执行器。
根读取失败阻止该次读取，不可读候选跳过；端口缺失和准备失败不回退宿主 IO。
目录链接不递归，选中正文在执行端重新打开并检查；取消不发布部分 attachment。

新增的前缀读取在 helper 内限制实际读取量，在父端聚合数据帧前再次核对总量。
因此超过 32 MiB 的文件仍可按既有预算召回，普通完整文件流继续支持大文件。
记忆路径、排序和选择规则、每项 200 行/4096 字节及每会话 60KB 注入预算不迁移。
该实现随 Nexus/nxs 配套发布，不新增或扩大能力协商承诺。

## 验证与日志

| 范围 | 结果 |
| --- | --- |
| 原生记忆读取 | 普通文件、文件链接、目录的拒绝/允许对照；选择后换链；manifest 拒绝内容；大文件召回均通过 |
| 前缀与兼容 | JSON 完整读取保留原限额；流式大文件、短文件、空文件、超额响应拒绝、无效请求、目录循环、无宿主回退和取消均通过 |
| 相关包 | memory 子包、worker、sandboxfs、executor、runtime 共 9 个包的普通与 race 测试通过；原生 opt-in 用例由独立非 race 基线承担 |
| 最终取消逻辑 | `cancellation-race.jsonl` 的定向 race 通过；`runtime-final-vet.log` 对应最终 runtime 代码，另保留相关包 vet 日志 |
| 产品固定基线 | 宿主策略、生命周期、真实 nxs、MCP、文件/媒体/Skill/上下文/配置等 50 项全部通过；[100 份原始日志](baseline-logs.tar.gz)保留 |

**原生 race 尝试未通过。** `native-race-failed.log` 中选定用例在 runtime 初始化阶段全部
出现 deadline exceeded，本次未把这次尝试计入原生通过。相关包的 race 与原生非 race
隔离测试是分开的证据。`recall-first-fix.log` 属于最初的较小回归集合，最终结果以固定
提交基线为准。所有日志的归档及逐文件 SHA-256 见 [manifest.json](manifest.json)。

复验入口：

```sh
node scripts/desktop/check-sandbox-baseline.mjs \
  --sdk-source /absolute/path/to/nexus-agent-sdk-go \
  --sdk-ref 5281d803376a0300a30becabdc111de863ffe8f9
```

入口显式使用 `GOWORK=off` 并导出固定提交。本批不启动真人 App、不访问主数据库、不调用模型、
不运行 Windows 测试；日志归档不包含二进制、SDK 源码包、配置目录、凭据或数据库。

## 尚未覆盖

记忆 store 初始化、Summary 文件/自定义模板/提示词与压缩读取、AutoDream 锁和完成标记、
历史会话目录扫描仍需要单独接入受控边界，不能因本次 recall 修复视为全部完成。
新 SDK 没有新增 App 全链路重跑；[已有 App 审批与切换](../2026-09-28-app-contracts/README.md)
继续按其 SDK `5a937a18` 和各自 Nexus 提交使用，不重新标注为本批来源。

任意脱离后代监督、一般 cleanup_unknown 恢复、秘密文件/句柄、Provider 网络出口、可信 MCP
代理与真实外部服务、完整 UI 与历史 App 数据库升级/回退、Developer ID/公证、Intel 和干净机器
安装仍按[唯一开发计划](../../../../explorations/desktop-sandbox/development-plan.md)推进。
`releaseAccepted=false`，Goal 保持 active。
