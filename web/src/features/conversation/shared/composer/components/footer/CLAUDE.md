# footer/

L6 | 父级: web/src/features/conversation/shared/composer/components

## 职责

- `composer-footer.tsx`: 只装配动作、Goal 标记、状态和元数据
- `composer-footer-actions.tsx`: 构造动作菜单并按动作表分派命令
- `composer-footer-status.tsx`: 展示唯一的当前运行状态
- `composer-footer-metadata.tsx`: 展示字符数、历史位置和当前 Agent 内核
- `composer-footer-model.ts`: 定义状态优先级和视觉投影

Footer 不解释 Composer 发送资格；它只消费控制器已经派生的状态。新增状态必须进入有序候选表，不能扩展 JSX 条件链。
上下文压缩沿用运行状态指示器和停止提示，并显示在 Composer 底栏的现有状态位置。
