# composer/

L4 | 父级: web/src/features/conversation/shared

## 职责

- `composer-panel.tsx`: Composer 各子域的纯视图装配
- `use-composer-interaction-height-guard.ts`: 在同一人工介入队列内持有输入壳高度高水位，并在恢复普通输入时一次释放，不让连续权限卡反复改变消息 viewport
- `controller/`: 草稿状态、消息投递、Goal/Loop、IME 与视图状态编排
- `composer-history-store.ts`: 按 Room/DM 逻辑聊天隔离并在当前浏览器或 App WebView 内持久化发送历史
- `use-composer-history.ts`: 将持久化发送历史接入上下键召回、游标与未发送草稿恢复
- `composer-model.ts`: 输入策略、键盘规则和布局状态表
- `composer-draft-store.ts`: 保存正文、图片/文件附件、Message/Goal 模式、Room Goal 负责人和 Mention 目标组成的完整草稿胶囊，并以修订号保护本地派发认领与失败恢复；Goal 提交额外保存 submitting/confirming phase、提交前 durable Goal version fence，以及带 exact transport identity 与恢复修订号的 failed-restored recovery receipt
- `composer-goal-observation.ts` 与 `composer-goal-submission-reconciliation.ts`: 有界观察 GoalPanel owner-scoped 快照，并在 ACK 未知或 post-send 失败恢复后，用更新过的 Goal ID/version 或原 Session exact `client_message_id` durable 控制记录精确收口原 scope
- `composer-draft-scope.ts`: 分别生成包含 Session ID 的 Room/DM 完整草稿作用域，以及排除 Session ID 的发送历史作用域
- `use-composer-mention.ts`: 以单一匹配对象管理 Room 成员提及，并复用共享 Mention 文本模型
- `slash-command-model.ts`: 解析输入框起始 Slash 查询，并以纯函数完成筛选和插入
- `use-composer-slash-command.ts`: 管理命令、模型与技能三级补全状态、选择、键盘导航、按需目录加载与草稿清空后的浮层收口
- `use-conversation-composer-handlers.ts`: DM/Room 对 Composer 的发送适配
- `controller/use-composer-controller.ts`: 除既有草稿/发送装配外，只接受同一 exact Session 的命名 WorkGraph 保存意图事件，把可见请求写入 Message 草稿并自动发送；不直接调用命名工作图创建 API
- `attachments/`: 以单一规则表统一附件分类、批量校验、上传准备和本地展示
- `components/`: 输入行、提交动作、Footer、Session 模型/权限控制、待发送队列和 Loop 选择器

DM 可在桌面端为当前 Session 挂载多个本机工作文件夹，Web 不公开入口。附加目录独立于 Agent workspace CWD，保存期间与模型/权限设置一样阻止新一轮提交。

输入、运行时、模式和动作状态先在控制器中分别投影，再组装为扁平视图契约；面板不得重新解释发送条件和提示文案。
运行时投影必须保留明确的发送、回复和上下文压缩阶段，Footer 不从通用 loading 状态猜测压缩行为。
发送目标先投影为 `send/enqueue + delivery policy`，消息提交按资格判断、附件准备、投递和收尾分阶段执行。
未发送草稿胶囊包含正文、图片/文件附件、Message/Goal 模式、Room Goal 负责人和 Mention 目标，以包含 Session ID 的 Room/DM 作用域保存在客户端内存 Store；切换 Session 时恢复各自完整待发送状态，切换逻辑聊天时同样隔离。成功投递的消息正文仍使用不含 Session ID 的逻辑聊天作用域保存在客户端本地持久化 Store，Web 浏览器与桌面 App WebView 各自独立，禁止接入服务端或跨设备同步；每个作用域最多保留 50 条，总持久化条目保持有界。弹层开关、上传中、错误提示、Mention 匹配浮层、历史游标和召回前的未发送正文属于瞬时 UI，不进入持久化历史。每次 Session 草稿作用域变化都要把 textarea 聚焦到正文末尾并显示最后一行，不能把光标停在首字符前；历史召回后同样把光标放到正文末尾。消息在本地协议派发并建立 optimistic/queue 请求后立即认领并清空提交时修订号仍未变化的当前 Session 完整胶囊；Goal、普通消息、编辑重跑与队列输入都由 exact durable transport owner 跨 Session 切换或新建继续等待，只有页面级请求取消；ACK/拒绝按 client_request_id 收口且只在原 Session 投影。发送前失败或后端明确拒绝只在用户没有继续输入时恢复该胶囊，受理未知绝不恢复，避免重复发送。Goal 的 post-send 明确失败另建 recovery receipt，迟到 durable Goal/控制记录只撤回同一恢复修订号和过时错误，用户后续编辑必须保留，新重试则原子替换旧 receipt；受理未知进入确认中。Goal 创建由 ACK、越过提交前 version fence 的 durable Goal，或原 Session exact client_message_id durable 控制记录收口。
中文输入法的 composition 保护属于控制器边界，键盘命令执行前必须按顺序经过 composition、Safari 补发 Enter、Slash 导航和 Mention 导航守卫；Safari 守卫只消费 composition 结束后的 Enter 并阻止浏览器默认提交。
Slash 命令目录只消费后端从版本化内置清单合成的快照中的公开名称、说明、参数提示和执行类型；输入恰为 `/` 时按公开命令名（忽略前导 `/` 与大小写）字母序展示当前快照的全部指令，继续输入字母后依次优先命令名前缀、命令名其他位置、说明或参数提示的子串匹配，同一优先级按命令名字母序展示。`/skills` 是宿主侧技能入口：一级命令只负责进入技能子面板，技能列表按快照中的当前 Agent ID 拉取并在子面板中筛选，Nexus 内置 Skill 使用共享双语说明参与展示与搜索，最终只把 `/<skill> ` 写回正文；未为当前 Agent 启用的 Skill 以弱化的“单次使用”状态显示并允许显式选择，完整 `SKILL.md` 的读取、参数展开和单轮上下文注入全部由所选 runtime 负责。`/model` 同样进入模型子面板，按当前 runtime 拉取 Nexus Provider 模型选项，并为 Claude runtime 合入版本内置别名；Provider 模型选择写回 `/model <provider>/<model> `，由 Nexus 原子更新当前 Session 的 Provider/模型覆盖，Claude 内置别名保持原生 `/model <alias> ` 透传。其他 host/runtime 选择只把 `/<name> ` 写回正文，发送和排队继续复用普通消息链。所有消息开头的 `/<command>` 都通过不接管指针的同步镜像显示为轻量命令标签，原生 textarea 仍独占输入、光标、选择、IME 与滚动；`/visualize` 同样只写回原始指令，后端在 runtime 投递边界展开简短的 Generative UI 提示，前端不得拼接隐藏提示。前端不得查询命令目录或按浮层打开触发 runtime，浮层查询和选中位置不进入草稿持久化。发送收尾或其他程序化草稿变更使正文不再匹配 Slash 查询时，浮层必须同步关闭。
输入区 Props 由 DM/Room 的真实消费面定义，不保留无调用者的兼容参数。
紧凑 Composer 只用于手机与窄窗专注模式：外层至少保留 16px 横向安全留白，较宽窄窗保持 720px 居中上限，底部留白必须覆盖常规间距与系统 safe area；不得把输入壳铺满整个视口。
常规桌面 Composer 在底部保留 8px 呼吸区，使输入壳贴近窗口底边但不截断边框与阴影；不得通过改变输入壳自身高度模拟抬升。紧凑模式继续取常规间距与系统 safe area 的较大值。
常规桌面 Composer 与消息轨道保持同一中心线，但使用独立的 880px 外层上限；桌面横向内边距扣除后，输入壳约 832px 宽，不得随超宽屏继续拉成长条。
Composer 输入壳以 20px 圆角、约 102px 空态高度和无分割线动作区形成独立聚焦面；只有输入壳保留黑色 3.5% 的短接触阴影，搜索框与普通表单不得继承这套尺寸。
Composer textarea 高度只以浏览器真实 `scrollHeight` 为准，并在 React 正文、原生 input/IME 组合输入与宽度变化时同步重测；测量必须包含实际字体、换行与内边距，短文本必须从旧上限立即回缩。正文最多把输入壳推高约 5 行，之后只在 textarea 内部滚动，不能继续挤压对话区。
Composer 输入壳外层使用绝对定位、`pointer-events: none` 的 `::before` 将自身上缘向正文羽化；普通输入与权限、问答、计划确认替换面共用同一外缘。羽化不得挂到消息 viewport 或全宽 BottomArea、增加 padding/clearance、遮挡 Task/回到底部 Dock，或改变输入壳和虚拟列表测量。
DM 或 Room 出现 pending permission、AskUserQuestion 或计划确认时，人工介入组件必须原位替换整个输入壳内容；不得悬浮在输入框上方，也不得在消息正文或 Thread 保留第二个操作入口。未发送草稿与附件继续保存在原 Session 草稿作用域，最后一个请求完成后输入壳原位恢复并重新聚焦。同一 pending interaction epoch 的输入壳外部高度只可增长，较短请求在壳内留出稳定空间，最后一个请求完成后再一次平滑恢复普通输入高度，禁止连续请求逐项挤压消息 viewport。多个请求按首次到达顺序在同一位置逐个接棒，重放的同 request 快照只能原位更新；Room 当前项显示请求 Agent 身份并按 `request_id` 回到原执行。
权限与计划确认采用紧凑决策面：首行只保留请求 Agent 与工具，正文只保留一句摘要及一个必要参数，底部只有拒绝和“允许本次”；持久范围进入相邻次级菜单，不平铺状态、时间和提示词元数据。
Composer Footer 使用输入壳命名容器形成“动作/权限—Powered by Nexus—模型/提交”三列；模型与权限只写当前 Session 覆盖，空覆盖表示继续继承 Agent 默认值。DM 隐式使用当前 Agent，直接展示权限和模型；Room 左侧权限一次作用于当前 Conversation 的全部 Agent Session，右侧模型先列 Agent，并在横向空间足够时悬浮级联其模型选项。模型配置目标只存在于浮层内部，关闭后忘记，Room 的消息路由仍只由当前 Room 与正文 `@` 提及决定，不得把模型目标伪装成发送对象或借此回写 Agent 默认配置。中心品牌标注属于壳内三级弱信息，颜色必须由 `text-soft` 与输入壳背景混合后进一步后退。发送、排队、Goal 启动和停止统一使用 32px 圆形图标按钮，由 `aria-label` 提供完整语义，不随容器宽度扩展文字标签；空输入时发送按钮保留中性灰底，不退成透明 ghost。Goal 模式的负责人控件不可被压缩到不可操作，控制与提交保持第一行、运行状态独占第二行，品牌在该模式退场；460px 以下再把提交行独立堆叠。scope 文案可以收敛，但负责人、取消、状态和提交动作不得重叠或被裁切。
新消息提交以 `auto` 贴住真实内容底部，随后每段正文增长都连续上推历史内容；只有用户显式点击“回到底部”或导航时才允许可见的平滑滚动。
Composer 的可用发送、排队与 Goal 确认使用 Nexus 品牌行动蓝，禁用发送回落为中性灰；Plus、附件与普通工具保持灰黑，停止和错误用红色，完成用绿色，字数临界用琥珀色，发送中/回复中/压缩中属于活动态而不是成功态。
队列命令和附件准备是 DM/Room 的共同能力；DM Composer 的停止针对当前会话，Room Composer 的“全部停止”必须在点击时冻结所有 active slot 的精确 `agent_round_id`，逐个复用定向停止，禁止退化为无目标的 session interrupt。
Mention 目标只投影成员标记和标签；匹配、插入、键盘与浮层规则归 `shared/ui/mention/`。Slash 键盘导航不得注册 document 级监听，必须由 textarea 或子面板搜索框显式分派；外部点击、Escape 收口及 resize/scroll 重定位统一复用 `shared/ui/overlay/anchored-overlay-layer.ts`。
附件必须先整批校验再上传；DM/Room 只提供目标作用域，不得复制格式规则或上传循环。
