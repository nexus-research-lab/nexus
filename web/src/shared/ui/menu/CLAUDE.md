# Shared Menu

- `menu-styles.ts` 的 getMenuItemLayout 同时拥有 compact/default、单行/说明行的渲染尺寸与估算高度；UiMenuActionRow、Action Menu 估算和 Mention Option 共用，禁止分别维护相同数字。
- `getMenuContentHeight` 统一累加行、分隔线、上下内边距和条目间距；Action、Workspace 与 Room 模型菜单直接消费。分隔线外观由 `MENU_SEPARATOR_CLASS_NAME` 持有，不再各自推导 footer 高度。Action 行透传原生 ref，且不参与 flex 压缩，长菜单通过外层滚动保留有限行高。
- `filter-select.tsx` 的 `UiFilterSelect` 是能力与联系人目录共用的具名筛选 Pattern：必填辅助名称，触发器只显示当前值与箭头，未筛选文案明确为全部分类/状态等；选项、值、命令和内容宽度由业务拥有，菜单与控件外观直接复用 `UiSelectMenu`；不接受前导图标或第二套按钮样式。交互和视觉合同见根目录 `design.md`。
- `select-menu-model.ts` 只计算当前选项、键盘遍历、高度估算和锚点几何，不得返回视觉类；`select-menu-styles.ts` 独占菜单共用的尺寸、表面、标签换行和选中态视觉 recipe。业务需要组合多选或特殊 listbox 时可以复用 style recipe，但不得从 model 导入样式。
- `use-select-menu-overlay.ts` 统一选择菜单的内部开关、锚点定位和触发键盘协议。
- `UiSelectMenu.resetKey` 由调用方描述选择上下文；变化时通过共享 resettable state 丢弃菜单打开态，不重建 trigger DOM，也不改变当前选值或外部焦点。Room Goal 传精确 Session 与候选身份集合，普通名称、顺序或语言更新不关闭菜单；无 resetKey 的既有消费者保持原行为。
- Select 进入 disabled 状态必须立即收起 listbox、清除 expanded/controls 关系并丢弃旧打开态；恢复可用后只能由用户重新打开。Select/Action Menu 的 Escape 和焦点归还交给共享 Overlay 仲裁，不自行抢先关闭父菜单。
- Select 的 `xs / sm / md / lg` 尺寸只由 `select-menu-styles.ts` 投影；目录筛选用紧凑档，和大号输入同排的字段用 `lg`，消费者不得通过 `h-*` 或 `buttonClassName` 另造高度、间距和阴影。
- `allowLabelWrap` 同时由该 recipe 持有触发器最小高度、垂直留白和外层自动高度；长活动标签必须完整位于触发器中，不能只换行文字再由业务修补固定壳高。普通单行模式保留原四档固定高度。
- `select-menu-primitives.tsx` 提供选择菜单共用的 `SelectMenuTrigger`、触发器内容、listbox 框架和 `SelectMenuOptionRow`；SelectMenuTrigger 统一单选与领域多选的原生 button、listbox ARIA、ref 和原生事件透传，直接消费既有样式投影，不改变调用者的开关、键盘或布局。所有单选、多选、Slash/Mention 类 listbox 条目由 OptionRow 持有原生 button、`role=option`、选中语义与基础交互底面，业务只组合行内容、密度和选择命令。
- `select-menu-view.tsx` 只渲染共享单选菜单，不读取业务状态或决定选值。
- 当前值与选项的可选徽标直接组合公共 `UiBadge`，不保留私有 badge helper、字号或颜色；`select-menu-styles.ts` 的各尺寸文字通过 `getUiTypographyClassName` 投影，具体档位以根目录 `design.md` 为准。
- Select trigger 通过 `form/field-accessibility.ts` 复用 Field 的精确说明/错误关联；菜单项不继承该字段身份，业务不必复制一套 ARIA 错误属性。
- 默认单选触发器与默认 Input 使用同一控件高度和 App `control` 文字角色；紧凑档位保留菜单原有密度。多选已选 Chip 的换行高度属于领域内容几何，不强塞进单行高度。
- `select-menu.tsx` 只编排共享单选语义和浮层生命周期；带搜索、异步状态或多选规则的菜单归真实业务所有者。
- 共用 `useSelectMenuOverlay` 的触发键盘入口先尊重 `defaultPrevented`，再通过 `isImeKeyboardEvent` 排除输入法组合事件；单选、Room 技能多选与历史菜单复用同一边界。单选触发器方向键继续即时改变受控值并打开菜单；显式点击/Enter/Space 打开后，定位可见才聚焦当前可用选项（否则首项/根），条目方向键/Home/End 只移动焦点，明确激活才选值；Tab 关闭并续接页面/模态焦点。多选与历史菜单的业务键盘语义保持独立。
- `action-menu.tsx` 保持外部受控，不复用 Select 家族的内部开关状态；业务可显式选择与锚点起点或终点对齐。级联浮层复用 `UiActionMenuContent` 的条目和底部动作，不复制 Action Menu 行结构。
- Action Menu 首次焦点必须等定位完成、浮层实际可见后进入首个可用条目；后续滚动/窗口变化只更新几何，不把用户当前条目焦点重置到第一项。
- `menu-keyboard.ts` 是 Action/Select、Room 模型和 Workspace 菜单的首项/当前选项焦点与方向键/Home/End 遍历所有者；按 menu/listbox 角色只遍历当前层可用项，不混入子层。它忽略 IME、已处理事件和外部 Portal 冒泡，输入框保留自己的编辑键。Tab 由调用方关闭并归还锚点，再通过 `overlay/overlay-focus-navigation.ts` 续接到同一页面/模态的相邻控件；全部禁用时菜单根仍可聚焦退出。级联进入/返回与实际命令仍由业务拥有。
- `menu-action-row.tsx` 是 Action Menu 与业务上下文菜单的唯一行级 DOM 所有者；它使用原生 button、普通 `role=menuitem` 或受控 `checked` 对应的 `menuitemcheckbox/aria-checked`、`aria-disabled`、有限密度和共享状态。业务只组合图标、标签、尾部内容与命令，不得重新导入菜单样式拼装按钮。
- Action Menu 的可选数组默认值必须引用模块级稳定空值；禁止在参数默认值中写 `[]`，否则锚定层的定位状态更新会让回调引用反复失效并形成 render loop。
- Action Menu 的重置等次级动作通过 `footerItems` 进入带分隔线的底部区域，不能混入主要选项伪装成普通值。
- Action Menu 默认普通行最小 36px，带说明最小 48px；compact 分别为 32/44px。默认/紧凑正文采用 control/supporting，说明采用 metadata/muted，统一 regular；完整标签/说明随内容增高，权限菜单保持默认密度，不靠截断隐藏范围或失败原因。固定几何的 Mention 与上下文行仍沿同一 recipe 的 fixed 档位，不接受自由尺寸。
- Action、Select、Mention、Slash、级联模型与 Workspace 上下文菜单统一使用 4px 内容内边距和 2px 条目间距；间距由 `menu-styles.ts` 的共享列表合同提供，业务层不得按页面覆盖。浮层必须把间距计入内容高度，在自身上限内随条目增长，达到上限后才滚动。
- `menu-styles.ts` 统一 Action、Select 与上下文菜单的行级圆角、焦点和状态层级；业务菜单不得复制整套条目样式。
- 16px 外框配合 4px 内容内边距时，菜单行固定使用 12px 圆角，保证选中底面与外框同心。
- Action Menu 的活动项使用中性活动底面；`primary` 只控制文字或小型状态提示，不再给整行铺品牌色。
- Menu 行统一使用正常字重；选择与危险状态由底面和色调表达，不靠加粗制造层级。所有 tone 的 active 都使用中性活动底面，disabled 项不响应悬停底色/前景。
- 活动项悬浮时继续保持活动底面，避免鼠标经过反而降低当前位置的辨识度；非活动项 hover 才使用更轻一档的中性底。
- `menu.test.tsx` 使用真实 Portal、点击和键盘覆盖 Select 的 listbox 选择、disabled 跳过、Escape，以及 Action Menu 的初始焦点、方向键、Home/End、选择后焦点归还；源码断言不能替代这组合同。
- `select-menu-trigger.test.tsx` 单独验证共用触发器的原生按钮/表单、ref、受控 ARIA 和禁用/键盘委派；Room 多选测试继续证明 Chip 移除与菜单开关互不触发。

菜单组件直接导入，不通过另一个菜单文件隐式转出。锚点定位、材质与浏览器生命周期统一复用 `shared/ui/overlay/`；业务需要扩大弹层时传 `menuMinWidth`，不得开放或使用菜单表面样式类覆盖定位、圆角、内边距与条目节奏。Action Menu 的高度估算必须包含带说明行、共享条目间距与完整 surface 内边距，正常内容不得因估算偏小产生内部滚动，只有真实可用视口不足时才允许滚动。单行触发器保留完整活动标签的共享 Tooltip 兜底，并独立声明正常行高，避免继承紧凑按钮的 `leading-none` 后裁掉英文下行字形。

单选菜单的可选 `onOpen` 在鼠标或键盘打开的用户事件内同步调用，供消费方发起需要用户手势的本机权限请求；方向键没有可用候选而未打开时不得触发，关闭、禁用、输入法组合事件也不得触发。
- `checked` 由调用方持有；Action Menu 提供非交互勾选标记，MenuActionRow 拥有 ARIA 状态，menu-keyboard 同时遍历普通动作与勾选项。条目名称、图标、说明与 trailing 只能包含非交互内容，不能在原生行按钮内再套 Switch/按钮。选择继续走唯一 onSelect 与关闭/焦点归还路径。

- `UiSelectMenu` 默认占位文案属于双语 catalog，空候选时禁用且不改变受控值；显式激活须匹配当前可用候选。单行选项保留完整原生提示，装饰图标不进入辅助名称。默认值在唯一组件入口解析，不保留透传包装层或按钮视觉 class 接口。会执行测试等命令的入口必须使用 Action Menu，不能借选择器的方向键选值副作用执行。

- `UiActionMenuContent` 透传内容根 ref；Action Menu 只测量未限高内容的 scrollHeight，加实际外框 padding/border 后交回原 Overlay 求解器。内容/字体/宽度变化通过 ResizeObserver 与既有 resize/scroll 更新，关闭/卸载清理；尺寸变化不得重置首项焦点。初始未测量使用同一行 recipe 估算，后续按内容增长/缩短至既有视口与 320px 上限。只有 footer 时不画孤立分隔线，估算与 DOM 保持同构。业务复合模型标签继续拥有主次布局并保留完整名称提示。

Action Menu 可传入非交互 header 作为身份说明，位于动作区上方并由共享分隔线隔开；header 纳入现有内容测高，不参与菜单键盘动作遍历。

- 单选与多选共用触发器不提供内部类别前缀或分隔线 API；需要字段说明时使用外部 Field 标签与关联，不能在当前选值前重复绘制字段名。

单选触发器的 `plain` 表面用于已有字段分隔的行内选择：透明、无边框，仍由公共 owner 提供 hover、focus 与 disabled 状态；消费者不得用后代选择器覆盖触发器内部样式。

SelectMenu 外层允许收缩到所属 Grid/Flex 字段宽度；长选项在触发器内部截断，不能以 intrinsic min-width 覆盖相邻只读数据或动作。
