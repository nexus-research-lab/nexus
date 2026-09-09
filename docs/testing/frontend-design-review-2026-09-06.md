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
  后为 485 项；A94 合并三处窄窗全屏壳新增领域 owner 后为 486 项。基线文件和
  删除证据继续保留。
- 基线公共 UI 的 125 个公开 React 组件沿用现有 Gallery 的完整名单与所有者，
  不另建组件库。对应 token、recipe、状态模型和消费者随组件一起检查。经 A6—A7
  删除已证明无生产用途的 5 个导出后，A7 时 Gallery 登记 120 个；原始范围与删除
  证据仍保留在清单中，不通过缩小基线计数宣称整个审查完成。后续标准筛选和
  源码编辑原语使 A50 时公共 UI 清单为 122 项；A51—A52 移除已迁移的重复
  包装后当前为 118 项，完整基线及移除证据保留。
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


## A49：Room 模型菜单共用内容几何并保留响应式焦点

完成 ComposerRoomModelControl 文件的代码/行为审视。此前宽屏与窄屏分别渲染
模型列表，布局切换会换掉 DOM；窄屏返回栏未计入总高度，且内容 Panel 再用固定
256px 覆盖外层的视口收敛。现在只有一份 RoomModelOptions，使用稳定组件位置/
key，宽窄切换保留模型行、焦点和滚动；被移除的 Agent 行或返回按钮才按当前层级
恢复焦点，而且只在焦点已掉到 body、当前浮层仍处于最上层时执行，不抢外部焦点。

composer-session-control-layout 统一 DM/Room 模型的 256px 内容宽度、返回栏 recipe
及当前可见面板总高度。Agent 列继续取 cascade-menu 宽度，列间距与留白来自同一
公共 preset；是否并排使用由这些内容尺寸组成的媒体查询，并复用 useMediaQuery
订阅，所以单纯 viewport 变化即使没有改变原 Agent 菜单几何也能更新悬浮规则。
只显示 Agent 时只预算该列表；逐级进入时把 40px 返回栏计入限高；两列同时显示
才取最大内容高度。选项区独立滚动，单栏 Panel 填满约束后的宽度。

共享锚定定位器接受明确的复合 contentWidth，但仍在同一 solver 内处理视口夹紧；
Room 删除 window.innerWidth、二次 left/width 修补及重复模型 JSX，没有新增 geometry
preset。DM 只迁移共享模型宽度来源，不改变权限或模型命令。新增架构门禁锁定
唯一模型选项视图、公共 media 订阅/定位与宽度 owner；上一批总高门禁也跟随新的
实际领域 layout owner，不再要求视图直接持有尺寸计算。

模型菜单为用户选中的 Agent/Session 保留临时绑定：目标被移除或换 Session 时
关闭，提交前也核对当前控制器的 exact pair；同一目录刷新/模型列表增长和普通
响应式变化保留当前选择。持久修改、继承/重置、busy 和失败对账仍由原控制器拥有。
自动关闭合并到同一 effect，避免 busy 与失效检查重复执行关闭。

新增七项组件回归：复合宽度夹紧一项；Room 宽/窄/180px 约束下的同节点/焦点/
滚动保持、移除焦点节点时的恢复与外部焦点隔离、只改变媒体条件时的 hover 更新、
返回栏限高与长目录、Session 替换/移除后的无命令关闭共六项。原键盘进入/返回、
IME、Tab、hover、模型更新/继承/重置和 disabled 场景继续通过。DOM 与坐标断言
不作为用户已经暂停的视觉或实际宿主验收。

清单仍为 485 项：338 pending、124 in_progress、8 retained、12 improved、
3 removed。两个涉及 TSX 的记录/摘要已同步，所有存活摘要一致；新增 layout 是
非组件 TS，公共组件仍为 122 项。直接 Session/权限控件与其他页面保持原范围待审，
整个 Goal 继续。

验证：最终 npm run check 通过，含 lint、typecheck、472 项合同、198 个文件的
667 项组件测试和生产 build，仍只有既有大型分块提示。权威日志为
/tmp/nexus-room-model-a49-check.log。此前目标集 35 项已通过；最终门禁还验证了
自动关闭 effect 合并和 Tooltip 焦点夹具的 act 修正，未运行浏览器或宿主视觉校验。


## A50：会话设置与权限范围菜单绑定当前业务身份

完成直接会话控件、选项投影、权限确认和范围选项四个文件的代码/行为审查。
直接模型与权限菜单原本只保存布尔展开值，切换 Session 后仍可能呈现另一个
目标的可操作菜单。现复用 useResettableState，以 target.sessionKey 在同一次
渲染中重置；busy/disabled 继续关闭并清除展开态，恢复时不自动重开。领先权限
入口删除 DM/Room 重复装配分支，保留 DM 选择继承模式写空 override、Room
显式写模式及恢复默认动作，不修改控制器事务或 Agent 默认值。

模型选项的 JSON 解码、继承值恢复和显式更新唯一归属
composer-session-control-options，DM 与 Room 都调用同一分派；非法/空值不再
可能成为清空配置请求。Room 原有 exact Agent/Session guard 和单一响应式
列表保持。模型名称占剩余空间，次要 Provider 名称最多占 40%，长名称可省略
并提供完整原文提示；保留公共紧凑菜单的字号、行高、分隔和选中状态，不新增
另一套菜单或宽度配方。删除重复模型分派、重复领先分支和无语义的 class 别名。

权限范围菜单按 request_id 重置；interactionDisabled、必需密钥不完整或
范围选项消失时立即隐藏并关闭，不在重新可提交后自动恢复。响应入口同步检查
禁用状态，成功后清空密钥与展开态，发送失败保留密钥供用户重试。范围项保持
纯投影：单次、Automation allow_task 与 runtime suggestions 各自表达真实
授权，选中原始建议原样回传，既不猜测规则也不退化为无建议的静默授权。
保留 Agent/工具元信息、摘要、有限高度参数、必要密钥和唯一决策行的层级。

新增八项 Session 控件回归及五项权限确认回归，验证精确 Session 切换/返回、
Provider 身份与继承/reset、DM/Room 权限差异、busy/disabled、不可重置状态、
无效编码、权限请求切换、禁用/缺密钥/失去范围、失败密钥保留和原建议重试。
Room 原 11 项级联/响应式/键盘/身份回归保持通过；共 27 项目标测试通过。
新门禁约束两个消费者不得再自行解码或直接分派模型设置。当前没有浏览器、
截图或宿主视觉校验，也没有扩充 Gallery。

清单仍为 485 项：335 pending、123 in_progress、9 retained、15 improved、
3 removed。上述五个相关 TSX 的记录和存活摘要已同步；公共组件仍为 122 项。
整个 Goal 继续，其他目录及页面没有因本批完成而被视为已经审查。

验证：最终 npm run check 通过，含 lint、typecheck、473 项合同、199 个文件的
680 项组件测试和生产 build，仍只有既有大型分块提示。权威日志
/tmp/nexus-session-a50-check.log；未执行任何浏览器或宿主视觉校验。


## A51：清理工作区透传/动作封装并收口公共状态布局

本批完成八个公共组件文件的代码审查：ResourceState、StateBlock、目录卡片
改善长文本边界，启动加载、工作区加载及侧栏空态保留其明确的语境布局，删除
两个没有独立公共职责的文件。ContactsDirectory 的搜索此前只经
WorkspaceSearchInput 原样转发到 UiSearchInput；现直接使用真正所有者，
保留受控查询、回调、明确占位与外部宽度。默认搜索占位与可访问名称随语言
更新，新增测试还发现显式 aria-label={undefined} 会由 props spread 覆盖
回退名称，已调整属性顺序；显式文案与 Field 关联不受影响。

WorkspaceActionBar / WorkspaceActionCard 只有 Room 降级页一个生产消费者，
其 pills 变体未被使用。三个原导航动作现由领域内稳定 ID 的动作数据装配，
直接使用现有 WorkspaceCatalogCard 的主按钮/hover/focus/圆角与公共 Typography，
保留 launcher、contacts、handoff 路由以及原三列断点，删除单独的原生卡片按钮
配方。最近会话的精确 Room 过滤、排除未开始草稿、排序和数量保持原逻辑；
该降级页的其他身份/元信息区域继续列为 in_progress，没有扩大完成结论。

状态块允许在窄父容器中收敛，连续路径/标识可换行；标题、说明及恢复动作受
可用宽度约束，图标作为装饰。ResourceState 的有限状态、单恢复动作/独立双向
决策、busy/disabled 保留，显式 action tone 不再被主次位置覆盖。目录卡片
同样可在网格内收缩和换行，Article 主动作与次动作仍保持 CSS 的局部命中隔离。
没有新增业务状态推断、第二套按钮或新的主题数值。

启动品牌加载保留静态 reduced-motion 帧与 supporting 文字，工作区加载保留
铺满 Frame 的 Spinner/标签布局，侧栏空态保留 caption 密度、impact/nextStep
去重和公共动作；三者不因都显示状态就合并为强制套卡的单一页面外形。原相关
行为回归继续通过。新增三项 Room 键盘导航、一项搜索语言/可访问名称以及一项
双向决策的忙碌隔离测试，目标集 42 项通过。该集为 DOM 功能回归，未做视觉、
浏览器或宿主检查；Gallery 仅删除退休示例、迁移已有搜索示例并更新登记。

清单仍为 485 项：326 pending、124 in_progress、12 retained、18 improved、
5 removed。十一条涉及记录已更新，存活 source_sha256 全部一致；两个被删除
文件的原始摘要保留作追溯。公共组件由 122 减至 119；减少的三个导出都有迁移
或唯一消费者证据，不缩减原审查基线。整个 Goal 继续。

验证：npm run check 通过，含 lint、typecheck、473 项合同、200 个文件的
685 项组件测试和生产 build，日志 /tmp/nexus-state-a51-check.log。最终复查
补上 ResourceState 动作容器的 min/max 宽度（避免 sm:w-auto 在桌面窄分栏
按长动作内容展开），随后 lint、9 项状态回归与 build 再通过；最终复验日志
/tmp/nexus-state-a51-final-{lint,state,build}.log。构建仍只有既有大型分块提示。
未运行任何浏览器、截图或宿主视觉校验。


## A52：目录内容、状态标记与列表三态控件

完成目录内容、图标框、列表分隔、复选框、行次动作与可移除 chip 的公共文件
代码审查，并删除 WorkspaceStatusBadge。它的两个生产消费者都属于定时任务
历史，实际只用紧凑圆点与状态模型提供的 label/tone；直接换成 UiBadge 的
xs/showDot 后参数和业务模型不变，既有 Gallery 图标/圆点例子也迁往同一 owner。
没有改写运行状态、权限、重试或投递规则。

按 TypeScript AST 清点全部生产/开发 JSX 后，删除没有消费者的 Catalog
Description minHeight（固定 40px 预留）与没有生产 grow=true 的 Body 开关；
唯一生产显式 false 来自联系人卡片，去掉该空操作仍保留用户选定结构。默认
标题、说明字号映射与有限行数不变，Header/Body 和文本可随分栏收窄，Footer
在空间不足时换行，不强撑动作。图标框保留实际 default/primary、圆形/圆角和
尺寸，删除无生产/开发使用的 success/warning 色表；显式布局 style 与主题
样式合并，undefined 不再覆盖 primary 的色彩。Gallery 只迁移旧 API 和登记，
未新增任何视觉场景。

UiListSectionDivider 原来需要调用方再手工传同名 aria-label，否则可见分组
名称不成为 separator 名称；现以内部稳定 ID 默认关联，同时尊重外部名称。
长名称可换行，去掉标签不会留下悬空引用。UiListAction 保留有实际职责的行内
可见性/事件隔离，并继续调用 IconButton；可移除 chip 保留紧凑几何、实体命名
和禁用删除边界，不因其也是按钮组合就删除有独立用途的模式。

新回归证实 UiCheckbox 在父级未接受全选时，原生点击会清除 indeterminate，
造成视觉 false 与 aria-checked=mixed 分离。现先发出原生选择意图，再按最近
已提交的 prop 恢复 DOM；useLayoutEffect 保持 props 与真实节点在绘制前同步。
另一个回归覆盖 onChange 内父级同步提交，防止旧闭包把已接受的新状态覆盖回去。
两份失败复现分别位于 /tmp/nexus-catalog-a52-repro.log 与
/tmp/nexus-catalog-a52-sync-repro.log。新增两项 checkbox 和一项分隔线命名回归；
先前目标集 41 项通过，完整门禁验证最终 committed ref 修正。

清单仍为 485 项：317 pending、126 in_progress、14 retained、22 improved、
6 removed。十条涉及记录已更新且所有存活摘要一致，公共组件由 119 减至 118。
历史弹窗/单项及联系人卡片仍按原范围保留 in_progress，不以公共组件迁移视为
全部页面审查完成。整体 Goal 继续，浏览器、宿主和所有视觉验收仍按用户要求暂停。

验证：最终 npm run check 通过，含 lint、typecheck、473 项合同、200 个文件的
688 项组件测试和生产 build，日志 /tmp/nexus-catalog-a52-check.log。最终版本
包含 mixed 的同步提交修正，构建仍只有既有大型分块提示；未做视觉验收。


## A53：文件树展开状态与目录面公共组件

回归复现父目录收起后，子目录自身卸载使原展开状态丢失。Tree 入口现持有按路径
保存的展开偏好，顶层默认打开；重新打开父目录、接收同一目录的新快照时保留
子级选择，已经消失的目录记录清除。折叠时仍只渲染可见节点，没有为保存状态
保留整棵隐藏 DOM。目录点击继续调用原 focusDirectory，文件点击、重命名、
删除与右键继续传递精确原路径或 entry，不引入自动文件请求或变更。

递归行的三个原生私有按钮改用 UiButton/UiListActionButton，行次动作复用公共
hover/focus/touch 可见性和危险色语义；选中文件的动作保持可见。同名文件的次
动作按完整路径具名，主入口保留完整路径提示。名称使用公共 supporting 与有限
字重，长名称可在面板内收敛，层级缩进保留原步长并限制最大占比。文件图标映射、
目录排序、当前文件和目录目标投影保留，未添加第二套控制样式。

层级表达采用具名嵌套列表、原生 disclosure 按钮、aria-expanded/controls 与
当前文件语义；Tab、Enter/Space 继续使用原生按钮行为，不声明需要另一套方向键
状态机的 tree widget。去标记列表按 [WebKit 的官方说明](https://bugs.webkit.org/show_bug.cgi?id=170179#c1)
显式保留 role=list，仅两个对应行说明 lint 例外，不降低全局规则；这是实现依据，
没有声明已完成 Safari/VoiceOver 或宿主验收。

文件浏览器初始加载改用具名 WorkspaceLoadingState，空目录改用 SidebarEmptyGuide，
删除无名字 Spinner 和私有空态图标卡片；刷新已有文件时继续展示原快照。新增六项
功能回归覆盖展开保留/清除、空快照后加载、具名列表/当前文件、键盘独立动作、
同名路径/右键事件以及加载/空态/刷新互斥。目标集包含既有右键菜单回归，共 17 项
通过；合同门禁进一步约束文件行按钮与字号只取公共 owner。

清单仍为 485 项：314 pending、127 in_progress、14 retained、24 improved、
6 removed。两个公共文件完成代码/行为审查，文件浏览器仍有外部分栏和调整尺寸
待审，不扩大为整个 Workspace 页面完成；公共登记仍为 118 项。全部存活摘要
一致，完整 Goal 继续。用户暂停的所有浏览器、截图和宿主视觉检查均未运行。

验证：最终 npm run check 通过，含 lint、typecheck、474 项合同、202 个文件的
694 项组件测试与生产 build，日志 /tmp/nexus-tree-a53-check.log。首次失败复现
见 /tmp/nexus-tree-a53-repro.log，目标集见 /tmp/nexus-tree-a53-target.log；构建
仍只有既有大型分块提示。


## A54：分栏鼠标拖动生命周期

代码检查发现文件目录和首页辅助面板分别维护同样的 mousemove/mouseup 监听，
只靠窗口收到松开事件结束拖动；失焦、文档隐藏或在窗口外松开后再次移入都没有
终止边界。两处现复用中立 useMouseDrag，只向尺寸 owner 转交仍按住主键的移动；
主键释放、失焦、文档隐藏、调用方停用和卸载统一结束或清理。主键仍按住时，
释放次键不会误终止。回调更新保留当前投影，停止后的迟到移动不再改变布局。

PanelResizeHandle 的中键/右键不再启动调整，主键启动阻止原生文字选择。
文件列表进入上下堆叠或专注预览时显式停用，重新恢复不延续旧拖动。目录保留
常规 200–360px、紧凑 160–280px 的边界和既有默认宽度切换；辅助面板继续使用
30–56% 的现有范围，不更改预览/Thread 的额外 CSS 尺寸限制。未挂载或零宽容器
不参与新宽度投影，防止零宽辅助容器计算出 NaN。侧栏已有独立指针捕获与折叠
热区，本批不将其改成另一套鼠标流程。

新增四份功能测试，共十项：共享拖动结束边界/回调更新/卸载、入口主键筛选，
以及两个真实尺寸控制器的接入回归。目标集先通过九项，完整门禁覆盖新增辅助
面板回归。架构门禁约束两个消费者引用唯一鼠标生命周期，不恢复私有窗口监听。
本批没有增加 Gallery 场景、浏览器或宿主验证，也未把 CSS 参数断言当作视觉证明。

两个清单条目从 pending 进入 in_progress。PanelResizeHandle 仍缺键盘调整、
分隔条语义与可见焦点；RoomWorkspaceView 仍有完整 Header/面板分支待审。
这些是明确待办，本次鼠标修复不代表公共原语或 Workspace 页面完整完成。
清单仍为 485 项：312 pending、129 in_progress、14 retained、24 improved、
6 removed；公共登记仍为 118 项，全部存活文件摘要一致。整体 Goal 继续。

验证：npm run check 通过，含 lint、typecheck、475 项合同、206 个文件的
704 项组件测试与生产 build，日志 /tmp/nexus-resize-a54-check.log；初始目标集
见 /tmp/nexus-resize-a54-target.log。构建只有既有大型分块提示。


## A55：分栏键盘调整与有效宽度投影（2026-09-07）

补齐 A54 明确遗留的分栏键盘、焦点和范围语义。PanelResizeHandle 现在是关联
右面板的具名竖向 separator：左右方向键每次移动 16px，Home/End 到有效最小/
最大宽度，原生 Tab 顺序保留，平台组合键继续传递给页面，主鼠标键仍走既有
拖动生命周期。焦点使用现有 ring token，在热区内部可见；普通状态继续保留
8px gutter 或 12px overlay，不添加常驻线条或拖手。当前像素宽度本地化播报，
没有测量值或可调整范围退化为固定值时不进入 Tab 顺序。

[WAI-ARIA 1.2 的 separator 定义](https://www.w3.org/TR/wai-aria-1.2/#separator)
明确区分静态边界与可聚焦调整 widget；实现只为该具名可聚焦节点解释一处 lint
例外，不降低全局规则。采用范围调整及明确面板关联；Enter/Space 不承载独立
业务页面关闭，关闭仍由 Thread/工作区自己的动作拥有。这是代码与 DOM 语义
实现依据，没有声明实际屏幕阅读器或宿主验收完成。

右栏原有百分比偏好与 CSS min/max 并不总一致。例如 1600px 容器中保存 56%
的 Thread 被 CSS 限为 560px；直接减百分比会在看不见的区间移动多次。新增
Room 宽度纯模型从一份边界数据同时生成原样 CSS 和有效像素范围，几何适配
观察真正的百分比父容器与视口；键盘从 560px 直接请求 544px，再交回原 owner
保存为 34%。容器改变只刷新投影，不改写偏好；未挂载/零宽禁用调整，卸载释放
观察器及窗口监听并忽略迟到通知。辅助面板与 Thread 的原 CSS 限制、常驻内容、
会话来源和关闭动作不变，未增加新的宽度持久化或业务命令。

文件目录从自身两档宽度 owner 提供同一像素合同，鼠标、键盘都服从原范围；
断点改变时即使偏好重置尚未完成，也先按当前范围投影，避免暴露越界的 ARIA
值。堆叠/专注预览仍停用横向控件。页面、Shell 和内容装配只转交原控制器命令，
不解释边界。现有静态 Gallery 示例只迁移必需参数和面板关联，没有新增场景
或开始视觉检查。

功能回归覆盖主键/焦点、方向键与端点、自然 Tab、页面快捷键、固定/未测量范围、
CSS 限宽后立即调整、父容器/视口变化不改写偏好、监听释放及两个实际尺寸 owner
的有限数值接入。目标集十项通过；新增架构门禁禁止叶子右栏重新复制 min/max
CSS 数值或键盘处理。公共分隔条与 inline Thread 装配完成代码审查，内部 Thread
内容独立计入清单，其他涉及页面按真实已审范围保持 in_progress。

清单仍为 485 项：307 pending、132 in_progress、14 retained、26 improved、
6 removed。八个改动条目的存活摘要已更新，全部其余存活摘要一致；公共组件
仍为 118 项。整体 Goal 继续，所有浏览器、截图与宿主视觉验收保持用户要求的暂停。

验证：npm run check 通过，含 lint、typecheck、476 项合同、207 个文件的
710 项组件测试及生产 build，日志 /tmp/nexus-resize-a55-check.log；目标集日志
为 /tmp/nexus-resize-a55-target.log。最终复查仅将四个新增 L3 合同移到文件顶部，
没有行为变更；构建仍只有既有大型分块提示。


## A56：工作区目录状态、专注切换与窄窗布局

完成 RoomWorkspaceView 与文件浏览器自身的装配/布局分支审查。原目录在首次
读取失败且没有文件缓存时仍显示“暂无文件”，同时出现全局失败提示，混淆了
读取失败与确认空目录。现由共享 UiResourceState 在目录正文提供唯一失败说明
和刷新动作；已有文件时继续显示缓存及可关闭的刷新反馈。只有成功读取且没有
文件时显示正常空态，加载和失败不再互相冒充。

资源 owner 将读取失败事实与反馈关闭偏好分开；关闭提示不能把未知目录变成
已确认空目录。原来仅被上层作为布尔值消费的 errorMessage、原始异常文本和
固定中文 fallback 已删除。原 Agent/request 序列栅栏、文件缓存、写事务状态及
刷新失败不覆盖已确认写入的语义不变；刷新仍只读，既不自动重放写命令，也不
让旧 Agent 的迟到失败污染新目录。命令反馈仍按原优先级表达独立操作结果。

目录正文把文件树、加载、空态和错误放在同一个滚动容器，删除嵌套的文件专用
滚动层。堆叠态保留 42% 高度和 320px 上限，移除短窗口中挤占预览的 220px
固定下限，所有状态可随容器收缩并滚动。目录 toolbar 的上传/创建仍消费共享
图标按钮及原 busy 条件；横向调整继续由 A54/A55 的共享生命周期和范围拥有。

专注预览以前卸载整个文件浏览器，返回后丢失展开状态；现在隐藏同一 Agent
目录实例并停用调整，退出专注后恢复原展开。浏览器 key 使用真实 viewAgentId，
进入另一工作区才重置，避免把相同相对路径的展开偏好混入另一 Agent。隐藏目录
不参与布局或焦点。根标题回退文字复用 metadata，删除单子项容器无效的 gap。
文件路径、Agent 切换、上传 input、弹窗和预览实例的原职责保持不变。

同时审查文件预览 router：完整类型映射、模块级稳定 renderer、直接预览类型
参数剥离和 Office 懒加载/Suspense 外壳均适合当前职责，予以保留；仅删除无语义
的 TextPreviewRenderer 中间别名并补 L3 合同。具体文件加载、编辑、Office 和
媒体实现仍按各自清单继续，不以外壳审查等同于整个文件预览业务完成。

新增四项真实控制器/目录接入回归，目录 API 使用离线受控结果，其余命令与状态
组合保留真实实现：首次失败和手动重试成功、缓存失败及关闭后真相保留、跨 Agent
迟到失败、专注来回保留展开且换工作区重置。专注测试只替换内容预览渲染器，
不执行实际文档解析；fetch 防护证明未使用网络。加上已有文件树/目录回归，目标
十项通过；唯一 owner 门禁约束目录反馈、按钮及根标题排版不恢复私有实现。

清单仍为 485 项：306 pending、130 in_progress、15 retained、28 improved、
6 removed。两个 Workspace 外壳改为 improved，类型 router 改为 retained；
对应存活摘要已更新且其余全部一致，公共组件仍为 118 项。整体 Goal 继续，
浏览器、截图和宿主视觉验收仍按用户要求暂停，未据 CSS 推导宣称视觉验收通过。

验证：npm run check 通过，含 lint、typecheck、477 项合同、208 个文件的
714 项组件测试及生产 build，日志 /tmp/nexus-workspace-a56-check.log；目标集
见 /tmp/nexus-workspace-a56-target.log。构建只有既有大型分块提示。

## A57：文件动作反馈与文本编辑状态布局（2026-09-07）

本轮完成文件 chrome、文本 Header/恢复视图、预览 Panel 和加载外壳的代码与
行为审查。正文渲染、完整文本控制器、Office 与媒体业务继续保留各自未完成范围；
不以本轮局部回归代替真实下载、宿主定位或整个编辑业务验收。

外部文件按钮原先把已翻译的完整反馈属性保存在状态中，连续操作时较早的失败
仍可覆盖较新的成功；语言切换也不能更新已出现的反馈。现在只保留失败事实，
在 render 使用当前语言投影公共 FeedbackBanner。反馈绑定 owner 代次、Agent、
路径、文件名与最近一次显式操作；owner 推进但尚未发布时也拒绝迟到回调，
作用域变化及卸载使请求失去反馈提交资格。Panel 原有 Agent/path key 已覆盖
普通文件切换，本次进一步约束动作自身生命周期；不宣称取消已发出的操作，
不引入重放、通用 mutation journal 或新的下载实现。

文本恢复视图删除重复的分支布局和两个只传递属性的私有状态组件，统一为一处
UiResourceState 与纯事实描述。读取失败优先、权限文案、保留旧内容的影响说明、
未知结果对账、冲突读取/审阅、明确可重试、危险覆盖及忙碌按钮均保留原语义。
标题栏删除两个同步状态包装及组件映射，状态文字只渲染一次，装饰图标隐藏于
辅助技术；同步继续作为轻量 metadata，不额外套 Badge，图标尺寸归共享 Header。

Panel 保留 exact Agent/path 的 renderer 边界，位置、专注和语言变化不重建草稿；
切换 Agent/path 或关闭预览才卸载。加载外壳保留为领域内必要的 ResourceState/
Spinner 组合，继续由调用者持有加载事实；无需为这个窄职责增加公共组件。

新增 27 项离线组件回归覆盖上述反馈时序、当前语言、显式恢复动作、revision
门禁、忙碌状态、Header 命令和 Panel 草稿生命周期；文件外部操作及 Panel 的
内容 renderer 使用受控替身，不读取文件或发起真实下载。加上既有正文和预览
状态回归共 38 项通过，见 /tmp/nexus-editor-a57-target.log。合同门禁改为约束
唯一 UiResourceState 和单句影响说明，不依赖原 JSX 属性写法。

清单仍为 485 项：304 pending、127 in_progress、17 retained、31 improved、
6 removed。对应存活源码摘要已同步，其余摘要全部一致；公共组件仍为 118 项。
整体 Goal 继续，所有浏览器、截图和宿主视觉验证仍按用户要求暂停。

验证：npm run check 通过，含 lint、typecheck、477 项合同、211 个文件的
741 项组件测试及生产 build，日志 /tmp/nexus-editor-a57-check.log；构建只有
既有大型分块提示。未修改后端，未扩大为 Go 或浏览器验证。

## A58：纯文本正文与大型文件分段预览（2026-09-07）

大型文本原先把分页、请求和视图放在一起，只在 effect 清理中 abort，请求回调
没有自己的提交栅栏；内部的偏移历史也不随身份重建。新控制器
`use-large-text-file-preview.ts` 集中拥有 exact owner 代次/Agent/path 与单片段
状态，切换后从零读取，导航或重试清除旧片段，过期/取消结果不能提交，owner
推进但尚未发布时也拒绝旧回调。同一批次连续导航只前进一步；返回历史页后
使用刚读到的 nextOffset，删除后续旧偏移。正常 Panel 已按 Agent/path remount，
这里补齐控制器自身边界与 owner 变化，未改变服务端协议或扩张为整文件缓存。

分页栏在加载和错误期间保留原按钮实例并禁用，避免每次翻页移除当前操作入口；
只读说明仅由分页栏显示，Header 只保留当前片段字节范围。工具行可按宽度换行，
说明和段序改用公共 metadata。错误面移除强制居中且不可滚动的容器，在有限
高度中保持恢复动作可滚动到达；仍只提供用户显式从头读取，不拼接内容或写文件。

无高亮纯文本和分段正文统一消费已有 14px/24px 源码度量，移除各自字号/行高；
源码样式 owner 还提供一个预览滚动/内嵌焦点配方。Body 的普通预览和分段正文
将其组合为按文件名命名、可 Tab 聚焦的 region；HTML 继续由内容宿主拥有滚动，
不增加第二层。Markdown 的 exact Agent 资源能力、Mermaid、懒加载语法高亮、
流式视图、编辑焦点/失焦 opt-out 与加载时禁写继续保持原边界，渲染器内部仍
按各自记录审查，本轮不据分派代码宣称它们全部完成。
两个只读 region 对 `no-noninteractive-tabindex` 使用逐行说明的窄例外：Tab
只为原生滚动建立焦点，不模拟按钮或手写方向键；命名、Tab 与公共焦点配方
由行为测试和所有权门禁覆盖，未放宽全局 lint 规则。

新增 17 项离线回归，目标合计 20 项通过，见 /tmp/nexus-text-a58-target.log：
视图与真实控制器验证旧内容隔离、翻页偏移/重复点击、作用域变化、取消、当前
语言与显式恢复；正文验证命名/Tab、精确模式参数、禁写和字面空白。另以真实
API 函数和假 Fetch 响应验证 512KiB 边界、中文/emoji 完整性、非 Range/偏移
不符/超限响应在读体前取消、非法 UTF-8 拒绝。纯传输测试单独使用 Node 环境：
本机 jsdom 的 `Headers({ Range })` 实测产生空头，而 Node 原生 Headers 保留
该头；没有为测试环境改写产品请求实现。全程无真实网络/文件下载。

清单仍为 485 项：304 pending、124 in_progress、17 retained、34 improved、
6 removed。三个文本视图完成本轮代码/行为审查，新增控制器属于既有业务所有权；
存活源码摘要已同步且其余全部一致，公共组件仍为 118 项。整体 Goal 继续，
视觉验证仍按用户要求暂停，实际渲染和宿主滚动几何未宣称完成验收。

验证：npm run check 通过，含 lint、typecheck、477 项合同、213 个文件的
758 项测试及生产 build，日志 /tmp/nexus-text-a58-check.log。构建仅有既有
大型分块提示；未运行 Go、浏览器、截图或宿主视觉验证。

## A59：原生媒体生命周期与 HTML 流式预览（2026-09-07）

图片/PDF 原先重复维护加载状态、重试计数和标题栏；图片错误又嵌套在带 padding
的内容面内，恢复容器强制至少 240px 高，在短面板中可能被外层裁切。现在两个
路由入口保留类型职责，共用 NativeMediaPreview 和 useNativeMediaPreview；
图片内在比例、留白及原生 PDF URL/沙箱仍保留。图片失败面单独滚动，去掉硬性
最小高度与双层 padding。二进制占位的宿主/语言提示与文件动作保持。

原生元素及回调绑定 owner 代次、Agent/path、本地切换代次与显式重新加载计数；
返回曾看过的文件也不能复用旧回调。文件/账号或重新加载产生新元素，标题、语言
与专注变化保留元素。owner 尚未发布时旧事件/重新加载就失去资格；这些代次只
拥有 UI 生命周期，不形成新的资源身份、缓存或服务端请求协议。

回归揭示图片与 PDF 不能等同处理：按 [MDN 的 iframe 事件说明](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/iframe#error_and_load_event_behavior)，
iframe 不提供可靠的 error 事件，load 也不能证明文件成功。已移除 PDF 的不可达
错误分支与独占中英文文案，增加始终可用的公共工具栏“重新加载预览”按钮；
load 只结束等待，状态名 settled 不表达文件成功。图片继续基于原生 error 事实
提供显式重试。不预读整份 PDF、不扩大沙箱、不自动重放；这次只查阅平台文档，
未启动浏览器或做产品视觉检查。

HTML 预览把流式提交收敛为单个 effect 的定时器与清理，删除 latest content/
pending timer 两个 ref、重复清理 effect/callback 及冗余/混合命名的返回字段。
原 250ms 节流期限、末次内容即时提交、半截 Head 保留已有画面、storage shim
插入与 opaque-origin 沙箱均保留。尚无已提交文档的半截 Head 改为共享源码排版
及按文件名命名的可聚焦滚动区域；局部 Tab lint 例外沿用 A58 的只读滚动理由。
文档本身继续使用 iframe 原有纸面与布局，没有注入 Nexus 字号或视觉样式。

新增 12 项离线回归，连同已有状态/文件动作回归共 21 项通过，见
/tmp/nexus-media-a59-target.log。覆盖图片重试、PDF 等待中/结束后的显式重新
加载、原生元素保持/重建、返回原文件的旧事件、未发布 owner 代次，以及 HTML
shim 插入顺序、沙箱属性、具名源码、250ms 合并、最终刷新和卸载清理。测试不
执行 frame 脚本/storage，也不加载 PDF 或图片资源；不宣称原生渲染验收完成。
所有权门禁约束一个媒体加载组合、共享源码配方和已移除的不可用分支。

清单仍为 485 项：303 pending、123 in_progress、17 retained、36 improved、
6 removed。两个媒体文件完成本轮代码/行为审查，存活源码摘要已同步且其余全部
一致；公共组件仍为 118 项。整体 Goal 继续，视觉与宿主验收仍按用户要求暂停。

验证：npm run check 通过，含 lint、typecheck、477 项合同、215 个文件的
770 项测试及生产 build，日志 /tmp/nexus-media-a59-check.log；构建仍仅有既有
大型分块提示。HTML 的提交间隔、storage shim 和 Head/document 准备函数与
本轮前版本逐字比对一致，未扩张为原生脚本/宿主验收。

## A60：Office 预览重试与资源生命周期（2026-09-07）

DOCX 失败时原先卸载了渲染容器。点击重试只推进计数，下载 effect 在 loading
重新挂载容器前捕获空 ref，随后提前返回并一直停在加载态。真实入口、控制器
和视图的离线回归先复现该失败（/tmp/nexus-docx-a60-repro.log），再通过保留
渲染与样式宿主修复。未完成内容隐藏并 inert，成功后才进入可访问树；离屏
解析、纸面 CSS、DOCX 选项、媒体归一化和缩放计算保持原有职责。

DOCX、XLSX 和 PPTX 现在消费同一个 useOfficePreviewScope，移除三处重复的
文件 key、重试 callback 和 loading 写入。owner 代次、Agent/path、本地切换
代次和重试计数共同限定当前工作；离开再返回同一文件也不得接收旧回调，owner
尚未通知订阅者时旧结果即失去提交资格。格式控制器继续拥有 AbortController、
离屏解析、DOM 和 object URL 的释放，公共 Hook 不发起 IO 或建立新服务端协议。
文件/账号切换重置表格选择与幻灯片页码；PPTX 迟到结果只释放自身资源，不触碰
当前文稿。DOCX 旧 RAF、ResizeObserver 和 window resize 回调也按请求隔离。

Office 懒加载容器和表格/幻灯片错误面改为受控滚动；共同错误组件只横向居中，
删除会把内容挤出短面板的纵向居中和额外大留白。继续使用公共文件动作、状态、
Typography 与 Choice/Button，不重新定义文档、工作簿或幻灯片内容排版。

新增 12 项行为回归，连同既有状态与翻页回归共 22 项通过，日志
/tmp/nexus-office-a60-target.log。覆盖 DOCX 真正重试完成、旧解析与尺寸回调、
XLSX 离开/返回同一文件的迟到成功和失败、未发布 owner 切换、工作表选择重置、
显式重试/卸载，以及 PPTX 当前/迟到/卸载资源释放。解析和 Canvas 使用受控
夹具，未验证 Office 文件解析保真或宿主渲染。新增所有权门禁防止三种格式重新
各自维护文件、账号和重试状态。

清单仍为 485 项：302 pending、119 in_progress、18 retained、40 improved、
6 removed。五个入口/视图完成本轮代码与行为审查；表格网格、幻灯片 Canvas
和各格式解析器仍由独立清单范围继续审查。五项源码摘要已更新，其余存活摘要
全部一致，公共组件仍为 118 项。整体 Goal 继续，视觉与宿主验收仍暂停。

验证：npm run check 通过，含 lint、typecheck、478 项合同、218 个文件的
782 项测试及生产 build，日志 /tmp/nexus-office-a60-check.log；构建仍仅有
既有大型分块提示。未启动浏览器、原生宿主或产品服务。

## A61：表格内容排版与幻灯片画布（2026-09-07）

表格字号原先除以 4/3 再套 10px 下限，12pt 最终只显示为 10px；已按
[Open XML 字号单位](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.spreadsheet.fontsize?view=openxml-3.0.1)
和 [CSS 绝对长度关系](https://www.w3.org/TR/css-values-4/#absolute-lengths) 改为
乘 96/72，去掉会覆盖源字体的小字号下限与提前取整。单元格使用自然行高和
2px 上下留白，默认单行、显式 wrap 才保留换行。原先作用于普通 div 的
vertical-align 没有效果，现由真实 flex 容器承载上/中/下对齐。纸面背景、
文字和边界统一消费已有 paper token，避免 App 深色底改变源文件配色关系；
坐标栏仍用 App 材质，两个方向共用 caption/medium/muted，不再各写 10px 字号。

虚拟化结果从平铺单元格收口成有序行；原有视口、尺寸、合并区间与补入离屏
锚点计算保留，补入的锚点参与行列排序并携带受工作表范围约束的 span。视图
按 [WAI table pattern](https://www.w3.org/WAI/ARIA/apg/patterns/table/) 与
[虚拟表格属性](https://www.w3.org/WAI/ARIA/apg/practices/grid-and-table-properties/)
输出 table/row/cell、总行列数、源索引和合并 span，去掉没有行容器和方向键
交互的 grid 声明。具名 region 提供原生 Tab/键盘滚动入口；视觉坐标栏隐藏于
可访问树，数据位置由行列索引表达。换工作表仍重建滚动区域，原滚动偏移不会
带入下一张表；行列标签继续随实际 scrollLeft/scrollTop 同步。

`layout/preview-viewport-styles.ts` 接管中立预览的 overflow/overscroll/focus
配方。文本、分段文本、HTML 源码与表格共同消费；从 source-text-styles 移动的
class 字符串已逐字比对一致，文本/HTML 的时序、内容排版、编辑与沙箱均未改动。
源码等宽排版继续由 form/source-text-styles 持有，没有把表格字体套成源码。

幻灯片曾为缩略图和主画布分别计算文字内边距，同一文稿因此具有两份内容布局；
现在缩略图直接缩放相同 SVG 内容，只保留外部缩略图阴影。段落/Run 的正文
拼接 key 会在重复内容时冲突，已用不可变解析序列的位置表达身份，删除两段
拼接函数和透传 shapeId/thumbnail 参数。源几何、图片比例、形状、颜色、字体、
项目符号与段落行距继续由 Canvas 持有，解析器没有改动。

新增/扩展 8 项行为回归，相关三个测试文件共 9 项通过；最初 6 项失败已在
/tmp/nexus-office-content-a61-repro.log 记录，覆盖原字号、对齐及缩略图/key
问题。最终门禁同时覆盖受控视窗中的真实合并投影、行列顺序、样式和滚动同步；
ExcelJS 只生成样式对象，未读取真实 XLSX，虚拟视窗为夹具输入，SVG 只检查 DOM。
这些证据不代表 Office 解析保真、真实测量、屏幕阅读器或宿主视觉验收完成。

清单仍为 485 项：300 pending、119 in_progress、18 retained、42 improved、
6 removed。网格视图与幻灯片画布完成本轮代码/行为审查；两者及三个配方迁移
消费者的摘要已更新，其余存活摘要全部一致。公共组件仍为 118 项；整体 Goal
继续，全部视觉与宿主验收仍按用户要求暂停。

验证：npm run check 通过，含 lint、typecheck、478 项合同、220 个文件的
790 项测试及生产 build，日志 /tmp/nexus-office-content-a61-check.log；仍仅有
既有大型分块提示。未启动浏览器、原生宿主或产品服务。

## A62：Composer 附件、目录与文本入口（2026-09-07）

普通文件/文本附件和本机目录原先分别维护 Chip 外壳、字号与移除按钮，现在
消费 UiRemovableChip 的 xs 档位；目录范围说明消费 Badge，添加仍用共享
IconButton。完整文件名/路径保留 title，附件移除动作包含文件名，移除与文本
预览没有嵌套按钮或冒泡耦合。图片仍保留 48px 内容缩略图和覆盖式独立删除动作。
删除未再需要的附件样式常量、固定中文类型表、重复删除布局，以及相同条件下
重复判断的目录组；目录变更按已有 blocksMutation 事实明确禁用，reload 继续
可用，不在视图新增授权或保存判定。

图片和文本预览合为一个公共 Dialog/Header 外壳，交给 Header 自动生成并
关联实例标题，去掉两套标题/关闭 DOM、固定 title ID、私有标题字号和关闭
按钮尺寸覆盖。文件名仍为唯一标题，图片/文本各自保留已有灯箱尺寸与正文行为。
错误和加载共用一处 ResourceState，可滚动的恢复面不再依赖 m-auto 居中；
文本复用公共源码排版和中立预览视口，按文件名命名且可 Tab 聚焦，仍只读取
512 KiB slice，不扩大文件读入上限。

图片失败原先会跟随同一个预览组件进入下一个附件；现在正文按附件 ID 隔离，
图片失败和本地 URL 进一步按 File 重置。File 替换时先同步清空旧 URL，旧
Object URL 在替换/卸载时释放。Session 变化同步清空临时选择，移除当前附件
也清空选择，之后恢复同一草稿不会自动重开预览；这些变化不移除草稿附件本身。
文本读取保留取消过期提交，加载状态按 File 同步重置。

新增/扩展 10 项回归，附件与目录共三个文件 12 项通过，日志
/tmp/nexus-attachments-a62-target.log；最初的四项失败记录于
/tmp/nexus-attachments-a62-repro.log。覆盖图片失败切换、并存弹窗名称、
有界文本读取及迟到成功/失败、具名滚动、预览/移除独立、Session/移除后的
预览关闭、URL 释放、精确路径动作、禁用与安全 reload。File 读入、图片事件
和目录控制器为离线夹具，未执行真实图片解码、原生目录选择或视觉验收。
所有权门禁约束普通 Chip、单一预览 Header 与公共源码/视口消费。

同时完成 TextFileEditor 入口的代码审查：exact Agent/path 控制器、模式投影、
Header/Body/Reliability 组合及超限 Range 路由职责清晰，予以保留。入口继续只
传递真实 revision、草稿、保存/对账忙碌和故障事实；已有 workspace-text-editor-scope.test.mjs
合同与视图控制回归提供支持，本轮没有改写保存控制器、传输或编辑行为。

清单仍为 485 项：296 pending、119 in_progress、19 retained、45 improved、
6 removed。四个入口/视图完成本轮代码/行为审查；三个变更源码摘要已同步，
保留项与其余存活摘要一致。公共组件仍为 118 项，整体 Goal 继续，视觉与宿主
验收仍按用户要求暂停。

验证：npm run check 通过，含 lint、typecheck、478 项合同、222 个文件的
800 项测试及生产 build，日志 /tmp/nexus-attachments-a62-check.log。类型门禁
先纠正了 Badge tone 枚举用法；最终构建仍仅有既有大型分块提示。未启动浏览器、
原生宿主或产品服务。


## A63 — Composer 底栏状态、动作和恢复入口（代码/行为）

完成 Footer、Actions、Status、Metadata、Context Usage、Session Settings Reliability
六个视图的代码与行为审查，提交按钮保留。普通三列/居中品牌、Goal 下一行状态、
发送/停止 32px 几何与停止优先级符合现有任务分工，不为统一而改写布局或命令。
底栏状态/计数/品牌与上下文详情消费共享 Typography；长 Agent 名称与状态可换行，
没有计数时不保留空 flex 容器。移除三个无独立职责的状态包装与双份失败渲染/重试分支。

发现 Goal 菜单行内嵌套原生 Switch 按钮：引入公共 Action Menu checked 契约，由
MenuActionRow 投影 menuitemcheckbox/aria-checked，共享 keyboard 遍历普通动作和
勾选项。Goal/Connector 移除私有开关、传播拦截和勾选图标，整行只触发一次命令。
契约参考 [WAI-ARIA Menu Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/menubar/)
的勾选项语义及激活后关闭行为；项目既有禁用项跳过和 Tab 焦点合同继续保留。
MenuActionRow 本轮代码审查收口；Action Menu 仍保持 in_progress，长说明溢出和
其余消费者不能因本次新增 checked 就被当作已经全部审查。

Context Usage 原先 focus/hover 先打开、click 随即反转关闭；修改为幂等打开，
鼠标离开也不关闭仍有键盘焦点的详情，可用快照消失后不恢复旧打开态。保留唯一
详情、共享宽高上限、固定组标题与列表内部滚动，不恢复猜测行高或第二份 Tooltip。
新增菜单/上下文用例在修复前共出现 3 项失败，日志 /tmp/nexus-footer-a63-repro.log；
快照消失/恢复用例也补齐了已有关闭行为的回归保护。

目录新增入口补用已有 blocksMutation，保持与目录 Chip 相同的权威锁；不改变保存
控制器。设置错误继续 mutation 优先，然后 Session/Provider/Connector 读取，只显示
一个 ResourceState。未知写入的显式核对按 settingsLoading 防重，busy 写锁本身不能
关闭恢复入口；不把读取重试当成 mutation 成功，也不展示无实际命令的重试动作。
新增/扩展真实菜单与反馈面测试，涵盖单次键盘/点击切换、精确 Connector、Goal
禁用、目录锁、恢复路由/忙碌/失败优先级及停止优先；共享类型完整的控制器夹具
消除测试重复装配。追加公共 owner 门禁防止 Footer 再引入私有字号或嵌套开关。

清单仍为 485 项：290 pending、118 in_progress、20 retained、51 improved、
6 removed。公共 UI 仍为 118 项；六个 Footer 视图改善、提交入口保留，公共菜单
增量审查记录及生产源码摘要同步。整体 Goal 继续，视觉/原生宿主按用户要求暂停。

验证：npm run check 全部通过，含 lint、typecheck、479 项合同、223 个文件的
813 项测试及生产 build，日志 /tmp/nexus-footer-a63-check.log；本轮新增 13 项
行为测试和 1 项所有权合同。首次类型检查纠正了 Typography 参数形状，最终无新增
lint 警告；构建仍仅有既有大型分块提示。未启动浏览器、原生宿主或产品服务。


## A64 — 待发送队列的共享折叠、键盘排序与派发边界（代码/行为）

完成 ComposerPendingQueue 和 PendingQueueItem 的代码/行为审查。原私有折叠头、
箭头和控制器折叠状态迁移到公共 Disclosure；标题使用 caption，正文 supporting，
保留有界列表、微型引导/删除入口和临时拖动目标标记。有序列表与每行内容描述
建立语义关系，长正文/附件名称提供完整 title，无内容时有可识别占位。排序手柄
独立于可选正文，既支持原生拖动，也可用共享 Action Menu 的上移/下移操作。
首尾禁用无效方向，没有相邻项或派发中关闭菜单，恢复后不重新打开旧菜单。

控制器检查发现：同位/目标消失拖放仍派发原顺序、引导保护依赖下一次 render
才生效、删除/重排没有相同的 Promise 边界，以及非队列拖动和到达滚动边界后
仍会申请动画帧。统一当前 IDs 的非空重排与同步 ref 派发保护，引导/删除/重排
都终止旧拖动；拒绝的 Promise 只收口，不重复创建错误面或自动重试。Conversation
原有 sendInputQueueCommand/sendConversationCommand 仍拥有连接失败投影，Promise
完成只表示本地命令派发阶段结束，不能据此推断后端受理。顺序继续使用服务端
props，不建立私有乐观排序或持久 mutation journal。

边缘滚动只接受当前队列真实拖动，并在可滚范围内夹紧；到边界/中间、源条目
消失、折叠、拖动结束和卸载时终止。ComposerPanel 通过完整 draftScopeKey 为
队列实例建立 Session 边界；本轮只审查该连接，Panel 继续 in_progress。清理
PendingQueueContentCandidate 和无必要布尔数组/类型断言、私有 Header，以及两条
仅由已移除折叠按钮使用的中英文文案。没有新增公共组件或另一套重排控件。

新增 5 项控制器用例在基线上全部失败（其中相邻移动为新增能力），日志
/tmp/nexus-queue-a64-repro.log。随后补齐 7 项真实组件回归，覆盖折叠、有序列表、
键盘菜单、精确 ID、有效原生拖放、内容描述/附件优先级、派发禁用和菜单恢复。
Session key 由实际 ComposerPanel 装配及所有权合同约束；未声称已经覆盖完整
Composer 端到端流程或原生浏览器拖动手感。

清单仍为 485 项：288 pending、118 in_progress、20 retained、53 improved、
6 removed；两个 Queue 视图改善，ComposerPanel 增量记录和三份生产 TSX 摘要
同步，公共 UI 仍为 118 项。整体 Goal 继续，视觉与宿主验收按用户要求暂停。

验证：npm run check 全部通过，含 lint、typecheck、480 项合同、225 个文件的
825 项测试与生产 build，日志 /tmp/nexus-queue-a64-check.log。本轮新增 12 项
行为测试和 1 项所有权合同；修正测试 focus 的 act 包装后最终无新增 React/ESLint
警告。构建仍只有既有大型分块提示，未启动浏览器、原生宿主或产品服务。


## A65 — 人工问答的请求隔离与统一决定动作（代码/行为）

审查 Composer 人工介入入口及四个问答视图。问题正文、选项和说明分别使用
body/control/supporting；长摘要允许换行，单选 name 按实例隔离，多选删除重复
勾选标记。自定义答案复用共享文本域测高，条件挂载由输入自身接入生命周期。
拒绝/提交通过领域配方显式保留触屏命中区，移除根容器对全部后代按钮的尺寸覆盖；
权限视图沿用同一配方和已有 SplitButton 几何，没有新增公共控件。

移除 PendingHumanQuestion 从未使用的 toolUse 覆盖入口，精确 request_id 继续
拥有草稿身份，后到 tool ID 不重置用户答案。提交期间冻结答案；同步异常释放派发
保护并保留草稿，业务 transport 仍是唯一错误投影，不自动重试或伪造受理。
异步提交控制器移除重复事务门面，以每次 scope 生命周期对象隔离 Promise；修复
React effect 重放后不再活跃，以及 A→B→A 旧请求阻塞新请求/迟到完成的问题。
StrictMode 用于证明生命周期稳健性，不声称当前产品根入口已经启用 StrictMode。

新增 4 项控制器和 3 项真实 Composer 集成用例，另增 1 项提交冻结回归：覆盖
同轮防重、false/异常保留、卸载、旧结果隔离、后到工具身份、精确拒绝和多个
表单的 radio 互不影响。控制器修复前 2 失败/2 通过，见
/tmp/nexus-question-a65-repro.log。初次集成测试纠正了自定义答案的既有协议形状，
冻结测试使用原生 fieldset 的 :disabled 语义，最终所有行为通过。

清单仍为 485 项：283 pending、118 in_progress、20 retained、58 improved、
6 removed；五个待审视图改善，权限视图增量记录和生产摘要同步。公共 UI 118 项，
整体 Goal 继续；视觉/宿主验收按用户要求暂停。

验证：npm run check 全部通过，含 lint、typecheck、480 项合同、227 个文件的
833 项测试及 build；日志 /tmp/nexus-question-a65-check.log。构建只有既有大型
分块提示，未启动产品服务、浏览器或原生宿主。


## A66 — 模型公式兼容、源文保护与流式边界（代码/行为）

响应 [Issue #262](https://github.com/nexus-research-lab/nexus/issues/262) 的公式渲染
反馈。截图中的科学记号裸露与未识别 LaTeX 括号分隔一致，但 issue 未提供原始
消息，因此不把截图推断当作原始输入证据。以明确的模型输出构造回归，新增共享
micromark text/flow 扩展支持 `\(...\)`、`\[...\]`；已有美元分隔继续使用
remark-math，行内双美元也投影为显示公式。没有修改模型提示、历史内容或服务端。

解析按真实 Markdown token 处理，代码、转义字符、链接目标由原解析器隔离。
列表/引用保持所属容器；块级分隔独占行，可包含空行。流式分块保留完整公式，
终态继续复用原起点和 KaTeX 节点。未闭合括号/美元块保留源文；错误公式沿用
rehype-katex 的局部降级，trust 显式关闭。块级闭符后混入同一行正文属于无法可靠
收口的格式，保留原文，不以迟到 tokenizer 回退破坏容器。普通括号/裸 LaTeX
不靠语义猜测，数字美元符号的歧义邻接形式也保持原文，不修补数学内容。

预处理单独取得保守保护范围，避免原有标识符星号转义、文件链接和 URL 尾部修复
改写公式；该范围只阻止改写，不成为第二份渲染语法。显示公式由唯一主题配方
提供横向滚动、最小内容宽度、上下余量和可见键盘焦点；错误/未闭合源文可换行。
保留数学字体与上下标比例，不缩小公式以塞入正文，不在插件中添加字号/行距设置。

19 项共享公式回归加 1 项真实 Conversation 入口回归覆盖科学记号、现有美元语法、
嵌套容器、代码/链接/转义、价格、文件预处理、逐字符未闭合、错误/不可信 LaTeX
及流式空行/终态节点稳定。初始 12 项用例中 10 失败、2 通过，见
/tmp/nexus-math-262-repro.log；其中包含本次新增兼容与可聚焦视口要求，并非十个
独立生产故障。依据 [micromark 扩展接口](https://github.com/micromark/micromark#creating-a-micromark-extension)
和 [KaTeX 配置](https://katex.org/docs/options.html) 实现；依赖只显式登记已存在的
字符工具与开发类型，两份锁文件未升级已有运行依赖。pnpm 锁在独立临时目录校验，
未重建工作区 node_modules。

清单仍为 485 项：282 pending、119 in_progress、20 retained、58 improved、
6 removed。共享解析配置进入增量审查，其他 Markdown 页面不能据此宣称整体完成；
公共 UI 仍为 118 项，视觉/宿主验收继续暂停，整体 Goal 继续。

验证：与 A67 导航名称修复一起运行 npm run check，lint、typecheck、480 项合同、
228 个文件的 854 项测试及 build 全部通过，日志 /tmp/nexus-math-navigator-check.log。
公式定向 19 项全部通过 /tmp/nexus-math-262-final-target.log。构建仅保留既有大型
分块提示；未启动产品服务、浏览器或原生宿主，未向 issue 发评论或关闭 issue。


## A67 — 会话预览卡的名称来源与 ID 兜底（代码/行为）

用户截图中的 `用户 · Agent 05d838` 来自 Session Navigator 的讲者摘要。
定位发现 DM/Room 两个投影都通过共享 Frame 创建 navigator，但 Frame 未传入
agentNameMap；缺少名称时视觉模型又把 ID 前六位拼到 Agent 后。补齐共享 Frame
的必传名称目录参数，DM 复用当前身份名称表，Room 直接使用现有目录 names，
并将缺失/空白名称改为中英文通称。名称变更继续随当前 props 投影，不持久化
旧目录或私自读取全局 Store；原 round 身份、点击目标、稳定配色均保留。

新增真实预览 DOM 回归覆盖无名称、后到名称和空白名称；新增用例在基线失败，
见 /tmp/nexus-navigator-id-repro.log，修复后两项视图用例均通过。共享 Frame 的
必传类型同时约束两条生产装配入口，避免仅替换兜底却继续丢失真实名称。

与 A66 合并执行代码门禁：lint、typecheck、480 项合同、228 文件的 854 项测试
及 build 均通过，见 /tmp/nexus-math-navigator-check.log；此后仅补充注释和文档，
不重复跑整套测试。新增 20 项公式与 1 项导航用例合计使 833 增至 854。
清单状态为 281 pending、120 in_progress、20 retained、58 improved、6 removed，
公共 UI 118 项；导航视图进入增量审查并追加输入链记录，整体 Goal 继续，视觉验收继续暂停。

后续定向代码审查线索（尚未修复，不能将本次结论扩大为全前端 ID 清理）：Thread
面板、私域姓名/头像、Execution 子智能体标签、Skill 部署失败与 Channels/Automation
资源候选仍存在以 agent_id 作为显示兜底的分支。按各自领域名称权威和错误语义
继续审查，不通过全局字符串替换破坏真实资源身份。

## A68 — 统一 Agent 展示姓名与身份边界（代码/行为）

接续 A67，新增无状态 `lib/agent-display-name.ts` 作为缺失姓名的唯一通称所有者，
只接受真实姓名、当前翻译能力和 Agent/Subagent 角色，不接受 ID。导航摘要迁入
此所有者并删除原局部翻译键。私域线程标题、头像可访问名称、事件来源/收件人，
Room Thread 面板以及 Execution 负责人/子智能体标签都使用同一规则。

所有目录、事件、文件能力与头像种子继续使用精确身份；没有全局清洗正文。
私域“自己”直接比较事件 source 与当前 Agent，目录缺项也不退回 ID。Thread 的
翻译函数成为必传输入，语言切换参与 hook 投影。Execution 保留无负责人和缺少
姓名的区别，且记录通称来源，不能把“智能体”通称误当真实姓名而裁掉目标前缀。

新增 11 项领域/DOM 用例覆盖中英文、空白/后到名称、自己、权限目标隔离、原始
正文与 source 不变、头像种子稳定，以及通称不能裁切目标。相关 TSX 仅按名称
分支进入增量审查，不把大型时间线或画布标为完成。

A67 的 Thread/私域/Execution 线索在上述范围内已处理；Group round-card 名称、
Skill 部署失败、Channels 分组与 Automation 候选项仍待按各领域语义继续审查。
尤其资源选择必须能辨认候选，不能简单地把所有选项换成相同通称后宣称完成。

## A69 — 目录公式摘要与正文排版分离（代码/行为）

用户截图显示聊天侧栏的显示公式侵入标题及相邻条目。原因是共享 summary 虽有
内联组件表，却仍调用正文 KaTeX 插件，生成了不受该组件表约束的公式排版树。
summary 现在复用现有数学解析结果，通过 `remarkMathSummary` 将已识别公式
和显式 math fence 投影为普通文本标记，并跳过 KaTeX；聊天侧栏与私域目录传入
当前语言的 `[公式]` / `[Formula]`，保留前后文字与原始摘要。

完整、错误和已识别的未闭合公式都走同一摘要规则。价格歧义保存原文证据并在
摘要中恢复纯文本，不继承正文 pending 的换行样式。普通括号、代码示例不被
当成公式；正文的完整公式、错误降级与滚动视口保持原规则。没有用 CSS 隐藏
已生成的公式，也没有新增一套正则匹配整段摘要。

新增 12 项共享摘要和 2 项真实 Sidebar 入口用例，覆盖多种分隔、未闭合/错误、
显式 math fence、价格/代码、语言/正文切换及原行导航。基线的 14 项新用例失败，
见 /tmp/nexus-sidebar-math-repro.log；其中包含预期的新摘要合同，不代表十四个
独立生产故障。首次修复后调整了测试对既有段落间文本空白的错误假设；没有为
满足测试删除摘要的段落边界空白。

A68/A69 合并定向 50 项回归通过，首次整套门禁通过 lint、typecheck、480 项合同、
233 文件的 879 项组件/模型测试及 build，见 /tmp/nexus-a68-a69-check.log。
随后收口价格摘要的正文样式继承，再做最终门禁，结果登记于本节末。

清单仍为 485 项：274 pending、127 in_progress、20 retained、58 improved、
6 removed；7 个消费者进入增量审查并更新源文件哈希，未扩大为整体组件完成。
公共 UI 仍为 118 项。整体 Goal 继续，视觉/浏览器/原生宿主验收继续按用户要求
暂停；本批只做代码与离线 DOM/合同测试，且仅本地提交、不推送。

最终验证：/tmp/nexus-a68-a69-final-check.log 的 lint、typecheck、480 项合同、
233 文件的 879 项测试与 build 全部通过。价格分支收口后的 33 项定向回归及
typecheck 也通过，见 /tmp/nexus-a69-final-target.log 与 /tmp/nexus-a69-final-types.log。
构建只保留已有大型分块提示；未启动产品服务、浏览器或原生宿主。

## A70 — 群聊归组去冗余与名称显示边界（代码/行为）

上一批本地提交 `3e4cd72bb` 为已验证进展；本批从干净工作区接续名称与所有权
审查。发现 `GroupRoundCardModel` 仍计算 `completedEntries` / `pendingEntries`，
但基线 `git grep` 在整个 web 中只找到模型自身的定义、计算和返回，没有消费者。
删除这两组数组、第二次 `buildRoomAgentRoundEntries` 以及只服务旧兼容结果的
单目标回复过滤；真实 `entries`、guided user 归属、权限过滤及排序算法保持。

结构归组同时移除姓名/头像字段。Feed 不再伪造空目录，GroupRoundCardGroup 的
结构 memo 不再依赖名称；GroupAgentReply 接收必传的完整当前目录，在显示时
用共享姓名所有者产生真实名称或当前语言通称。两个薄装配文件完整审查后保留
其职责：一个拥有 root/guided user 顺序、稳定 entry key 与精确 Thread/停止动作，
另一个将结构 entry 绑定到唯一 MessageItem 执行壳；不重复基础视觉或时间线状态机。
用户消息分隔、内容列、边界提示与字段能力均保留，未为名称变化重建 shell。

配对分组优先采用当前目录的非空名称，缺项时保留响应中的历史名称，最终使用
共享通称；重新分组依赖当前语言。既有配对对象、ID 筛选、更新与外部标识不变。
Skill 部署失败反馈也复用该通称，保留准确失败数、前三名与剩余提示，不回显
runtime 错误；它仅由技能操作控制器使用，因此从 detail 归入 controller 并移除
反向依赖。没有更改技能更新/部署事务或用户操作的结果语义。

新增 6 项中英文回归：真实卡片在缺名/改名/切换语言后仍保留 shell、精确两个
执行轮的 Thread/停止目标；配对名称优先级不改变分组身份；Skill 反馈保留数量与
部分成功。定向通过见 /tmp/nexus-a70-target.log。首次整套门禁暴露旧 JS 渲染夹具
未提供类型已要求的 `stoppingAgentRoundIds`，此前被视图伪缺省掩盖；生产投影
已从 conversation 传入真实数组。补齐三处旧夹具的空数组，不恢复生产假可选合同，
也未删除任何原有断言。最终结果记录于本节末。

清单为 485 项：272 pending、127 in_progress、60 improved、20 retained、6 removed；
两个完整审查的薄视图记为代码层 improved，全部存量源哈希核对，公共 UI 仍为
118 项。视觉/浏览器/原生宿主验收继续暂停，整体 Goal 仍在推进；本批只本地提交。
下一处明确待审：Pairing 与 Automation 的 Agent 选择器仍直接构建姓名选项，
缺名或同名候选的辨认、绑定丢失后的展示和统一选择器所有权需要一并处理，不能
仅用通称覆盖选项后宣称全前端名称问题完成。

最终验证：/tmp/nexus-a70-final-check.log 的 lint、typecheck、480 项合同、236 文件
的 885 项组件/模型测试及 build 全部通过；原有公开顺序、guide 归属、精确停止、
Composer 唯一人工介入及流式外壳断言均保留。构建仅有既有大型分块提示。
未运行视觉验证、产品服务或原生宿主；无后端改动，也未推送远端。

## A71 — Agent 选择文字与缺项绑定（代码/行为）

上一轮 `54df178af` 已提交且工作区干净，属于已验证进展。本轮接续 Pairing 与
Automation 选择器：确认 Agent 名称预检明确允许重名；现有菜单只显示 name，
Automation 若缺名还回退内部 ID。另发现 Pairing 创建在当前 Agent 离开目录时
会自动把草稿改为第一项，这会将原选择误转给另一对象。

新增纯 `lib/agent-selection-options.ts`，复用共享姓名通称并统一同名/缺名选项。
完整目录中的同名组按创建时间排序，同时间/缺时间使用稳定 ID 比较；显示为
`1 · Nova` / `2 · Nova`，前置序号在窄控件截断时仍可见。它只是当前目录中的
辨认辅助，新增/删除/改名或语言改变时可以更新，不持久化，不代替名称或 ID。
调用方原候选顺序保持；Room 子集传入完整目录作上下文，筛选不重新编号，目录
中的其他成员不能因此成为当前 conversation 的可选对象。

Pairing 创建/列表改绑/筛选与 Automation Agent/Room 成员选项均使用此所有者，
删除仅转发的 `buildAgentOptions`。表单内部会话名称索引也使用同一显示标签，
Session key、Room default host、暂停过滤、执行与投递身份保持原领域规则。
当前绑定未出现在候选中时只加一项禁用显示，value 仍是原 ID，不能变成空选择
或默认房主。该显示项不进入资源真相或资格计算，服务器验证与既有提交规则不变。

Pairing 创建仅在没有初始选择时沿用首项默认；已有选择缺项时保留外部对象、
路由等草稿并禁止提交，等待目录恢复或用户主动重选。恢复或排序变化继续使用
原选中 ID。标题/外部对象字段的固定 DOM ID 同时迁为 useId，两个实例的标签
不会定位到另一个表单。菜单、字段和浮层继续复用已有公共组件，没有新建包装层。

新增 9 项回归覆盖中英文同名/缺名、目录重排与成员子集、缺项显示和恢复、真实
创建表单防止自动改绑/强制 submit、真实列表更新 ID、基础与 Room 表单保留绑定，
以及两个配对弹窗的标签归属。已有 Room 资源合同补充同名目录、非成员排除和
无有效 host Session 时不能默认执行的断言，没有删除旧断言。

定向 14 项（增加最后一项实例标签回归前）通过 /tmp/nexus-a71-target.log。
最终 npm run check 的 lint、typecheck、480 项合同、238 文件的 894 项组件/模型
测试及 build 全部通过，见 /tmp/nexus-a71-check.log。收尾复查补充目录空白姓名
不能覆盖当前领域已知姓名的边界及断言，最后再运行完整门禁，结果记录于本节末。
构建保留已有大型分块提示，未做视觉/浏览器/宿主验收，未启动产品服务或改后端。

清单仍为 485 项：272 pending、127 in_progress、60 improved、20 retained、6 removed，
公共 UI 118 项。四个已有 in_progress 消费者追加本次证据并核对全部存量源哈希；
这些页面尚有其他字段/文案和完整视觉范围，不能因一个选择规则收口而标记全页完成。
整体 Goal 继续；本轮仅本地提交。下一项明确线索是共享 Select Menu 的字号/徽标
仍有局部 recipe（特别是 select-menu-view 的 9px badge），以及 Room 选择标签的
内部 ID 兜底，需继续按公共所有者和对应资源语义审查。

最后验证：/tmp/nexus-a71-final-check.log 的 lint、typecheck、480 项合同、238 文件
的 894 项测试与 build 全部通过。目录空白名称不会压掉当前领域已知名称的
补充断言也通过；其后仅更新文档与检查清单。工作保持本地，不推送。

## A72 — 公共单选菜单的徽标、排版与输入法边界

复查确认 A69 已将主侧栏和私域目录的公式摘要收为本地化文本标记，正文仍使用
完整公式渲染；本批继续处理 A71 记录的共享菜单线索。

Select 当前值及选项原有 9px 徽标重复维护圆角、颜色与内边距，现直接组合公共
UiBadge 的 xs / primary，删除私有 SelectMenuOptionBadge。徽标使用既有 10px
紧凑阶梯和可读品牌前景。各尺寸文字通过 App Typography 投影：原触发器
12/13/14px 与选项 12/13px 字号保持，字号及行高不再由菜单分别拼装；单行固定
高度、允许换行时的自动高度、菜单条目间距及浮层几何保持其既有所有者。

检索覆盖 UiSelectMenu 的设置、联系人身份、记忆、定时任务、配对、联络、
引导及目录筛选消费者；徽标实际承载定时任务 Session 的 Room/IM 来源。没有
修改这些消费者的选项、值或命令。生命周期 Hook 的直接消费者为单选、Room
技能多选和 Room 历史。补齐 defaultPrevented 与共享 isImeKeyboardEvent 边界，
输入法组合或旧式 229 键码不再误执行 Enter/Space 开关或上下键选值；普通键盘
操作继续走原流程，不改写重复选值或业务保存语义。

新增 3 项真实菜单 DOM 回归覆盖输入法、229 和已处理事件的关闭/打开两种状态，
并证明普通方向键、Enter 与焦点路径仍有效；既有选择测试同时验证带来源徽标的
选项选择后在触发器继续展示。架构合同补充公共 Badge/排版所有者约束。
定向 4 文件 24 项测试通过，见 /tmp/nexus-a72-target.log。完整 npm run check 的
lint、typecheck、480 项合同、238 文件的 897 项组件/模型测试与 build 全部通过，
见 /tmp/nexus-a72-check.log；构建仅保留既有大分块提示。

清单仍为 485 项：272 pending、127 in_progress、60 improved、20 retained、6 removed；
公共 UI 118 项。菜单视图保留 in_progress，追加证据并核对其余存量文件哈希；
不以局部 recipe 收敛代替完整菜单与消费者审查。未执行视觉、浏览器或宿主验收，
未启动产品服务、改后端或推送。整体 Goal 继续。

## A73 — 定时任务选择文字与目录缺项状态

复查资源模型发现 Room 目标、执行/投递会话仍在缺名时回退到内部 ID；Room 与
Session 的已选值缺项时又落为“请选择”，不能表达保留中的绑定。当前会话字段
始终位于选定的 Agent/Room 下方，因此去掉重复父名称，仅显示标题和真实渠道
徽标；同名会话按当前候选的精确 key 添加前置序号。此简化同时删除两套父名称
索引、为旧统一函数签名传递的无用参数/分发表，以及时间模型中的会话格式函数。
DM 执行与投递的选项展示改为同一个内部 helper；资格过滤、共享 Room 会话去重、
成员/房主解析、精确 Session 路由和提交行为保持原所有者。

Agent 既有排序/缺项算法提升至 shared/lib/selection-options.ts，Agent 适配及
Automation Room/Session 文字复用它，不复制第三套编号算法。Room 使用完整真实
群聊目录生成标签后才按执行资格筛选，缺名使用本地化未命名群聊，DM-backed
Room 不参与编号；普通唯一名称与候选顺序不变，显示序号不持久化。表单的
执行/投递目标及会话统一把缺项当前值显示为禁用项，“不在列表中”不推断删除，
也不把显示项加入资源候选或自动清空/改绑；恢复后原 value 找回原对象。

新增 8 项回归：6 项双语资源模型验证同名/缺名、目录重排、暂停与 DM 排除、
相同会话标题、缺失父名称、Room 共享路由和来源徽标；2 项真实基础/高级表单
覆盖 Agent/Room 场景的加载、失败、缺项与恢复，确认无自动动作且重选仍发送
精确原始 value。既有 Agent 适配测试保留，证明通用算法迁移不改变成员子集、
目录优先级或缺项语义；既有 Node 场景只更新标签/API，保留全部权限和路由断言。
定向 3 文件 16 项测试与 typecheck 通过，见 /tmp/nexus-a73-target.log 与
/tmp/nexus-a73-types.log。首轮完整门禁在两处旧 Node 翻译替身失败：它们只实现了
带参数的编号文案，无法响应通用算法先读取回退名称的合法调用；两处已改为
真实词条翻译，原资格/路由断言保留，见 /tmp/nexus-a73-check.log。最终完整
npm run check 的 lint、typecheck、480 项合同、239 文件的 905 项组件/模型测试
及 build 全部通过，见 /tmp/nexus-a73-final-check.log；构建保留既有大分块提示。

485 项清单仍为 272 pending、127 in_progress、60 improved、20 retained、6 removed；
公共 UI 118 项。本批没有新建 TSX/控件，新增中立纯算法与领域文字适配，两个表单
视图追加下游模型证据，所有存量源哈希已核对。整体 Goal 继续；视觉/浏览器/宿主
验收仍暂停，未改后端或启动产品服务，保持本地提交、不推送。

## A74 — 历史产物归属与公共文件动作反馈

从定时任务看板/运行记录继续审查，发现历史结果 Markdown 已按 run Session
绑定执行者，但文件按钮仍读 task.agent_id，任务改绑后会把旧产物指到另一工作区。
现文件按钮与正文共用 getRunWorkspaceAgentID；缺失、非法或 Room shared Session
无法证明执行 Agent 时，保留原生禁用按钮和关联的本地化说明，不回退任务当前
Agent 或全局选择。复制诊断也明确区分当前任务 Agent/执行类型与历史 Run Agent。

文件预览已有完整失败反馈，而正文产物与历史文件按钮只有 catch 日志。将已验证
的 owner/文件/最近操作栅栏提升到 hooks/agent/use-workspace-file-external-action.ts，
三个现有按钮适配器共同消费。历史额外传入 Job/Run 来源，即使两次运行指向同一
Agent 和文件也不能串入旧失败。所有失败复用公共 FeedbackBanner 和现有双语文案；
切页/换账号/卸载只丢弃迟到反馈，不取消或重放文件动作。目录文件树的业务命令
仍归目录控制器。顺带删除历史动作中仅转发公共 Button 属性的 RunActionButton。

将既有 9 项文件操作回归同时运行于文件预览和正文产物，保留所有 StrictMode、
账户代次、当前语言、最近请求、文件切换、卸载与显式重试断言；新增历史测试
覆盖 Web/桌面动作、任务改绑仍用历史 Agent、三种不明确归属、同文件跨 Run
迟到结果与诊断身份区分。定向 3 文件 28 项测试通过（增加最后一项诊断测试前），
见 /tmp/nexus-a74-target.log。新增架构合同禁止三处消费者重新直接调用下载 API
或读取当前 Agent，命令/反馈只由公共 Hook 拥有。最终 npm run check 的 lint、
typecheck、481 项合同、239 文件的 921 项组件/模型测试与 build 全部通过，见
/tmp/nexus-a74-check.log；构建仅保留既有大分块提示。

485 项清单更新为 270 pending、128 in_progress、61 improved、20 retained、6 removed；
公共 UI 118 项。正文产物动作这一薄适配器已完成代码/行为审查，文件预览继续保持
improved；历史动作仅完成文件路径，运行状态/操作文案仍需继续，不标为全项完成。
本次还发现看板运行图标的 primary 语义、历史状态/确认文案与固定标题 ID 等线索，
留待对应完整业务切片继续处理。所有存量哈希已核对，视觉/浏览器/宿主验收暂停，
未改后端、未启动产品服务；只本地提交，整体 Goal 保持进行中。

## A75 — 运行历史语言、请求状态与弹窗命名

继续完成历史行与动作视图的代码审查。运行/投递/任务状态、时长、操作标签与说明、
诊断控件、日期缺省值和两个确认框改为读取当前语言；复制诊断使用当前语言格式化
日期和时长，技术字段名与原始输出继续保留。未知 wire 状态显示中立的本地化标签，
不显示内部原值，也不因此新增可执行动作。日期格式器接受显式 locale，保留既有
计划摘要的默认语言；零时间戳正常格式化，缺失、非有限或超出 Date 范围的值使用
缺省文案，避免一条异常记录使历史列表渲染失败。

动作模型将实际在途 busy 与原有 disabled 资格分开投影到 UiButton。未确认结果、
任务正在运行及删除保护继续按原有优先级禁用，待核对投递仍要求保存的精确次数，
释放占用与人工重投仍经过原确认流程。刷新只读取/对账，读取中标记 aria-busy 并
原生禁用，不能把一次点击变成重复 mutation。历史标题改为 UiDialogHeader.title
组合任务名和公共 Badge，删除固定标题 ID 和业务自行维护的 h2，自动使用公共实例
标题协议。语言切换保持 Disclosure 展开状态和嵌套确认的 exact run/投递 attempt。

新增 15 项行为回归，覆盖语言切换、三种动作的请求/未确认状态、两种删除保护、
缺少投递次数、未知状态与保留字段名、日期边界、多实例标题、刷新防重复以及真实
资源/命令 Hook 下两个确认流程的目标保持。原历史图片/文件归属、诊断、执行动作
和迟到反馈回归保留；Node 的单结果合同仅补传真实翻译函数。定向 2 文件 24 项测试
通过，见 /tmp/nexus-a75-target.log；无浏览器、视觉或宿主检查。

485 项清单更新为 270 pending、126 in_progress、63 improved、20 retained、6 removed；
公共 UI 118 项。历史行与动作视图已完成本批代码/行为审查，所有存量哈希已核对。
历史装配和详情仍为 in_progress：异步命令反馈目前仍保存已翻译文本，错误摘要
映射与详情异常 Panel 也留待继续整理；不把本批展示层改进称为整个历史业务完成。

最终 npm run check 的 lint、typecheck、481 项合同、240 文件的 936 项组件/模型测试
及 build 全部通过，见 /tmp/nexus-a75-check.log；构建仅保留既有大分块提示。本批只
修改前端、合同与审计文档，未改后端或启动产品服务；只本地提交，整体 Goal 继续。

## A76 — 历史异步反馈与进入代次

继续审查历史控制器，发现仅比较 owner+Job 字符串不足以区分离开后再返回同一
任务：旧命令仍可能发布反馈、发起旧刷新，并删除同名的新防重项。现每次进入
生成独立代次，公开动作验证当前 scope 和 run.job_id，旧完成只结束已发送命令，
不再写新弹窗状态或发起刷新；finally 只删除自己注册的 Promise。Promise 先注册
再执行，同步抛错也不会留下已经结束却仍被当作在途的防重项。原命令确认、真实
投递 attempt 与目录的持久未确认锁继续独立，不因本地 pending 清空而解除。

历史反馈改存 completed/refresh_failed/failed/blocked/clipboard 等结果事实，当前
语言由 scheduled-task-run-feedback-model.ts 在渲染时投影。删除冗余 RunActionCopy、
未被界面读取的内部 message 拼接和两条不会再使用的兜底词条。最近显式操作拥有
反馈，旧命令仍可独立结束其在途状态；命令已提交但刷新失败保持 warning，装配
明确把 warning tone 传给公共 ResourceState，避免此前统一 error 布局把它显示成
危险错误。复制成功/失败、FailureCore effect 与删除拦截均在切换语言后即时更新，
不能触发额外命令。输出模型从未产生 label，删除该字段与两套条件标题/间距分支；
异常输出保留原内容与 danger 正文，边界直接采用公共 Panel，不维护局部混色边框。

新增 16 项 Hook 回归和 1 项真实装配回归：覆盖语言切换期间/之后的完成、已提交但
读取失败、四种 effect 和未来 effect、同步抛错、Job/owner/关闭后原身份恢复、旧
回调、同名新请求防重、复制与命令竞争、剪贴板迟到、删除拦截及非法 scope；原有
历史归属与精确确认回归全部保留。定向 3 文件 41 项测试通过，见
/tmp/nexus-a76-target.log。Node 单结果合同只去掉已删除的 undefined label 字段。

485 项清单更新为 270 pending、125 in_progress、64 improved、20 retained、6 removed；
公共 UI 118 项。历史装配完成代码/行为审查，详情仍保留 in_progress：已确认
scheduled-task-error-copy.ts 的两个已知错误映射仍写死中文，看板消费者也需随业务
整理；历史内容页的权限恢复/错误重试控件仍待完整切片审查。所有源哈希已核对，
不把关联文件的局部改善计作完成。视觉、浏览器与宿主检查继续暂停。

最终 npm run check 的 lint、typecheck、481 项合同、241 文件的 953 项组件/模型测试
及 build 全部通过，见 /tmp/nexus-a76-check.log；仅保留既有大分块提示。本批未改
后端、未启动产品服务，只本地提交；整体 Goal 继续，不推送。

## A77 — 历史读取状态与可读错误摘要

完成历史内容状态和详情的代码/行为审查。读取失败的三个分支收为同一个公共
ResourceState 重试动作，busy 直接消费资源读取状态；尤其是权限失效后重读时，
错误面仍保留但重试按钮必须原生禁用，不能连续提交读取。真实资源 Hook 回归
证明 401/403 先清旧记录、恢复期间不闪回；普通读取失败保留原结果节点，重试
成功才替换。首次失败、恢复为空目录与语言切换也维持真实状态，重试没有运行或
投递副作用。列表追加混合记录回归，证明释放占用按 exact Run、投递锁按 Run 与
当前 delivery_attempts 隔离，不能误锁另一条或沿用旧 attempt。

错误文案继续由 scheduled-task-error-copy.ts 唯一拥有，两个既有精确错误映射
接入当前语言，未知错误改用中立摘要。卡片不再把第一行技术错误作为摘要，历史
主输出也不直接显示完整运行/投递错误；原文进入默认收起的诊断行，保留换行并
按原生文本渲染，复制诊断不改写原文。已知错误的技术原句仍附在诊断解释后，
看板注意事项中的显式诊断复用同一 owner。该函数只映射文案，不解释恢复资格或
副作用结果。现有权限、删除、运行状态和正常结果 Markdown 保持原职责。

新增 10 项回归：四项真实读取恢复、一次混合列表锁隔离、三个历史错误摘要/诊断
回归和两个卡片/诊断双语回归。覆盖未知错误中的内部 ID、路径、多行与脚本文本，
证明普通摘要不泄露这些详情，显式诊断和复制仍保留原始证据且不执行 HTML。
定向 3 文件 38 项测试通过，见 /tmp/nexus-a77-target.log。前两批历史身份、异步
反馈和确认回归继续保留。

485 项清单更新为 267 pending、126 in_progress、66 improved、20 retained、6 removed；
公共 UI 118 项。历史内容与详情完成代码/行为审查；看板卡片和注意事项弹窗只记录
本次错误文案切片，不把其余文案、时间、权限/删除分支和视觉 recipe 计作完成。
所有存量源哈希已核对，视觉、浏览器和宿主验证仍暂停。

最终 npm run check 的 lint、typecheck、481 项合同、241 文件的 963 项组件/模型测试
及 build 全部通过，见 /tmp/nexus-a77-check.log；构建仅保留既有大分块提示。本批未改
后端、未启动产品服务；只本地提交，不推送，整体 Goal 继续。


## A78 — Composer Agent 活动条比例与窄空间访问

按用户指出的小头像与外框比例问题，活动条通过共享 activity toolbar recipe
改为确定的 36px 外框：32px 命中区、上下各 1px 留白、1px 边框，左右仍各 4px。
不再用 32px 控件加上下 4px 留白、边框再叠加最小高度的方式推算外框。独立 Task
和 Room 协作状态继续使用原活动 chip 基线。工具条仅使用短轻投影，工作图图标
和分隔统一为 16px，按钮继续消费公共 ghost 交互；不新增页面独立 hover 配方。

Dock 的头像图像从 22px 增至 26px，中性框从 24px 增至 28px；右下状态点是唯一
活动色，去掉重复彩色描边与放大光环。运行仍为绿色、阻塞为警示、完成转回中性，
完整图的节点尺寸、状态框、当前/选中语义保持。按钮 Tooltip 唯一承载 Dock 说明，
子头像不再叠加原生 title；其他密度节点仍保留既有提示。

窄空间只允许头像集合横向滚动，右侧工作图入口独立、不收缩；没有头像时不保留
空集合或孤立分隔。原有一级节点去重、五人上限、顺序与精确 round 跳转由同一
模型继续负责，完整工作图仍承载更多节点，不扩展后端业务或制造第二套投影。
公共 IconButton 新增 focusInset，把被滚动裁剪的键盘焦点保持在命中区内，默认
外环不变；原生按钮不接收该样式属性。门禁最初发现页面局部 ring-inset 覆盖，
已改为公共属性投影，未放宽样式门禁或增加例外。

新增 8 项活动条行为回归，覆盖多人成组的 Tab/Enter 顺序、精确 round、两个导航
不可用分支、空集合、三种活动状态及完整图状态保持。公共按钮的现有 13 项回归
一同运行，最终定向 2 文件 22 项通过，见 /tmp/nexus-a78-target.log。空集合夹具
显式清空 Work Items，保留现有无 Graph 时从持久工作项投影节点的行为；测试数据
使用 satisfies 保持实际必需字段类型，不改变生产协议的可选性。

485 项清单更新为 266 pending、126 in_progress、67 improved、20 retained、6 removed；
公共 UI 仍为 118 项。活动条完成本批代码/行为审查，节点原语仅完成 Dock 切片，
其他密度与图形类型继续 in_progress。公共按钮保留原审计阶段并追加焦点所有权
证据。已核对本次三个生产 TSX 的源哈希；工作区另有 WorkGraph 保存流程对三个
TSX 的修改，不覆盖其清单证据，也不纳入本批提交。视觉、浏览器及宿主验证继续
按用户要求暂停；完整 Goal 仍有其他目录和领域切片待审查。

完整门禁使用基线 8c68a41b4 加本批精确改动的隔离快照，避免并行修改中的保存
对话框临时类型错误污染验收。隔离快照全部 485 项源哈希与清单一致；最终 lint、
typecheck、481 项合同、241 文件的 971 项组件/模型测试及 build 全部通过，见
/tmp/nexus-a78-check.log；仅保留既有大分块提示。快照位置与文件清单见
/tmp/nexus-a78-review.json。本批只提交活动条及其公共所有者、回归和规范，另批
保存流程的工作区修改继续保留；未运行 Go、产品服务或视觉验证，不推送。


## A79 — 工作图局部步骤与运行详情

局部任务清单原先从共享模型取得归一化计数，却用原始 run.todos 选取窗口并渲染。
现在整个视图消费同一份 taskState.todos，旧 task/activeForm、空内容和异常项继续
服从既有归一化所有者，不另写一份兼容 parser。标题/计数改用 metadata，任务
正文改用 supporting；移除局部 9/10px 字号与单行永久截断，状态图标进入等宽列，
长任务可换行，运行文案与 title 一致。默认仍展示当前步骤附近五项，通过公共
Button 展开全部/收起，原生序号保留在完整列表中的位置。展开只受当前 Agent 与
agent round 身份约束，语言与同轮数据更新不重置，多实例的 heading/list ID 独立。

运行详情原先将未知状态和缺失正文回退成 wire 状态或 Run ID，状态映射也会命中
对象原型键；现使用当前语言的可读空态，并用自有键检查拒绝原型继承。Canvas 的
Attempt 标签与 NodeRun 标签、节点与历史中重复的耗时函数统一进入
execution-run-presentation.ts；零耗时保留，负值/非有限值/不可安全表示的数值不
展示为时长，无合法时长时尝试有效的结束/开始时间。统一先舍入再分解分钟和秒，
不产生 1 分 60 秒；时间及单位随当前 UI 语言变化，既有展开选择不会因语言或
历史追加而关闭。运行 ID 仍作为精确数据键保留，不再冒充普通详情。

历史错误使用公共 InlineNotice 的 warning、静态 note 与 aria-live=off，替换私有
警示底色/边界。正文与错误保留换行，缺少摘要时独立错误码仍可查看，不以空详情
覆盖技术证据。文件引用的安全路径判断、原 workspace 参数及 structured Artifact
自身 owner 优先级保持；没有新增重试、恢复或其他业务命令。

新增 28 项回归：七项局部步骤行为，21 项运行详情行为；连同既有文件引用及真实
Canvas 检查器回归，定向 3 文件 30 项通过，见 /tmp/nexus-a79-target.log。覆盖旧
载荷归一化、键盘展开、实例/Agent/round 隔离、语言更新、原型键、空记录、零值、
亚秒与分钟进位、非法耗时/观测时间、历史展开保持、纯文本错误与 Artifact owner。

485 项清单更新为 265 pending、126 in_progress、68 improved、20 retained、6 removed；
公共 UI 仍为 118 项。局部任务清单完成代码/行为审查；运行详情和 Canvas 继续
in_progress，尤其文件原语缺少明确 workspace 身份时仍会回退当前 Agent，需沿
Artifact 业务继续审查，不能把本批文案与时间改进称为完整图形业务完成。另一批
WorkGraph 保存代码仍在工作区中，本批只记录并提交自身范围。视觉、浏览器和
宿主验证继续按用户要求暂停，整体 Goal 保持进行中。

并行保存流程已以 4f97994f8 合入，本批同步该提交涉及的三个既有 TSX 源哈希，
保留其 pending/in_progress 与待复查说明，不把外批代码计作本批审查完成。最终
校验快照基于此新提交并仅叠加本批改动；保存对话框仍在进行的后续修改未纳入。

最终验证记录见 /tmp/nexus-a79-verification.md。新基线的 lint 与 typecheck 通过；
482 项合同中 481 项首跑通过，execution-invalidation 测试进程收到 SIGBUS 后退出，
未报告断言失败；同一隔离测试单独复跑 1/1 通过，未修改测试或放宽门禁。余下
243 文件的 1007 项组件/模型测试与 build 均通过；构建只保留既有大分块提示。
具体日志分别为 /tmp/nexus-a79-check.log、/tmp/nexus-a79-invalidation-retry.log、
/tmp/nexus-a79-components.log 与 /tmp/nexus-a79-build.log。未运行 Go 或产品服务；
仅本地提交，不推送。


## A80 — 文件产物来源与阅读型卡片

文件原语过去在缺少 workspace Agent 时订阅全局当前 Agent，同时预览资格只检查
path/handler，外部下载却另外检查 Agent。现将两种动作的路径与来源资格收口到
同一纯投影，取消原语的全局选择订阅；缺失路径或来源时保留名称/目录，以当前
语言说明原因并通过 aria-describedby 关联到禁用的文件按钮，来源清空立即撤销
旧动作。没有预览 handler 时，独立下载/桌面定位仍可用。

结构化适配器成为 Artifact 自带非空 owner → 明确消息/节点 workspace → 无来源
的唯一解析入口。正文、归档收起过程、live 工具收起段和 NodeRun 历史都传入原
上下文；Room Thread 稳定回调修复仅传 path、丢掉 workspaceAgentId 的实际桥接
问题。文件原语不再自行猜测来源；Markdown 的既有上下文适配与子智能体宿主
绑定保持原合同，后者的完整 Thread 身份映射仍需随子智能体领域继续审查。

文件名使用 body/supporting，目录与集合标题使用 metadata，打开提示使用 caption；
两行复合文件按钮和图标框保留阅读型领域几何，材质继续由 content recipe 拥有。
删除重复解构别名、硬编码中英文标签和“没有预览 handler 就隐藏整个文件集合”
的分支；缺少 handler 不再隐藏用户已生成的文件证据。

新增 14 项回归，连同既有 Markdown、文件动作和运行历史，定向 5 文件 43 项通过，
见 /tmp/nexus-a80-target.log。覆盖四个真实消息/集合入口、Artifact/上下文优先级、
空白或缺失来源、选择切换、来源清空、当前语言、无预览时外部动作、稳定 Thread
回调更新/清理及 NodeRun 旧文件。类型检查发现一次上下文参数编辑重复，删除
重复属性后重新通过；没有改变业务类型以规避检查。

485 项清单现为 260 pending、129 in_progress、70 improved、20 retained、6 removed；
公共 UI 仍为 118 项。文件原语与结构化适配器完成本批代码/行为审查；其他消息
和图形文件只登记本批来源切片，保持进行中。同步外部已提交的 63f7afc07 保存
对话框源码哈希并保留待复查状态，不把其工作计为本批审查。视觉/浏览器/宿主
验证继续按用户要求暂停，未运行产品服务或 Go 测试，整体 Goal 保持进行中。

完整门禁在 63f7afc07 加本批精确改动的隔离快照通过：lint、typecheck、481 项合同、
245 文件的 1024 项组件/模型测试及 build 全部成功，见 /tmp/nexus-a80-check.log；
构建仅保留既有大分块提示。快照文件清单见 /tmp/nexus-a80-review.json，485 项
生产审计源哈希全部匹配。只本地提交，不推送。


## A81 — 子智能体详情与 Thread 产物链路

沿 A80 的后续审查确认：服务端 task.agent_id 为 runtime 子任务接收者身份，
host_agent_id 才是承载它的 Nexus Agent；transcript reader 将宿主 Agent 传入
消息投影。共享 Thread 过去将显式 null 工作区通过 ?? 回退为展示任务身份，
子任务预览闭包又覆盖文件已解析的 owner，导致预览与独立下载可能指向不同来源。
现仅在省略 workspace 参数时保留既有展示 Agent 默认值，明确空值维持未知；
预览回调完整透传文件来源，不额外改绑宿主。消息来源为空白时使用上游明确
workspace，不阻断合法来源，也不使用全局当前 Agent。

真实 MessageItem/Thread 测试揭示上游可见块注册表遗漏 workspace_file_artifact，
导致文件原语虽存在，独立文件回复却不显示。现保留该已知块，并把最终回复尾部
文件与相邻正文一起投影到稳定 final surface；Room 最终轮优先级接受无正文文件
交付，不能因此退回旧说明。完整 transcript 展示文件，过程检查器排除最终尾部
文件，但更早工具过程的文件仍保留且只显示一次。补齐当前消息规范和 L3 合同，
没有修改传输、持久化、模型输入或业务命令。

详情 Header 直接使用 UiSeededAvatar 的 xs/32px，移除从列表跨视图导入头像后
私自改为 28px 的覆盖。任务标题仍按 name/description/type 的原优先级，缺失时
与目录一起交给共享当前语言通称。底栏使用实体 panel token、单条分隔和 metadata，
去掉私有混色/模糊；说明与动作组可换行，Button 明确 aria-busy，停止确认、能力
门槛和未知停止防重维持原语义。加载/空记录改用公共 ResourceState；已有输出
按 code 保留字面换行，读取失败保留输出但不再同时声称暂无记录。

控制弹窗沿 controller 的 canonical sessionKey 使用同步 reset 原语，删除重复
作用域字符串和 effect 清空；继续保留 exact scope 确认守卫。实际共享 Confirm/
Prompt 验证取消不发送、确认只执行一次、任务/来源切换立即移除旧弹窗及草稿，
返回旧任务不恢复已清空的操作。

新增 24 项回归，定向 6 文件 27 项通过，见 /tmp/nexus-a81-target.log；类型检查
通过。完整链路回归推动修复了文件入口遗漏、空白来源遮挡已知 workspace，以及
只有文件的 Room 交付丢失，未用增加伪造正文或缩小断言绕过失败。中英文任务名、
两种布局的文件来源、Artifact owner 优先级、忙态、未确定停止、加载/读取失败与
保留输出均有证据；首次测试对刷新文案和列表同名标题的定位已按实际共享文案与
可操作任务行修正。没有进行浏览器/原生/视觉验证或运行产品服务。

485 项清单为 255 pending、132 in_progress、72 improved、20 retained、6 removed；
公共 UI 仍为 118 项。两份子任务详情/装配文件完成本批代码行为审查；列表和共享
Thread 仅登记本批切片，其他状态与滚动保持进行中。同步外部已提交 7b7d489aa 的
WorkGraph 编辑器哈希并保留待复查说明，不将外批业务计作本批完成。Goal 继续。

完整门禁在 7b7d489aa 加本批精确改动的隔离快照中通过：lint、typecheck、481 项
合同、250 文件的 1052 项组件/模型测试和 build 全部成功，见 /tmp/nexus-a81-check.log。
最后将三份纯模型的 L3 合同及新增 import 移到文件顶部，逻辑不变；最终快照再次
通过 lint、typecheck 和 build，见 /tmp/nexus-a81-final-check.log。构建只保留既有
大分块提示；快照清单见 /tmp/nexus-a81-review.json。485 项生产源哈希核对一致。
未执行 Go/产品服务/浏览器或宿主验证，只本地提交，不推送。


## A82 — 子智能体目录状态、时间与保留快照

目录完成本批代码与行为审查。旧模型将未知状态默认视为 pending，原 completed
分组同时装入失败/停止；现在把运行、历史、未知分开，排队、失败、停止使用公共
Badge，未知观察使用本地化中性文字而不回显 wire 值。具名 section、整行键盘
选择、公共 dense 密度及共享头像保持一致，移除无动作标题的右侧预留空隙。
初次加载仅保留一处 busy/loading；读取失败保留可读行，但不同时声称任务为空。
明确不支持观察的 runtime 继续服从原能力边界，不借缓存绕过。

状态与 runtime 别名只接受映射自身属性；未知不会当作 active 或终态。同时间
已知状态优先于未知，已知终态继续压过同时间 active，更晚的观察仍优先。能力
归一化保留缺失/null 的既有继承，对出现的非布尔值关闭该项，避免真值字符串
打开控制。详情的未知状态不再声称“继续已结束任务”，明确能力允许时仍可补充
指令；停止仍要求已知 active。审查消费者发现停止动作的 effect 将所有非 active
当成终态，现只允许已知终态清除未确定的停止结果，不重放任何命令。

观察时间只接受正值、有限且在日期范围内的数据；无效 updated_at 回退有效
started_at，否则隐藏时间。删除目录私有天/小时/分钟分支，复用共享相对时间的
compact 模式，保留默认秒精度语义。time 提供 ISO 日期和完整本地日期提示；
新增中立 useMinuteClock，仅可见时沿分钟边界更新，隐藏、停用和卸载释放计时器，
恢复前台立即同步。一个列表共用一个展示时钟，不请求任务、不改变分组或运行态。

新增 18 项回归，定向 6 文件 30 项通过，见 /tmp/nexus-a82-target.log；覆盖别名、
非法 capability、观测先后、未知/历史分组、快照保留、单一加载/失败反馈、中英文、
精确键盘选择、非法时间隐藏、完整日期、可见分钟更新和未确定停止结果。类型检查
指出旧 wire 缺字段夹具需明确经过 unknown，已在测试边界修正，未弱化生产类型。

485 项清单为 255 pending、131 in_progress、73 improved、20 retained、6 removed；
公共 UI 仍为 118 项。目录从进行中推进为本批代码/行为审查完成，详情追加未知状态
切片证据。没有进行视觉、浏览器或宿主验证，也没有运行产品服务或 Go 测试；整体
Goal 继续，后续页面和组件仍按原清单推进。本批只本地提交，不推送。

完整门禁在 0908fe23a 加本批 23 文件的隔离快照通过：lint、typecheck、481 项
合同、254 文件的 1070 项组件/模型测试及 build 全部成功，见 /tmp/nexus-a82-check.log；
快照清单见 /tmp/nexus-a82-review.json，485 项审计源记录一致。Lint 保留上一批即有的
WorkGraph metadata editor effect-ref 提示，构建保留既有大分块提示，均非本批新增。

首次完整门禁的 execution-invalidation 测试断言通过后进程以 SIGSEGV 退出；
独立复跑再次在断言后以 SIGBUS 退出，随后同一快照独立复跑和完整门禁均通过。
原日志见 /tmp/nexus-a82-check-first.log、/tmp/nexus-a82-invalidation.log，独立通过
见 /tmp/nexus-a82-invalidation-retry.log。未改变该测试、依赖、断言或跳过任何检查；
记录非确定性的进程退出问题，不将复跑成功描述为已修复其底层原因。


## A83 — 子任务定向导航、调用者选择与窄窗模态

审查共享子任务 Surface、Room 适配和窄窗挂载，复现旧成员数组每次 render 都触发
定向 effect，导致手动切换调用者立即被拉回；目录重排还会重置选择。现把请求与
手动选择分别保留在 exact source/request 生命周期内，请求只取得一次选择权，
手动选调用者/任务或返回会消耗未到达的旧目标；切换会话后永久失效，返回旧会话
也不复活。成员补载可完成尚未处理的新请求，目录重排/名字刷新不覆盖手动选择，
成员移除后提交有效回退，不在成员重新出现时跳回。

共享层保留 keyed source 资源卸载边界，caller 变化同步清空任务选择。省略/null
筛选仍保留 DM 及通用未筛选语义，Room 明确没有调用者时的空串不再放大成全部
任务；显式失去 observe 支持时目录和选中详情使用同一可见边界。任务 key 与
ToolUse identity 共同确认定向目标，不能通过名称或另一任务补位。真实测试发现
成功目录缺少所点任务时无刷新入口，现用公共 InlineNotice 提供一条说明与动作，
读取失败优先、刷新 busy、成功空态互斥，允许用户选其他任务来取消等待。

新增 Surface 内的焦点交接：进入详情聚焦返回动作，回目录聚焦原任务行，缺项时
退到可用动作/具名 Surface；状态刷新不重新聚焦。保持现有公共 Header、列表、
Thread 与文件来源链，三个装配层直接使用 WorkspaceFileOpenHandler，移除重复
签名。Thread 返回/关闭动作改用既有当前语言键，成员头像退出菜单可访问名称，
避免把首字母和姓名重复播报；这些相邻文件只登记本批切片，不冒充全部完成。

窄窗任务层补上共享模态行为、当前语言名称和真实 modal root；原任务层只有浮层
z-index，没有焦点/滚动锁。实际嵌套测试发现缺少 root 标记会让菜单 Portal 落在
背景层，Escape 关闭菜单同时关掉任务页，现沿既有 Dialog/Overlay DOM 合同修齐。
同样补齐移动会话 Switcher 的 root：聚焦关闭按钮产生的 Tooltip 现在属于该模态，
Escape 先关提示、再关父层，与共享 Dialog 一致；测试明确验证两层而非忽略提示。
菜单和输入弹窗均先独立关闭，外层关闭归还原触发器焦点，缺 source 不挂载或锁滚动。

新增 18 项行为回归，定向 6 文件 37 项通过，见 /tmp/nexus-a83-target.log。覆盖延迟
任务/成员、手动选择取消、重复请求、目录重排/删除、source 切换及旧读迟到、DM
无筛选、文件来源、读取失败/显式 unsupported、无调用者、具名 modal、Tab 循环、
菜单/Tooltip/输入弹窗逐层 Escape 与焦点归还。迭代先修正测试中的实际菜单文案
定位，再由真实缺项和键盘场景推动上述修复；首次新提示误用了不存在的 info tone，
已改用现有 neutral，未新增私有提示风格。异步事件使用 act 等待真实状态提交。
最终补强目录/详情的具体焦点断言，交给完整门禁再次验证。

485 项清单为 251 pending、132 in_progress、76 improved、20 retained、6 removed；
公共 UI 仍为 118 项。三个任务 Surface 完成本批代码/行为审查，其他共享入口保留
未完成部分；全部生产审计源哈希一致。视觉、浏览器和原生宿主验证继续按用户要求
暂停，没有启动产品服务或 Go 测试。整体 Goal 保持原范围继续，只本地提交，不推送。

首次完整门禁指出旧样式合同仅用 error ? 正则提取错误分支，合并定向缺项后匹配
为空。现将禁止私有按钮/任意圆角及复用 InlineNotice 的约束扩大到整个任务列表，
删除依赖控制流拼写的截取；真实 DOM 回归继续验证错误/缺项互斥与 busy 动作。
两份新导航/模态集成测试加入必要行为套件清单。最终代码还允许返回原任务行时
由浏览器在目录内显示焦点目标，Header/fallback 焦点仍不主动滚动。

修正后的最终快照以 31098b1ba 为基线、仅叠加本批 26 文件，完整通过 lint、
typecheck、481 项合同、256 文件的 1088 项组件/模型测试及生产构建，见
/tmp/nexus-a83-check.log；快照清单见 /tmp/nexus-a83-review.json。首次控制流
正则失配保留于 /tmp/nexus-a83-check-first.log。最终焦点断言已纳入完整回归，
未跳过测试。Lint 仅保留既有 WorkGraph metadata editor effect-ref 提示，构建
仅保留既有大分块提示。本批代码与审计源哈希一致，未进行视觉或宿主验收。


## A84 — 紧凑成员头像与受控选择显示

沿活动条比例反馈复核当前实现：A78 的 36px 外框、26px 图像/28px 中性框、
32px 命中区和固定工作图动作仍存在，本批不再更改其几何。继续审查相邻共用
成员切换器，删除独立 RoomAgentAvatar 及其图片、5px 圆角、微字号和边框实现。
公共 UiAgentAvatar 新增真实选择器使用的 xxs 16px 档位，以 control-xs 6px 圆角
和单完整字符回退收口；共享图片失败恢复与装饰身份协议一起生效。普通 xs22、
其他头像及完整工作图几何不变，公共头像整体仍保留 in_progress。

Room 选择器保持 Panel 112×28px 与 Task 最大 144px 的既有密度，名称左对齐，
选中勾改用中性图标色。缺名/重名由共享 Agent 选择文字 owner 提供本地化通称
和稳定前置序号；完整可访问名称与原生 title 保留截断信息，装饰头像不重复播报。
当前身份不存在时显示不可用的禁用选项，而非无条件冒充第一位成员；候选存在时
允许用户明确改选，无候选时禁用触发器，完全没有绑定或候选时不留空入口。
受控身份或候选集合变化同步清空菜单状态，名称刷新/目录重排则保留正在使用的
菜单，候选恢复不会复活旧浮层。业务上层仍独占回退身份的决定。

Room 任务面板把完整成员目录单独交给姓名投影，只允许实际有任务的成员进入
候选，切换仍提交精确 Agent ID 并显示对应任务。单成员缺名来源复用共享通称；
进程模型及完整任务摘要留待后续，不把本次姓名切片计为完整组件完成。

新增 12 项行为回归，定向 5 文件 33 项通过，见 /tmp/nexus-a84-target.log；覆盖
中英文缺名/重名、缺项禁用与显式改选、完整目录但有限候选、外部选择和目录变化、
键盘选择/焦点归还、Unicode 缩写与失败图片换源恢复，以及真实任务候选与任务切换。
既有 Room 子任务与窄窗模态集成同时通过。头像和两份成员/任务回归加入必要
套件清单；所有权合同禁止成员切换器重新写 img、头像源/缩写或任意圆角。
后续补强未绑定按钮的唯一名称，并同步中性选中标记，由最终完整门禁验证。

485 项清单为 250 pending、132 in_progress、77 improved、20 retained、6 removed；
公共 UI 仍为 118 项。全部现存审计源哈希一致。视觉、浏览器、产品服务与原生
宿主验证继续按用户要求暂停；本批只做离线代码/行为验证，只本地提交，不推送。
整体 Goal 仍有未审条目，保持进行中。

最终隔离快照以 0f0f5a860 为基线，仅叠加本批 14 文件；lint、typecheck、481 项
合同、257 文件的 1100 项组件/模型测试及 build 全部通过，见
/tmp/nexus-a84-check.log。完整快照清单见 /tmp/nexus-a84-review.json；现存
485 项审计记录的源哈希逐一核对。未跳过测试，未运行浏览器或宿主；保留既有
WorkGraph metadata editor effect-ref 提示及大分块提示。最终未绑定名称和
中性选中标记也已纳入完整门禁。提交前只补充验收记录，不再修改产品代码。


## A85 — 会话任务摘要、明细浮层与进程选择

完整审查公共 WorkspaceTaskPanel、Room 任务适配与纯选择模型。旧 Task Strip
在 DOM 中直接绝对定位展开，没有统一 Escape/外部指针关闭；详情只按行号保存，
列表重排或同长度替换会继承其他任务的展开。Room 手动选中的成员退出后，即使
旧进程还在输入集合，也不能覆盖有效成员目录；旧模型会选中该无效进程，随后
查不到成员而使整个面板消失。现先按有效候选限定手动选择，失效时提交有效回退，
旧成员/进程恢复不重新夺回选择；未手动选择时继续跟随最新有效进程。

共享 Task 明细接入既有 Anchored Overlay Portal、reference-list 几何、材质、
非模态范围和打开标记，向上居中并夹回视口，移除私有宽高/底部坐标公式。没有
新增模态遮罩、页面滚动锁或消息布局高度。完整列表在上限内滚动，长任务和说明
允许断行。摘要变化时通过同一定位入口重算，普通状态/进展更新不重置阅读焦点。

打开时聚焦具名非模态根，控件沿自然 Tab 顺序访问；反向退出回摘要、末项前向
退出续接摘要后控件，外部点击保留目标焦点，Escape/IME 沿共享逐层仲裁。把菜单
内部的 Tab 焦点续接函数移到 overlay-focus-navigation.ts，菜单和 Task 共同
消费，未复制第二套规则。子 Portal 的选择仍属于 Task 内部，关闭菜单不会同时
关闭父层；源按钮和详情关联各自完整说明，头像作装饰避免重复播报。

DM 与 Room 都传入精确会话 scope。会话变化/任务清空关闭旧层，来源或任务结构
变化清空详情。Todo 没有持久条目 ID，唯一名称只用于本地 DOM 连续性而非业务
身份；结构变化不沿用旧行号，同名任务的状态/详情变化保守关闭。正常状态与进展
更新保留详情，唯一名称重排保留控件焦点；删除正在聚焦的任务后才把丢失的焦点
交回明细根。未从正文或时间猜测新的任务身份。

核对出根设计已要求 Task 平面摘要，而实际共用活动 recipe 仍提供完整边框/阴影。
本批在唯一 recipe 增加 plain 表面并由 Task 显式选择；控件/多动作 Dock 材质保持，
不在页面覆盖。摘要继续保留 44px 命中与可换行的 32px 视觉基线，来源去掉私有
20px/5px 覆盖并采用公共 xs 22px；完成图标归 14px，运行点消费 running token，
焦点消费 ring token。Root 规范同步精确表达换行、向上居中和非模态语义，清除
旧的单行/桌面右对齐描述。Room 摘要与菜单都使用完整目录的稳定去歧义标签，
头像仍取原始展示姓名，不把序号变成头像缩写。

新增 14 项行为回归，定向 4 文件 38 项通过，见 /tmp/nexus-a85-target.log。覆盖
双语非模态名称、关联说明、Tab/Escape/外部焦点、IME、嵌套选择、scope/空目录
恢复、唯一/同名任务重排和替换、状态刷新与移除焦点、摘要重定位、自动最近进程、
手动选择保留/失效/恢复，以及真实来源名称和精确任务切换。初次定向测试的菜单
角色与多 ID aria-describedby 断言已按真实共享组件修正，记录保留在
/tmp/nexus-a85-target-first.log；聚焦 Tooltip 的直接调用也已用 act 提交，最终
定向无此警告。既有菜单与共享浮层行为一并通过，未替换或模拟其状态机。

485 项清单为 248 pending、132 in_progress、79 improved、20 retained、6 removed；
公共 UI 仍为 118 项。Task Strip 与 Room 适配完成本批代码/行为审查，DM 主面板
仅登记 scope 装配切片。全部现存审计源哈希一致。视觉/浏览器/原生宿主验证仍
按用户要求暂停，不启动产品服务；整体 Goal 继续，仅本地提交、不推送。

首次完整门禁拦截了把 Tab 监听直接放在非交互 dialog 根的写法。已保留正确的
非模态角色，改为和现有 Dialog 一样在打开期间注册/清理键盘监听，并严格限制
到当前根内的 Tab 事件；未禁用或放宽可访问性规则。目标文件 lint 与 38 项定向
回归重新通过，首次门禁记录保留于 /tmp/nexus-a85-check-first.log。

最终隔离快照以 16b5ae9b4 为基线、叠加本批 21 文件，完整通过 lint、typecheck、
481 项合同、257 文件的 1114 项组件/模型测试与生产构建，见
/tmp/nexus-a85-check.log；快照清单见 /tmp/nexus-a85-review.json。既有
WorkGraph metadata editor effect-ref 提示及大分块提示仍保留，没有新增 lint
告警，也未放宽门禁。提交前再次核对全部现存审计源哈希、精确文件集合和已测试
快照，仅补充验收证据，不再修改产品代码。未运行视觉/浏览器/宿主或 Go 校验。

## A86 — Goal 编辑、状态说明与确认生命周期

共享 Goal 编辑原先通过 parseInt 接受部分预算文本，100.5 与 100abc 均会成为
100；部分非法输入会退成 null，反而删除预算。本批用一个完整正安全整数/空值
解析器同时守住表单和命令边界，非法文本原样保留并由公共 Field 关联错误，不
发 mutation。空值继续区分移除已有预算的 null 与原本无预算的省略字段，创建
Goal 的宿主控制命令不变。编辑字段使用实例 ID，Dialog 使用公共标题注册协议，
不再共享固定 ID；目标、预算、保存和确认文案随当前语言更新。

草稿与确认使用现有 useResettableState，owner、Session 或 Goal 身份改变就同步
清空。清除确认额外绑定打开时的 objective、当前清除资格和可操作状态，失效后
即使旧绑定或正文恢复也不重开；普通版本/用量更新和语言切换保留有效确认与
草稿。旧提交的成功回调只能清除当时那份草稿，不能抹掉后来会话的编辑。原
resource 的 owner/session fence、全局命令互斥和未知结果核对事务继续由其原
所有者负责，本批不宣称完整资源事务审计或修改后端协议。

Goal 状态条使用公共 Panel/Badge/Typography/Button/Spinner；主状态保持单一
生命周期/执行槽，文字按当前语言投影，未知 wire 状态保持中性且只提供刷新，
未知绑定不可清除。元信息和动作在窄空间可换行，阻塞原因与所需输入使用完整
supporting 文本，不再一行裁断；目标摘要仍保留既有单行和完整标题。忙碌归属
实际动作，读取核对不再冒充保存。已证明写入成功但读取仍旧的反馈继续表达
成功并提供只读恢复动作；编辑期间同一恢复提示只在弹窗展示，关闭后回到原 lane。

删除未被读取的 requiresIdle 字段、静态中文确认配置、旧预算截断函数与异步
重置 effect；五个仅在纯模型内部使用的导出收为私有。翻译继续归 conversation
分片，视觉规则更新根 design.md，Goal L2 保留领域责任。Room 负责人选择器、
Room 包装层与 Plan/权限 continuation hold 的创建侧文案继续留待下一批审查，
未把这些未改文件或全部 Goal 创建/资源事务计为完成。

四个 Goal DOM 套件列入合同门禁，另有纯模型 Vitest 回归，覆盖非法预算不提交、空值移除/省略、
真实双语目录、独立字段与弹窗命名、完整阻塞文本、精确忙碌、未知结果保留与
只读恢复、确认失效不复活、普通进度保留以及旧异步结果不清除新草稿。第一轮
45 项定向行为测试通过；类型检查发现测试夹具遗漏 owner generation 与 Promise
回执类型，已补齐真实类型，并追加读取/保存区分测试。没有通过断言豁免来掩盖
生产问题。首次完整门禁发现纯模型测试误登记到要求真实 DOM 事件的套件清单，
已将其从 DOM 名单移出，继续由 Vitest 正常运行；原 DOM 合同没有放宽。首次日志
保留在 /tmp/nexus-a86-check-first.log。最终五个 Goal 套件 46 项回归全部通过，
相较本批前新增 44 项。

485 项清单更新为 245 pending、131 in_progress、83 improved、20 retained、6 removed；
公共 UI 仍为 118 项。四个共享 Goal 视图完成本批代码/行为审查，全部存量源哈希
已核对。视觉、浏览器与宿主验证按用户要求暂停；未启动产品服务，只本地提交，
不推送，整体 Goal 继续。

最终完整 npm run check 在基线 9b0e0bc96 与本批 20 个精确文件的隔离快照通过：
lint、typecheck、481 项合同、260 文件的 1158 项组件/模型测试及 build 全部完成，
见 /tmp/nexus-a86-check.log。仅保留既有 workgraph-metadata-editor-dialog effect-ref
lint 警告与大分块构建提示。快照为 /tmp/nexus-a86-review-6i7oupz0，清单保存在
/tmp/nexus-a86-review.json；提交前再次核对工作树/暂存区与快照字节及审计哈希。

## A87 — Room Goal 负责人选择与当前 Goal 展示

Room Goal Footer 原先有独立原生 select、字段背景/边框和微字号，本批改为公共
xs Select Menu，沿现有 Composer container query 收缩，采用共享定位/键盘/Portal。
该业务原生 select 是门禁最后一处例外，现在移除例外，原生 select 仅允许公共
form-control owner。负责人使用统一 Agent 名称目录，重名/缺名有稳定序号，缺失
绑定显示禁用的不可用项，当前可访问名称包含动作与选择；空选择仍可显式选择。

UiSelectMenu 新增可选 resetKey，现有消费者不传时行为不变。Room Goal 传入精确
Session 与排序后的候选身份集合，变化时用已有 resettable state 丢弃打开态，
保持 trigger DOM 与外部焦点，旧上下文恢复后不自动重开。名称/语言或排序更新
保留菜单；禁用态仍即时收起。该纯上下文机制归公共 overlay adapter，页面不
复制浮层状态或以 React key 重建整个控件。

原负责人草稿控制器会在成员暂空或被移除后用 effect 改写为默认成员，可能导致
原本选择的负责人被自动替换。本批保留显式草稿，失效就要求用户改选；初始 null
草稿继续沿既有宿主/唯一成员默认规则，显式空值仍表达未选择。控件可用性与
set_goal 派发边界共用同一当前成员校验；只要目标不在当前 Room 就不发命令。
实际 Goal 创建仍使用独立宿主 set_goal、replace_existing 和单个真实 target，
不调用普通 send_message，预算仍为 null；Loop objective/metadata 构造没有改变。

Room Goal Panel 原先缓存另一份 Goal，且只在子面板 onGoalChange 回调后更新，
读取进行中时会显示默认负责人或旧会话的权限提示。本批移除重复 Goal state 和
回写回调，通过共享 GoalPanel 的可选展示函数直接消费同一 scoped Goal；原
onGoalChange 直接转交外部观察者。负责人标签、Plan 解释与创建选择沿同一姓名
算法，空白名称不回显 ID。Room 范围和 DM/Room continuation hold 文案按当前
语言投影，既有单成员、明确负责人和群主接管分支不变；hold 不写回 Goal 状态。
移除固定中文范围常量、单次透传 hold 包装和重复参数别名，补齐三个领域文件 L3。

新增 17 项回归，定向 10 文件 78 项与 typecheck 全部通过：双语/缺名/重名、缺项
不可点击、空选择、禁用态、候选和 Session 变化不复活旧菜单、公共 trigger/focus、
真实草稿 Store 的 Session 恢复与 owner reset、成员失效禁止 set_goal、有效 Goal
只走宿主控制命令、刷新中直接读取当前 Goal 与新的 owner/Session 展示、DM 语言
切换保留刷新序列。日志 /tmp/nexus-a87-target.log 与 /tmp/nexus-a87-types.log；
两个新增 Room DOM 套件进入合同必备清单，hook/纯模型测试由 Vitest 正常运行。

485 项审查清单为 243 pending、131 in_progress、85 improved、20 retained、6 removed；
公共 UI 仍为 118 项。两个 Room Goal 视图完成本批代码/行为审查，共享 GoalPanel
追加当前 Goal 投影证据，SelectMenu 只完成上下文重置切片，完整消费者审查继续。
全部存量源哈希已核对。未做视觉、浏览器或宿主验收，未启动产品服务，不改后端。
完整门禁证据如下；整体 Goal 继续，只本地提交，不推送。

最终完整 npm run check 在基线 7daaa7921 与本批 30 个精确文件的隔离快照通过：
lint、typecheck、481 项合同、264 文件的 1175 项组件/模型测试及 build 全部完成，
见 /tmp/nexus-a87-check.log。仅保留既有 workgraph-metadata-editor-dialog effect-ref
lint 警告与大分块构建提示。快照 /tmp/nexus-a87-review-645m4zyd，文件清单
/tmp/nexus-a87-review.json；提交前核对工作树、暂存区与快照字节及全部审查哈希。


## A88 — 会话 Header 身份与群聊成员入口

DM、Group 与联络 Header 原先把较小头像拉满外层 40px 基座，再覆盖自身圆角、
边框和阴影；共享 Header 另持有一套多层阴影。本批三个生产 identity 消费者
直接选择公共 md Avatar，Header 只提供 40px 对齐空间，删除局部尺寸/外形覆盖
与整段重复阴影 CSS。Gallery 已直接消费公共 SeededAvatar，无需新增变体或
修改场景。联络窄屏的返回按钮保持原位置和命令；设置/目录的 section 图标不变。
共享 Header 只完成身份切片，不将完整导航/响应式审查标成完成。

群聊成员入口保留共享 Header 的 36px 高度、四枚 22px 头像与 6px 轻叠，并沿用
窄窗收为图标的现有 container query。移除成员私有 ring，额外人数使用公共
sm/pill Badge，数字可增长而非挤在固定 22px、8px 字号的圆内。成员头像与窄窗
图标是装饰，不形成额外无障碍身份或逐个原生 title；唯一按钮名称和提示含真实
人数，中英目录各有对应文本，缺名使用公共通称，不回显 ID。空 Room 仍可进入
成员管理，只有没有有效 Room 或目录正在加载时禁用入口，并提供 aria-busy。

外层 RoomSurfaceHeader 原有 Room key 已能挡住正常切房间后的迟到弹窗，本批
补齐 Header 自身的 lifecycle：目录准备单飞，Room/owner 或卸载使旧请求失效，
A→B→A 也不复活旧打开态；旧表单完成的关闭回调不得关闭新弹窗。标题/目录
更新不清空当前弹窗。辅助目录读取失败后仍沿用 AgentStore 既有行为，打开当前
成员与已保留目录，不把读取失败改成成员写事务或自动重试。新增私有 Hook，
补齐三个 Header 视图的 L3 与所属 L2；不变更 Room 管理提交协议或会话导航所有权。

新增 9 项回归；定向 5 文件 17 项通过，见 /tmp/nexus-a88-target-final.log。
覆盖共享头像几何归属、四枚装饰图、双语完整计数和大数字 Badge、空目录、加载/
禁用时鼠标与键盘防重、Room/owner/卸载迟到隔离、旧回调隔离与辅助目录失败。
现有页面导航集成现直接渲染真实 DmConversationHeader / GroupConversationHeader，
继续通过真实页面资源、命令、路由与 Store 覆盖延迟创建、历史打开、标签切换、
关闭/重开、固定与持久恢复；只替代 HTTP 结果与 jsdom 缺失的滚动 API。成员入口
DOM 测试只替代成员表单内容，未声称重新验证其独立 Skill 读取和编辑协议。
Header、成员摘要与页面导航套件加入必备 DOM 清单，源码合同禁止重复头像基座
与成员计数外形。typecheck 通过，见 /tmp/nexus-a88-types-web.log。

485 项清单为 239 pending、132 in_progress、88 improved、20 retained、6 removed，
公共 UI 仍为 118 项；全部存量源哈希已核对。DM/Group Header 与成员摘要完成本批
代码/离线行为审查，公共 Header 与联络根保留 in_progress。按用户要求不做视觉、
浏览器或宿主验收，不启动产品服务，不改后端。整体 Goal 继续，只本地提交。

首轮完整门禁在合同通过、DOM 套件运行中主动中断，以将额外人数移到头像轻叠
区域之外，避免负间距与徽标留白相互抵消；空头像组不生成无内容布局项。原日志
保留 /tmp/nexus-a88-check-interrupted.log，未记录测试失败；以最终重跑结果为准。

最终完整 npm run check 在基线 ae12ba277 与本批 21 个精确文件的隔离快照通过：
lint、typecheck、481 项合同、265 文件的 1184 项组件/模型测试及 build 全部完成，
见 /tmp/nexus-a88-check.log。仅保留既有 workgraph-metadata-editor-dialog effect-ref
lint 警告和大分块构建提示。快照 /tmp/nexus-a88-review-ytxsn6j_，文件清单
/tmp/nexus-a88-review.json；最终证据只更新本文，提交前核对工作树、暂存区与
快照的精确字节及全部审查哈希。


## A89 — 公共 Header 导航与无消费者接口清理

本批继续 A88 的公共 Header 审查，逐一检查 DM、Group、联络、联系人详情、Provider
与现有 Gallery 消费入口，并读完 Header 样式。普通页面窄窗视图原先单独组合
Button、ActionMenu、蓝色选中框与本地开关，本批直接使用公共 sm Select Menu：
32px 高度、13px supporting、160px 且不超过可用空间的宽度、共享键盘/Portal/
ARIA/focus 协议。当前视图进入唯一可访问名称，长名有完整提示；未命中当前值时
只显示选择提示，不把首项伪装成当前项。候选身份或当前值改变消费打开态，名称/
语言刷新保留；候选匹配后才调用真实 onChangeTab，不用字符串强制断言派发业务值。

窄窗开关仍由 Header CSS 的 container query 决定，局部尺寸 observer 只读取结果，
不复制 breakpoint 数字。隐藏 trigger 后投影 disabled，公共 Select 消费旧打开态，
重新显示不复活旧浮层；保留外部焦点，卸载断开 observer/listener。带 Session 导航
的 Header 从来不展示此选择器，现在直接不挂载隐藏副本。已有视图条和协作入口
继续使用原 container 收缩，UiTabs 直接提供 metadata 排版，删除两处等值 12px
覆盖；禁用成员入口不再被 Header 的 hover/expanded CSS 重新高亮。

删除生产中没有消费者的 narrowMode、titleTrailing、tabsNavAnchor、subtitle 与
对应私有子组件/CSS；subtitle 仅剩测试/Gallery 示例，本次仅删现有示例的失效
prop，没有扩展或运行 Gallery。leadingClassName 的唯一消费者是联系人目录返回
按钮，通过强制宽高/背景绕过图标壳；改为明确 action 插槽，由按钮自身持有外观。
所有真实身份仍直接传公共 md Avatar。删除无使用者的 section 图标壳、旧新建标签
文案 class 与动态 tab item class，主标题保留完整原生 title。

ProviderSettingsPanel 的两处真实入口在设置和运营中始终传 embedded，独立 Header
分支不可进入。本批删除该分支、embedded prop、单项 SettingsTabKey/SETTINGS_TABS
及相应进口，两处调用者同步收口。运营 public scope 与 section 布局、私有 Provider
默认 scope、资源/草稿/写事务都没有改变。Header 外形与公共 Provider 的壳层所有权
进入源码门禁；补 Provider presentation、Settings 入口 L3，并同步相关 L2/design。

公共 Header DOM 套件从 2 项扩为 7 项，定向 6 文件 38 项与 typecheck 通过，日志
/tmp/nexus-a89-target.log 与 /tmp/nexus-a89-types-final.log。覆盖公共 Select 选项/
aria-controls、鼠标与键盘选择、Escape/focus、未知值、名称/语言/候选/当前值更新、
CSS 可见性变化与外部焦点、observer 清理、Session/空导航不挂载菜单、返回动作。
真实 DM/Group 页面标签导航与成员入口、联络确认和共享菜单回归继续通过。DOM
只显式提供 CSS 可见性结果，不把 jsdom 测试冒充 container query 或视觉验收。
旧测试还传入已删除 subtitle 时的首次 typecheck 按预期失败，迁移后通过；未保留
只为旧测试存在的生产接口。完整门禁结果以最终隔离快照为准。

485 项清单为 237 pending、133 in_progress、89 improved、20 retained、6 removed；
公共 UI 仍为 118 项。Header 完成本批代码/离线交互审查，联系人详情和 Provider
根只完成上述切片，Settings/Operations 两处机械接线仍保留 pending，不把整页
审查记成完成。全部存量源哈希已核对。按用户要求未运行视觉、浏览器或宿主验证，
不启动产品服务、不改后端；整体 Goal 继续，只本地提交，不推送。

最终完整 npm run check 在基线 69864a69c 与本批 20 个精确文件的隔离快照通过：
lint、typecheck、482 项合同、265 文件的 1189 项组件/模型测试及 build 全部完成，
见 /tmp/nexus-a89-check.log。仅保留既有 workgraph-metadata-editor-dialog effect-ref
lint 警告和大分块构建提示。快照 /tmp/nexus-a89-review-nih2ckmm，文件清单
/tmp/nexus-a89-review.json；最终证据只更新本文，提交前核对暂存区、工作树与
测试快照字节及全部审查哈希。

## A90 — 公共单选菜单与 Provider 显式测试动作

逐项检查公共 Select 的 model、recipe、overlay、trigger、view 与共享键盘边界，
并按 TypeScript AST 提取全部 27 处生产调用的属性；调用证据保存在
/tmp/nexus-a90-select-consumers.json。覆盖设置、Provider、身份、记忆、配对/联络、
定时任务、订阅运营、Room 与 Header。尺寸仍由现有 xs/sm/md/lg recipe 持有，
唯一业务 buttonClassName 覆盖是 Provider 测试入口；其他 className 只承担布局。
各领域硬编码文案、独立业务资源与状态继续归其未完成审查，不将机械调用视为整页完成。

单选原默认中文占位改为 catalog 的双语文案；显式 placeholder 保留。没有候选时
禁用并消费旧打开态，不派发选值。显式点击/Enter/Space 打开后，菜单定位可见才
聚焦当前可用项，否则首项或全禁用的根；后续几何更新不重置焦点。浮层内方向键、
Home/End 只浏览，明确激活才提交当前可用候选；Tab 关闭并使用共享 overlay 焦点
续接。触发器方向键即时选值/打开的既有合同保持，trigger Tab 保留自然顺序。
IME、已处理事件、外部 Portal 与可编辑输入边界继续共用 menu-keyboard；按角色
隔离 menu/listbox 项，不引入另一套定位、退出或多选状态机。

单行选项补完整原生提示，前导/箭头/勾选保持装饰语义，选中勾与活动底面使用中性
图标色；标签使用已有 metadata role。Panel 只增加可选键盘委派与 tabIndex=-1，
领域多选/Slash/历史的行为继续由原调用方持有。删除单选冗余默认值包装层和
buttonClassName 公共/视图透传，尺寸、表面与包裹模式仍由唯一 recipe 决定。

Provider 原用 value="" 的 Select 执行测试，箭头选择会直接调用真实测试命令。
改为公共 xs Button + Action Menu，方向键只浏览，点击或 Enter/Space 明确选择才
提交原 onTestSelection；真实测试、自动选择模型值与命令互斥未改。菜单以 exact
Provider ID、候选身份、编辑/权限/忙碌边界重置，切换后返回不复活旧打开态；名称/
语言刷新保持。空候选/缺 Provider 禁用，实际测试时保留共享 Spinner/aria-busy。
详情身份、状态、动作允许随可用宽度换行，全名不再截断，状态复用公共 Badge。

定向 9 文件 89 项测试与 typecheck 通过，日志 /tmp/nexus-a90-target.log 和
/tmp/nexus-a90-types.log。新增 25 项覆盖明确打开与提交、当前项焦点、禁用跳过、
Home/End、重定位不抢焦点、双向 Tab、IME/已处理事件、全禁用与空候选、语言提示，
以及 Provider 菜单的执行次数、身份/候选/权限/编辑切换、名称刷新和原启用开关。
已有菜单、Field、真实嵌套 Overlay、公共 Header 与 Provider 排版继续通过。严格
I18n Context 保持，旧裸 Select 测试夹具显式加入真实 Provider。迭代首轮发现误用
不存在的 Button variant，类型与原 Spinner 回归同时失败，修正为公共 surface 后
通过，未增加别名接口。源码门禁约束执行菜单所有权和单选唯一视觉 API，新行为
套件进入必跑清单。完整门禁以最终隔离快照结果为准。

485 项清单为 236 pending、131 in_progress、92 improved、20 retained、6 removed；
公共 UI 仍为 118 项。单选编排/视图与 Provider 详情头完成本批代码/离线行为审查，
Panel 原语保留多领域审查、Provider 根保留配置工作面的未完成范围。全部现存源
哈希已核对；无新增 Gallery、视觉/浏览器/宿主测试或产品服务，不改后端。活动 Dock
继续保留 A78 已落地的 36px 外框、32px 命中区与 26px 头像，不因同一反馈反复改尺寸。
整体 Goal 继续，只本地提交，不推送。

首轮完整门禁的 483 项合同通过，组件为 1213 通过、1 失败：Provider 配置多实例
标签测试仍要求点击 Select 标签后焦点停留 trigger。更新该夹具，明确验证标签关联
到本实例的 aria-controls listbox、当前选中项获得焦点、Escape 后只关闭该菜单并
回到本实例字段；没有放宽多实例关联断言。随后 10 文件 95 项定向回归通过，见
/tmp/nexus-a90-target-final.log；首轮日志保留为 /tmp/nexus-a90-check-initial.log。

最终完整 npm run check 在基线 c91dc4c2a 与本批 21 个精确文件的隔离快照通过：
lint、typecheck、483 项合同、266 文件的 1214 项组件/模型测试及 build 全部完成，
见 /tmp/nexus-a90-check.log。仅保留既有 workgraph-metadata-editor-dialog effect-ref
lint 警告和大分块构建提示。快照 /tmp/nexus-a90-review-pe69otfw，文件清单
/tmp/nexus-a90-review.json；最终证据只更新本文，提交前核对工作树、暂存区与
测试快照字节及全部审查哈希。

## A91 — 动作菜单完整文案、内容高度与共享状态

延续 A90 审查公共 ActionMenu、Content、MenuActionRow、menu-styles 与 Overlay
求解/生命周期，逐项读取全部 13 处生产调用的属性和条目结构：权限/模型、Room
模型内容和成员、窄窗动作、Composer 附件/Connector/排队、工作图历史、任务、
Skill、联系人与 Provider 测试。AST 调用证据保存于
/tmp/nexus-a91-action-consumers.json；未扩展或运行现有 Gallery。各业务开关、
身份、资源和命令继续由领域拥有，不把本次公共内容审查冒充整页完成。

原 Action Menu 说明使用 10px soft，标签/说明都强制 truncate，权限范围与任务
失败原因可能不可见。现默认/紧凑正文分别为 control 14px/supporting 13px，说明
为 metadata 12px/muted，统一 regular。普通行最小 36/32px，说明行最小 48/44px，
保留垂直留白；label/description 完整换行。行高度和初始估算仍由 menu-styles
单一 recipe 持有，MenuActionRow 的 contentSized 只选择 min-height，固定建议与
上下文行保持 fixed 档位。Mention 同步消费 48px 说明行与一致估算，不另造数字。

Action Menu 透传内容 ref，只观察未被 maxHeight 限制的内容 scrollHeight，加
当前实际外框 padding/border 后交回原 Overlay 求解器；打开/宽度与内容变化进行
测量，ResizeObserver 观察字体/内容尺寸变化，关闭/卸载清理。首帧无测量仍用
原 recipe 估算，后续可增长或缩短，仍受 320px 与视口上限约束。不能以已被裁切
的外壳高度作下一轮估算，否则长内容无法重新展开。重定位继续保留当前聚焦项，
没有另写全局定位、监听器或键盘所有者。只有 footer 时去掉孤立分隔，估算同改。

图标装饰槽不进入辅助名称。共享状态 recipe 排除 disabled 的 hover 前景/底色，
所有 tone 的 active 都用中性活动底面，删除随选择额外加粗的样式；保留 primary/
danger 语义前景和所有原生禁用/点击合同。DM/Room 共用模型选项中的 Provider
次级标签也从私有 10px soft 改为 metadata/muted，保留 40% 宽度、单行结构与
完整 title；复合模型标签仍由领域拥有，没有把模型长名扩展成另一套菜单。

定向 9 文件 97 项通过（/tmp/nexus-a91-target.log），随后菜单、直接 Session 与
Room 级联的 3 文件 49 项通过（/tmp/nexus-a91-target-final.log）。新增回归明确
验证长文案未裁切的 DOM 合同、语义字号、装饰图标、禁用/选中状态、footer-only
动作，以及内容高度 170→600→70px 时外框按实际留白得到 180→320→80px 上限，
当前项焦点不重置且卸载清理 observer。该测量由 jsdom 显式夹具提供，不声明实际
浏览器排版已验收。任务动作、权限确认、精确模型选择/继承重置、成员选择、
Workspace 级联与 Mention 回归保持；测试回调的首次 TypeScript 推断过窄已修正，
最终 typecheck 通过（/tmp/nexus-a91-types-final.log）。最后少量 recipe 状态/留白
与源码门禁调整以最终完整隔离快照验证为准。

485 项清单为 236 pending、130 in_progress、93 improved、20 retained、6 removed；
公共 UI 仍为 118 项。Action Menu 完成本批代码/离线行为和消费者内容审查，公共
行与 Session 选项追加证据，其他领域业务仍按各自清单推进。全部现存源哈希已
核对。规范与代码地图同步，未启动产品服务、视觉/浏览器/原生验证，也不改后端。
整体 Goal 保持 active，只分批本地提交，不推送。

最终完整 npm run check 在基线 acf3c527a 与本批 13 个精确文件的隔离快照通过：
lint、typecheck、484 项合同、266 文件的 1217 项组件/模型测试及 build 全部完成，
见 /tmp/nexus-a91-check.log。仅保留既有 workgraph-metadata-editor-dialog effect-ref
lint 警告和大分块构建提示。快照 /tmp/nexus-a91-review-nhimyc28，文件清单
/tmp/nexus-a91-review.json；最终证据只更新本文，提交前核对暂存区、工作树与
测试快照字节及全部审查哈希。

## A92 — 联系人详情保存反馈与对象动作

沿 A91 的动作入口消费审查，继续读取联系人详情 Header 与两处子组件、编辑器
持久化状态回调及当前工程地图。窄窗动作真实有唯一详情消费者，不能误删为死码；
它只用本地 useState，切换 Agent 后旧打开菜单会沿用新回调，末尾默认分支还把
任何未知值解释为删除。现按 exact Agent ID 通过 resettable state 消费旧打开态，
姓名/语言刷新不打断；入口与菜单使用完整本地化姓名，缺名复用 getAgentDisplayName，
不读取或显示 ID。聊天、建群、删除按显式值分派，未知值无动作；删除继续只调用
原页面确认入口。移除冗余原生 type，图标/唯一 Tooltip 与菜单复用公共所有者。

保存反馈原使用 caption/soft，错误文案 aria-hidden，移动错误浮层自行写绝对定位、
视口公式和 sm breakpoint，未接入 Escape/外部关闭/Portal 仲裁。现状态采用 metadata
12px 与 muted/success/danger，唯一 live status 始终包含完整原始文案；宽窗可见
摘要限宽，父级现有窄窗判断直接传入 compact，避免 559px 与 sm 的两份边界。
错误在宽窄两种布局都可由 32px 公共图标按钮显式查看，按钮具名、关联完整状态，
并关闭自动 Tooltip 和原生 title，避免重复浮层。

错误详情复用 useAnchoredOverlayLayer、reference-list preset、模态范围与关闭
仲裁，正文 supporting 13px、完整换行与内部滚动。非模态 dialog 开放自身焦点以
支持键盘阅读；Escape 回入口，Tab/Shift+Tab 关闭并从入口续接相邻控件，外部点击
保留实际目标焦点。Agent、保存阶段/文案或 Header 密度变化消费旧详情，返回不
复活；UI 仅解释收到的消息，不推断保存资格、不重放或新增保存/重试动作。原有
Spinner reduced-motion 行为保持；删除私有定位公式、手写边框与延迟关闭 effect。

定向 5 文件 57 项与 typecheck 通过，见 /tmp/nexus-a92-target-final.log 和
/tmp/nexus-a92-types-final.log。保存状态套件从 2 项扩为 11 项，新对象动作套件
3 项，覆盖完整辅助文案、宽窄入口、Portal/ARIA、键盘 focus/Escape/双向 Tab/IME、
点击外部、同错误内容跨 Agent 切换、阶段/文案/密度失效、模态内第一/第二次 Escape，
以及三个动作精确次数、名称刷新、A→B→A 关闭和英文/缺名兜底。首次夹具误用
Backdrop 的 ariaLabel，DOM 与类型检查共同失败；改为公共 aria-label 后通过，
未修改生产接口或放宽命名断言。原菜单、Header、嵌套 Overlay 回归保持。两套
行为测试进入必跑清单，源码合同约束真实 Agent 接线与共享浮层/动作所有权。

485 项清单为 234 pending、130 in_progress、95 improved、20 retained、6 removed；
公共 UI 仍为 118 项。保存反馈与窄窗动作完成本批代码/离线行为审查，联系人详情
根仅完成 Header 接线，其自动保存资源/完整编辑生命周期仍保持 in_progress。
此次也确认 RoomMobileActionsMenu 有真实入口，Room 会话绑定与窄窗业务留待后续
独立审查，没有混入本批。全部现存源哈希已核对。规范与地图同步；不启动产品
服务，不做视觉/浏览器/原生验证、不改后端。整体 Goal 继续，只本地提交，不推送。

首轮完整门禁在 lint 阶段发现非交互 dialog 的 JSX onKeyDown 不符合可访问性规则。
按现有只读浮层模式改为打开时在详情根绑定原生 Tab 退出监听，关闭/卸载清理，
既有 IME/defaultPrevented 和共享焦点续接保留；没有禁用规则、给正文伪造按钮角色
或新建全局监听。首轮日志保留于 /tmp/nexus-a92-check-initial.log。

后续源码复查确认 buildPersistenceState 把 error 与 warning feedback 都投影为 error
阶段；因此将详情入口/标题改为中性双语“保存状态”，增加待确认结果回归，禁止从
该阶段断言“保存失败”。改文案前的完整门禁已通过 485 合同与 1229 项组件/模型
测试，日志保留 /tmp/nexus-a92-check-before-copy-review.log；最终新增用例和文案
仍以最终隔离快照复跑结果为准。

最终完整 npm run check 在基线 9d4433f89 与本批 15 个精确文件的隔离快照通过：
lint、typecheck、485 项合同、267 文件的 1230 项组件/模型测试及 build 全部完成，
见 /tmp/nexus-a92-check.log。保存反馈套件最终为 12 项，待确认结果原文与中性入口
同时受回归约束。仅保留既有 workgraph-metadata-editor-dialog effect-ref lint 警告
和大分块构建提示。快照 /tmp/nexus-a92-review-z0_2bg67，文件清单
/tmp/nexus-a92-review.json；最终证据只更新本文，提交前核对暂存区、工作树与
测试快照字节及全部审查哈希。

## A93 — 窄窗导航作用域与 Room 成员入口复用

窄窗 Surface 原有多份无作用域 useState，成员目录 await 后可沿旧 Room 打开，
任务层还按 Session 对象引用判等。现成员准备 hook 从 group/header 晋升到 members，
桌面与窄窗共用同一 owner/Room/挂载代次与单飞保护。成员编辑仍属于 Room，切换同
Room 的 Session 保留表单；读取失败沿既有辅助目录语义保留当前成员，不新增写入。

窄窗菜单、历史切换器和辅助/任务层按 owner、Room、会话与 DM Agent/session key
消费旧打开态，返回旧身份不复活；标题/目录/等价身份快照刷新保持当前层。任务打开
状态与请求归入同一个本地状态，删除旧对象引用门槛。明确选择辅助页、任务文件或
会话切换器会撤销待完成成员打开，真正打开成员窗则消费旧菜单。其他持久导航、
会话创建/选择、聊天 viewport、草稿与工作区数据 owner 不变。

动作菜单移除任意字符串到辅助页的断言，只分派六个明确命令。保留空工作图入口、
DM 隐藏成员和无来源任务禁用；目录加载仅禁用成员项。继续复用圆形 lg IconButton、
ActionMenu 与现有 190px 最小宽，不引入私有样式或复制公共键盘/定位行为。

初轮定向 3 文件 19 项及 typecheck 通过，见 /tmp/nexus-a93-target.log 和
/tmp/nexus-a93-types.log。新增两套 DOM 回归纳入必跑清单，使用真实 Header、菜单、
历史切换器与成员准备，聊天/任务/表单边界用类型化替身隔离独立 HTTP/runtime。
后续增加迟到成员打开与新导航意图、成员打开关闭菜单的回归，最终以隔离门禁为准。

485 项清单为 231 pending、131 in_progress、97 improved、20 retained、6 removed；
公共 UI 仍为 118 项。动作菜单/模型完成代码与离线审查；Surface 仅完成临时导航
生命周期，聊天、Thread 及辅助页面完整审查继续，保持 in_progress。没有视觉、
浏览器或宿主验证，没有运行产品服务。整体 Goal 继续，只本地提交、不推送。

最终隔离 npm run check 全部通过：lint、typecheck、485 项合同、269 文件的
1245 项组件/模型测试及 build；新增 Surface 11 项、动作菜单 4 项。日志见
/tmp/nexus-a93-check.log，精确 17 文件快照清单见 /tmp/nexus-a93-review.json，
基线 a156b991f。仅保留既有 workgraph-metadata-editor-dialog effect-ref lint 警告
与大分块构建提示。提交前只追加证据并校正清单的公共 lg 图标按钮尺寸说明为
36px，生产代码保持受测字节；再次核对全部源哈希、工作树/暂存区与测试快照。

## A94 — 窄窗全屏壳与页头阅读层级

继续审查上一批保留的辅助层、Thread 和 Header。辅助页与 Thread 原只有 fixed
全屏外观，子任务层独自接入共享模态协议；现三个入口共同使用领域内的
RoomMobileOverlayFrame，统一材质、纵向骨架、真实模态根与 Dialog 行为。删除
重复层级 class、模态属性和生命周期装配，公共 Dialog 的栈、滚动锁、IME、焦点
循环与逐层 Escape 保持唯一 owner，不另建全局监听或第二套键盘规则。

辅助页关闭回到原触发器，模式切换重新进入新页；同页名称或语言更新不重建。
保留平台 Header 和业务内容，空 WorkGraph 继续显示真实共享空态；其 Agent 去重
数组仅构造一次，图与展示目录共用。Thread 仍消费既有 live model/完整消息面板，
缺源不挂载；模态可访问名称用该模型的 Agent 展示名与当前语言，不插入内部 ID。
子任务原有四项真实目录/任务/嵌套输入弹窗回归随同验证。

窄窗 Header 副标题从 caption/soft 改为 metadata/muted（12px/18px 行高）；主标题
保留 sectionTitle（14px/20px），原平台高度、拖窗、按钮命中和下缘渐隐不变。
空白/等价标题统一，重复身份不占第二行；全名通过实例独立描述 ID 与单份 title
保留，装饰箭头不命名，按钮内部改为 phrasing 节点。返回/切换命令及展开态不变。

定向 6 文件 30 项与 typecheck 已通过，见 /tmp/nexus-a94-target-final.log 和
/tmp/nexus-a94-types-final.log。覆盖真实空工作图、Thread live source/面板、
四种菜单/嵌套模态/空根场景、焦点返还、语言更新、辅助页模式更换与标题可访问
描述。初版测试把 spy 作为 beforeEach 返回值，被 Vitest 当成清理函数无 receiver
调用；改为显式 void setup。类型检查同时拦下 Workspace 替身传入原接口没有的
第二参数，已沿真实单参数命令修正，未改变生产协议；原 fallback 文案断言改为
当前双语目录事实。首次日志保留 /tmp/nexus-a94-target.log 和 /tmp/nexus-a94-types.log。
通过后清理 Thread fixture 的 act 警告与 Header 的按钮内容语义，最终以隔离门禁为准。

新领域外壳登记后清单为 486 项：228 pending、131 in_progress、101 improved、
20 retained、6 removed；公共 UI 仍 118 项。本批完成三个窄窗挂载组件和 Header 的
代码/离线行为审查，不把其内部工作区、简介编辑器或 Thread 消息组件一并标记
完成。地图、唯一视觉规则与必跑测试同步，不做视觉/浏览器/宿主验收或启动产品
服务。Goal 继续；仅本地提交，不推送。

最终隔离 npm run check 通过 lint、typecheck、485 项合同、272 文件的 1259 项
组件/模型测试及 build，日志 /tmp/nexus-a94-check.log。新增测试无 act 警告；
仅保留既有 workgraph-metadata-editor-dialog effect-ref lint 警告和构建大分块
提示。基线 f533872b8，17 文件快照清单 /tmp/nexus-a94-review.json；全部 486 项
审计源哈希一致。通过后只追加本段证据，提交前复核工作树、暂存区与测试快照。

## A95 — Thread 执行入口与状态面

沿 A94 追踪 Thread 两种布局、控制 Provider、live 发布 Hook 和真实消费模型，
确认现有 use-room-thread-source 已按 conversationId 关闭旧目标、卸载清理源；
不再在 Provider 或界面补第二套重置。新增独立发布者/消费叶子的 DOM 回归，
验证同 Agent 不同 agent_round、重复同目标、legacy null、目录刷新、显式关闭、
A→B→A 与缺失会话清理，原按钮节点保持挂载。控制 Provider 完整审查后保留。

执行头 Thread 和停止统一选择公共 xs Button（28px、metadata 12px），控制条
最小 32px、2px 内边距；停止图标 14px 且只装饰，stopping 同时 disabled/aria-busy，
详情仍可进入。终态短反馈使用 metadata/muted；停止/Thread 顺序、无动作空条
隐藏、稳定 MessageItem 外壳及已有活动/终态证据投影保持。Thread 可见标签继续
为 Thread，完整辅助名称含当前 Agent 展示名/通称，以 aria-expanded 表达打开
详情，替换会随动作名改变的 pressed 语义；不新增技术 ID 或第二套名称所有者。

Thread 等待/无额外详情现在复用公共 sm/plain ResourceState 的几何、live status
和 busy 语义。保持 supporting/muted 的小型行内说明与 16px 公共 Spinner，明确
关闭默认空态图标底座；没有边框、附加动作或另一份加载判断。补齐文件 L3。

定向 6 文件 16 项及 typecheck 首轮通过，日志 /tmp/nexus-a95-target.log 与
/tmp/nexus-a95-types.log。测试包括真实 MessageItem 执行壳、真实双轮卡片和精确
命令、Thread live 生命周期、双语按键/缺名开关、等待/空态唯一播报，以及上一批
真实窄窗 Thread。无测试替身修改生产接口，无视觉/浏览器/宿主检查或产品服务。

486 项清单更新为 225 pending、130 in_progress、104 improved、21 retained、
6 removed；公共 UI 仍 118 项。完成执行壳、Thread 按钮和空态适配审查，保留轻量
控制 Provider；未把发布/消息模型或整个 Room 页面计为完成。唯一规范、目录地图
和必跑回归同步，最终提交以完整隔离门禁及源哈希核对为准。Goal 继续，本地提交。

最终隔离 npm run check 通过 lint、typecheck、485 项合同、275 文件的 1269 项
组件/模型测试及 build，见 /tmp/nexus-a95-check.log。基线 fbd3ddf4e，18 文件
快照清单 /tmp/nexus-a95-review.json；全部 486 项源哈希一致。没有新增测试警告，
仅保留既有 workgraph-metadata-editor-dialog effect-ref lint 警告与大分块构建
提示。通过后仅追加本段证据，提交前核对暂存区、工作树及受测快照字节。


## A96 — Room 简介导航与成员选择

简介页保留当前 44px 头部、12px 横向内边距和 112×28px 成员入口；栏目直接
使用公共 compact UiTabs（32px、metadata、中性底线），移除局部 28px 覆盖，
成员与栏目间隔从 20px 收到 8px，横向收缩和滚动继续由公共导航拥有。未强行
套用 60px 的 Workspace 页面 Header；内联设置、记忆目录/正文与联络布局保留。
同步修正目录地图中与根 design.md/公共导航已不一致的旧轻底说明。

原成员选择只在渲染时回退，目录恢复会重新展示失效的旧目标；现把回退提交到
本地选择。显式请求与导航改用公共同步重置状态，去掉 effect 双状态重置。
桌面请求在其所有者按 owner/Room/当前 Agent 隔离，简介消费同一业务边界；
同 Room 的 Session 切换、可见性、成员对象/顺序/姓名更新保留人工选择与栏目。
联络仍接收当前 Room/Conversation，配置保存与校验仍绑定当前展示 Agent，
不新增草稿副本或自动写入。重复显式打开可重新定位同一成员及栏目。

新增真实 Tabs/成员菜单与桌面请求控制器的双语 DOM 回归；编辑器、记忆和联络
只用类型化内容边界隔离 API。覆盖键盘进入记忆、成员切换保留栏目、同 Room
Session 更新、移除/恢复、缺失请求目标、重复请求、Room A→B→A、相同 ID 下
owner 更换，以及精确保存/校验命令。登记为必跑回归；本批不新增视觉检查、
浏览器/原生测试、Gallery 或产品服务。

486 项清单现为 224 pending、130 in_progress、105 improved、21 retained、
6 removed；公共 UI 仍 118 项。只将简介装配标为本批完成；布局控制器仅审查
简介请求切片，不把整个 Room 计为完成。Goal 保持 active，仅本地提交。


定向 3 文件 23 项与 typecheck 首轮通过（/tmp/nexus-a96-target.log、
/tmp/nexus-a96-types.log）。最终隔离 npm run check 首轮通过 lint、typecheck、
485 项合同、276 文件的 1277 项组件/模型测试及 build，见 /tmp/nexus-a96-check.log。
基线 a067a88a6，10 文件快照清单 /tmp/nexus-a96-review.json；全部 486 项源哈希
一致。无新增测试警告，保留既有 workgraph-metadata-editor-dialog effect-ref
lint 警告与大分块提示。完整门禁后仅追加本段证据，提交前核对工作树、暂存内容
与受测快照字节；没有推送。


提交前主分支新增独立后端修复 14a4a6607（隐藏 Goal continuation 历史边界），
相对受测基线的 web 代码、配置及门禁输入完全未变；保留该提交与其 CHANGELOG
条目。提交父级更新为 14a4a6607，受测前端仍为上述精确快照，不重复运行无变化
的门禁；manifest 同时记录 tested_base 和新的提交 base。


## A97 — 桌面辅助工作面的按需挂载与导航保留

沿 RoomSurfaceContent 确认右栏只在非 chat 视图挂载，关闭右栏本就卸载内容；
原辅助面板只隐藏非活动页，却在任意页首次打开时同时创建工作图、文件浏览器
和配置编辑器。现保留原页面数据表，在第一次进入具体页时才挂载内容；之后在
同一右栏内切换已访问页时保持相同实例与编辑/浏览状态，避免无用资源初始化。
访问记录复用公共同步重置状态并按 owner/Room 隔离，DM 另含 Agent；同 Room
的 Session、目录更新和右栏宽度变化保持访问记录，内容资源仍由各自领域更新。
关闭右栏不新增后台保活；子任务仍只在激活且有精确来源时挂载。

完成当前装配与分栏样式审查：8px 软分栏、辅助宽度模型、44px 子页头部及原
文件/保存/校验/尺寸命令保持，不再新增几何、颜色或字段副本。实际辅助区域
增加本地化可访问名称，和分隔条已有稳定 aria-controls 绑定。无额外外框或标题。

新增真实辅助区域/分隔条配合类型化内容边界的 DOM 回归，验证首次访问的资源
初始化、已访问页同节点及输入保留、简介可见性、Session/成员刷新、Room/DM
Agent/owner 变化、子任务来源与挂载、文件路由的精确参数及百分比宽度 owner。
不启动文件/配置 API、产品服务或浏览器，不增加视觉验收或 Gallery 场景。

486 项清单现为 224 pending、129 in_progress、106 improved、21 retained、
6 removed；公共 UI 仍 118 项。辅助面板完成代码审查，父级主聊天与 Header
仍按独立清单推进；Goal 保持 active，仅本地提交。


定向 4 文件 28 项及 typecheck 首轮通过，日志 /tmp/nexus-a97-target.log 和
/tmp/nexus-a97-types.log。最终隔离 npm run check 首轮通过 lint、typecheck、
485 项合同、277 文件的 1285 项组件/模型测试和 build，见 /tmp/nexus-a97-check.log。
基线 f018c96b8，8 文件快照 /tmp/nexus-a97-review.json；全部 486 项源哈希一致。
保留既有 effect-ref lint 警告与大分块提示，无新增测试警告。完整门禁后仅追加
本段证据，提交前核对工作树和暂存区均与受测快照一致；没有推送。


## A98 — 桌面 Header 与聊天布局连续性

完成 RoomSurfaceHeader、RoomSurfaceLayout 和 RoomSurfaceContent 的代码/装配
审查。Header 的 DM/群聊分支原本重复传递相同会话回调，现只构造一份导航参数，
分支继续拥有各自身份与成员管理。重复点击当前辅助栏目仍关闭右栏，Header key、
共享尺寸/头像、会话标签集合、渐隐裁剪与成员准备不变，不新建会话导航 owner。

Room Thread Provider 的打开回调原先随任意页面快照重新创建，造成控制 Context
变化；现在回调只依赖原页面切换命令。标题/宽度刷新不会通知无关的 memo 控制
消费者，替换导航命令时仍立即使用最新命令；不改变 live store 或再建关闭状态。
主内容层经完整阅读与集成回归后保留：辅助页在原分栏位置切换，主聊天保持同一
实例；Thread 由精确 source 更新，进入辅助页关闭 Thread，打开 Thread 返回 chat。
Session 变化继续由已存在的 live 发布 owner 清理，DM 不挂 Room Thread 上下文。

现有页面级多标签回归升级为真实 RoomSurfaceHeader，仍经过真实目录/API 边界、
页面命令和导航 Store，覆盖 DM/群聊的延迟新建、历史打开、切换、关闭/重开与
固定恢复；新增辅助页开关不改写 Session 集合的断言。新增布局 DOM 回归使用
真实 Surface/Thread 控制、发布和空阅读面，仅将聊天连接、外部顶栏和辅助资源
替换为类型化边界，验证挂载/输入连续、精确 Thread 互斥和 Session 清理、控制
稳定、DM 读取失败三种影响及显式刷新、DM/Room 子任务来源和缺源关闭。没有
新增视觉/浏览器/宿主测试、Gallery 或产品服务；样式规则保持现有唯一 owner。

486 项清单现为 222 pending、128 in_progress、108 improved、22 retained、
6 removed；公共 UI 仍 118 项。两处装配改进、主内容装配审查后保留，未把其
所有聊天/文件/编辑器叶子或整个 Room 页面宣称完成；Goal 继续，仅本地提交。


定向 4 文件 20 项与 typecheck 首轮通过，见 /tmp/nexus-a98-target.log、
/tmp/nexus-a98-types.log。最终隔离 npm run check 首轮通过 lint、typecheck、
485 项合同、278 文件的 1293 项组件/模型测试及 build，见 /tmp/nexus-a98-check.log。
基线 4d0c830d3，9 文件快照 /tmp/nexus-a98-review.json；全部 486 项源哈希一致。
只有既有 effect-ref lint 警告与大分块提示，没有新增测试警告。通过后仅追加
本段审计证据，提交前核对源文件、暂存区和受测快照字节；不推送。


## A99 — Agent 身份字段布局与配置动作行

身份页标签原先依赖整窗 sm/lg 断点，在宽窗口的窄辅助面板中仍会强制双列；
现在同一标签区通过 CSS auto-fit/minmax 按容器宽度排布，15rem 为正常最小
列宽，更窄容器单列可缩至 100%。保持资料、业务标签、风格标签、模型、简介
阅读顺序和 dialog/inline 的 16/20px 组间距；未添加 JS 测量、断点订阅或第二套
字段树。删除 profile/model 单子项包装及其无作用 space 属性、重复 min-width
布局表项，字段根直接拥有收缩能力。创建模板、内联普通 Agent 的完整 AGENTS.md、
主 Agent 隐藏文件与默认模型、兼容编辑描述、各领域状态和命令保持原语义。

配置动作原先只有保存按钮选择尺寸，内联删除仍落到默认 36px；现将参数改为
buttonSize 并在整行共用，内联 32px/supporting、弹窗 36px/control。成功确认
删除 280px 截断和原生 title，使用公共 supporting/success 及唯一礼貌状态播报，
长文完整换行且不抢焦点；warning/error 仍复用原 Notice/RecoverySummary。
动作只执行上层给出的命令与 enabled，不增加保存事务、自动重试或删除确认。

补齐动作的双档独立键盘命令、禁用保存、可选动作省略、完整成功播报和焦点保持；
身份回归验证切换 dialog/inline 后同一字段节点及业务/风格两份待加草稿保持，
提交仍进入对应集合。继续复用既有名称反馈、模板加载、模型反馈、IME 标签与
资料文件保存回归，不增加视觉/浏览器/宿主检查、Gallery 或产品服务。唯一设计
规范、领域目录合同及 L3 同步；整个 Editor 只推进动作区切片，其他栏目另审。

486 项清单现为 221 pending、126 in_progress、111 improved、22 retained、
6 removed；公共 UI 仍 118 项。三处字段/动作视图完成代码审查，Editor 继续
in_progress；Goal 保持 active，仅本地提交。


定向 6 文件 27 项及 typecheck 首轮通过，见 /tmp/nexus-a99-target.log 和
/tmp/nexus-a99-types.log。最终隔离 npm run check 首轮通过 lint、typecheck、
485 项合同、278 文件的 1297 项组件/模型测试及 build，见 /tmp/nexus-a99-check.log。
另核对构建 CSS 已生成 repeat(auto-fit,minmax(min(100%,15rem),1fr))；这是
代码/编译证据，不代表浏览器几何或外观验收。基线 fd9217d21，14 文件快照
/tmp/nexus-a99-review.json；全部 486 项源哈希一致。只有既有 effect-ref lint
警告与大分块提示，没有新增测试警告；通过后仅追加本段证据，提交前核对工作树、
暂存区与受测快照字节，不推送。


## A100 — 公共头像入口与选择浮层

统一 Agent/Room 的公共 lg 与个人资料 xl 档位，移除个人 72px 覆盖和三个入口的私有 hover 边框；头像和辅助图标作为具名触发器中的装饰。网格选择保持正方形并可收缩到单元格，选项名称在视图本地化，纯数据模型不再输出技术名称。横向滚动选择器尚未完成本轮审查，保留原状态。

公共浮层在定位后聚焦当前或首个选项，Tab 边界延续父表单顺序；Escape/outside 继续使用公共 overlay 机制。禁用、目录或值变化关闭旧浮层；身份字段按 scopeKey 仅重建头像入口，名称字段不重建。最小高度改为零以服从真实可用视口，未改选择命令、个人保存期间权限或领域持久化。

新增离线交互测试并纳入公共门禁；不做视觉、浏览器或宿主验收，不启动产品服务。486 项为 218 pending、126 in_progress、114 improved、22 retained、6 removed，公共 UI 仍 118 项；improved 仅表示代码审查与离线证据，不表示外观验收。

验证：定向 4 文件 14 项及 typecheck 通过；隔离 npm run check 通过 lint、typecheck、485 项合同、279 文件 1303 项组件/模型测试及 build。第一次受沙箱 loopback 限制，允许离线夹具监听后重跑通过。日志 /tmp/nexus-a100-check.log，快照 /tmp/nexus-a100-review.json。仅既有 lint effect-ref 和构建大分块提示。并行 WorkGraph 后端/文档改动不纳入本批提交；无视觉验收或推送。
