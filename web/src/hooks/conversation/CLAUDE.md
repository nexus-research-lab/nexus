# hooks/conversation/

L3 | 父级: `web/src/hooks/CLAUDE.md`

## 成员清单

- `use-assistant-content-merge.ts`: 合并并去重一轮内多条 Assistant 消息的内容块，维护流式输出索引
- `use-message-height.ts` 在 DM 与群聊虚拟列表中估算轮次高度。相同列宽下复用当前列表中正文片段未变的测量结果，工具块按当前消息计数，活跃轮次使用固定基线。
- `use-message-height.test.ts` 覆盖同一轮次更新正文、工具块或列宽时的估高复用与失效。
- `use-session-loader.ts`: 加载 Session 消息并管理请求生命周期
- `use-session-round-index.ts`: 按 exact Session 读取轮次索引；失败保留最后成功快照并暴露 stale/error/retry，scope 或权限失效清空

## 边界

- 高度估算只消费消息契约中可见且有稳定估算规则的内容；新增内容块类型必须显式声明是否参与文本或固定块高度。
- 活跃轮次不得消费尚未 reveal 的完整 backend target 估高，避免先膨胀再由 measured size 大幅回缩。
- 消息指标在一次遍历中完成，禁止为文本、工具或后续指标分别扫描同一轮消息。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
