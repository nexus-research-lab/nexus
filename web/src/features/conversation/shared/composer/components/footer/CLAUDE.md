# footer/

L6 | 父级: web/src/features/conversation/shared/composer/components

## 职责

- `composer-footer.tsx`: 以输入壳命名容器装配动作、Session 设置、Goal 标记、状态、元数据和提交动作，并在中心显示可按容器宽度收敛的 `Powered by Nexus`
- `composer-footer-actions.tsx`: 构造附件、本机文件夹、Goal/Loop 动作菜单、当前 Session Connector 显式开关并按动作表分派命令
- `composer-session-controls.tsx`: 为 DM 装配直接模型/权限菜单，为 Room 装配统一权限与右侧模型入口
- `composer-room-model-control.tsx`: 复用公共锚定浮层和 Action Menu 内容，按 Room Agent 级联其当前 Session 模型
- `composer-session-control-options.tsx`: 统一投影 DM 与 Room 共用的模型/权限选项
- `composer-context-usage*.ts*`: 把 runtime 每轮快照投影为模型控件左侧的只读上下文占用环；Room 入口显示最高占用，弹层逐 Agent 展示各自快照
- `composer-footer-status.tsx`: 展示唯一的当前运行状态
- `composer-footer-metadata.tsx`: 展示字符数和历史位置
- `composer-footer-model.ts`: 定义状态优先级和视觉投影

Footer 不解释 Composer 发送资格；它只消费控制器已经派生的状态。新增状态必须进入有序候选表，不能扩展 JSX 条件链。
Footer 的普通动作、模型/权限触发器和返回动作必须使用共享 `UiButton` / `UiIconButton`；只有同时承担菜单行布局、悬浮选择或数据可视化的复合控件可以保留专用 button DOM，不能复制圆角、hover、focus 或 disabled recipe。
模型与权限控制只写 Session 覆盖；空值表示继承。DM 不重复显示 Agent 目标，直接展示权限与模型；Room 权限复用同一直接菜单，由后端事务同步到当前 Conversation 的全部主 Agent Session，Room 模型则在右侧先列 Agent。横向空间足够时，悬浮或点击 Agent 直接在右侧级联模型选项；空间不足时点击后在同一浮层逐级进入。模型目标只存在于浮层内部并在关闭后忘记，不能暗示它参与消息路由。具体模型/权限列表不添加“跟随默认”伪选项，统一在底部提供重置；权限项用语义图标、标题和单行短说明，保持 288px 默认密度，模型项保持单行并使用 256px 紧凑密度，Room Agent 选择面板再窄一档。运行中禁用设置，失效事件到达后按 Session key 精确重读。
普通模式两侧使用等分的 `minmax(0, 1fr)` 保证品牌相对输入壳物理居中；品牌颜色必须浅于普通 `text-soft`，但不得影响两侧操作的对比度。窄壳响应只通过 `nexus-chat-composer` 容器查询完成。Goal 模式不沿用等分三列：控制与提交共享第一行，运行状态独占第二行并居中，品牌退场；460px 以下再把提交动作放入独立第三行。“目标”标签始终保持单行，只允许收敛 scope 等说明，不得裁切负责人、取消、状态或提交动作。
上下文占用只消费 runtime 在 round 终态推送的权威快照，不轮询 transcript，也不由前端估算。DM 显示当前 Session；Room 按 Agent ID 保存并在同一个弹层逐行展示各自最近快照，入口圆环使用最高占用提示风险，不能暗示当前消息只发给某个 Agent。入口直接使用 `UiIconButton size="sm"` 的 28px 命中区，首个快照到达前保留同宽的不可交互槽位；发送与停止动作统一为 32px 圆形图标按钮，避免 round 终态改变 Footer 几何而使 Composer 跳动。
上下文压缩沿用运行状态指示器；正文流与停止按钮已经表达回复进行中，Composer 不再重复显示“回复中”文案或动效，只保留可执行的停止快捷键提示。停止提示只在 DM Composer 明确提供停止能力时显示，Room 的停止按钮由 Agent slot 头部渲染。

Footer 的 Connector 初始读取使用共享 `md` muted Spinner，Room Agent 模型更新使用 `sm` muted Spinner；菜单与级联控件不得自行维护尺寸、颜色、旋转或 reduced-motion class。
