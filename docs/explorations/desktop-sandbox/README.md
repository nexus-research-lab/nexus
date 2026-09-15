# 桌面沙箱改造文档入口

状态：实验集成，默认关闭。此目录为 non-normative 的开发资料；当前产品合同唯一入口是 [桌面沙箱规范](../../specs/desktop-sandbox-spec.md)。

## 阅读顺序

| 需要了解什么 | 文档 | 维护方式 |
| --- | --- | --- |
| 现在实现了什么、有什么问题、相比 Codex 缺什么 | [2026-09-15 现状评估](current-assessment-2026-09-15.md) | 固定审计快照，不追加实施流水 |
| 完整目标架构、阶段依赖、下一步和 Goal | [完整开发计划](development-plan.md) | 剩余工作的唯一状态入口 |
| 当前实际生效的产品行为 | [当前规范](../../specs/desktop-sandbox-spec.md) | 只随已实现合同更新 |
| 怎么验证、哪些测试真正通过 | [验收矩阵](../../testing/desktop-sandbox-acceptance.md) | 记录命令、退出码和证据范围 |
| Codex 参考路径与 Windows 实验差异 | [固定源码审计](codex-source-audit.md) | 保留固定提交及原生证据，新的执行状态进入开发计划 |
| 原 Windows broker 候选及部署约束 | [Windows 部署候选](windows-broker-plan.md) | 候选，不能当作已选架构 |
| 过去做过哪些尝试 | [原始计划与实施历史](implementation-history.md) | 冻结归档，保留失败证据 |

## 维护约定

- 当前合同、未来设计、审计快照与历史实验分别维护；代码存在不等于安装包完成验收。
- 每项进度必须区分源码、单测、原生行为、跨仓进程与安装包实机证据。
- 已完成阶段更新当前规范和开发计划，不在多份文档复制状态。
- Windows 每条命令经过高权限 broker 的方案尚未选定；先完成参考启动组合与隔离验收。
- SDK、Bridge 和 Nexus 的版本、dirty 范围及实际打包 binary 必须同时记录。
