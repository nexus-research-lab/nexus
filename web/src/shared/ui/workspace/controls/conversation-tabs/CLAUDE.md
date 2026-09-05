# conversation-tabs/ - Workspace 会话标签

- `conversation-tabs-model.ts` 只根据通用标签身份、活动项、轨道宽度与固定边缘动作计算稳定溢出和宽度分配；不读取 Room 协议、Store、创建时间或生命周期。溢出判定不得读取宽度动画中的瞬时 DOM 溢出状态。
- `use-conversation-tabs-layout.ts` 只绑定容器测量、共享宽度算法和滚动控制。业务打开集合、切换、新建、关闭、最后标签替换与固定偏好统一归 `features/navigation/conversation-tabs/`；此目录不保留业务控制器或兼容转发。
- `use-conversation-tabs-scroll.ts` 维护标签带溢出测量、活动标签归位、宽度动画稳定后的二次边界校正、触控板滚动和鼠标拖拽；滚轮入口必须以非 passive 原生监听显式归一 `deltaX`/`deltaY`，不得依赖 WebView 的默认横向滚动。
- `conversation-tabs-scroll-rail.tsx` 只渲染溢出时可操作的滚动轨道；轨道默认保持视觉静默，鼠标进入标签带时以中性色浮现，只有实际按下拖动或键盘聚焦时才增强为强调色。
- 上级 `workspace-conversation-tabs.tsx` 只消费 `WorkspaceConversationTabItem`、活动身份、busy 和独立命令；左侧接受调用方提供的历史入口，中央标签视口独立横向滚动，创建入口固定在右侧，窄宽度下两端动作不得随标签移出可视区。边缘动作组合共享 `UiIconButton`，共享层不维护会话清单、持久偏好或异步事务。
- Session 导航带高 36px、无外框和承托底色：左侧历史、中央标签视口、右侧创建入口只用 1px 低对比 hairline 分区。中央承托整排 32px、8px 圆角 Session 标签，左右保留 4px；宽度分配、溢出判断和滚动归位必须共同扣除两端固定动作与中央内边距，保证首尾标签完整。活动 Session 标签只使用中性活动底面、文字权重与蓝色状态点，不绘制完整边界或外投影；非活动标签保持透明并使用清晰的次级文字，连续非活动标签之间绘制 16px 低对比竖线，当前标签两侧及悬停/聚焦标签两侧不得出现分隔线。
- `workspace-conversation-tab-model.ts` 统一推导单标签的活动态样式、宽度、标题、固定/关闭态和稳定状态类；当前标签使用浅底和状态点，非当前标签保持透明并由短分隔线组织远端标签。
- `workspace-conversation-tab.tsx` 只渲染单个标签；图钉切换固定偏好，共享 `UiTabDismissButton` 只关闭标签，两者不得合并语义；模型只给关闭按钮提供可见性，按钮几何、原生 title 和事件隔离由该原语拥有。不得自行推导会话集合状态或状态样式。
- 调用方提供的活动标签必须属于受控打开集合；共享视图不通过 Effect 修正领域状态。
- 窄容器仍保留当前会话标题；创建入口始终保持纯图标和可访问名称，但使用比普通 ghost 工具明确一档的中性浅底，避免核心入口消失在 Header 中。
- 桌面标签轨道随共享顶栏使用 36px 导航带和 32px 标签/动作高度；宽度分配仍只由容器模型决定。
- `workspace-conversation-tabs.test.tsx` 验证受控命令与 busy 投影；`conversation-tabs-model.test.ts` 验证固定边缘空间、活动权重和稳定溢出阈值；`workspace-conversation-tab.test.tsx` 验证关闭动作的键盘、标题和事件隔离。
