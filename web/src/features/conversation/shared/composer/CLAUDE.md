# composer/

L4 | 父级: web/src/features/conversation/shared

## 职责

- `composer-panel.tsx`: Composer 各子域的纯视图装配
- `controller/`: 草稿状态、消息投递、Goal/Loop、IME 与视图状态编排
- `use-composer-history.ts`: 发送历史记录和上下键召回
- `composer-model.ts`: 输入策略、键盘规则和布局状态表
- `use-composer-mention.ts`: 以单一匹配对象管理 Room 成员提及，并复用共享 Mention 文本模型
- `use-conversation-composer-handlers.ts`: DM/Room 对 Composer 的发送适配
- `attachments/`: 以单一规则表统一附件分类、批量校验、上传准备和本地展示
- `components/`: 输入行、提交动作、Footer、待发送队列和 Loop 选择器

输入、运行时、模式和动作状态先在控制器中分别投影，再组装为扁平视图契约；面板不得重新解释发送条件和提示文案。
运行时投影必须保留明确的发送、回复和上下文压缩阶段，Footer 不从通用 loading 状态猜测压缩行为。
发送目标先投影为 `send/enqueue + delivery policy`，消息提交按资格判断、附件准备、投递和收尾分阶段执行。
中文输入法的 composition 保护属于控制器边界，键盘命令执行前必须按顺序经过 composition、Safari 补发 Enter 和 Mention 导航守卫；Safari 守卫只消费 composition 结束后的 Enter 并阻止浏览器默认提交。
输入区 Props 由 DM/Room 的真实消费面定义，不保留无调用者的兼容参数。
紧凑 Composer 只用于手机与窄窗专注模式：外层至少保留 16px 横向安全留白，较宽窄窗保持 720px 居中上限，底部留白必须覆盖常规间距与系统 safe area；不得把输入壳铺满整个视口。
常规桌面 Composer 在底部保留 16px 呼吸区，使输入壳与窗口边缘分离；不得通过改变输入壳自身高度模拟抬升。
常规桌面 Composer 与消息轨道使用 `conversation-panel-styles.ts` 的同一宽度阶梯；超宽屏允许 Composer 比正文轨道略宽，但两者必须保持同一中心线。
队列命令、停止动作和附件准备是 DM/Room 的共同必需能力，不恢复无真实消费者的可选处理器分支。
Mention 目标只投影成员标记和标签；匹配、插入、键盘与浮层规则归 `shared/ui/mention/`。
附件必须先整批校验再上传；DM/Room 只提供目标作用域，不得复制格式规则或上传循环。
