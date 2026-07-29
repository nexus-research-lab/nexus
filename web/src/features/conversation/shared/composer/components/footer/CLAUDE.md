# footer/

L6 | 父级: web/src/features/conversation/shared/composer/components

## 职责

- `composer-footer.tsx`: 以输入壳命名容器装配动作、Goal 标记、状态、元数据和提交动作，并在中心显示可按容器宽度收敛的 `Powered by Nexus`
- `composer-footer-actions.tsx`: 构造动作菜单并按动作表分派命令
- `composer-footer-status.tsx`: 展示唯一的当前运行状态
- `composer-footer-metadata.tsx`: 展示字符数和历史位置
- `composer-footer-model.ts`: 定义状态优先级和视觉投影

Footer 不解释 Composer 发送资格；它只消费控制器已经派生的状态。新增状态必须进入有序候选表，不能扩展 JSX 条件链。
Footer 两侧使用等分的 `minmax(0, 1fr)` 保证品牌相对输入壳物理居中；品牌颜色必须浅于普通 `text-soft`，但不得影响两侧操作的对比度。窄壳响应只通过 `nexus-chat-composer` 容器查询完成。Goal 模式在 520px 以下把品牌移到第二行居中，第一行让 Goal 控制与提交动作共享完整宽度；只允许收敛 scope 等说明，不得裁切负责人、取消或提交动作。
上下文压缩沿用运行状态指示器；停止提示只在 DM Composer 明确提供停止能力时显示，Room 的停止按钮由 Agent slot 头部渲染。
