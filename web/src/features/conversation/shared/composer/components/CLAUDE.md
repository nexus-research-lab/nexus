# components/

L5 | 父级: web/src/features/conversation/shared/composer

## 职责

- `composer-input-row.tsx`: 装配 Mention、textarea 和快捷键提示
- `composer-submit-button.tsx`: 以单一投影选择停止、加载、Goal 或发送动作
- `footer/`: 动作菜单、Goal 标记、运行状态和输入元数据
- `pending-queue/`: 待发送消息、拖拽重排和队列命令
- `loop-picker/`: Loop 目录资源、筛选、选择事务和 Dialog 展示

组件只消费控制器或本子域模型的明确结果，不重新派生发送资格、运行时阶段或跨域协议状态。
