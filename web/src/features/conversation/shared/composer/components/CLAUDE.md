# components/

L5 | 父级: web/src/features/conversation/shared/composer

## 职责

- `composer-input-row.tsx`: 装配 Slash/Mention 补全与 textarea，提交和其他动作留在底部工具行
- `slash-command-popover.tsx`: 展示 runtime 命令目录及加载、不可用和空筛选状态
- `composer-submit-button.tsx`: 以单一投影选择停止、加载、Goal 或发送动作
- `interaction/`: DM 等待用户确认、回答或批准计划时，原位替换输入壳的唯一交互 surface
- `footer/`: 动作菜单、Goal 标记、运行状态、输入元数据和提交动作
- `pending-queue/`: 待发送消息、拖拽重排和队列命令
- `loop-picker/`: Loop 目录资源、筛选、选择事务和 Dialog 展示

组件只消费控制器或本子域模型的明确结果，不重新派生发送资格、运行时阶段或跨域协议状态。
