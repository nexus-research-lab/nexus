# hooks/conversation/

L3 | 父级: `web/src/hooks/CLAUDE.md`

## 成员清单

- `use-assistant-content-merge.ts`: 合并并去重一轮内多条 Assistant 消息的内容块，维护流式输出索引
- `use-message-height.ts`: 将消息一次投影为可见文本和工具块指标，批量估算虚拟列表轮次高度
- `use-session-loader.ts`: 加载 Session 消息并管理请求生命周期
- `use-session-round-index.ts`: 从 Session 消息投影轮次索引

## 边界

- 高度估算只消费消息契约中可见且有稳定估算规则的内容；新增内容块类型必须显式声明是否参与文本或固定块高度。
- 消息指标在一次遍历中完成，禁止为文本、工具或后续指标分别扫描同一轮消息。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
