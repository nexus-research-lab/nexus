# attachments/

L5 | 父级: web/src/features/conversation/shared/composer

## 职责

- `composer-attachments.ts`: 以有序规则表统一附件分类、文件选择过滤、批量校验和上传投影
- `composer-local-attachment-model.ts`: 管理剪贴板动作、本地标识、批次投影和待发送附件模型
- `use-composer-attachments.ts`: 按动作表执行粘贴策略，并管理错误翻译和发送前准备生命周期
- `composer-local-attachments.tsx`: 只负责待发送附件的展示和移除交互

附件批次必须先完整校验，再产生上传副作用，避免留下半批资源。
Agent Workspace 与 Room Conversation 只提供上传目标和作用域字段，不复制分类规则或上传循环。
协议拒绝结果使用结构化错误码，用户文案由 Composer 的 i18n 消费层统一生成。
剪贴板只投影为原生粘贴、追加文件、追加长文本或拒绝 Goal 附件四种动作，Hook 不维护条件链。
