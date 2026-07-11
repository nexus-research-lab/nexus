# 共享 UI 原语

- 根目录只保留无法归入具体交互职责的基础原语。
- 组件按 `button/`、`form/`、`display/`、`list/` 与 `navigation/` 分组，消费者直接导入职责文件。
- 不提供聚合导出，也不在共享层解释 Agent、Room 或其他业务协议。
