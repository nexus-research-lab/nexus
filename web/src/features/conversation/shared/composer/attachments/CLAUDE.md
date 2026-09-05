# attachments/

L5 | 父级: web/src/features/conversation/shared/composer

## 职责

- `composer-attachments.ts`: 以有序规则表统一附件分类、文件选择过滤、批量校验和上传投影
- `composer-local-attachment-model.ts`: 管理剪贴板动作、本地标识、批次投影和待发送附件模型
- `use-composer-attachments.ts`: 按动作表执行粘贴策略，消费草稿胶囊中的受控附件，并管理错误翻译和发送前准备生命周期
- `composer-local-attachments.tsx`: 展示可点击图片缩略图、可点击文本胶囊、普通文件胶囊及独立移除动作
- `composer-attachment-preview-dialog.tsx`: 复用共享 Dialog，以仅保留文件名的紧凑标题栏提供适度尺寸的大图灯箱和最多 512 KB 的只读文本预览
- `use-composer-local-file-url.ts`: 独占图片缩略图和灯箱使用的 Object URL 创建与释放

附件批次必须先完整校验，再产生上传副作用，避免留下半批资源。
Agent Workspace 与 Room Conversation 只提供上传目标和作用域字段，不复制分类规则或上传循环。
图片与文本预览属于瞬时 UI；切换 Session 时关闭，草稿附件本身仍按 Room/DM 作用域恢复。预览按钮和移除按钮必须保持独立，不能由冒泡导致误删除或误打开。
图片和文本预览统一使用 `UiButton`，所有移除动作统一使用 `2xs` IconButton；图片角上的移除层只定义定位，复用共享的圆形、danger 与 surface 状态。48px 图片框属于内容缩略图几何，不再拥有第二套按钮 DOM、圆角、hover 或焦点规则。
附件 Chip 与输入壳的圆角由共享 recipe 唯一拥有；Composer 样式只保留自身壳层标记和布局，不重复覆盖共享值。共置测试覆盖独立命令、Session 预览重置与 Object URL 释放，浏览器夹具验证真实文件解码、预览/移除和键盘焦点。
附件文件名是预览唯一可见标题；不得再重复“图片预览”“文本预览”等可由内容直接判断的类型说明。
协议拒绝结果使用结构化错误码，用户文案由 Composer 的 i18n 消费层统一生成。
剪贴板只投影为原生粘贴、追加文件、追加长文本或拒绝 Goal 附件四种动作，Hook 不维护条件链。
