# 全前端设计与代码审查

**Non-normative。** 本文跟踪 2026-09-06 新 Goal 的范围、判断与交付证据，
不新增另一套设计规范。工程规则继续归
[frontend-engineering-spec.md](../specs/frontend-engineering-spec.md)，
视觉与交互规则继续归根目录 [design.md](../../design.md)。基线为 `9a64ac3a6`。

## 审查对象与退出条件

- [逐文件清单](frontend-design-review-2026-09-06.csv) 覆盖基线中的 482 个生产 TSX
  文件，包含页面、领域视图、内部子组件、共享组件及装配/上下文文件；测试、开发
  Gallery 及其专用入口排除。公开组件名称用于定位，不能替代文件内部视图审查。
  A30 将跨领域 `UiFilterSelect` 从多组件文件中提取为独立生产文件，追加登记后共
  483 项；A35 新增公共源码编辑 owner 后为 484 项，A36 新增领域预览加载 owner
  后为 485 项。基线文件和删除证据继续保留。
- 基线公共 UI 的 125 个公开 React 组件沿用现有 Gallery 的完整名单与所有者，
  不另建组件库。对应 token、recipe、状态模型和消费者随组件一起检查。经 A6—A7
  删除已证明无生产用途的 5 个导出后，A7 时 Gallery 登记 120 个；原始范围与删除
  证据仍保留在清单中，不通过缩小基线计数宣称整个审查完成。后续标准筛选和
  源码编辑原语使当前公共 UI 清单为 122 项。
- 全部 16 个产品页面入口都在范围内；单页中的详情、编辑、错误、空态、加载、
  权限和窄屏分支也属于该页。Settings、Operations 和 Room 不因路由只有一个
  入口就按一个静态截图验收。
- 每个文件只能在有记录后标为 `retained / improved / removed`；`pending` 和
  `in_progress` 不计为完成。保留要说明设计为何适合其任务，删除要提供当前
  生产/动态入口和引用证据；生成协议与第三方文档数据不按简单零引用删除。
- 审查明确区分信息层级、字号/行高、间距/分割、密度、主要动作、可点击性、
  焦点/键盘、禁用/运行态、长文本、三主题与中英文宽窄屏。规范自身不合理时，
  修改唯一规范及其公共实现；不在页面私自覆盖。
- 当前 Goal 持续到上述清单和页面分支都有结论、已选问题已修复并验证、冗余代码
  审计收口。公共组件 Gallery 覆盖不等于全部业务页面已通过视觉审查。

用户已要求 Windows 实际宿主验收暂缓，并在 A36 明确要求先暂停所有视觉验收、
专注代码重构。当前不再安排截图、浏览器外观复查或扩充视觉测试场景；功能回归、
静态合同、类型检查和构建按修改范围继续。历史视觉待验收记录保留事实，但不再
作为当前代码推进的阻碍，也不宣称已实际验收。此前浏览器启动审批超时不再重试。
本轮先本地提交；上一轮的推送授权已完成，本轮不会自动沿用为持续推送。

## 页面与领域顺序

| 批次 | 页面 / 专用组件范围 | 审查重点 | 状态 |
| --- | --- | --- | --- |
| A | 所有公共 primitive / pattern | 文字可读性、控件密度、尺寸、分隔、状态与组合关系 | 进行中 |
| B | Launcher、Home、主侧栏、首次空会话、全局导航 | 品牌与工作区层级、空间分配、搜索、窄屏导航 | 待审查 |
| C | Contacts、Agent 详情/编辑/授权/私域、Memory | 目录扫描、信息主次、长内容、表单与保存反馈 | 进行中（目录、身份、联络和记忆编辑；完整页面待验收） |
| D | Room、DM、Thread、子任务、Composer、Goal | 阅读节奏、消息身份、过程/最终回复分层、状态堆叠、输入空间 | 进行中（Composer、历史与输入弹窗；其余待审） |
| E | Workspace、文件/文档/表格/演示预览、Artifact、WorkGraph | 画布与工具栏、缩放、长文档、详情、图形专用几何 | 进行中（源码编辑；预览与画布继续审查） |
| F | Skills、Connectors/Custom MCP、Loops、WorkGraph 目录与详情 | 目录/详情一致性、筛选、说明、次级操作、授权流程 | 进行中（Connector、目录选择器与配置字段；其余待审） |
| G | Channels、Pairings、Scheduled Tasks 及编辑/历史 | 配置阶段、任务状态、日期时间、复杂表单与失败恢复 | 待审查 |
| H | Settings 全部栏目、Operations 全部栏目 | 分组与分割、说明密度、设置行、模型/成员/套餐表单 | 进行中（通用配置与 Provider；其余待审） |
| I | Login、Setup、OAuth 回调、引导、独立桌面设置窗口 | 独立壳层、错误/加载、响应式及原生入口差异 | 待审查 |
| J | 全前端引用与依赖复核 | 旧适配层、无效分支、重复样式/常量、失效测试与文档 | 待审查 |

共享改进跟踪到全部消费者；已有专用几何（图形节点、文档阅读、品牌等）逐项
评价必要性。具有独立任务的组件可以保留自己的布局，普通控件外观仍由共享层拥有。

## A1：公共字段占位文字

**发现：** `UiInput / UiTextarea / UiSearchInput` 使用 `--text-soft`，与既有
设计规范要求的 `--text-muted` 不一致。真实 Gallery 中的搜索字段按当前 CSS
背景颜色层合成，浅色约为 4.18:1、深色约为 3.33:1、雨天约为 2.74:1；后两者
的页面装饰背景包含图像，因此这些数值只用于初筛，不声明为全部像素的精确比值。
基线观察记录于 `/tmp/nexus-design-review-2026-09-06/fields-before.json`。

**判断：** 占位文字传达填写示例和搜索范围，需要保持可读。
[W3C 对比度说明](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html)
明确将占位文字纳入文字对比度要求；普通尺寸文字的目标为至少 4.5:1。
本批检查限定为可用的空白字段，不借此调整 disabled 语义。

**落实：** 在现有两个公共表单所有者中统一采用 muted；保留当前字号、行高、
尺寸和业务输入行为。雨天 muted 的透明度从 0.84 调整为 0.92，颜色仍由一个
主题 token 拥有，未给页面或占位文字新建单独色板。Gallery 增加真实空白字段状态，在不带装饰图像的当前主题
底色上检查 Input 的 dialog/surface、Textarea 和 Search 四种组合。测量使用
浏览器颜色合成，遇到图片、滤镜或整体透明度时拒绝猜测；截图用于复核效果。

**回归过程：** 雨天主题旧实现首先按真实空白字段复现 2.606:1 的失败；仅改用
muted 后，浅色/深色的 24 组通过，雨天的 12 组仍只有 4.288:1。调整雨天 token
后，Chromium/WebKit 三主题、双语和宽窄屏的全部 36 组通过。测试没有降低阈值。

**验证：** `npm run check` 的 lint、typecheck、454 项合同、149 个文件的 364 项
组件测试和生产构建全部通过；完整浏览器批次 792 项通过（Chromium 528、WebKit
264），无失败、跳过或重试。四种空白字段的 144 份测量中，三主题最低底色对比
分别约为 6.006 / 5.617 / 4.800。这里计算 CSS 颜色层，不将前景雨滴动画、图案、
抗锯齿或整个页面所有文字视为已取得精确对比度结论。

`make app-check-ui` 在实际 WKWebView 通过三主题、双语及 360/1280pt 的 12 组
既有原生控件交互，证据位于 `/tmp/nexus-native-ui/c1e74696b9db4c59a2369725e09b0475/`。
该套件验证原生输入、选择器、嵌套弹窗和隐藏恢复；四种空白字段的专门对比度断言
属于浏览器套件，不将它混称为原生专门测量。

已人工复核浅色英文桌面、WebKit 深色/雨天中文窄屏的空白字段，以及原生雨天
字段和弹窗。浏览器报告位于 `web/playwright-report/`，完整结构化结果另存
`/tmp/nexus-design-a1-browser.json`，选取的截图位于
`/tmp/nexus-design-review-2026-09-06/`。这些产物不提交。

**状态：** A1 已修正并验证。清单中的表单文件仍处于 `in_progress`：这不代表
UiField 的标签、错误、布局与全部消费页面已审查完成；整个新 Goal 仍进行中。

## A2：字段层级、紧凑控件与设置选择器

**发现与判断：** 同高的紧凑 Button / Select / Input 原先采用不同字号；设置页又通过
跨文件常量把选择器压成 28px / 11px，并覆盖公共焦点环。标签使用 soft，比说明更淡；
字段说明同时叠加容器 gap 和 `mt-2`。这些差异没有业务依据，统一到公共所有者。
设置行本身的两列宽度与窄屏堆叠保留：左侧解释后果，右侧负责选择，需要独立布局。

**落实：** 普通字段标签采用 13px / 20px、medium、default；说明及下方错误采用
supporting，去掉额外上边距，组内由容器提供 8px 间隔。Button / Select 的 xs 和 sm
文字分别与同高 Input 对齐，默认尺寸与选择命令不变。Settings 的描述、控件标签和
导航分组同步采用清楚的文字层级；General 的默认模型、默认权限和 Runtime 的网页
搜索选择器直接选择公共 sm，删除已无引用的两个私有高度/选择器样式常量。

门禁新增具名常量导入、别名和具名转导出追踪，保留作用域遮蔽和循环终止；只解析
AST，不执行模块。修正前命中 8 个消费者的 39 项覆盖：3 个设置选择器和 5 个能力目录
条目。后者改用 `UiListRow variant="outlined"`，目录常量只保留 80px 最小内容高度和
内边距；静态计划条目不再继承可点击的 hover 边框。两类旧 recipe 的实现均已删除，
没有新建转发壳。动态样式、命名空间样式常量和星号转导出仍需人工审查。

**验证范围：** 新 Gallery 直接渲染 `SettingsDefaultModelRow` 与
`SettingsPermissionsSection`，使用本地状态记录 exact role/value 回调，不调用偏好
服务。浏览器检查两档三个公共控件的实际高度/字号、标签点击、说明间距、设置文字
对比度、可见键盘焦点、选择回调与保存锁；根目录规范与模块所有权文档同步更新。
旧的组件断言保留原有命令、状态与身份检查，仅同步过期字号预期；不因文字层级变化
删除业务回归。Runtime 搜索行沿用已有组件/合同检查，本批不宣称整个 Runtime 页面
及其全部高级状态均已完成视觉验收。

**检查结果：** 完整前端 lint/typecheck、456 项合同、149 文件 / 364 项组件测试和
构建通过；完整浏览器矩阵 **828 项通过，0 跳过、0 flaky、0 失败**。新设置场景覆盖
36 个项目，324 份文字对比度测量的主题最低值为浅色 6.157:1、深色 6.233:1、雨天
5.036:1（此处显示值为摘要，断言使用未舍入值）。沿用 A1 的平面 CSS 合成测量边界，
不把雨滴装饰、所有错误文字或整页像素对比度视为已验收。

已人工复核浅色英文桌面、WebKit 深色中文与雨天英文窄屏截图：设置后果说明保持
可读，窄屏设置行自然堆叠，紧凑组合不溢出；测试用三列极窄选择器仍按公共规则截断
长值，真实设置选择器使用整行宽度。截图位于 `/tmp/nexus-design-review-2026-09-06/a2/`。
日志为 `/tmp/nexus-design-a2-check.log`、`/tmp/nexus-design-a2-browser.log`，完整结果
为 `/tmp/nexus-design-a2-browser.json`；这些临时报告不提交。

本批 `make app-check-ui` 在加载实际 WKWebView 后停于原生窗口激活等待：窗口状态为
`visible=true, key=false, occluded=true`，未进入可信输入检查。记录位于
`/tmp/nexus-design-a2-native.log` 及
`/tmp/nexus-native-ui/cf1a54dfb9dd4f1db8319ccaa48f052c/`。只读系统状态进一步确认
`CGSSessionScreenIsLocked=true`，本批原生可信输入须待实际桌面可用时补验；不把
A1 的原生通过记录沿用为本批通过证据。Windows 继续按用户要求暂缓。

**状态：** A2 实现及 Web 验证已收口，原生输入验证存在上述缺口。
这批处理通用控件和几个设置/目录消费者，不代表整个 Settings、Capability 或 Button
全部视觉状态已完成审查。

## A3：状态颜色、配对前景与控件材质（候选，待浏览器复核）

**源代码与离线证据：** 原成功实底 Button 直接使用状态色并固定白字，按源码的
sRGB 色对计算，浅色/深色/雨天分别约为 2.527 / 1.949 / 1.882:1；这些数值只说明
该不透明色对，不伪称实际页面截图测量。初筛计算保存在
`/tmp/nexus-design-a3-color-calculations.json`，其中候选值属于探索过程，不能当作
当前代码已经达到对比度目标的证明。

深色/雨天 `--material-chip-background` 和输入材质曾声明为 linear-gradient，
却通过别名进入只接受颜色的 `color-mix()`。使用现有 DOM CSS 解析器可复现：混入
渐变的值被拒绝，换成 RGBA 后被接受。新合同检查解析三主题八个公共控件颜色槽及其
别名，并用旧渐变作为负例；真实浏览器还要验证 CSS.supports 和实际绘制。
`--material-input-background` 虽没有直接 TSX 消费者，但由 `--input-shell-background`
进入现有输入壳，因此保留并修正类型，未按单文件零引用误删。

**当前候选实现：** 在唯一主题 token 中调整浅色状态色、雨天危险/成功色和辅助
soft 色；危险/成功 Button 与 Counter 使用已有语义色的主题配对前景。Badge 与
Choice 的小型行动蓝文字复用 brand-action。控件的 chip / input / focus 材质改为
可混合颜色，专用 canvas / avatar 图像材质保留。Windows 浅色危险画刷同步 Web，
静态投影合同通过，不代表 Windows 实机验收。

**冗余清理：** ghost / text / icon 三处完全相同的非中性按钮 tone 配方合并；
Badge 的 active/success 共享相同配方但保留业务 tone。`--status-info-soft-*` 的
九处旧主题定义在基线全仓搜索中只有声明，未发现别名、生产或动态构造消费者；
已删除，当前 info Badge 继续从唯一 recipe 派生。

**验证状态：** 已通过 lint、typecheck、149 文件 / 364 项组件测试与构建，记录于
`/tmp/nexus-design-a3-local-check.log`；137 项相关合同通过，记录于
`/tmp/nexus-design-a3-contracts.log`。配方去重后补跑 Button/Form 的 23 项行为测试、
typecheck 及 token/control/native-theme 的 21 项合同亦通过。这里只报告已执行的
检查，不将其代替完整浏览器或 macOS 输入验证。

新 Gallery 直接组合全部徽标 tone、文字 tone、Button tone/variant、Counter、
Choice 和字段错误，在 page/card/overlay 三种表面测量，卡片上另测全部按钮 hover。
实际浏览器启动的自动审批连续两次超时，进程均未启动；已请求用户明确授权，尚未
收到答复。本批没有浏览器基线或完成截图，不沿用 A2 的 828 项结果。当前实现和规范
调整以本地中间提交保存，仍须真实渲染及回归复核；提交不代表验收通过，Goal 继续进行。

## A4：字段标签、说明和错误归属（实现与离线验证，待浏览器复核）

**发现与范围：** 检查全部 74 处生产 `UiField` JSX 调用点，原说明只显示文字、未
关联控件；显式业务错误没有统一写入控件的错误关系。原生错误消失时直接删除
`aria-invalid / aria-errormessage`，可能擦掉调用方已有或刚更新的业务属性。
源码调查记录位于 `/tmp/nexus-design-a4-field-audit.json`；这是调用点证据，不是
全部页面或动态实例已通过浏览器验收的证明。

**落实：** `field-accessibility.ts` 在 Field 与公共输入/Select trigger 之间提供
唯一声明式关联。明确的 `htmlFor/id` 配对接收可见说明和错误，已有描述合并保留；
原生错误结束后恢复当前调用方属性。只有最近的 Field 显示原生错误，首个错误的
定位忽略不参与校验的 disabled 控件；业务恢复受控草稿时清除过期的原生错误。
业务错误不会因为 native validity 已满足而消失，字段值、ref 与提交事件继续透传。

补齐频道配置、配对创建/行内 Agent 选择和 Provider 形态/格式的标签关联后，
68 处调用以 `htmlFor/id` 明确指向目标控件。其余 6 处为 Custom MCP 的分段配置、
参数/环境输入组、Skill 来源认证选择和 Discord 授权动作区：无 `htmlFor` 的具名
Field 统一表达有可访问名称的 group，不再生成无目标 label；整组错误只属于该组，
各输入继续保持自己的名称与原生错误身份，不批量复制同一个 ID。

删除了手写设置/删除错误属性的路径、颜色 Gallery 的重复错误关联和控件参数的
同名别名。新增内部依赖门禁只允许 Field 与 Select trigger 导入关联实现，业务层
通过公共组件消费；Provider 配置文件补上与当前职责相符的顶部合同。

**已执行验证：** lint、typecheck、149 文件 / **376 项组件测试**及构建通过，
包含新增 12 项字段关联与错误归属用例。配对行测试额外证明点击可见标签打开正确
选择器，原有状态/删除命令仍然通过。完整组件和构建记录位于
`/tmp/nexus-design-a4-components-final.log`、`/tmp/nexus-design-a4-build-final.log`。
142 项相关静态合同已通过（`/tmp/nexus-design-a4-contracts.log`）；更新后的 141 项
依赖/所有者检查通过，单独记录于 `/tmp/nexus-design-a4-ownership-final.log`。这些检查证明 DOM、行为和
源码边界，不等同于实际屏幕阅读器发声或宿主输入验收。

既有浏览器场景加入真实可访问说明/错误断言，颜色夹具直接消费公共错误关系。
浏览器启动仍受 A3 所述自动审批超时及待答复授权影响，未尝试绕过；A3/A4 以本地
中间提交保存当前实现，待实际浏览器回归后继续修正。整个页面/组件清单继续保持未完成。

## A5：分段选择所有权与交互状态（实现与离线验证，待浏览器复核）

**源码发现：** 私有 Skill 来源的认证方式用两枚普通 Button 判断 tone/variant，
未暴露当前选中值。公共分段控件自身还有两个状态冲突：active recipe 的
`box-shadow: none` 会覆盖全局键盘焦点层，hover recipe 未排除 disabled。带图标
的文字选项也未声明行内布局，不能依靠 SVG 的默认 display 保证同行。

**落实：** 认证方式改用 `UiSegmentedControl`，删除页面上的选中配方与按钮循环。
选择器继续只传回 `none/bearer`；Token 草稿、已存凭证留空、等待锁和提交载荷保持
由原编辑器负责。公共分段控件统一图标/文字同行与禁用外观；active 只拥有底色与
边界，键盘焦点仍由唯一全局层绘制，disabled 不再应用 hover 底色。

对 feature 中直接 `UiButton` 的内联条件 tone/variant 与 aria-pressed 做了
AST 候选扫描，剩余 8 处分别是提交、启停成员、修改密码、Thread 开关、浏览器
设置、权限菜单、文件编辑/保存和 Agent 保存动作。按当前按钮命令语义保留；不能
因为颜色随状态变化就把动作迁入互斥选择器。记录位于
`/tmp/nexus-design-a5-button-candidates.json`。该扫描不涵盖别名或模型间接输出的
所有状态配方，也不表示这些文件的其他视图已审查完毕。

**已执行验证：** lint/typecheck、150 文件 / **378 项组件测试**、149 项相关合同
及构建通过。新增真实来源管理弹窗测试，覆盖 pressed 状态、认证分支往返后的
Token 草稿保留、已有凭证留空、等待锁和精确保存载荷；测试没有导出内部编辑器，
也没有调用远端来源。记录为 `/tmp/nexus-design-a5-components.log`、
`/tmp/nexus-design-a5-contracts.log`、`/tmp/nexus-design-a5-build.log`。

新 Gallery 使用公共组件覆盖两档密度及文字/图标文字/纯图标三种模式，浏览器场景
检查已选项的键盘焦点、图标文字对齐、disabled hover、回调和文字对比度，并记录
实际字号/高度。因前述浏览器审批仍待用户答复，这些新断言尚未执行，不作为实际
绘制或视觉验收证据。`playwright test --list` 已识别完整 900 项矩阵，结果位于
`/tmp/nexus-design-a5-browser-list.log`；列举场景未启动浏览器或本地服务，不计为测试通过。
共享配方目前仍使用 11px caption，其字号、行高和窄屏宽度
必须与设置、Mermaid、联系人、导入和五选项定时配置一起复核，本批未宣称该设计
已最终合理。实现先保存为本地中间提交，浏览器回归仍是待完成事项。

## A6：无生产用途的组件与静态资产

**引用证据：** 从 Vite 明确构建的 `index/app/settings/oauth-callback` 四个 HTML
入口出发，复用现有 TypeScript 依赖解析器检查 1,439 个 TS/TSX 文件。图包含普通、
副作用、重导出、类型和字符串动态导入；扫描未发现无法解析参数的动态 import，
也没有 `import.meta.glob`。保守可达集合为 1,269 个文件。类型边使这个集合偏宽，
因此“可达”不能证明每个导出都被执行，本批不把模块图当作全部冗余代码已清零。

五个未从生产入口到达的候选中，`src/test/setup.ts` 由 `vitest.config.ts` 的
`setupFiles` 加载，应保留。其余四个文件只用于 Gallery：`UiMetaGrid/UiMetaItem`
的旧 metadata 网格，以及 `GlassMagnifier`、其 SVG Filter 和动画 Hook。全仓按
模块名、导出名与资源路径检索后，没有产品、宿主或其他动态入口；当前也没有真实
业务任务需要这些实现。删除四个源码文件共 440 行，并删除放大镜独用的三张 PNG
共 43,824 字节。`GlassSwitch` 的生产消费者、滤镜和两张资源保留，通用离线导出
脚本没有放大镜专用分支，继续服务当前开关资源。

Gallery 移除相应演示、四个导出登记和未使用图标导入，工作面示例改为一行普通
内容，不把旧网格搬成另一套私有组件。液态玻璃文档同步移除旧生命周期说明与
个人机器路径。原始三个 TSX 审查条目保留并标为 removed，关联 Hook/PNG 的删除
也在本节记录；它们不再占用后续组件视觉优化范围，但原 482 项总范围保持可追踪。

**验证：** typecheck、Gallery/Switch 的 10 项行为测试、123 项相关架构/覆盖合同
与生产构建通过。删除后生产可达计数仍为 1,269，唯一剩余候选是已确认的测试入口。
覆盖合同确认当前 121 个公开组件登记完整且不重复；构建目录中的 liquid-glass
只剩开关实际使用的 displacement/specular 两张 PNG。临时引用报告与删除前哈希
分别位于 `/tmp/nexus-design-a6-reachability-before.json`、
`/tmp/nexus-design-a6-reachability.json`、`/tmp/nexus-design-a6-deletions.json`；
检查日志为 `/tmp/nexus-design-a6-components.log`、`/tmp/nexus-design-a6-contracts.log`
和 `/tmp/nexus-design-a6-build.log`。该删除没有改变生产调用链；Gallery 的整体浏览器
回归仍与 A3—A5 一起等待前述授权，不借此宣称全部前端已完成验收。

## A7：可达文件中的旧目录动作导出

**引用证据：** 通过 TypeScript checker 扫描当前程序中的 1,443 个非依赖/非声明
文件，检查生产模块自有的 2,540 个具名函数/变量值导出。别名跟随原符号，生产、
测试、开发与配置引用分别记录；动态使用 namespace 时保守保留该模块全部导出。
只有 `WorkspaceCatalogAction` 没有生产引用，全仓检索进一步确认它只出现在
Gallery 和相应登记中。删除这个图标动作转发层及无用类型/导入，Gallery 用仍有
两个生产消费者的 `WorkspaceCatalogTextAction` 承担原有次动作验证，保留主次命令
计数，不另造私有替身。联系人和群聊的文字动作继续保留。

此扫描不是通用 dead-code 证明：同文件内部使用仍按生产引用保留，默认导出、
class、类型、生成协议、外部脚本和函数内分支不由它裁定。模块可达也不等于每个
导出都应公开。扫描前证据为 `/tmp/nexus-design-a7-export-candidates-before.json`，
清理后证据为 `/tmp/nexus-design-a7-export-candidates.json`。原始 CSV 的组件名称
列继续保留基线，目录动作文件标为 in_progress，并记录其中一个导出已删除；
保留的文字动作仍需随真实目录复核，不能把整文件提前标为 removed。

## A8：侧栏搜索密度与动作所有权（实现与离线验证，待浏览器复核）

**设计判断：** 当前桌面侧栏默认/最小宽度为 264px，导航轨占 64px，搜索行还要
容纳外边距、图标和创建动作。旧搜索字段在内部另设 13px 字号，英文目录名称与
尾部动作争用空间；尾部原生按钮又单独定义 hover，缺少公共 disabled 状态，并
使用原生 title。其文档却描述了已经不存在的抬升基座。

三个真实侧栏统一保留有范围的可访问名称，使用简短的“搜索 / Search”占位文字；
输入继承公共 control 的 14px 字号，不改变列表筛选字段、清除命令或创建路由。
创建动作改为组合 `UiIconButton`，共享状态、禁用、键盘焦点和唯一 Tooltip。
Pattern 只保留配套几何：桌面 36px、窄于 560px 为 48px，圆角使用 control /
control-lg token，图标 18px。侧栏宽度及导航轨保持当前值，业务不再传冗余行高。
唯一设计规范和 Form/Home 文档同步更新。

**验证与边界：** A7—A8 的 lint、typecheck、47 项组件行为测试、135 项所有权/
样式/文件/Gallery 合同、8 项依赖边界检查和生产构建通过。新增行为用例覆盖完整名称与短提示、
输入/清除/创建命令隔离、清除保留焦点、原生表单默认 type=button 及创建禁用时
搜索仍可用；没有用 JSDOM 声称几何或颜色已经验收。对应日志为
`/tmp/nexus-design-a7-check.log`、`/tmp/nexus-design-a7-components.log`、
`/tmp/nexus-design-a7-contracts.log`、`/tmp/nexus-design-a7-boundaries.log` 和
`/tmp/nexus-design-a7-build.log`。构建产物确认响应式 token 圆角 utility 已生成；
这只证明样式被编译，最终布局仍以浏览器测量为准。

既有 App-shell 浏览器场景已扩展到真实联系人侧栏：测量最窄/桌面视口的输入和
按钮高度/圆角、占位文字宽度/对比度、键盘提示、筛选与清除焦点，并保存截图；
沿用已有隔离目录，不允许业务请求穿透。该场景尚未执行：浏览器启动自动审批
曾连续两次超时，仍等待此前人工授权答复。当前实现先保存本地中间提交，实际
三主题、双语和宿主视觉复核继续列为未完成；不能用源码计算替代渲染结果。

## A9：普通分组与运行详情的文字层级（实现与离线验证，待浏览器复核）

全仓符号与 class 检索确认 overline 只有四个生产所有者、六处调用，以及 Gallery
示例/分节标题。它们表达普通配置分组、输出标签、截断提示和连接准备，不需要
独立的 10px / 全大写 / 0.16em 配方。按具体用途迁入现有角色后，删除 typed role、
映射和 CSS；不留同样样式的别名。10px token 仍供紧凑 Badge、头像和图形使用。
Gallery 移除旧样例，并补齐已存在的 control 角色样例。

运行设置的高级分组成为 supporting / medium 的 h4，并增加分割线到标题的留白；
定时历史的输出标签使用 metadata / medium。节点运行详情的折叠标题、结果、错误
和截断说明使用 supporting，分组/计数使用 metadata，短技术身份仍用 code。
RichMail 准备说明移除重复小标题，只保留实际下一步，步骤正文使用 supporting。
未改变查询、保存、运行身份、默认展开规则、产物作用域或文件打开行为。

lint、typecheck、13 项现有组件回归、135 项公共所有权/样式/文件/Gallery 合同和
生产构建通过，日志为 `/tmp/nexus-design-a9-{check,components,contracts,build}.log`。
组件回归验证最新 NodeRun 默认展开、安全引用与历史 Agent 资源归属，不能证明
字号、换行或视觉密度已通过验收。四个文件仍为 in_progress，公共角色源码归本节
记录；实际页面/检查器三主题、双语和窄屏渲染仍待此前浏览器启动授权。

## A10：运行设置字段与唯一分段组名（实现与离线验证，待浏览器复核）

删除 Runtime 私有 SettingsField 和装饰性字段图标，输入及 Provider 选择直接组合
UiField，并以 useId 派生的实例级 htmlFor/id 精确关联。密钥输入、清除按钮和获取
地址不再嵌入同一个 label；高级面板也不共用固定 ID。AnySearch 的原有 JSON
对象校验继续在 blur 时提交，错误统一交给 Field，一次播报且只关联当前输入；
保留原有三句纠正说明，JSON 字段改用公共 sm / code，不再私设字号和错误容器。
公共 Field 的容器允许在网格/弹性列内收缩，保留此前 Runtime 私有容器的布局能力。

UiSegmentedControl 的 showLabel 只表达需要可见组名的组合：通过 Field 显示 title，
且只保留外层的一个具名 group。默认无可见标签的调用继续由控件自己提供 group。
Runtime 引擎/搜索深度/提取深度与 Skill 来源认证复用该组合，移除包裹 label 或
重复同名 Field。Gallery 的两档文字组选项也覆盖 showLabel，既有浏览器场景补上
唯一 group 断言；源码、规范和组件地图同步更新，没有扩张业务控制器写入范围。

新增 10 项真实 Runtime 页面测试，覆盖六种服务的全部当前显式标签、分段组名、
数字夹限、文本 trim、密钥替换/清除、禁用和两份页面的 JSON 错误/面板隔离。
全组件 151 文件 / 389 项、149 项相关合同、lint/typecheck 和生产构建通过；日志
位于 `/tmp/nexus-design-a10-{all-components,contracts,check,build}.log`。另用 TS AST
确认本页 14 个 onBlur、24 个 onChange、2 个 onClick、27 个 disabled 和 3 个
required 属性与提交前完全一致，报告为 `/tmp/nexus-design-a10-behavior-attributes.json`。
这支持现有业务处理器未被此次布局迁移改写，不代替后端事务或完整页面视觉验收。

当前仍是本地中间提交。公共 Field、分段控件和两个领域视图继续为 in_progress；
实际窗口中的 label 对齐、JSON 高度、组宽与焦点显示仍待此前浏览器启动授权。

## A11：Room 上下文详情被裁切（用户反馈修复，待浏览器复核）

用户截图中第三名成员的 token 数被浮层底边截断。当前代码将 `36 + 行数 × 32`
作为整个详情的最大高度，但实际成员包含两行文字和内边距；外层 overflow-hidden
又无法让内部单独设定的 max-h-52 滚动区适配较小的父级上限。

删除该行高估算和列表私有高度上限，继续使用公共 status-list 的 232px 宽度及
248px / 视口高度上限。外层改为纵向弹性布局，标题不收缩，成员列表允许缩小并
在内部滚动。少量成员按实际内容展开，不把上限变成固定高度；逐成员快照、入口
最高占用、唯一详情和打开/关闭焦点策略不变。列表改用具名 ul/li，原有成员身份
仍是 React key。设计规范与 Footer 组件地图同步更新。

36 项组件/浮层/Gallery 测试、144 项相关合同、lint、typecheck 和生产构建通过。
日志为 `/tmp/nexus-design-context-height-{components,contracts,build}.log`；新增 DOM
回归覆盖三行不会被估算上限限制、锚点变化后重算可用空间及打开时增至十二行。
Gallery 增加三成员与十二成员实组件夹具，浏览器场景检查最后一行完全进入可见
滚动区、滚动时标题固定、原有宽度及 240px 矮窗口边界。该场景尚未执行，不能用
jsdom 的样式属性断言代替真实布局验收；此前浏览器启动的自动审批复核连续两次
超时，继续等待已有授权问题的回复。本项清单标记 in_progress，未宣布视觉验收完成。

自定义 MCP 动态参数/秘密行审查发现的行号 key 与同名输入，在 A12 继续收口。

## A12：自定义 MCP 动态字段与技术文本（实现与离线验证，待浏览器复核）

自定义 MCP 参数和秘密行原先按当前数组下标作为 React key，秘密键/值也重复使用
同一个可访问名称。删除前一项会复用另一行的输入节点，使选择区、原生错误与输入
身份有串位风险。参数现在由带本地 id 的草稿行持有，秘密行也保留独立 id；初始
身份由实例级 dialog ID 派生，新增身份只在视图事件中产生。纯模型按显式身份
重建草稿并投影保存输入，不生成随机值，不把本地身份写入协议。名称、命令和
URL 的原有 trim、参数/秘密值空格保留、重复键校验与已配置 null 保留语义不变。

动态输入和删除动作通过双语名称区分当前行；当前行号仅用于说明位置。普通字段
与弹窗标题改用实例 ID，类型和认证使用公共 showLabel 的唯一具名组。命令、
参数和键名复用 Input 的 code 角色，字号与尺寸继续由公共输入控件持有；密码
仍使用原来的 password 输入。移除无目标的嵌套字段组，不建立新的字段样式或
另一份保存协议；写入互斥、恢复对账与忙碌锁继续由原控制器/表单负责。

新增 14 项真实 Dialog 测试和 4 项纯模型测试，覆盖双语三种传输、同值参数的
节点/选择区保留、删除后的秘密输入/原生错误隔离、null 与替换值保存、隐藏认证
字段排除、双实例标签和忙碌关闭锁。相关 50 项组件测试、143 项合同、lint、
typecheck 与构建通过；日志位于 `/tmp/nexus-design-a12-{components,contracts,build}.log`。
测试类型修正后单独重跑 Dialog 的 14 项也通过，记录为
`/tmp/nexus-design-a12-dialog-final.log`。

Gallery 新增实际 CustomMCPDialog 的隔离编辑入口，只计数本地保存，不访问
后端或展示秘密。浏览器新增双语窄屏列布局、等宽技术文字、增删行身份与保存关闭
场景。当前浏览器清单为 972 项（`/tmp/nexus-design-a12-browser-list.log`），仅确认
已注册，未执行；此前启动审批超时仍待用户回复。此领域视图继续 in_progress，
不把 DOM 测试或测试清单当作完整视觉验收。

## A13：Provider 详情栏表单密度与只读端点（实现与离线验证，待浏览器复核）

延伸 A12 检查的显式下标 key 命中中，Sidebar/Scheduled Board 使用的是加载骨架，
StructuredContentRenderer 使用的是协议内容块序号；这些命中不属于可编辑表单行，
没有机械替换，也不据此宣称这些文件或所有历史分支已完成审查。

Settings 与 Operations 复用的 Provider 配置表单仍按整窗 md 断点排列三列，其中
两列固定占 180/260px；详情栏被目录挤窄时，名称字段可能只剩很小空间。新增
专用布局 owner，按该表单实际宽度切换：不足 480px 为单列，480px 起名称独占
首行、下方两个选择器按 2:3 分配，720px 起才恢复原来的三列宽度。此时名称至少
获得 248px（扣除原有 16px 列距）；长标签允许换行，同行控件底部对齐。此处是
布局选择，实际文字与平台字体仍需浏览器复核。

形态控件统一使用现有 md 尺寸，与 API 密钥/地址一致；获取密钥链接使用公共
supporting 角色。名称、密钥和地址不再共用固定 DOM ID。固定端点从没有输入目标
的 label 和私有 input-shell 改为 Field 的只读具名组与静态 UiListRow，保留格式
Badge、完整地址及公共 code 文字。所有字段回调、required、disabled 与 preset
条件均保持原义，没有更改 Provider 配置、授权或保存事务。

新增六项实际配置表单测试，覆盖多实例标签、原样文本/精确选择回调、失焦提交、
固定/可编辑切换、管理与单选项锁、编辑模式下的空秘密保留和预设名称限制。
相关 41 项组件测试、143 项合同、lint、typecheck 和生产构建通过；日志为
`/tmp/nexus-design-a13-{components,contracts,build}.log`。构建产物包含 480/720px
容器查询，记录为 `/tmp/nexus-design-a13-css.log`；这只能证明样式已进入构建。

新增真实 ProviderSettingsConfigForm 的 Gallery 场景，在同一个视口内改变容器
上限为 320/560/800px，检查列布局、36px 控件、横向边界、失焦回调及只读端点。
浏览器目前注册 1008 项（`/tmp/nexus-design-a13-browser-list.log`），尚未执行；
此前浏览器启动的审批复核超时仍待回复。完整 Provider 页面、其他模型弹窗和三
主题实际画面继续待审查，本表单保持 in_progress。

## A14：Provider 模型与占用确认弹窗（实现与离线验证，待浏览器复核）

三个弹窗此前使用没有高度上限的 content Shell；仅给正文 overflow 不能保证短
窗口中的操作可见。现选择公共 `adaptiveMax`，由唯一 Body 滚动，删除确认移除
第二层列表滚动。模型完整标识和受影响 Agent 名称允许换行；占用列表复用静态
UiListRow。名称投影去除首尾空白，缺失时展示本地化说明，不再回退内部 Agent ID。

标题、Model ID、上下文/输出限制和 JSON 字段按实例关联；初始焦点交给公共
Backdrop。字段与保存/取消统一 md 尺寸，技术输入只选择 code 字体，说明使用
supporting。五段能力开关合并为一份保序目录，仍逐项更新原草稿；保存中保留
动作名称，添加、保存和删除按各自真实 pending kind 提供 aria-busy。未改动
请求载荷、命令互斥、权限、原始草稿或调用方关闭策略。

新增六项真实 Dialog 行为测试，覆盖初始/归还焦点、五种能力的独立更新、原样
数值/JSON 草稿、多实例标签、权限/忙碌禁用、名称回退和显式删除。相关 53 项
组件测试通过（`/tmp/nexus-design-a14-components.log` 的 30 项与
`/tmp/nexus-design-a14-fields.log` 的 23 项），136 项架构/公共 UI 合同、lint、
typecheck 和构建通过，日志为 `/tmp/nexus-design-a14-{contracts-final,lint,typecheck,build}.log`。
首次合同运行发现 Settings 原生按钮门禁扫描了测试 Harness；现在两项 Settings
DOM 所有权检查只排除 test/spec 文件，所有生产文件仍受原有检查。

Gallery 增加实际三个弹窗，模拟长模型标识、16 位长名称使用者与保存中状态，
所有回调仅改本地夹具。新增浏览器检查在 360px 高窗口验证完整边界、正文唯一
滚动、最后一项可达、Footer 位置、14px 技术输入、36px 控件与焦点归还。
当前浏览器清单 1044 项（`/tmp/nexus-design-a14-browser-list.log`），只是注册，
尚未执行；此前启动审批复核超时仍待用户回复。三个视图标为 in_progress；完整
Provider 页面、数值校验、请求中关闭策略及三主题实际画面继续待审查。

## A15：恢复 Session 多标签的增量持久化（行为回归修复，待实际浏览器复查）

用户反馈点击新建或切换历史后只剩一个 Session 标签。已在控制器测试复现
一个丢标签条件：当前页仍持有旧会话列表时，其他页面或新命令打开的标签会被
展示过滤排除，原来的 effect 随后把过滤后的完整集合写回，导致两个标签变回
一个。修复前失败记录为 `/tmp/nexus-session-tabs-stale-before.log`。这一复现
证明状态覆盖缺陷；用户实际页面是否还有其他触发原因仍需浏览器验证。

现在普通选择和路由激活只追加精确目标，关闭只从同 owner 的最新持久快照
移除精确目标；列表刷新只更新展示，不清除暂时缺失的标签，也不反复争抢其他
页面的活动偏好。创建时间排序、当前标签的邻居回退、关闭不取消固定、显式
删除清理及最后标签的替换事务保持原有职责。没有修改后端会话或草稿规则。

24 项回归通过，覆盖真实页面路由的新建和历史选择、DOM 中多个标签共存、
重新挂载恢复、活动/非活动关闭、固定保留、创建单飞及最终替换；存储测试使用
独立模块实例验证旧页面操作和重新加载。155 项持久化、身份及架构合同检查、
lint、typecheck 和构建通过，日志为 `/tmp/nexus-session-tabs-tests-final.log`、
`/tmp/nexus-session-tabs-contracts.log` 和
`/tmp/nexus-session-tabs-{lint-final,typecheck-final,build}.log`。

此前隔离浏览器启动的自动审批复核超时，实际浏览器验收仍待允许启动；不把
jsdom 的 scroll/pointer API 占位视为布局、点击命中或原生宿主证明。标签业务
入口标为 in_progress，整体视觉审查继续保留。此前进行中的 Settings 开关
统一工作另行完成和提交，不混入本次回归修复。

## A16：设置开关行与说明关联（实现与离线验证，待浏览器复核）

常规设置的五项二元偏好与运行引擎的工具发现此前各自拼接图标、标题、说明、
右侧重复标签和开关。窄屏会把后两项另起一行；多个控件只以“启用”命名，无法
从控件本身判断其作用。现统一为设置域 `SettingsToggleRow`，复用原有标题、
说明和图标配方；左右留白 16px、上下 12px、列距 12px，文字收缩换行，sm
开关保持尺寸。标题作为名称，useId 生成实例说明关联，静态行不新增点击命令。

GlassSwitch 新增可选 aria-describedby 并原样转发到唯一原生按钮，不改动
折射几何、点击、键盘或受控状态。Browser CDP 保留独立风险卡，以实例 ID 同时
关联风险与说明，说明提升为 supporting / muted；添加模型弹窗保留表单内联
布局，仅补充启用说明关联。授权列表、模型列表和能力开关仍沿用各自明确的
领域组合，没有为了统一行而改变权限、确认或提交边界。

删除已经无引用的六个英文、六个中文重复 toggle label；常规四项偏好继续使用
Preferences 加载/保存锁，Echo 继续使用自身恢复、加载和保存锁，工具发现继续
使用 Runtime 控制器状态。新架构门禁阻止 General 和 Runtime 绕过该行所有者
直接重建 GlassSwitch 行。新增域内 Pattern 不增加公共 UI 导出数量。

34 项组件测试通过，覆盖双语真实名称、说明关联、多实例命令隔离、单一命中区、
独立锁与精确叶子更新、CDP 权限门禁，以及已有键盘、焦点和表单行为。144 项
架构与样式合同、lint、typecheck、生产构建通过；日志为
`/tmp/nexus-design-a16-components-final.log`、
`/tmp/nexus-design-a16-contracts-final.log`、
`/tmp/nexus-design-a16-{lint-final,typecheck-final,build}.log`。

Gallery 使用实际共享设置行及双语长说明，浏览器检查覆盖窄屏边界、标题与开关
分离、13px 说明、主题对比度、键盘切换、静态说明点击和保存锁。清单现有 1080
项（`/tmp/nexus-design-a16-browser-list.log`），仅完成注册；此前浏览器启动的
自动审批复核超时仍未解除，因此没有执行浏览器或声称原生宿主验收。六个相关
视图/原语保留 in_progress；482 项清单当前为 430 pending、49 in_progress、
3 removed，所有存活条目的源码摘要已同步。

## A17：公共列表焦点与 Connector/MCP/Loop 目录动作（待浏览器复核）

全局焦点层仅覆盖原生控件，带 role=button 的 UiListRow 没有消费同一焦点
配方。现由 ListRow 内部样式为可交互行提供 2px、语义 ring、内嵌焦点环，
避免滚动容器边缘裁切；active 行继续显示焦点，静态和 disabled 行不增加
Tab 入口。原有 Enter/Space、阻止冒泡和 busy/muted 语义保持不变。

Connector 卡片、自定义 MCP 编辑/删除和 Loop 复制入口统一使用
UiListActionButton，删除私有 IconButton 包装与动作事件 helper。MCP 开关
仍保留自己的事件隔离边界，busy 或 recovery_required 阻止原有写操作，
查看详情仍可用；复制保持原始指令文本并不触发详情导航。Connector 卡片
改为由模型输出本地化 key，视图使用当前语言及真实名称；删除私有规则数组和
不必要的泛型查找 helper，保留忙碌、已连接、即将推出、应用/凭证配置、普通
连接的原有优先级。目录仍不会展示 coming_soon 占位资源。

Connector 与 MCP 的私有加载/空态改用 UiResourceState，消除单独字号、
高度和边框组合。目录首次空集保留原有添加动作，过滤无结果不新增动作，
读取失败继续显示原有显式恢复入口；未改变读取、过滤、授权或 CRUD 协议。

31 项组件/模型测试与 155 项架构、样式和资源可靠性合同通过；lint、
typecheck、构建通过。最终组件日志没有异步 act 警告，相关日志为
`/tmp/nexus-design-a17-components-final.log`、
`/tmp/nexus-design-a17-contracts-final.log` 和
`/tmp/nexus-design-a17-{lint-final,typecheck-final,build-final}.log`。

实际 Gallery 已补充键盘顺序与内嵌焦点检查，覆盖侧栏、紧凑、flush、active
行、静态/禁用项跳过及尺寸稳定。浏览器清单现有 1116 项，仅完成注册
（`/tmp/nexus-design-a17-browser-list.log`）；此前启动的自动审批复核超时
尚未解除，不将注册项算作执行结果。五个相关条目保留 in_progress，清单为
428 pending、51 in_progress、3 removed，存活源码摘要一致。目录长名称与
元信息密度、Connector 详情中的硬编码中文及实际主题/宿主效果仍需继续审查。

## A18：Connector 详情与按工作面换行的身份区（待浏览器复核）

Connector 详情的主动作、认证/连接状态、准备说明、工具说明和能力弹窗辅助文字
接入现有双语目录。服务端能力名称、内容、权限列表和端点保持原样；RichMail
真实中文菜单名称在英文说明中继续原样引用。合并 OAuth 应用配置按钮的重复
实现，保留首次配置/重新配置的动作样式、busy 禁用、精确参数和资格分支。

加载、失败与缺失面使用 UiResourceState 的同一尺寸，刷新失败仍保留已读取
详情；未引入自动重试或连接。RichMail 准备区复用 filled UiPanel，步骤作为
可换行正文，移除私有品牌色组合、装饰图标和整句操作徽标。连接事实按实际
工作面宽度分列；详情数据没有 Token 有效期字段，删除“通常 7 天有效”的
静态推测。能力行使用公共 compact ListRow，去除逐行重复成功图标。

CapabilityDetailIdentity 统一按自身可用宽度排列身份与动作，空间不足时动作
换行，长名称和说明可断行；避免大窗口内窄面板仍受整窗断点挤压。此共享修改
覆盖其现有消费者，其他能力详情的具体业务和视觉仍需按清单继续复核。
能力弹窗原有实际 dialog 没有可访问名称，现通过实例唯一 Header ID 命名，
并以隐藏描述关联 Connector 身份；正文和 scopes 可换行，scopes 保持默认
折叠，切换 Connector 会关闭旧预览。

35 项组件/模型测试通过，包括 19 项多标签模型、控制器、真实组件和页面
导航回归，以及 16 项能力布局与 Connector 详情回归。覆盖新建、历史切换、
旧列表覆盖、关闭和重新挂载；详情覆盖双语动作、精确参数、busy、状态优先级、
失败保留、能力预览隔离、原始内容与说明关联。171 项架构、样式、资源可靠性、
OAuth 应用替换和标签持久化合同通过；lint、typecheck、生产构建通过。
日志：`/tmp/nexus-design-a18-components-final.log`、
`/tmp/nexus-design-a18-contracts-final.log`、
`/tmp/nexus-design-a18-{lint-final,typecheck-final,build-final}.log`。

Gallery 直接渲染实际详情、长名称/端点和能力弹窗，以本地状态模拟连接和忙碌，
不执行认证或 MCP 请求。新增浏览器案例覆盖窄工作面、短视口、自然换行、禁用、
弹窗名称/描述、scopes 展开和键盘返回；1152 项浏览器清单仅完成注册
（`/tmp/nexus-design-a18-browser-list.log`），此前浏览器启动的自动审批复核
连续超时仍未解除，未运行浏览器或原生宿主验收。相关五个视图/布局保持
in_progress；482 项清单为 425 pending、54 in_progress、3 removed，
存活源码摘要一致。多标签修复仍不能代替用户当前窗口的实际复现验收。

## A19：最后可见标签替换仍保留其他打开项（待实际窗口复现）

沿多标签完整关闭链路发现 A15 的遗漏：最后可见标签的替换事务仍会将持久集合
整批写成替代项。旧列表未包含其他已打开标签，或者等待创建期间另一页面打开
新标签时，事务完成会误清它们。新增两个真实 Hook/Store 回归分别覆盖事务前
已有及事务中追加；修改前均复现只剩替代标签
（`/tmp/nexus-design-a19-before.log`）。

完成回调现复用精确关闭命令，从最新 owner 快照只移除原目标、加入替代项，
其他标签和固定偏好保留。删除最后一处整组覆盖调用和无剩余消费者的 Store
接口，测试夹具通过正常打开命令建立偏好。原有单飞、最后标签 runtime 关闭
顺序、失败/较新导航守卫和仅复用当前内部 draft 的规则保持不变。

21 项标签模型、Hook、真实组件和页面路由测试通过；131 项持久化、架构与
基础组件合同通过，lint、typecheck、构建通过。日志为
`/tmp/nexus-design-a19-components.log`、`/tmp/nexus-design-a19-contracts.log`、
`/tmp/nexus-design-a19-{lint,typecheck,build}.log`。本批没有 UI 几何修改或新增
生产 TSX，482 项视觉审计数量保持不变；浏览器启动自动审批复核超时仍未解除，
不宣称用户当前窗口的实际多标签操作已验收。

## A20：公共弹窗自动命名与实例隔离（待浏览器复核）

此前标准 Header 和实际 dialog 根的名称关联全靠消费页手动连接，扫描发现
12 个产品入口已有公共 Header.title 却没有关联：Skill 导入、来源管理/编辑、
外部 Skill 预览、OAuth 应用配置、Device Flow、RichMail 配对、直接凭据、
飞书手动/方式选择、WorkGraph 选择器与 Loop 选择器。该扫描只证明当前静态
接线缺项，不将源码存在等同于这些业务页面已逐一视觉验收。

现在 Header 在布局提交阶段将实际标题 ID 注册到最近 Backdrop，标题变化
保留实例身份，标题消失释放关联，嵌套 Portal 不污染外层名称。同名弹窗使用
各自 useId；StrictMode 下的注册/释放已覆盖。已有显式 labelledBy、
aria-labelledby 和 aria-label 继续优先。自定义 Header.children 或无标题
预览仍显式命名，复杂正文不会被自动扁平化为描述。删除能力详情弹窗和标准
Gallery 的重复标题 ID 接线，保留 Connector 身份的独立描述关联。

新增公共命名回归在修改前复现缺名；修改后 25 项定向组件测试通过，包括动态
标题、同名/嵌套隔离、自定义标题、显式名称优先，以及飞书步骤切换、凭据提交、
来源编辑和权限范围预览。公共 primitive 影响所有消费者，因此补跑全量组件
测试：161 个文件、462 项全部通过；154 项架构/样式/OAuth 合同、lint、
typecheck、生产构建通过。相关日志：`/tmp/nexus-design-a20-before.log`、
`/tmp/nexus-design-a20-components-final.log`、
`/tmp/nexus-design-a20-components-all.log`、
`/tmp/nexus-design-a20-contracts-final.log`、
`/tmp/nexus-design-a20-{lint-final,typecheck-final,build}.log`。

新增 import-aware 源码门禁要求产品 Dialog 提供显式名称或同一模态内的标准
Header，嵌套 Dialog 内的 Header 不能替外层命名；实际有效文本与条件分支仍由
行为测试和浏览器负责。标准 Gallery 现直接消费自动命名，既有浏览器案例补充
标题 ID 关联、嵌套身份分离和关闭后外层名称保留。浏览器仍只有 1152 项注册
（`/tmp/nexus-design-a20-browser-list.log`）；启动自动审批复核超时尚未解除，
未执行实际浏览器/原生宿主检查。公共 Dialog 与能力预览保持 in_progress，
清单为 424 pending、55 in_progress、3 removed，存活源码摘要已同步；其余
12 个入口的文字、布局和业务视觉仍保留各自待审状态。

## A21：Composer 目录选择器与异步选择收口（待浏览器复核）

Loop 和工作图选择器改用公共 Panel 持有外边界，删除两处私有圆角/边框组合，
加载和空/失败主面使用 ResourceState 的共享 sm 尺寸。Loop 连续行使用 flush
ListRow，动作与分类/触发元信息分别采用 supporting 和 metadata，避免 11px
淡字；标题允许长单词换行，原用途摘要保留两行。工作图 Slash 和内置来源信息
改为可读 metadata，预览标题/说明和使用按钮复用公共排版/尺寸，保持目录和
只读草图的既有层级。

两种选择器都通过公共 Dialog.initialFocusRef 把初始焦点交给搜索，删除 Loop
与模态默认聚焦竞争的额外 effect。工作图 listbox 以选中项作为唯一 Tab 入口，
方向键、Home/End 同时移动焦点与预览；浏览不会插入命令，明确使用才按原格式
写入 `/<slash_name> `。删除从 Composer 向 owner 目录传递、却从未消费的
Session 参数，保留父级入口资格判断，不更改命令的当前 Composer 目标。

Loop 选择发现两个可复现遗漏：连续调用在 React 提交 busy 前可重复启动；
关闭并重新打开后，旧启动成功仍调用 onClose。修改前两项回归失败
（`/tmp/nexus-design-a21-before.log`），现同步认领单飞并在开放作用域结束后
忽略旧结果的 UI 收尾。已经开始的业务操作继续由原命令负责，不自动重放或
因为关闭选择器而取消。分类匹配删除逐行构造 Set/布尔数组的无必要 helper，
仍按同一分类和原始字段搜索。

34 项定向组件测试通过，包含 7 项新增选择器/控制器回归和已有公共 Dialog、
List 行为。覆盖搜索焦点、过滤/清理、精确启动、错误重试、访问失效隐藏、普通
刷新失败保留、键盘选项、原始 Slash、重新打开以及迟到结果隔离。156 项
架构、样式和资源可靠性合同通过；lint、typecheck、生产构建通过。
日志：`/tmp/nexus-design-a21-components-final.log`、
`/tmp/nexus-design-a21-contracts.log`、`/tmp/nexus-design-a21-lint.log`、
`/tmp/nexus-design-a21-typecheck-final.log`、`/tmp/nexus-design-a21-build.log`。

浏览器夹具挂载实际完整选择器，目录 GET 由案例内固定响应拦截，选择只写本地
output；其他 API 请求拒绝转发。新增短窗口案例检查搜索焦点、12px 元信息、
宽度边界、选项键盘与明确使用。现有 1188 项仅完成注册
（`/tmp/nexus-design-a21-browser-list.log`），浏览器启动审批复核超时仍未解除，
不作为执行证据。五个相关 TSX 条目保持 in_progress；482 项清单为
419 pending、60 in_progress、3 removed，存活源码摘要一致。

## A22：会话历史可读性与编辑键盘边界（待浏览器复核）

桌面历史的时间从 caption / soft 改为 metadata / muted，选择数量从 10px
改用同一公共元信息角色；全选标签采用 supporting，底部按钮使用 sm 并允许
窄宽度换行。菜单标题复用 sectionTitle，空历史保留短说明而不添加装饰图标。
未确认的批量删除结果通过 InlineNotice 保留原有数量、影响和核对步骤，移入
正文滚动区，避免长英文说明扩大固定顶部区域、挤掉列表与底部操作。未更改
删除顺序、草稿锚点、失败集合或重放策略。

历史次动作原先在外层再次隐藏，绕开公共 ListAction 的行内键盘焦点与无 hover
规则。现只传 visible / hover 语义，删除外层透明度和过渡组合；固定动作宽度、
行标题和外部来源布局继续由原有 List 所有者负责。删除无意义的空继承类型。
标题编辑复现中文输入法 Enter 提前重命名，修复后使用已有公共 IME 判定；
候选选择的 Enter/Escape 留给输入框，普通 Escape 先取消草稿，保存/取消后
恢复重命名按钮焦点。随后 Tooltip 与历史菜单继续按公共最上层协议逐次关闭。

移动切换器原先没有传递 locale，英文界面仍使用中文相对时间；列表为空也没有
说明，且所有实例共用同一标题 ID。现跟随当前语言，以 metadata / muted 显示
时间，过滤后为空时显示本地化短状态，标题命名按实例隔离。保留原有草稿过滤、
选择命令、列表密度、专用顶部下拉材质和公共模态生命周期。

修改前四项新增回归失败：IME 误保存、英文时间、中文空态及重复标题 ID，
证据 `/tmp/nexus-design-a22-before.log`。现 31 项定向组件/键盘测试通过，包含
5 项新增历史/移动切换器回归；153 项架构、样式、token 与历史删除模型合同
通过，lint、typecheck 和构建通过。日志分别为
`/tmp/nexus-design-a22-{components,contracts,lint,typecheck,build}.log`。

浏览器新增真实历史/移动入口的隔离夹具，编辑、删除与切换只写本地结果，API
全部拒绝转发；案例覆盖短窗口、公共次动作焦点可见性、编辑取消、未确认结果
与底部操作、移动时间及空历史。1224 项只完成注册，浏览器启动审批超时仍未
解除；不声称执行或宿主验收。复核公共 token 时另修正 A21 案例把 metadata
误写成 13px 的断言及记录：实际 canonical 角色为 12px，未改动字体 token。
本批三项 TSX 保持 in_progress；482 项清单为 416 pending、63 in_progress、
3 removed，存活源码摘要一致。

## A23：公共输入弹窗与输入法关闭边界（待浏览器复核）

沿 A22 的输入法问题检查公共 Prompt、模态和锚定浮层。后两者在修改前会消费
候选 Escape，Prompt 的字段名称还直接使用占位示例；有效修改前证据见
`/tmp/nexus-design-a23-before.log` 的模态/浮层及字段命名回归。该初次日志中的
两个 Prompt IME 用例缺少 Provider，是夹具问题，修正后才纳入有效行为检查。
现三处事件边界复用既有 isImeKeyboardEvent；不新增 composition 延时/标记状态，
保留非输入法单行 Enter、多行普通换行和 Cmd/Ctrl+Enter 提交。模态与浮层
仍各自拥有关闭仲裁和焦点规则。

Confirm/Prompt 的默认按钮及多行快捷键说明通过 i18n 提供，显式业务文案优先。
输入默认用标题命名，Shopify 通过 inputLabel 明确命名为店铺子域名；占位文字
保留示例职责。业务消息关联到弹窗，UiField 精确绑定控件 ID、字段错误和
多行快捷键说明；删除 Prompt 私有小字提示及 kbd 色块。Prompt busy 同时限制
字段、按钮、键盘与遮罩退出，由调用方提供状态，不猜测命令结果。

已核对全部三个生产 Prompt 入口：Workspace 创建文件/目录及重命名接入既有
isMutating，三个模式分别验证原始值与执行中关闭锁；Shopify 保留相同域名
规范化与一次性 Promise 结算，删除模型中的中文提示常量，invalid 状态由
视图本地化为字段错误，切换语言保持草稿，用途说明不被错误替换；子任务补充
指令继续传递自身发送文案、五行输入、专用快捷键说明与原有 scope/capability
判断，本批不改其控制命令或任务资源。最后一项为调用点复查，不能替代整个
子任务页面的独立验收。DecisionFrame 的宽度、Footer 与 Button 组合保持现有
所有权，未增加另一套弹窗骨架。

公共行为变更后全量组件回归最终通过：166 文件、486 项测试，包含本批新增
12 项共享/Workspace/Shopify 回归。145 项架构、文件合同、样式与 token 门禁
通过；lint、typecheck 和构建通过。日志为
`/tmp/nexus-design-a23-all-components-final.log`、
`/tmp/nexus-design-a23-contracts.log`、`/tmp/nexus-design-a23-lint-final.log`、
`/tmp/nexus-design-a23-typecheck-final.log`、`/tmp/nexus-design-a23-build.log`。

Gallery 在原有嵌套 Prompt 案例中补充多行输入、双语名称/说明、短窗口与原始
换行提交，并保留默认/显式动作测试。1224 项仅完成注册；浏览器启动审批超时
仍未解除，DOM 合成 composition 事件也不能作为原生输入法验收。本批四个
TSX 条目保持 in_progress，482 项清单为 412 pending、67 in_progress、
3 removed，存活源码摘要一致。

## A24：复选行的可读文字、说明身份与禁用一致性（待实际浏览器复查）

公共 `UiCheckboxRow` 的 compact 标签仍使用 11px caption，与常规紧凑字段
不一致；说明使用 12px metadata，且处在隐式 label 内，没有独立描述关联。
原生 checkbox 虽已禁用，外层仍无条件响应 hover。已检查全部两个生产消费
文件：Runtime 的私有网络/服务商提取选项，以及 Scheduled Task 的启用项。
新增五项 DOM 回归在修改前均因名称或描述关联不符合预期失败，记录为
`/tmp/nexus-design-a24-before.log`；这不是五个独立缺陷，也不是视觉证据。

compact 标签与两档说明统一为 supporting / 13px，标准标签维持 control；
compact 内容间距调整为 8px，保持原有最小高度与水平留白。长文字可以换行，
装饰图标不参与名称；实例级 label/description ID 保留调用方显式名称和
额外描述。原生 input 或所属 fieldset 禁用时不产生行 hover，选择框、文字
与图标分别弱化一次。整行点击、Space 和业务布尔值回调继续由 native checkbox
持有，不新增点击代理。设计与工程合同分别更新唯一规范。

Runtime 删除无状态、只转发属性的 `SettingsCheckSetting`，两处直接消费公共
行；三个提取服务的实际页面回归证明两个选项分别提交精确 patch，保存时均
禁止变更。Scheduled Task 的启用回调保持不变；此次只审查该行，不宣称复杂
调度表单已完成。ChoiceButton / RadioChoice 仅完成所有者定位，其他密度和
业务选择分支仍待后续审查。

本批针对性 50 项测试通过；随后全量组件 167 文件、494 项测试通过，145 项
架构/文件/控件样式/token 合同通过，lint、typecheck 和生产构建通过。日志为
`/tmp/nexus-design-a24-components-final.log`、
`/tmp/nexus-design-a24-all-components.log`、`/tmp/nexus-design-a24-contracts.log`、
`/tmp/nexus-design-a24-lint.log`、`/tmp/nexus-design-a24-typecheck.log`、
`/tmp/nexus-design-a24-build.log`。

Gallery 增加真实 compact 复选行和可切换 fieldset 禁用范围，浏览器案例检查
双语名称/描述、13px 文字、窄行换行、键盘与禁用 hover。1260 项仅完成注册
（`/tmp/nexus-design-a24-browser-list.log`），浏览器启动自动审批复核超时仍待
用户许可；没有执行完整 `npm run check` 中需监听端口的夹具或浏览器门禁，
也不将 DOM 回归当作布局/原生宿主验收。482 项清单为 410 pending、69
in_progress、3 removed，本批相关条目保持 in_progress，存活源码摘要一致。

## A25：选择控件的公共状态与权限信息完整性（待实际浏览器复查）

检查 `UiChoiceButton / UiRadioChoice` 的全部当前生产调用位置：十一处 Feature
文件及共享 IconPicker，包含执行位置/高级设置、星期多选、日期/时间、来源过滤、
幻灯片缩略图、WorkGraph 版本、成员参与状态、Agent 权限模式和工具授权范围。
普通 surface 的旧 xs/sm/md/lg 字号为 11/12/12/13px；日期/时间分别复制了禁用、
动效和选中色，使用 `pointer-events-none` 代替禁用命中，还存在 40% 与公共
disabled token 不同的透明度。Radio label 对 fieldset 原生禁用没有配套视觉。

现在普通选择项使用 12/13/14/14px 的公共 Typography 与 medium，28/32/36/40px
最小高度配套调整垂直留白，图标间距为 8px。日期保留 32px 网格格高，改为
13px 等宽数字；时间列保留 40px block 行和 16px 等宽数字，二者共用公共 primary
前景/背景。头像、缩略图、版本胶囊与星期选择的内容几何和业务状态保留。
四种 variant 共用 native/fieldset disabled、opacity 与 ring，不再复制禁用
样式参数、穿透命中区或在 disabled 时覆盖选中 tone。删除重复数字样式、默认值
转发 helper 与 Radio 的重复焦点/禁用配方。

工具权限的两个展示实例原先以同一个 request ID 命名原生 radio group；新增测试
确认其 name 相同且没有具名描述组，修复前记录为 `/tmp/nexus-design-a25-before.log`。
这证明 DOM 身份/关联问题，不声称 jsdom 已验证浏览器的方向键分组：尝试的
user-event ArrowRight 因环境缺少 CSS.escape 无法执行，已移除该断言并将它交给
真实浏览器案例，没有用伪造键盘处理器替代。当前视图用 useId 隔离原生组，直接
组合 UiRadioChoice 与 UiField；删除 PermissionChoice 纯包装和仅为 DOM 分组
传入的完整请求。业务控制器仍按原 request ID 重置索引并持有允许/拒绝命令。
三项视图回归覆盖独立实例、精确建议索引、具名禁用原因和没有可复用范围时的
完整输入/原因保留，选择本身不执行授权。

Agent 权限卡移除标题截断、两行说明限制和补偿性的原生 title；control 标题与
supporting 说明完整换行，卡片随内容增长。实例级名称/描述分别关联到实际按钮，
原模式值、危险提示和工具/Connector 开关命中行为保持不变。

全量组件 169 文件、507 项测试通过；145 项架构/文件/控件样式/token 合同、lint、
typecheck 与构建通过。最后检查保持数字选择 block 布局及 8px 公共间距后，相关
12 项回归与构建再次通过，没有重复无关全量检查。日志为
`/tmp/nexus-design-a25-all-components.log`、`/tmp/nexus-design-a25-contracts.log`、
`/tmp/nexus-design-a25-lint-final.log`、`/tmp/nexus-design-a25-typecheck-final.log`、
`/tmp/nexus-design-a25-numeric-final.log`、`/tmp/nexus-design-a25-build-final.log`。

Gallery 新增四档尺寸、四 variant、fieldset 禁用和两个真实权限视图；浏览器案例
覆盖计算字号/高度、数字选中色对比度、禁用 hover/cursor、方向键组内循环和
Agent 卡片完整说明。1296 项只完成注册，没有执行浏览器或原生宿主验收；启动
自动审批复核超时仍待许可，完整 check 中需要监听端口的夹具也未运行。482 项
清单为 398 pending、81 in_progress、3 removed；本批十四个相关条目保持
in_progress，消费者记录只覆盖选择项用法，不宣称完整页面已完成，源码摘要一致。

## A26：定时任务字段关联、说明与调度密度（待实际浏览器复查）

基础与高级表单、计划面板原先使用固定字段 ID，两个实例同时存在时标签会关联
到另一个实例；间隔数值没有名称，执行/投递/权限与星期选择的标题和说明也未
形成具名组。新增回归在修改前出现 8 项失败、1 项通过
（`/tmp/nexus-design-a26-before.log`），证明这些 DOM 关联问题，不代表布局验收。

基础/计划面板现以实例 ID 派生字段身份；窄 Session 字段以及独立有效期、专用
会话字段各自持有身份。选择组组合 UiField 统一名称与 supporting 说明，移除
重复标题/说明结构及默认禁用谓词包装，高级摘要使用 metadata。保留实际条件
投影 helper、选项禁用判断和所有原始回调，不重写业务草稿、资源或提交控制器。

调度选项改用既有分段控件的独立组名与等宽布局，释放选项横向空间，没有新增
公共变体。间隔数值/单位各自具名、并排对齐，输入与选择器使用相同默认高度；
每月日期和原始 Cron 也复用默认输入尺寸，Cron 使用公共 code 文字角色。原生
min/max/step/required、星期独立切换、时区、指令与失败对账分支保持。弹窗删除
固定标题 ID，复用公共 Header 的自动命名，提交按钮显式投影已有 busy 状态；
关闭锁和固定响应式弹窗几何保持。

新增及扩展的 9 项表单测试通过：覆盖 Agent/Room 高级字段和四种调度的双实例
关联、具名组与说明、原样间隔/指令和精确选项回调。全量组件 170 文件、515 项
测试通过；164 项架构、文件、控件样式、token 及定时任务身份合同通过，后者
含 12 项既有业务回归，覆盖 DM/IM/Room 候选、独立执行者/接收者、提交载荷与
编辑重绑。lint、typecheck、构建通过（构建保留既有大 chunk 提示）。日志为
`/tmp/nexus-design-a26-all-components.log`、`/tmp/nexus-design-a26-contracts.log`、
`/tmp/nexus-design-a26-lint.log`、`/tmp/nexus-design-a26-typecheck-final.log`、
`/tmp/nexus-design-a26-build.log`。

Gallery 组合真实草稿/调度 Hook 与生产面板，固定本地候选，不挂接资源请求或
任务提交。浏览器案例覆盖窄栏选项边界、间隔高度/标签焦点、说明关联、原样
输入与 Cron 字体。1332 项只完成注册（`/tmp/nexus-design-a26-browser-list.log`），
尚未执行；此前浏览器启动被自动审批复核连续超时拦住，仍待许可，完整 check
中需要监听端口的夹具也未运行。482 项清单为 397 pending、82 in_progress、
3 removed；四个生产视图保持 in_progress，源码摘要一致。实际完整弹窗、资源
失败/对账、三主题和原生宿主继续待复查，不宣称定时任务页面已完整验收。

## A27：公共分段选择的密度、完整标签与提示所有权（待实际浏览器复查）

审查公共分段选择与九个生产消费者中的十三处用法：Settings 外观/行为/Runtime、
联系人视图、Mermaid、Custom MCP、Skill 导入/来源认证及定时任务。此次记录只
覆盖这些选择器，不将消费者整页标为完成。原先两档文字均为 11px semibold，
强制单行，文字选项高度也与同档纯图标项不一致。当前常规/紧凑分别采用
control 14px / supporting 13px、medium 和 32/28px 选项最小高度；保留外壳
padding/border、图标尺寸与所有选项值。文字可在受限栏宽内换行，同组文字项
随内容等高；stretch 按内容伸缩填满栏宽，避免较长短词被强制等分宽度挤压。
没有改变 native button、aria-pressed、业务回调或 Preferences 保存边界。

纯图标选项复用 UiTooltip 的 hover/focus 与 Escape 生命周期，移除按钮及组的
原生 title，文字组不再额外提示自身名称。删除只有 Gallery 主题选择曾使用的
组图标参数、展示分支及独占 CSS/token；类型检查发现并收口该 Gallery 调用，
生产选项图标继续保留。General 默认消息行为移除手写标题/包装，使用 showLabel
形成唯一可见 Field 组，排队/打断命令及禁用条件保持。

新增回归先在完整语言夹具下证明两项旧合同不符：图标提示仍是原生 title，
默认消息行为的可见标题未关联到 group（2 failed / 10 passed，
`/tmp/nexus-design-a27-before-valid.log`）；原生 prop/fieldset 禁用用例原本即通过。
本批四项新增组件回归覆盖唯一键盘提示、Escape 焦点保留、受控值、禁用恢复、
不提交外层表单及精确偏好命令。样式门禁加入 UiSegmentedControl，验证别名、
展开属性中的字体/颜色/圆角覆盖被拒绝，外部布局继续允许。

全量组件 171 文件、519 项通过，153 项架构/文件/控件/token 合同通过；lint、
typecheck 和构建通过。最终横向布局调整后，相关 32 项组件回归与构建重新通过，
没有重复无关全量检查。日志为 `/tmp/nexus-design-a27-all-components.log`、
`/tmp/nexus-design-a27-contracts.log`、`/tmp/nexus-design-a27-lint-final.log`、
`/tmp/nexus-design-a27-typecheck-final.log`、`/tmp/nexus-design-a27-layout-final.log`、
`/tmp/nexus-design-a27-build-final.log`。构建仍保留既有大 chunk 提示。

Gallery 增加两档长名称，并扩展实际浏览器检查：计算字号/字重、图文统一高度、
窄栏完整换行与等高、唯一键盘提示、禁用及 280px 调度短选项单行；分组逐一截图，
避免长夹具整体截图中的滚动裁切。1332 项仅完成注册，尚未执行；此前浏览器
启动被自动审批复核连续超时拦住，仍待许可，完整 check 的监听端口夹具也未运行。
482 项清单为 393 pending、86 in_progress、3 removed，所有源码摘要一致。字号
改变后的真实业务布局、三主题与宿主验收继续待核对；Skill 导入及来源编辑中
发现的固定字段 ID 留在各自业务表单审查中，不把本批分段改进视为全表单完成。

## A28：Skill 导入与来源管理的完整字段、说明和动作边界（待实际浏览器复查）

沿 A27 的消费者继续检查 Git/本地导入、格式指南和来源列表/认证编辑。Git 与
私有来源编辑原先使用固定 ID；两个实例并存时，标签会指向另一份字段。新增
用例在修改前连同来源动作组/提交 busy 检查共出现 4 failed / 3 passed
（`/tmp/nexus-design-a28-before.log`）。当前字段以 useId 隔离，地址、branch 与
path 使用公共 code 文字角色，提交原始值、原生 URL/必填验证保持。双编辑器
测试显式作用于第二个实例，不把 jsdom 视为原生模态遮挡或点击命中证明。

来源列表组合 filled Panel 与静态 flush ListRow，让公共所有者负责表面和行
密度；名称使用 control、地址使用 metadata 并完整换行，凭据状态和本地化错误
使用 supporting，不再截断或缩到 soft caption。编辑/删除/Switch 形成以来源名
命名的动作组，Switch 关联本行地址和状态说明；整行仍不可点击，删除仍先确认，
原始诊断不回显。认证编辑维持不可改地址、Token 留空沿用、模式切换保留草稿和
保存失败保留编辑器；仅为原在途状态增加 aria-busy。移除早已被本地化分支覆盖
的 private_registry 英文映射，不改变其他 kind 的回退或目录排序。

导入区删除四个单次调用的转发/状态包装，直接组合公共分段、Button 与 Spinner；
Git 和 zip 动作显式展示 importing busy。zip 说明和格式规则改为 supporting，
frontmatter 示例用 filled Panel 替代局部 color-mix，示例内容与指南下载保持。
保留 SOURCE_VIEWS 模式映射、草稿/提交 Hook 和文件输入引用，未改市场请求、
恢复控制器、导入格式解析或后端协议。

9 项相关组件回归通过，覆盖双实例身份/精确命令、Token 保留、具名独立来源
动作与删除确认、Git 草稿跨模式保留、本地文件入口、原样提交、关闭重置和
importing 下的输入/模式/提交/Escape 锁。全量 172 文件、524 项组件测试通过；
157 项架构、文件、控件、token 和 Skill 恢复合同通过，后者验证操作身份隔离、
只读对账及不能从 Git/本地导入读取猜测写成功。lint、typecheck、构建通过，
构建保留既有大 chunk 提示。日志为 `/tmp/nexus-design-a28-focused-final.log`、
`/tmp/nexus-design-a28-all-components.log`、`/tmp/nexus-design-a28-contracts.log`、
`/tmp/nexus-design-a28-lint-final.log`、`/tmp/nexus-design-a28-typecheck-final.log`、
`/tmp/nexus-design-a28-build.log`。

Gallery 使用真实导入/来源弹窗、固定本地目录与命令记录；不发业务请求、不
持久化、不记录 Token 值。浏览器案例检查长名称/地址/说明、短窗口 Footer、
既有凭据留空、技术字段、原生文件入口与跨模式草稿，长指南按条截图；文件
夹具只证明入口连通，不冒充 zip 解析验收。1368 项仅完成注册
（`/tmp/nexus-design-a28-browser-list.log`）；此前浏览器启动被自动审批复核连续
超时拦住，实际运行及完整 check 中监听端口的夹具仍待许可。482 项清单为
390 pending、89 in_progress、3 removed，五个相关视图保持 in_progress，源码
摘要一致。完整来源请求/恢复、真实文件与下载、三主题及宿主视觉继续待验收。

## A29：多标签整条页面链路复查（未新增行为修复，待实际窗口复现）

用户继续反馈多页面标签逻辑像被移除，因此暂停联系人目录的视觉整理，优先复查
标签回归。当前源码中的 A15/A19 增量打开、精确关闭修复仍存在；26 项既有标签
模型、Hook、共享组件与路由测试通过。只读检查正在运行的本地 Vite 服务，也确认
它提供的是包含两处修复的控制器代码。此检查没有访问用户会话数据，也不能证明
用户窗口已加载该模块或操作正常。

新增两个 DM/Group 页面联合回归，组合真实 `useRoomPageData`、页面命令、会话
投影、路由协调、`RoomConversationTabs` 与 `RoomHistoryMenu`；只有 HTTP 结果
使用本地夹具。覆盖「＋」等待目录刷新时保留正文/忙碌状态、刷新后两个标签共存、
当前空草稿复用、从真实历史菜单打开第三个标签、切换活动标签、固定后关闭、历史
重新打开，以及 owner 重新绑定后从 Room 根路由恢复三个标签与固定状态。

全部 526 项组件测试、160 项导航持久化与前端合同检查、lint、typecheck 和生产
构建通过，日志为 `/tmp/nexus-tabs-review-{components,contracts,lint-types,build}.log`；
完整 check 中依赖监听端口的夹具仍未运行，未将离线子集等同于完整门禁。

本批没有修改生产行为，也没有把未复现的现象认定为已解决。新增联合回归中的
scroll/pointer 占位仅用于 jsdom；实际浏览器启动仍受此前两次自动审批复核超时
及未答复授权限制。482 项生产 TSX 审计状态和摘要不变，完整 Goal 仍在进行。

## A30：联系人目录与跨领域筛选所有权（待实际浏览器复查）

联系人 Agent 网格原先同时渲染 compact/comfort 两套完整卡片，再按断点隐藏；
标题、权限、元信息、聊天与建群动作各有副本，新建卡片与搜索也各渲染两份。列表
又把名称、权限、标签、Provider 和计数挤在同一行，元信息使用 11px / soft 并截断。

用户复查后明确保留原来头像居上、名称和元信息居中、操作居底的网格结构。因此
最终卡片使用一份内容和动作，沿用已有 Workspace Catalog 的 comfort 密度与网格，
没有新增响应式 Card API。网格头像使用已有 lg / 56px 档、标题采用 objectTitle / 20px，
不在操作区添加分隔线；列表采用 md / 40px 头像和 sectionTitle / 14px 标题。
名称、权限、预览标签与 Provider 允许换行，Provider
和工具/技能数采用 metadata / 12px / muted。内部 `ContactsAgentMetadata` 与
`ContactsAgentActions` 同时服务网格和列表，列表原来按断点隐藏文字的动作改为
具名公共 IconButton 和唯一 Tooltip。整卡主动作继续由共享 Card 持有，删除业务
层重复的 pointer-events/z-index；描述仍是网格两行、列表单行预览，业务标签仍只
预览两项加余量，列表窄屏仍隐藏标签，完整编辑与协作命令继续进入原页面回调。

目录只保留 Header 内的一份搜索和每种视图的一份创建入口。三种筛选迁入共享
`UiFilterSelect`：该 Pattern 从能力域布局文件提取，原能力页消费者全部直接引用
新所有者，不保留兼容转发。必填“文字标签｜当前值”结构、紧凑尺寸和无前导图标
规则迁到唯一 `design.md`；默认宽度与既有能力页命令保持，联系人仅按内容调整
容器宽度和菜单最小宽度。空匹配采用 `UiResourceState`，清除动作同时重置搜索及
业务标签/Provider/权限，保留视图和创建入口；没有更改过滤模型或新增资源请求。

筛选结构的原 DOM 测试迁到共享所有者；新增三个目录联合测试，卡片测试验证每种
视图只有一份身份/动作、精确聊天/建群/编辑命令与键盘提示隔离。开发 Gallery 使用
真实 ContactsDirectory 和本地事实，覆盖长中英文名称、长 Provider、标签、网格/
列表、空匹配恢复以及共享主动作的坐标点击；已有筛选形状测试增加联系人作为跨域
比较。静态门禁新增 UiFilterSelect 的私有颜色/字形/圆角覆盖检查，仍允许容器宽度。

全部 529 项组件测试（175 文件）、155 项合同、lint、typecheck 和生产构建通过，
日志为 `/tmp/nexus-design-a30-{components,contracts,lint-types,build-list}.log`。
按用户反馈恢复居中结构后，重新通过相关 14 项组件测试、lint、typecheck 与构建，
日志为 `/tmp/nexus-design-a30-adjusted-{components,lint-types,build}.log`。
1404 项浏览器场景仅完成登记，未执行：此前浏览器启动被自动审批复核连续超时
拦住，仍待许可；完整 check 中的监听端口夹具也未运行。本批不能替代实际长文本、
布局、点击命中或原生宿主验收。清单为 385 pending、95 in_progress、3 removed，
483 项中所有存活文件摘要一致；完整 Contacts 详情/联络和全前端 Goal 仍未完成。

## A31：公共头像回退与完整字符边界（待实际浏览器复查）

Room 拼图原先嵌套 `UiAgentAvatar size="md"`，再强制覆盖宽高、圆角、边框和
阴影。九宫格中的普通 14px 双字符回退因此超过格宽；普通头像与 Launcher 又各有
一份 UTF-16 截断逻辑，会拆坏 emoji、组合附标和扩展汉字。图片失败也没有回退。

`avatar.tsx` 内部现在由一份 `AvatarContent` 承担图片与回退，地址变化才重建
失败状态，同地址不循环重试；Agent/Room 根节点各有唯一可访问名称，群内成员
作为装饰。多成员只取一个完整字符，字号由 Room 尺寸与格数共同决定；最多九名，
Header 的四名限制和双成员错位轻叠保留。空 Room 仍使用稳定默认图片，图片失败
再显示 Room 图标。普通头像尺寸与圆角数值保持，圆角改为读取 control token；
图形节点和任务条仍有真实 `imageClassName` 消费者，本批不删除该 API。数学曲线
头像已有稳定身份、语义圆角与运行态 API，七项回归通过，本批保留实现。

`lib/text-graphemes.ts` 抽取原流式字符切分作为无状态基础 owner，供姓名缩写、
Hero 的测量失败回退和 Markdown 共用。删除 Launcher 私有缩写、Hero 的重复
Segmenter 类型适配、旧流式切分导出和无语义转发函数；所有消费者直接迁移。
流式层继续独立拥有尾字符重分、backlog 和调度，跨 delta 的 ZWJ/组合附标修复
仍通过原行为测试。Hero 的 pretext 主路径、渐显与 FadeSlideIn 未作设计调整，
因此该文件只登记部分审查。

改前新增测试记录四种 Unicode 截断失败及 Avatar 合同缺口。改后全部 **546 项
组件/基础函数测试（178 文件）、170 项选定合同、lint、typecheck 与生产构建
通过**；合同包含字符增量、Gallery 覆盖和 Launcher 命中路径。产物中确认语义
圆角及外环 calc 均实际生成。日志位于 `/tmp/nexus-design-a31-{before,focused,
components,contracts,build-list,final-lint-types}.log`。

Gallery 增加五档普通头像、三档 Room 的 0/1/2/4/9 成员与破图夹具；浏览器用例
检查尺寸/圆角、回退文字格内边界、双成员重叠及截图。`--list` 登记 **1,440 个
矩阵用例**，没有启动浏览器，不能作为几何或宿主验收。此前两次自动审批超时仍
未解除，本批不重试启动。审查清单为 **382 pending、98 in_progress、3 removed**，
共 483 项，全部存活源码摘要一致。用户偏好的居中联系人卡片结构继续保留；
全前端 Goal 仍在进行。

## A32：Agent 身份表单与长模型字段（待实际浏览器复查）

身份页仍有单独的标签样式表，内联标签使用 12px、全大写、扩大字距与 soft
前景；名称、模型、描述与创建模板的 label 没有绑定具体输入，名称错误也没有
关联。模型选择器虽启用 `allowLabelWrap`，公共壳仍固定高度，因此业务另有
两套高度/内边距覆盖。标签输入未过滤 IME 回车，移除名称还写死中文。

所有身份字段改用实例级 `UiField`，名称声明 required 并关联当前校验/错误；
正在校验时继续隐藏旧错误，反馈清除后不保留占位。字段共用 14px medium 标签、
13px 说明与 36px 基础控件密度；模型长名称的增高与垂直留白由公共 Select
recipe 持有，删除业务的模式样式表。主 Agent 跟随默认模型、保留暂不可用的
选择、显式恢复默认和 Provider 读取失败的分支不变。创建模板保留源码文字、
加载/重试与创建来源边界，既有普通 Agent 的 AGENTS.md 文件编辑器不在本批更改。

Tag 复合字段继续由现有 `IdentityTags` 独占，复用 Field、输入壳、Typography、
Chip 和 Button；当前没有第二个业务消费者需要通用 Tag 状态机，因此不新增
公共组件或复写集合规则。业务与风格草稿的独立 scope、trim/去重和 Enter 清理
重复草稿保持；IME/229 回车不添加，添加/移除后回到原输入，空白/重复值禁用
添加按钮，移除名称按中英文翻译。内部横向滚动保留，标签数量不推动模型区域。
`design.md` 中早期“Chip 另起一行”的文字与现有单行 owner 不一致，本批按稳定
字段布局收口到唯一规范。头像入口继续使用原 56px 和上下文对齐，删除两项相同
的尺寸映射；其浮层仍委托公共 Picker。身份页补充 reduced-motion 回退。

改前定向测试出现 **5 个失败**：名称没有可访问标签、IME 确认被拦截，以及添加
动作没有恢复输入焦点。改后全部 **557 项测试（181 文件）、157 项选定合同、
lint、typecheck 和生产构建通过**。新增回归包含实例标签隔离、错误替换/清理、
原生 IME/229、两类草稿独立和切换 scope、精确移除/焦点、本地化、创建模板
加载/重试、主 Agent 锁定与兼容编辑描述；相关保存控制器没有修改。日志位于
`/tmp/nexus-design-a32-{before,focused,identity,components,contracts,checks}.log`。

Gallery 增加真实创建身份与内联主 Agent 表单、四档长标签 Select。浏览器用例
检查共同 label、重复实例、标签轨道高度、模板和错误关联、长 Select 文本边界。
`--list` 登记 **1,476 个矩阵用例**，没有执行浏览器或原生宿主；此前自动审批
连续超时尚未解除，本批不重试启动，也不把 DOM 结果称为视觉验收。目录卡片
继续保留用户选定的居中结构。清单为 **376 pending、104 in_progress、3 removed**，
清单共 483 项，所有存活源码摘要一致；详情 Header、完整 AGENTS.md、联络/记忆及其他页面
仍在原 Goal 范围内。

## A33：联络目录与好友决策弹窗（待实际浏览器复查）

联络目录原来私自组合 32px 搜索与 36px 添加按钮，空目录只提示去右上角；
刷新期间使用过滤后数量判断首次加载，可能把有效快照的无匹配结果换成加载态。
本批复用 `SidebarSearchField / SidebarSearchAction`，由公共 owner 管理桌面与
窄屏几何；空目录可直接添加，无匹配可清除搜索。过期快照只保留一份失败分支，
无可信快照的失败继续隐藏旧列表，不把权限失效解释为空目录。联系人名称仍按
导航行单行截断，完整名称提示委托 `UiListRow`，没有另做长名称排版规则。

添加弹窗使用实例级标题 ID 和初始搜索焦点，区分没有可添加 Agent 与没有匹配
结果。搜索隐藏已选好友时，备注字段显示其身份并建立可访问说明；当前有效
候选是提交目标，目录移除该候选后禁止提交。提交期间同步防重并锁定搜索、
候选、备注、按钮和关闭；未完成添加时保留草稿，离开 Agent 后旧请求的迟到
成功不会关闭新弹窗。此处只增加当前视图的短期交互锁，不修改写入命令、
服务端回执、失败分类或重试权限。

删除好友确认改为保存打开时的目标快照，使用共享危险确认并投影控制器已有
`isRemoving`；当前选择变化不改写确认对象，旧请求完成也不能关闭新确认。
原后果文案继续明确保留隐藏 Room 与消息历史。Controller 只新增状态投影，
Session、Header 标签、历史、Composer、发送与添加/删除 API 逻辑没有改动。
完整联络工作面的导航/消息状态，以及 mutation 失败在弹窗内的呈现仍待后续审查。

全部 **567 项组件测试（182 文件）、157 项选定合同、lint、typecheck 和生产
构建通过**。新增 10 项回归覆盖刷新中的搜索、可保留/不可保留快照、搜索隐藏
选择、提交前候选失效、同一时刻双提交、busy 关闭锁、确认目标与迟到响应隔离。
View 测试复用真实 Header、Tabs 与 ConfirmDialog，只隔离 Composer，并为 jsdom
补充 media-query 测试边界。日志：`/tmp/nexus-design-a33-{focused,components,contracts,checks}.log`。

Gallery 增加真实联络目录与受控 pending 添加夹具，无 API 或持久写入；浏览器
用例覆盖窄窗弹窗、初始焦点、隐藏选择说明、busy 控件、关闭锁和过期快照刷新。
`--list` 登记 **1,512 个矩阵用例**，仅登记而未启动浏览器。此前自动审批连续
超时仍未解除，实际 Web/macOS 视觉验收未完成，Windows 继续按用户要求暂缓。
清单为 **373 pending、107 in_progress、3 removed**，共 483 项，所有存活源码
摘要一致；公共清单仍为 121 项，没有为这批业务流程新增通用组件。联系人管理
卡片保留用户偏好的居中结构；全前端 Goal 继续进行。

## A34：记忆目录、正文标题与窄栏状态（待实际浏览器复查）

记忆目录仍把完整搜索提示、内嵌小刷新按钮和 86px 类型菜单塞在同一行；
分组名称采用 `text-2xs / uppercase / soft`，截断说明也使用弱色。真实空目录
同时满足无匹配投影，因此存在两份不同空提示。目录本批复用侧栏搜索行与
刷新动作，类型改为下一行的标准 `UiFilterSelect`；分组和说明复用 Typography。
真实空目录与无匹配互斥，无匹配的动作只清除 query/type，不请求刷新或写文件。
空说明改为用户可理解的阅读/编辑用途，删除运行器名称与内部目录细节。

正文 Header 原来把标题与写入状态挤在同一行并截断标题，时间使用 soft。
现在标题与动作可按宽度换行，标题、元信息与写入状态均复用 Typography，
写入状态位于标题下方，并提供 status 语义。目录与正文的初始加载改用具名
ResourceState，继续委托其 lg Spinner。正文访问失败原来直接返回失败面，
使窄栏没有返回入口；现在保留“返回记忆目录”，不暴露受限标题、正文或编辑动作。
冲突对照原来依赖窗口 lg 断点，现在依据正文容器自身宽度，640px 以下为单列。
文档类型描述、软分栏、阅读轴、编辑内容、revision/保存/删除与恢复命令没有更改。

全部 **575 项组件测试（184 文件）、162 项相关合同、lint、typecheck 和生产
构建通过**。新增 8 项 DOM 回归覆盖单一空态、双筛选清除、刷新中仍可选文档、
完整路径身份、具名加载、访问失败返回、冲突双版本显式覆盖和未知保存只读核对。
额外执行现有 Memory 删除恢复与文档并发合同；没有重复运行 Go 全量检查。
日志：`/tmp/nexus-design-a34-{focused,components,contracts,checks}.log`。

Gallery 新增真实 Catalog/Header 的纯本地夹具与长标题、筛选、空目录、写入态
和编辑动作浏览器用例；夹具不调用 Memory 资源 hook、API 或持久写入。
`--list` 登记 **1,548 个矩阵用例**，仅验证测试登记，实际浏览器与 macOS 宿主
复查仍受此前自动审批连续超时影响。本批没有再次启动浏览器；Windows 继续暂缓。
正文原生编辑器、冲突文字区域的焦点/滚动、删除问题通知与索引仍需继续审查，
本批 DOM 与构建结果不等于这些区域的实际视觉验收。清单保持原有全部范围，
本批 4 个文件转为 in_progress，所有存活源码摘要一致；全前端 Goal 继续进行。

## A35：公共源码编辑与 AGENTS.md 保存确认（待实际浏览器复查）

工作区 Body 和记忆的普通编辑/冲突草稿/已保存版本共维护四处原生 textarea，
等宽字体、滚动、禁用与焦点各自实现，部分显式移除全部键盘焦点效果。本批
新增纯展示 `UiSourceEditor`，统一 14px/24px 等宽输入、内部滚动、内嵌焦点、
只读/禁用和 native ref/event；默认关闭源码拼写/大小写纠正。它不提供草稿、
保存、Tab 缩进、快捷键或预览状态机。四处调用迁移到同一 owner，Memory 的
阅读轴/对照留白继续由 Panel 拥有；普通表单 Textarea 的 code 角色保持独立。
样式门禁覆盖新的源码控件，所有权合同拒绝 File/Memory 恢复原生 textarea。

AGENTS.md 编辑器原标签没有绑定控件，保存后的无条件退出还可能隐藏较新的
草稿；Agent 切换后，旧保存完成也可能关闭新确认。现在使用 UiField 的独立
`labelAction` 与实例级输入 ID：预览表达具名 group，编辑时绑定真实输入。
Field 标签动作是 label 的兄弟节点，窄空间可以换行，不改变其他字段的布局。
文件编辑实例按 Agent 隔离，当前确认同步防重、提交期间锁定关闭；旧实例的
完成不改新实例。较新草稿继续编辑，相同草稿成功后沿用退出行为，失败或确认
已过期时回到既有文件恢复面。revision、写入回执、读取/冲突/对账控制器没有改动。

全部 **587 项组件测试（187 文件）通过**；本批新增 12 项覆盖 Field 名称/动作
与错误隔离、源码原始空白/换行/IME/Tab/ref、只读选区与禁用、Workspace 默认
失焦退出与 Profile opt-out、确认防重、迟到成功、新草稿保留、缺失 revision 禁用和冲突替换确认。
真实 Body/Source/ConfirmDialog 参与 Profile 回归，仅隔离文件控制器和预览边界。
lint、typecheck、构建与现有 Memory 并发及 Workspace scope 合同通过；166 项选定
合同之后，含新增所有权门禁的 122 项 foundation 合同单独通过。日志在
`/tmp/nexus-design-a35-{focused,components,contracts,checks,ownership,final,final-build}.log`。

Gallery 新增源码可编辑/只读/禁用场景、原始文本记录与焦点/内部滚动浏览器用例，
公共组件清单增加为 **122 项**。`--list` 登记 **1,584 个矩阵用例**，实际浏览器
没有执行。此前自动审批连续超时尚未解除，本批没有重试启动；Web/macOS 的
真实输入法、焦点色、CJK 字形与 Profile 保存过程仍待宿主验收，Windows 暂缓。
全量审查清单新增 Source owner，共 484 项（368 pending、113 in_progress、3 removed），
全部存活源码摘要一致，其余原有范围保留；全前端 Goal 继续进行。

## 待进一步判断

- A8 已统一侧栏搜索文字与动作所有权，仍需执行真实双语目录的宽度、对比度和
  提示回归，不只放大 Gallery 容器；导航行与目录空/错误/加载分支继续单独审查。
- 辅助文字与微标签的 `soft` 使用范围：可读信息与纯装饰分别判断，避免只靠
  降低对比制造层级。
- A2 已调整 Settings 标签/说明、删除私有选择器 recipe 并统一紧凑文字尺寸；
  仍需在完整设置页面、Operations 和目录的真实布局里检查长内容及不同状态。
- A3 已形成状态色、配对前景与控件材质的候选修正；还需通过实际 page/card/overlay
  和 hover 矩阵复核 Badge、Button、Counter、Choice 与错误文字，不能只以 RGB
  计算或 CSS 类型检查代替最终可读性验收。
- A4 已补齐 Field 的单控件与复合组关联；浏览器可访问树与实际宿主输入仍需复核。
  A10—A12 已收口 Runtime、Skill 来源与 Custom MCP 的分段组名，并验证 Custom MCP
  动态行名称、输入与错误身份；其他动态字段和复合 Field 继续按各自业务审查。
- A9 已移除普通 overline 配方；其他直接使用 text-2xs 的菜单说明、Composer 元信息、
  记忆分组和图形微标签继续按内容任务判断，不能把本批六处迁移当作全部微文字完成。
- A6—A7 已完成失联模块和具名值导出的第一轮保守检查；无必要公开的内部 helper、
  函数内部旧分支、重复数据映射和未使用样式仍需沿业务调用继续复核。
- A23 已处理共享模态/浮层的输入法候选键；其他业务自有快捷键仍需随各页面
  核对，原生输入法、实际短窗口与三主题复查不能由 DOM 键盘事件替代。


## A36：文件预览状态所有权与重复代码清理

按用户最新要求，本批暂停视觉验收和 Gallery 场景扩充，集中完成代码重构。
PDF、图片、Office 懒加载、DOCX、表格、幻灯片、普通文本及大型文本的等待面
统一消费领域级 `WorkspaceFilePreviewLoading`，其本身只组合现有 ResourceState
与 Spinner，不持有读取、解析或重试状态。标题栏移除重复的加载、失败和泛化成功
提示及装饰图标，保留已加载的页数/工作表数量；独立文本写入状态维持原职责。
移除两个全仓无生产消费者的翻译键和三个私有状态/加载视图，简化 Office 的
单字段描述符。空文件选择与不支持的文件类型复用公共资源状态；后者按界面语言
和宿主说明现有工具栏的下载/显示文件夹操作，删除固定中文和过期格式列表。

行为边界：exact Agent/path 的 Router key、媒体 URL、PDF sandbox、DOCX
测量与样式容器、缩放、Office 解析器、表格切换、幻灯片翻页、Range 上限和文本
渲染器保持原有职责。新增 9 个离线 DOM 回归覆盖三种 Office fallback 的唯一
加载提示和原文件动作、PDF load、图片失败后新节点重试、双语/双宿主说明、DOCX
宿主复用与失败重试、文本等待语言切换。PDF iframe load 不证明内部查看器已成功
解析，测试不模拟其不可靠的非冒泡 error 事件，也不宣称 PDF 渲染验收。

验证：编辑器目录 6 个测试文件 / 16 项行为测试通过，边界/依赖/文件合同、公共
所有权、控件、token 与文件编辑 scope 共 163 项通过；lint、typecheck、生产
build 通过（已有大 chunk 提示仍在）。日志见 `/tmp/nexus-design-a36-behavior.log`
及 `/tmp/nexus-design-a36-contracts.log`。本批未启动浏览器或执行视觉验收。

当前清单 485 项：360 pending、122 in_progress、3 removed，现存摘要全部匹配；
公共 UI 数量仍为 122，新增的是领域组合。预览 Header 的异步文件操作、完整文件
资源生命周期与其他页面继续审查，不能据本批将全 Goal 标记完成。


## A37：恢复记忆搜索与类型筛选同行

用户明确偏好原有单行结构，撤销 A34 中让筛选单独占满下一行的布局决定。
目录改为一排：`UiSearchInput` 使用剩余宽度，刷新由其已有 action 槽承载公共
`UiIconButton`，右侧继续使用标准具名 `UiFilterSelect` 并限定紧凑宽度。删除
第二行容器及全宽覆盖，没有新建搜索框、菜单或按钮样式；筛选、查询、刷新、
清除与选中文档仍使用原回调。`design.md` 与目录所有权说明同步更新。

这是局部可逆布局调整，复用现有 5 项目录/筛选行为回归，未新增实现镜像测试。
现有 136 项公共所有权/控件合同、lint、typecheck 与生产构建均通过；日志见
`/tmp/nexus-memory-inline-{behavior,contracts,checks}.log`。按用户要求未做视觉
验收或新增 Gallery 场景；全量清单仍为 485 项，范围与进度数量不变。


## A38：记忆页继承同级详情背景

用户指出记忆页与工具等同级栏目底色不同。源码对照确认：工具/技能编辑器继承
Agent 详情容器底色，而 Memory 另设目录背景、分栏画布背景、正文 72% 混色与
羽化阴影。本批删除这四层专用处理及窄栏中已无必要的透明背景覆盖，让目录、
正文与栏间空隙都直接显示同一上层背景；不在 Memory 再复制一个主题色值。
目录/正文切换、分栏宽度、响应式、异常/编辑面和刚恢复的单行工具栏保持原样。

这次只删除背景/阴影声明，未改动业务代码；lint、生产 build 与 8 项 token 合同
通过，日志见 `/tmp/nexus-memory-background-checks.log`。不新增样式实现镜像测试，
也不重复刚通过的目录行为测试。按用户要求未执行视觉验收；清单来源摘要全部
匹配，数量与整体 Goal 范围不变。


## A39：限定记忆目录恢复范围，保留单篇正文

用户进一步明确：恢复只针对记忆目录单页，不能改变进入单篇记忆后的工作面。
本节修正 A37/A38 的范围，当前规则以本节及更新后的 `design.md` 为准。
恢复正文的原有 72% 混色、羽化阴影、宽容器分栏底色和窄目录底色；仅在已有
单页布局断点内让目录背景透明、搜索/筛选同排。宽容器中的窄目录继续分行，
不把目录单页的紧凑筛选宽度套到正文侧栏。没有新增或改变文档选择/返回状态，
仍由原容器断点与 `data-document-open` 决定目录和正文可见性。

lint、生产构建及 8 项 token 合同通过，日志见
`/tmp/nexus-memory-scope-correction.log`；按用户要求未执行视觉验收。
既有单篇渲染和业务控制器均未修改，清单范围和数量不变。


## A40：整套恢复用户偏好的记忆页面

用户要求记忆页使用上一版本整套结构，不再对单页/正文分别试改。历史核对确认
本轮 Memory 改版起于 `ec9732b67`，其前一版本为 `a4d9254ba`；本批以该版本
恢复正常目录、搜索提示、86px 当前值筛选、目录分组字级、单行正文标题与状态
排列、背景和完整分栏 CSS。`memory-view.css` 与该基线一致，正文 Header 仅保留
更明确的返回可访问名称。冲突对照恢复原有布局和紧凑标题层级。

为保持业务可靠性，保留共享 SourceEditor、互斥空状态/清除筛选、具名加载和访问
失败返回；读取、保存、删除、实时文件对账及编辑冲突控制器均未回退。旧 Gallery
测试中要求正文标题完整展开的断言随布局恢复移除，保留完整内容与命令检查；未
新增视觉场景或执行浏览器。本节取代 A34/A37—A39 的正常工作面外观决定，用户
随后仅询问统一底色的建议，尚未据该讨论再次改动页面。

验证：9 项 Memory DOM 行为、149 项公共所有权/控件/文件/token/Memory 并发合同、
lint、typecheck 和生产构建通过。日志：`/tmp/nexus-memory-restore-{behavior,contracts,checks}.log`。
本批检查时工作树同时包含独立的公共选择条清理，其结果另记 A41；源码与文档
分批提交。页面视觉验收仍按用户要求暂停，Goal 范围不变。


## A41：普通选择条删除闲置关闭分支，收口六个公共所有者

按用户暂停视觉验收、优先代码重构的要求，本节的 retained/improved 表示完成了
相应文件的代码与行为审查，不代表已验证渲染效果。保留全部原审查范围，其他
文件不能因消费这些组件而自动标记完成。

`UiTabs` 的 `onDismissActive` / `dismissActiveLabel` 在全仓只剩定义与一项测试，
没有生产或动态调用。删除这两个参数、仅供该分支的重复包装/关闭节点与中文
默认文案；普通视图和筛选仍使用原具名 group、独立按钮、aria-pressed、稳定
option 包装、tour anchor 和当前值回调。样式接入公共 metadata 角色，移除重复
字号/字重映射，保留高度、底线、横向滚动与焦点状态。

实际会话链路的 `WorkspaceConversationTabs`、`WorkspaceConversationTab`、
会话模型和 Session store 均未修改；其仍使用 `UiTabDismissButton`。后者继续
具有真实生产消费者，不作为无用导出删除。移除的仅是与已删除 API 配套的测试，
保留并运行真实会话的创建状态、选择、固定、鼠标/Enter/Space 关闭命令隔离回归。

其余逐文件结论：SplitButton 保留两个独立共享按钮及可选菜单语义，适合权限
操作；Disclosure 保留原生 details/summary、有限样式与受控/非受控切换；
Breadcrumb 保留唯一层级、链接/返回动作、当前项与截断 owner；DirectoryTabs
保留跨业务使用的紧凑布局预设。各具体理由和测试入口写入 CSV，不为已有合理
结构新增平行组件或无意义改写。

验证：导航、按钮、Disclosure、Workspace 会话标签与表格选择共 26 项 DOM 回归
通过，日志 `/tmp/nexus-navigation-cleanup-behavior.log`。与 A40 一并运行的 149
项合同、lint、typecheck 和生产 build 通过，相关日志见 A40；之后只更新文档
和清单。未执行视觉验证。当前共 485 项：354 pending、122 in_progress、5 retained、
1 improved、3 removed；现存源码摘要全部匹配。全 Goal 尚未完成。


## A42：按确认方案统一基础底色，保留上一版布局

用户接受“保留上一版布局，统一基础底色”的建议。本批只移除目录自身背景和
布局画布背景，让目录及 8px 分栏间隙继承工具、技能等同级页面底色；窄栏中的
透明覆盖随之成为冗余并移除。正文的混色与阴影保持原样，作为内部阅读层次；
搜索/类型排列、目录行、Header、编辑及分栏结构均未改变。当前决定在 A40
整套恢复的基础上，仅取代其基础底色，不再次叠加 A37—A39 的布局试改。

lint、生产 build 和 8 项 token 合同通过，日志
`/tmp/nexus-memory-approved-background.log`。本批只有三个背景声明变化，未改动
业务逻辑，未重复上一批 Memory 功能测试，也未执行视觉验收。清单摘要全部
匹配，数量与整个 Goal 范围保持不变。

## A43：收口反馈组件生命周期与流式源码展示

按用户要求继续代码与行为审查，未进行视觉验证。逐一核对二维码、骨架占位、
流式源码、活动字符和反馈定位的公共所有者与生产消费者；本节的 improved /
retained 沿用 A41 的代码审查含义，不能作为渲染或真实宿主验收证据。

`UiQRCode` 保留原有 Panel、Typography、固定扫描尺寸与调用方授权流程，
默认加载/失败反馈接入中英文目录。动态生成仍只接受当前 payload 的结果，
旧 effect 已取消时不再开始编码；内嵌与生成图片的解码错误现在都进入失败态，
更换 payload 重新开始。调用方定制说明和隐藏原文选项保留，组件不推断授权动作。
七项生命周期回归覆盖旧任务晚成功/失败、图片失败、新 payload 恢复、语言切换、
生成拒绝/空结果、原文隐私以及首尾空白不触发重复生成。

`UiSkeleton` 原有颜色、形状与 reduced-motion 配方适合装饰占位，保留。
`UiSkeletonCardList` 改为一条本地化加载状态，各卡片只作装饰，删除每张卡重复
的中文播报和冗余参数绑定，保留任务历史的数量与布局输入。

`TypewriterFileView` 共用新增的 `form/source-text-styles.ts` 度量所有者，
源码编辑与写入预览不再分别维护等宽字体、字号和行高。行数按真实换行计算，
窗口尚未测量或变窄不会改变计数，空文件与末尾换行按当前插入行计数；徽标放在
正文之外，避免覆盖文件内容。保留原文与追加时跟随底部，使用公共 running 徽标，
光标移到静态主题 recipe 并支持减少动态效果。移除 pretext 测量常量、行数派生
状态、宽度传参、Body 专用 ResizeObserver 和运行时 style 注入；pretext 仍有
其他生产用途，不能删除依赖。新增三项行为回归及共享字体/静态动效所有权门禁。

LoadingOrb 的两种语义帧型、固定装饰尺寸与静态动效配方继续保留；
FeedbackBannerViewport 的单条反馈定位、窄屏布局和语义 layer 也继续保留，
没有建立替代组件或附加状态。Lottie 及领域页面仍在后续审查范围。

清单仍为 485 项：349 pending、122 in_progress、7 retained、4 improved、
3 removed；现存文件摘要全部匹配。只新增内部样式模块，公共 UI 仍为 122 项；
Gallery 只删除已不存在的宽度参数，未增加场景或执行视觉校验。整个 Goal 继续。

验证：`npm run check` 中 lint、typecheck 及 466 项合同先通过；唯一受沙箱限制
的临时 localhost HTTP/WS 夹具测试（`listen EPERM`）在获准监听端口后单独通过，
合计 467 项合同通过，未启动浏览器或连接运行中的业务服务。随后完整组件测试
190 个文件、606 项回归及生产 build 全部通过；构建仍提示大型依赖分块超过
500 kB。日志分别为 `/tmp/nexus-feedback-a43-check.log`、
`/tmp/nexus-feedback-a43-fixtures.log`、`/tmp/nexus-feedback-a43-components.log`
和 `/tmp/nexus-feedback-a43-build.log`。未把拆开完成的门禁记录成整条命令成功。

## A44：统一装饰动效偏好，删除播放、测量和监听的重复实现

继续按用户选择只做代码与行为校验。上批已提交并保持工作树干净，本批接续
审查 Lottie、Hero 字符渐显及通用进入容器；保留全部前端范围，未将本批成果
当成整个 Goal 已完成。

`LottiePlayer` 原本同时开启 autoplay 并持有实例再调用 play，且不消费系统
减少动态效果偏好。现统一使用共享偏好，普通模式循环、低动态模式静态展示，
装饰 Canvas 不进入可访问树。检查当前安装的 dotlottie-react 类型及实现后确认
autoplay 只在加载配置中读取，普通 prop 更新不会暂停已播放实例，因此偏好切换
使用不同 React key 销毁旧生命周期并创建当前模式，源变更仍交给第三方加载。
删除重复播放 effect/实例 state、冗余参数绑定及只有两个 undefined 生产调用的
inlineStyle API；两个 Launcher 消费者只删除无值参数，原尺寸与位置不变。

媒体偏好原本与 `useMediaQuery` 各自维护同一监听逻辑；减少动态效果的初值还是
false，导致已启用偏好的用户先进入动画路径。现只保留 useMediaQuery 一份首次
读取/变更/清理 owner，无 matchMedia 环境返回默认 false；偏好 Hook 只提供查询。
四项回归覆盖首个 render、切换/卸载、替换 query 拒绝旧事件、缺失媒体 API，
以及真实流式 Markdown consumer 不先隐藏完整内容。Home ASCII 的既有平台选择
不在本批改写，普通响应式布局仍使用原查询。

Hero 原先只消费 pretext 的 segments，却为此读取计算字体并维护 effect、派生
state 和 16ms 定时器；没有消费宽度或其他测量结果。现在直接复用唯一 Unicode
grapheme owner，首次渲染即包含完整文本与名称，保留相同字符的稳定身份。
字符只做轻微 opacity 渐显，以普通 inline 保留自然断词；不再逐字位移或缩放。
FadeSlideIn 保留时序、纵向偏移、样式透传和 child identity，两者动效收口静态
theme recipe；backwards fill 仅作用于等待阶段，完成后无常驻 transform，低动态
模式立即显示。删除两个挂载定时器，不在业务层产生第二套 CSS 动画。

新增三项 Hero 内容/交互、三项 Lottie adapter 生命周期回归和一项所有权门禁。
这些测试证明 React 逻辑与静态配置，未模拟 WASM 绘制或宣称真实屏幕效果；
未运行浏览器或原生视觉校验，未新增 Gallery 场景。pretext 仍被消息高度测量
使用，因此只删除这里的无用调用，不删除依赖。

当前清单 485 项：346 pending、123 in_progress、7 retained、6 improved、
3 removed，现存摘要匹配；公共 UI 仍为 122 项。两个 Launcher 页面只记部分进展，
专用按钮/输入、Header 与整体页面审查仍未完成。

验证：22 项目标组件回归与 typecheck 先通过；最终完整 `npm run check` 成功，
含 lint、typecheck、468 项合同、193 个文件的 616 项组件回归及生产 build。
临时 localhost 夹具在授权范围内完成，未开启浏览器或访问运行中的产品后端。
构建仍只有既有大型分块提示。日志：`/tmp/nexus-motion-a44-components.log`、
`/tmp/nexus-motion-a44-typecheck.log`、`/tmp/nexus-motion-a44-check.log`。

## A45：Launcher 查询归并公共字段，修复 Mention 输入法确认

继续审查实际页面代码，不做视觉验收。Launcher 查询曾单独实现输入字号、
颜色、placeholder、去焦点样式及一层玻璃字段壳，现在直接使用 UiInput 的
lg/surface 档位，外层只保留输入与角色发送的同行布局。字段改用公共 44px 高度、
字号、边框和焦点；角色热区仍为 44px，草稿、Mention ref/光标、禁用和全部输入
回调保持同一 owner。移除重复玻璃壳和无业务信息的前置图标，保留原 420px 列宽
与舞台缩放。发送按钮现在始终有本地化名称，忙时有 aria-busy/disabled；图像
改为装饰，避免名称依赖图片在等待时消失。两个场景按钮共享 ring 和 fast 动效
token，品牌箭头遵循减少动态效果；不为从未改变 transform 的云朵 wrapper
保留无效过渡。

代码检查发现输入与可见 Mention 全局捕获都未完整排除 IME：前者只检查
isComposing，后者会把输入法 Enter 当成选值。两处现在共同使用唯一
isImeKeyboardEvent，保留 composition ref，并拒绝 Process/229 等兼容事件。
新增实际 Hero + 公共字段 + 公共 Mention 的 7 项离线回归，覆盖受理后 trim/清空、
空白忽略、拒绝保留草稿与外部替换、等待防重复点击、IME 后独立提交、@Agent /
#Room 选择与光标回位，以及工作台导航/主 Agent 交接。测试只替换装饰 Pile 与
Lottie，不替换查询 Hook 或 Mention 行为，不请求业务后端。

沿引用核实后移除五组无消费者的 Launcher 主题变量：input-icon、
input-placeholder、divider-color、meta-text、submit-border；同时移除配置字段、
三主题值和投影。前两项随公共字段归并失去用途，后三项此前只剩声明。品牌入口
仍消费的 input 材质与发送等待态颜色保留，未删除有意义的主题分支。

最近入口完整核对现有纯模型、布局、稳定标记、长名提示和导航职责，保留共享
Button/Tooltip/FadeSlideIn 组合；删除重复 inline-flex 包装，交接文字和箭头
间距交给 Button。品牌复合入口、角色发送和云朵几何继续是已有规范中的场景
例外，不新加公共变体或取消场景身份。HeroBlobShell 保留稳定独立 SVG ID、
主题材质与不拦截指针的装饰层，补齐合同并删除重复参数绑定。

审计清单仍为 485 项：343 pending、123 in_progress、8 retained、8 improved、
3 removed；现存文件摘要匹配，公共 UI 仍为 122 项。Hero 与最近入口完成本批
代码审查，Console/Pile、Mention 的整体浮层/行控件审查仍未完成；未因父页面
消费它们而自动勾选。整个 Goal 继续，未扩展或执行视觉测试。

验证：9 项目标组件回归与 typecheck 通过，随后完整 `npm run check` 成功，
含 lint、typecheck、469 项合同、194 个文件的 623 项组件回归及生产 build。
末次检查将品牌箭头悬停位移限制为 motion-safe（适配 Tailwind 的独立 translate
属性），只重跑受影响的 CSS 生产构建，通过；未重复无关行为测试。日志在
`/tmp/nexus-launcher-a45-components.log`、`/tmp/nexus-launcher-a45-typecheck.log`、
`/tmp/nexus-launcher-a45-check.log` 和 `/tmp/nexus-launcher-a45-build.log`。
构建继续只有已知大型分块提示，无视觉或原生播放验收结论。

## A46：Mention 候选复用公共浮层与选项行

对公共 Mention 的定位、候选结构、命中区、键盘与关闭生命周期继续审查。
原实现直接 Portal 到 body，使用一次性 DOMRect、52px 行高估算和私有最大高度，
无法跟随滚动刷新，也没有参与模态范围关闭仲裁。现消费真实 input/textarea ref，
复用 anchored layer 与 reference-list preset 自动选位、限宽/限高、监听滚动和
窗口变化，并进入当前 Dialog Portal。Launcher 和 Composer 只传原编辑器 ref，
删除重复矩形计算、强制上下方向、私有定位函数/类型/常量；模型只保留文本匹配、
插入、筛选和键盘动作。Gallery 仅适配既有 Mention 场景为真实输入锚点，未增加
或执行任何视觉场景。

候选复用 SelectMenuPanel/SelectMenuOptionRow 与 Menu 行密度，外框材质、层级、
圆角、行间距和状态不再另写。单行 36px/说明行 44px，标记是装饰性 24px 中性
圆角方块；标题保留完整可访问文本和原文提示，说明复用 caption role。原始
onMouseDown 立即选择改为按下保持输入焦点、click 单次选择，以支持普通点击与
辅助技术激活。键盘仍留在编辑器，通过 aria-controls/activedescendant/autocomplete
关联实际候选，关闭或替换输入节点时恢复原属性，不接管值、role 或输入命令。

候选方向键/Enter/Tab 只接受当前模态范围最上层、且来自本锚点/列表的事件；
IME 和其他输入框不被截获。Escape、外部点击和焦点策略统一归 Overlay；为父级
会阻止输入事件冒泡的场景增加显式 captureEscape，仍服从相同最上层/模态/IME
仲裁，其他消费者继续原冒泡阶段。嵌套弹窗先关闭候选，下一次才关闭父级。

Action 行的四档渲染尺寸及对应高度归入 getMenuItemLayout，Mention、UiMenuActionRow
和 Action Menu 估算共用；删除 Action Menu 另一份高度表，现有菜单几何保持。
滚动回归同时暴露公共定位器在锚点越过视口上缘时会返回负坐标；top/bottom
最终结果现在都夹紧到视口留白，新增三项越界/强制方向坐标回归。

Mention 新增七项代码行为回归，覆盖当前候选关联及属性恢复、输入节点替换、
外部输入/IME 隔离、单次点击与焦点、外部点击、滚动/过滤、嵌套真实 Dialog Portal
和后台候选隔离；已有 Launcher 实际查询回归继续参与。它们证明 DOM、事件与
坐标投影，不代替用户已暂停的视觉或真实宿主验收。

清单仍为 485 项：340 pending、124 in_progress、8 retained、10 improved、
3 removed；摘要匹配，公共 UI 仍为 122 项。Action Menu 全部键盘语义和 Composer
的其他分支保持待审，不因共用已改进的底层而自动完成。整个 Goal 继续。

验证：目标组件及既有菜单/浮层回归通过，新增 Mention 测试单独通过；最终完整
`npm run check` 成功，含 lint、typecheck、470 项合同、194 个文件的 633 项
组件回归及生产 build，仍只有既有大型分块提示。日志为
`/tmp/nexus-mention-a46-components.log`（添加完整模态用例时的初次结果）、
`/tmp/nexus-mention-a46-focused.log`（补齐 jsdom 可见性夹具后的通过结果）、
`/tmp/nexus-mention-a46-typecheck.log` 和 `/tmp/nexus-mention-a46-check.log`
（最终全部通过）。未运行浏览器或真实宿主视觉验证。


## A47：动作与级联菜单共用键盘和焦点目录

Action、Workspace 文件上下文菜单与 Room 模型菜单统一消费 menu-keyboard 的
首项焦点、当前层级方向键/Home/End 遍历。查询只包含当前 menu 的可用条目，
跳过 native/fieldset disabled 和 aria-disabled；不跨进子菜单，不截获编辑框的
移动键、IME、已处理事件或外部 Portal 的 React 冒泡。空菜单或全部不可用时
聚焦可退出的菜单根。Action 原有 private 键盘实现已删除，选值和命令合同不变。

Tab 回归暴露 Portal 关闭后原始事件目标消失会令焦点落到 body；菜单现在先
关闭并归还锚点，再按页面/模态的相邻 Tab 位置移动，模态内保持循环，页面边界
没有相邻项时留在锚点。Dialog 原有可用 DOM 目录移到中立的 focus-navigation，
两者共用可见性、inert、原生/fieldset 禁用、负 tabindex、radio 组及 Tab 排序，
dialog-focus 只保留自身焦点位置与无滚动聚焦，未引入第二份 selector。

Room 模型入口在完成定位后进入 Agent 菜单；点击/右方向键进入模型，左方向键
或第一次 Escape 返回此前 Agent，第二次 Escape 才关闭。宽屏悬浮只投影目标，
不移动键盘焦点；悬浮已打开同一目标后，键盘进入立即聚焦，不等待不会发生的
状态提交。窄屏返回通过 Agent identity 找到重新挂载的行，模型/重置菜单都有
名称和归属关系。模型更新、继承值重置、显式底部重置与 busy 保持原控制器命令。
菜单行高估算删除两份领域常量，直接消费 getMenuItemLayout。

Workspace 主/子菜单复用同一遍历，点击/右方向键进入打开方式，左方向键/Escape
逐层返回。显式关闭归还打开前焦点，外部点击保持目标焦点；StrictMode effect
重放不会把返回位置覆盖成菜单内按钮。补齐文件 INPUT/OUTPUT/POS。其坐标碰撞、
Portal/模态仲裁和鼠标级联边界仍待后续收口；Room 的宽高及响应式切换也继续
审查，未因复用了菜单行就标记整项完成。

新增 15 项组件回归覆盖共享导航、Tab/反向 Tab、真实 Dialog 内的循环边界、
全禁用菜单、原生焦点目录、宽窄模型选择/返回、悬浮与键盘衔接、模型继承/重置、
busy 和外部 disabled、Workspace 键盘级联与 StrictMode 下的退出。现有菜单、
Workspace 桌面打开命令及 Dialog 回归一起通过。没有启动浏览器或实际宿主，
DOM/夹具不作为用户已暂停的视觉验收证据。

清单仍为 485 项：338 pending、126 in_progress、8 retained、10 improved、
3 removed；三个涉及视图的源码摘要已更新，所有存活摘要一致。新增文件都是
中立 TS 或行为测试，公共组件清单仍为 122 项。整个 Goal 继续。

验证：完整 npm run check 成功（lint、typecheck、470 项合同、197 个文件的
648 项组件回归及生产 build，保留既有大型分块提示），日志为
/tmp/nexus-menu-a47-check.log。随后补齐悬浮衔接和 StrictMode 焦点边界，并整理
Tab helper，最终再次完成 lint、typecheck 和六个目标文件的 37 项 DOM 回归，
日志分别为 /tmp/nexus-menu-a47-final-lint.log、
/tmp/nexus-menu-a47-final-typecheck.log 和 /tmp/nexus-menu-a47-final-focused.log。


## A48：Workspace 上下文菜单统一浮层、尺寸与关闭边界

完成 WorkspaceContextMenu 文件的代码/行为审视。原控制器按 root/directory/file
硬编码高度，视图另算 180/200px 主层、180px 子层、偏移和窗口碰撞；这些估算
与真实分组/行高不同，应用目录变化后也不会重新计算。现在交互状态只保留源
元素、原始指针点和文件；键盘 contextmenu 从源元素下缘调用，scope 切换清空。
删除 resolveWorkspaceMenuPosition、目标高度表及其桌面环境依赖。

主层/子层分别调用 resolveUiPointOverlayPosition / resolveUiSideOverlayPosition，
复用既有 cascade-menu preset（224px、12px 视口留白、320px 上限），没有新增
另一个尺寸档位。原始点夹回视口，子层按真实 menuitem ref 对齐与向左翻转，
滚动/窗口变化重新定位，长应用目录在内部滚动。UiMenuActionRow 透传 button ref，
并保留不可压缩行高；getMenuContentHeight 和共用分隔线 recipe 同时用于 Action、
Workspace 与 Room 模型，移除各自重复的 footer 高度/组合数字。

两层 Portal、外部指针、Escape 和模态范围统一归 anchored-overlay-layer，删除
Workspace 私有全局监听和级联偏移。Context source 采用显式 outside 命中策略，
点击调用区域也收起菜单，但子 Portal 继续属于父级内部。父子间隙不触发立即
关闭或新 timer；进入另一主项才切换，hover 不抢焦点，显式进入才聚焦子层。
失效源元素在重定位/指针/Escape 时清理，不抢新模态的按键或焦点，空 ref 的
延迟挂载语义保持。全部菜单关闭的 Tab 同时排除跨 Portal 的父层，避免焦点落
到马上卸载的父项；祖先关系读取同一 Overlay registry，不猜 DOM 同级顺序。

原桌面默认应用/Finder/Terminal/指定应用路径、Web 外部文件、复制、加入聊天、
创建、重命名和删除动作表保持。WorkspaceDialogs 只增加源元素传递；原 Prompt
锁定与三种输入模式回归继续通过。WorkspaceContextMenu 按已明确的代码阶段
标为 improved，视觉/实际宿主验收仍暂停；Room 模型宽高/响应式和其他 Action
消费者继续 in_progress，未以底层复用替代整个业务组件审查。

新增十二项组件回归：指针/侧向几何三项，真实交互事件/scope 一项，Workspace
边缘与 resize、动态长应用目录与滚动、父子间隙、真实模态两层 Portal、后台模态
隔离、三种失效源清理、跨层 Tab 等八项；既有行测试增加原生 ref 验证。新架构
合同阻止 Workspace 恢复私有位置表和全局关闭监听，并要求三处复用总高 owner。
这些是 DOM/坐标/事件证据，不作为浏览器或实际宿主视觉验收。

清单仍为 485 项：338 pending、125 in_progress、8 retained、11 improved、
3 removed。五个涉及 TSX 的审查记录和摘要已同步，所有存活摘要一致；公共组件
仍为 122 项。原 Goal 全量范围保持。

验证：最终完整 npm run check 成功，包含 lint、typecheck、471 项合同、198 个
文件的 660 项组件回归及生产 build，只有既有大型分块提示。最终证据为
/tmp/nexus-workspace-a48-check.log。目标集初次已通过 56 项；随后失效源夹具因
手动移除 React 节点在 teardown 报错，恢复节点后由最终完整门禁验证通过。
未运行浏览器、截图或实际宿主视觉检查。
