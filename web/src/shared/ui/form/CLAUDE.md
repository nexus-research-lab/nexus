# 表单原语

- 本目录拥有选择项、原生复选框、复选行、表单控件和分段控制器。
- 生产 TS/TSX 文件必须在首条代码前保留真实、非空的 `INPUT / OUTPUT / POS` 合同；`scripts/frontend-file-contract.test.mjs` 递归检查本所有者，新文件同样受约束。
- 这里只处理通用输入语义，不维护业务草稿或提交事务。
- `source-text-styles.ts` 是源码编辑、流式写入与无高亮纯文本/分段预览共用的字体、字号和行高所有者；预览不复制测量字体常量。普通文本正文与分段正文还共用其预览滚动/内嵌焦点配方；消费者负责具名 region、键盘可聚焦语义和外层空间，不把 HTML iframe 包进第二个滚动面。
- `UiSourceEditor` 是工作区、AGENTS.md 和记忆源码的无边框文本编辑原语，统一等宽 14px/24px、内部滚动、只读/禁用及内嵌键盘焦点；它保留 native textarea 的值、选区、IME 和 Tab 行为，不接管草稿、自动保存、快捷键或退出编辑。Field 关联复用内部 accessibility owner，源码控件不得从业务覆盖字体或焦点。
- `UiField.labelAction` 将字段操作放在 label 的兄弟节点，窄宽度允许换行；可见 label 不得包裹按钮。预览等非输入内容使用具名 group，进入编辑后再绑定 exact 控件 ID。
- `UiField` 的精确标签、说明与错误关联合同归 `docs/specs/frontend-engineering-spec.md`；`field-accessibility.ts` 是 Field、原生输入与 Select trigger 共用的内部上下文，不允许业务自行导入后写第二套属性投影。
- 默认字段与搜索文字使用 App `control` 角色、常规字重；紧凑尺寸是有意保留的密度档位，不由消费者覆盖字号。尺寸和行高只在 `form-control-styles.ts` 投影。
- 普通字段与搜索字段的占位文字统一使用 `--text-muted`；空白可用状态必须保持可读，不能用装饰性的 soft tone 代替。Gallery 的空白字段矩阵验证实际主题表面的文字对比。
- Input/Textarea 的 `textRole="code"` 用于路径、命令标识与源码模板，保留共享尺寸并使用等宽字体；单行 `textRole="verification"` 拥有验证码的加大居中命中区与字距。内容角色只改变展示，不推断 `type`、`inputMode`、长度、自动填充或校验规则，不转换输入值。
- 公共表单调用方的 `className / inputClassName / style` 同样受静态视觉覆盖门禁约束；搜索壳与内部输入都不能私设字号、字重、颜色或焦点状态。
- `UiInput / UiTextarea / UiNativeSelect` 分别拥有普通文本、多行文本和原生下拉字段；`form-control-styles.ts` 是这些 primitive 的内部投影，业务层不得导入后再手写 DOM。嵌入 Composer 等复合控件内部的无壳原生字段必须由该 pattern 明确拥有，不能假装成普通 Field。
- `UiSearchInput` 自己持有可本地化的共享 IconButton 清除动作，并用具名 `searchbox` 语义替代宿主语言生成的原生 search shadow 控件；`surface / dialog / menu / toolbar` 分别覆盖页面、表单、菜单首行和复合工具条，消费者不得靠 class 重做这些壳层。搜索壳不是 `<label>`，不得把清除按钮嵌入另一个 labelable control；消费者不得另造清除按钮。
- `search-query.ts` 是客户端搜索的唯一字符串语义：统一做 `NFKC`、去除首尾空白和稳定小写化，默认空查询直通，并允许业务选择包含或前缀匹配。业务模型必须显式声明可搜索字段、空查询结果、类别/权限/状态条件以及搜索范围，不得再手写 `trim().toLowerCase().includes(...)`。侧栏搜索默认只筛当前导航数据，不承诺下探子目录；本地、远端或未来跨域深搜由具体页面的资源 controller 决定，其 debounce、请求取消和最短长度也留在该层。
- `SidebarSearchField` 只统一侧栏搜索壳层和可选动作，不持有业务状态；可见短提示由公共翻译提供，消费者的完整 `label` 作为搜索框可访问名称。字号继承 `UiSearchInput`，不另设密度；`SidebarSearchAction` 组合 `UiIconButton`，只拥有与搜索框配套的桌面/触屏几何和图标尺寸，状态、焦点、禁用和唯一 Tooltip 归公共按钮，消费者只传业务图标与命令。
- `UiChoiceButton` 持有按钮式选择，`surface / picker / calendar / icon` 分别覆盖文字选项、紧凑选择器、日历单元格和图片图标；`UiRadioChoice` 持有互斥表单选择的原生 radio、整项热区、焦点和禁用语义。`choice-styles.ts` 只是二者的内部视觉投影，业务不得导入后手写第二套 button 或 label/input DOM。生成式问答等稳定领域 Widget 可以按自身合同保留原生选项。
- Choice 的四种 variant 共用单一焦点/禁用配方和公共 opacity/ring；数字选择共用 primary 前景/背景，surface 与普通控件共用字号角色。disabled 与 fieldset 禁用由原生元素决定，不再由样式参数复制状态或移除 pointer events；`choice.test.tsx` 覆盖全部按钮 variant 的键盘/表单/禁用和 native radio 互斥。
- `UiCheckbox` 是所有普通原生 checkbox 的尺寸、品牌色、焦点、`indeterminate` mixed 语义与 disabled 入口；带说明或整行热区的选择继续组合 `UiCheckboxRow`，其 `default / compact` 密度分别对应标准表单与紧凑设置。生成式问答等自绘选择器不属于该原语。
- `checkbox-row.tsx` 直接拥有行密度、装饰图标、实例名称/说明和原生 disabled 视觉；调用方的显式名称与额外描述保留。`checkbox-row.test.tsx` 覆盖两档整行点击、Space、实例隔离、描述更新和 input/fieldset 禁用；最终几何与主题验收归 Gallery 浏览器案例。
- `UiRemovableChip` 是标签输入和多选字段中“已选实体 + 移除动作”的唯一原语；实体集合由业务持有，移除必须是具名 native IconButton。复合选择器的菜单触发器与移除按钮必须是兄弟节点，禁止把 `span role=button` 或真实 button 嵌入另一个 button。
- `UiSegmentedControl` 是有限互斥选项的唯一入口；选中态使用背景与文字对比，不加阴影，普通设置不使用胶囊圆角。业务页面只提供选项、当前值和尺寸密度，不得再定义私有分段选择器。
- 需要可见组名时使用 `UiSegmentedControl showLabel`；它用 UiField 显示 title，且只由该 Field 命名一个 group。消费者不再包裹 label 或重复命名的 Field；未开启时仍由控件自己的 group 提供名称。
- stretch 填满可用宽度但按内容伸缩，避免长短标签必须等分宽度；无论是否 stretch，受限容器中的长名称都允许换行。
- 分段选项的 icon/text 使用同一个行内布局；全局 `focus-visible` 继续拥有键盘焦点，不能被选中底面的 `box-shadow: none` 清除。禁用选项继续显示当前值，但不触发 hover 背景或变更命令。
- 分段常规/紧凑文字与最小高度直接在该组件投影，长名称可换行并随组等高；纯图标选项组合 UiTooltip，组不产生重复原生 title。仅供 Gallery 装饰的组图标参数及独占 recipe/token 已移除，产品选项图标继续保留；样式门禁覆盖其消费者，独立回归验证原生/fieldset 禁用和键盘提示。
- `form-controls.test.tsx` 与 `removable-chip.test.tsx` 以真实 invalid/input、select、键盘和点击事件覆盖 Field、Input、NativeSelect、SearchInput、Checkbox/CheckboxRow、Choice/RadioChoice、RemovableChip 与 SegmentedControl 的 ARIA/状态合同。

- UiSearchInput 直接供目录和页面复用，不再通过 WorkspaceSearchInput 透传包装；未指定 placeholder 时采用当前语言的 common.search，可访问名称默认与它一致，显式 undefined 不能覆盖掉回退名称。显式名称、占位文案、Field 关联和受控查询仍由调用方传入。

- UiCheckbox 的 indeterminate 是受控展示：原生点击意图先交给 onChange，随后 DOM 保持最近已提交的 mixed 属性；父级在回调内同步提交时以新值为准，不能把视觉与 aria-checked 拆成两份状态。字段标签、checked 与批量选择集合仍归调用方。
