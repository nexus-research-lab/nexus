# Launcher Hero

- `launcher-hero-stage.tsx` 只渲染首屏、输入框和最近入口。
- 查询字段使用 `UiInput` 的 lg/surface 档位，字体、占位文案颜色、边框与焦点由公共输入 recipe 持有；外层只排列字段和角色发送热区，不另套输入玻璃壳。品牌复合入口和角色图像按钮保留已登记的场景几何例外，发送按钮始终具有本地化名称，等待时保持 busy/disabled。
- 标题渐显、容器进入和装饰 Lottie 委托公共反馈组件；业务只传内容、布局与进入时序，不测量标题字体、注入动画样式或控制播放器实例。
- `launcher-recent-entry-model.ts` 只投影 DM/Room 标签、可访问名称与截断提示，不得返回 class、style、颜色、尺寸、阴影或动画参数。
- `launcher-recent-entry-layout.ts` 只拥有 Hero 最近入口的排列和渐入时序；按钮形状、尺寸、字阶与交互状态仍归共享 Button。
- `launcher-recent-entry-styles.ts` 只从稳定入口键投影语义色身份点；不得把按钮底色、边框、字号或交互态带回业务配方。
- `launcher-recent-entries.tsx` 只编排最近入口与主 Agent 交接动作；两类动作固定复用透明 `UiButton`，DM 以彩色身份点替代机器人图标，Room 保留 `#` 语义，入口说明复用 `shared/ui/overlay/tooltip`，不得恢复原生按钮、常驻胶囊底或局部层级的 Tooltip。
- `use-launcher-query-input.ts` 拥有受控输入、IME、Mention 和提交交互。
- 输入键盘与公共 Mention 捕获都复用 `isImeKeyboardEvent`，composition、Process 和 229 不能触发选择或提交；同步受理成功才清理草稿，拒绝保留原文，外部恢复和 Mention 光标插入仍由本输入 owner 处理。
- `use-launcher-stage-scale.ts` 拥有唯一的响应式缩放系数；云朵画布上移居中（锚点 40%）、Token 堆锚定视口底部共用系数，禁止再引入断点补丁 CSS。
- `pile/` 独立拥有 Agent Pile 的描述表、Matter 生命周期和 Token 视图，不回流 Console。
- Surface Theme 只投影 CSS 变量。
- Surface Theme 只保留实际消费的场景变量；普通输入颜色不能在此另建覆盖。云朵 SVG 是不可交互的品牌装饰，独立 ID、样条和羽化边沿留在 HeroBlobShell，不晋升为通用 Panel。

Hero 不直接调用 Launcher、Room 或 Agent API。输入匹配和插入复用 `shared/ui/mention/`，本目录只决定触发符对应的目标分类。

Launcher 查询提交使用共享 `md` Spinner 并继承提交按钮颜色；Hero 不再用边框 div 自制加载动画。
