# footer/

L6 | 父级: web/src/features/conversation/shared/composer/components

## 职责

- `composer-footer.tsx`: 以输入壳命名容器装配动作、Session 设置、Goal 标记、状态、元数据和提交动作，并在中心显示可按容器宽度收敛的 随当前内核切换的 `Powered by Nexus` / `Powered by Claude`
- `composer-footer-actions.tsx`: 构造附件、本机文件夹、Goal 动作菜单、当前 Session Connector 显式开关并按动作表分派命令
- `composer-session-controls.tsx`: 为 DM 装配直接模型/权限菜单，为 Room 装配统一权限与右侧模型入口；领先权限入口共用一条装配分支。直接菜单按 target.sessionKey 重置，busy/disabled 关闭后不得自动恢复。DM 选择继承权限写空 override，Room 仍显式写所选模式。
- `composer-room-model-control.tsx`: 复用公共锚定浮层、菜单键盘和 Action Menu 内容，按 Room Agent 级联其当前 Session 模型；定位完成才给 Agent 菜单初始焦点，点击/右方向键进入模型，左方向键或首个 Escape 返回原 Agent，第二次 Escape 才退出。悬浮只更新级联目标，不抢焦点；选择和 Tab 退出归还入口。行高估算共用 getMenuItemLayout，领域只拥有菜单宽度与级联结构。
- `composer-session-control-layout.ts` 统一 DM/Room 模型的 256px 内容宽度、Room 返回栏 recipe 和可见面板总高。双栏是否可用由两列内容、公共间距和视口留白组成的媒体查询决定，订阅使用公共 useMediaQuery；定位/限宽继续交给 Overlay，单栏 Panel 填满约束后的宽度。宽窄两种布局只渲染同一份 RoomModelOptions，保留真实列表 DOM、焦点和滚动；返回栏计入整体限高，正文独立滚动，不让两套选项 JSX 漂移。
- Room 菜单为用户选中的 Agent/Session 保存临时绑定，目标失效或换 Session 时关闭；提交前还要对上当前控制器目标，不因目录刷新把迟到点击落到另一个 Session。持久设置和失败对账仍完全归控制器。布局变化只在原焦点节点已移除且焦点掉到 body 时恢复，不抢外部控件或上层模态焦点。
- `composer-session-control-options.tsx`: 统一投影 DM 与 Room 共用的模型/权限选项，并唯一分派模型值的解码、继承重置与显式更新；控制器继续拥有持久化/失败对账，非法值不能变成清空配置。模型名占剩余空间，Provider 元信息最多占 40%，两者保留完整原文提示；Provider 元信息直接使用公共 metadata/muted，复合标签继续保持单行几何，不能回到私有 10px 淡字。
- `composer-context-usage*.ts*`: 把 runtime 每轮快照投影为模型控件左侧的只读上下文占用环；Room 入口显示最高占用，唯一详情浮层逐 Agent 展示各自快照；详情按实际内容展开到公共 preset/视口上限，标题固定、列表内部滚动，不用估算行高裁切成员；入口关闭 IconButton 自动 Tooltip，只读详情开关保持原焦点
- `composer-footer-status.tsx`: 展示唯一的当前运行状态；只把 model 的语义 tone 和 indicator 交给共享 Typography/LoadingOrb，不让状态正文整体 pulse
- `composer-footer-metadata.tsx`: 展示字符数和历史位置
- `composer-footer-model.ts`: 定义状态优先级、语义 tone 和加载阶段，不得返回 class、字符帧或动效名称

Footer 不解释 Composer 发送资格；它只消费控制器已经派生的状态。新增状态必须进入有序候选表，不能扩展 JSX 条件链。
Footer 的普通动作、模型/权限触发器和返回动作必须使用共享 `UiButton` / `UiIconButton`；只有同时承担菜单行布局、悬浮选择或数据可视化的复合控件可以保留专用 button DOM，不能复制圆角、hover、focus 或 disabled recipe。
模型与权限控制只写 Session 覆盖；空值表示继承。DM 不重复显示 Agent 目标，直接展示权限与模型；Room 权限复用同一直接菜单，由后端事务同步到当前 Conversation 的全部主 Agent Session，Room 模型则在右侧先列 Agent。横向空间足够时，悬浮或点击 Agent 直接在右侧级联模型选项；空间不足时点击后在同一浮层逐级进入。模型目标只存在于浮层内部并在关闭后忘记，不能暗示它参与消息路由。具体模型/权限列表不添加“跟随默认”伪选项，统一在底部提供重置；权限项用语义图标、标题和单行短说明，保持 288px 默认密度，模型项保持单行并使用 256px 紧凑密度，Room Agent 选择面板再窄一档。运行中禁用设置，失效事件到达后按 Session key 精确重读。
普通模式两侧使用等分的 `minmax(0, 1fr)` 保证品牌相对输入壳物理居中；品牌颜色必须浅于普通 `text-soft`，但不得影响两侧操作的对比度。窄壳响应只通过 `nexus-chat-composer` 容器查询完成。Goal 模式不沿用等分三列：控制与提交共享第一行，运行状态独占第二行并居中，品牌退场；460px 以下再把提交动作放入独立第三行。“目标”标签始终保持单行，只允许收敛 scope 等说明，不得裁切负责人、取消、状态或提交动作。
上下文占用只消费 runtime 在 round 终态推送的权威快照，不轮询 transcript，也不由前端估算。DM 显示当前 Session；Room 按 Agent ID 保存并在同一个弹层逐行展示各自最近快照，入口圆环使用最高占用提示风险，不能暗示当前消息只发给某个 Agent。入口直接使用 `UiIconButton size="sm"` 的 28px 命中区，首个快照到达前保留同宽的不可交互槽位；发送与停止动作统一为 32px 圆形图标按钮，避免 round 终态改变 Footer 几何而使 Composer 跳动。
上下文压缩沿用运行状态指示器；正文流与停止按钮已经表达回复进行中，Composer 不再重复显示“回复中”文案或动效，只保留可执行的停止快捷键提示。停止提示只在 DM Composer 明确提供停止能力时显示，Room 的停止按钮由 Agent slot 头部渲染。

Session 设置可靠性提示在输入壳外显示紧凑单行，影响说明点击后通过共享 Dialog 展示，刷新/重试独立可操作并显示忙碌状态；不得嵌入整块资源错误卡撑高输入框。

可靠性提示的刷新图标固定紧邻标题，详细说明在弹窗中展示；打开弹窗不改变刷新位置，图标保留可访问名称与忙碌反馈。

修改 Session 设置失败必须自动弹出完整错误 Dialog，关闭时清除此 Session 的本次 mutation failure；不为修改失败保留输入区错误条或刷新动作。资源读取错误仍使用独立读取恢复入口。
Footer 的 Connector 初始读取使用共享 `md` muted Spinner，Room Agent 模型更新使用 `sm` muted Spinner；菜单与级联控件不得自行维护尺寸、颜色、旋转或 reduced-motion class。

- Footer 的品牌、Goal/运行状态、计数和上下文详情消费 Typography；辅助文字层级以根 design.md 为准，空计数不保留 flex 间距。状态投影仍由 footer-model 唯一排序，视图不复制 LoadingOrb 或优先级。
- Footer Actions 的 Goal/Connector 使用 Action Menu 受控 checked；每行只有一次激活，不嵌套 Switch 或用 stopPropagation 保留第二条切换路径。目录选择服从控制器已有 blocksMutation，附件和其他无关命令保持独立。
- Context Usage 的 hover/focus/click 都展开同一详情；点击不反转 focus 刚打开的状态，鼠标离开不关闭仍有键盘焦点的指标。快照可用性变化清空打开态，普通占用更新保留详情；内容和关闭时序不改变终态 snapshot 权威。
- Session settings reliability 的资源读取错误按 Session/Provider/Connector 路由到对应恢复命令，读忙碌防重；没有恢复命令的资源不显示空操作。

- 读取错误详情打开态按目标 Session、资源种类和写入错误遮挡状态隔离，读取恢复后再次失败不会自动重开旧详情。
