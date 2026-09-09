# provider-settings/

L4 | 父级: web/src/features/settings

## 职责

- `provider-settings-panel.tsx`: Provider 设置入口与视图装配
- `provider-settings-api.ts`: 私有与公共 Provider API 族选择
- `workspace/`: Provider 列表资源、请求代次和纯状态迁移
- `use-provider-settings-controller.ts`: Workspace、配置动作、模型动作、纯展示投影装配，以及 CC Switch 默认选择后的运行时偏好刷新
- `actions/`: 唯一命令互斥；`config/` 管理配置事务，`model/` 管理模型交互与命令
- `model/`: 预设、配置、模型列表和展示映射的纯模型；展示模型统一产出标题、格式能力、端点和徽标状态
- `components/`: 侧栏、按字段组拆分的配置表单、共享禁用状态的详情头和按 Header/Row/Toggle 分层的模型列表；详情头只显示 Provider 名称、真实状态与操作，不展示预设宣传说明或未本地化描述
- `dialogs/`: 新增模型、模型参数和删除占用确认；实例级标题/字段、公共限高与正文滚动；模型能力目录复用同一设置行，保存动作在忙碌时保留名称

Provider 列表、选中项、表单模式和草稿属于同一个 workspace，刷新时必须原子替换；
桌面 Provider 工作区保持左侧目录、右侧详情；窄屏改为上方双列限高目录、下方完整宽度详情，不能用固定 190px 侧栏挤压表单。
CC Switch 返回默认选择后必须刷新运行时偏好快照，让对话与后台任务模型立即生效；
Workspace 刷新必须按请求代次提交，过期结果不得写状态、反馈或全局可用性缓存；
删除弹窗使用带类型的单一状态，不增加目标、确认框和占用框的平行布尔状态；
Provider 表单弹窗使用 plain chrome：添加模型只显示 Model ID 与启用开关；模型能力使用设置行而非图标卡；占用删除只显示目标、真实后果与 Agent 名称，不暴露内部 Agent ID。
三个弹窗使用 `adaptiveMax` 与唯一可滚动 Body；模型标识和占用名称允许完整换行。初始焦点由公共 Backdrop 管理，字段文字尺寸与 code 字体由 UiField/Input/Textarea 的角色拥有，不添加私有字段壳或字号。
所有异步命令共享一个基于 ref 的互斥入口，不增加镜像 busy/submitting 状态。
模型与测试动作只依赖 `PersistProvider` 窄命令，不读取配置动作控制器的完整状态。
模型同步、添加、更新和测试分别声明所需的 API 子集，不依赖完整模型 API 门面。
侧栏目录、格式选项、标题和能力标志只由纯展示模型推导，控制器与面板不得重复解释 Provider 规则。
Provider 图标按资源表解析；没有已知资源时统一回退名称首字母，不渲染空蒙版。
Provider 的名称、凭证、可编辑端点与形态/格式标签按实例显式关联公共控件，DOM 字段 ID 不参与配置 identity 或保存载荷。固定端点是具名只读组，复用静态 UiListRow，不伪装成输入。
`components/provider-settings-config-form.css` 只拥有按详情栏实际宽度切换的字段网格；形态字段与凭证/端点统一使用公共 md 控件，名称在中等宽度独占首行，宽栏才呈现三列，长标签可换行且同一行控件底部对齐。
Provider 标题、字段名、说明、状态、模型标识和图标首字母只选择 App Typography 语义角色；实际字体、字号、行高、字重与 tracking 由 `shared/ui/typography` 和主题 recipe 统一拥有。状态、计数与格式标识复用 `UiBadge`，业务文件不得拼接局部文字或徽标配方。
Provider 目录加载与模型启停使用共享 `md` Spinner，Header、按钮和弹窗命令使用 `sm`；静止状态保留动作图标，业务文件不得自行拼旋转、颜色或 reduced-motion class。
Provider 预设用 `endpoint_mode` 区分固定目录端点、资源级 Base URL 与完全自定义端点；Azure 只开放资源 Base URL，deployment name 通过手工添加模型进入模型卡。
内置 Provider 侧栏按英文显示名排序；目录声明顺序不承担展示顺序语义。

- `ProviderSettingsPanel` 只输出设置和运营壳层内的管理内容；`layout` 控制页面或分区留白，`visibilityScope` 保持私有/公共资源范围。没有独立 Provider Header、单项 Settings Tab 或 embedded 分支；上层继续持有唯一页面导航。

- 详情头完整显示对象名，窄工作面允许状态与动作换行；状态复用公共 Badge。测试由公共 Button + Action Menu 明确提交，方向键只浏览候选；精确 Provider 身份、候选身份、编辑/权限/忙碌边界变化消费旧打开态，名称/语言刷新保留。真实测试命令与互斥仍属于 actions；目录为空时禁用测试入口。

Provider 目录/详情以自身 720px 工作面宽度作为横向分栏边界，设置和运营嵌入态复用；详情不再叠加额外横向内边距，字段沿统一内容轴排布。
