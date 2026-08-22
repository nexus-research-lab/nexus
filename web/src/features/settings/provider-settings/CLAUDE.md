# provider-settings/

L4 | 父级: web/src/features/settings

## 职责

- `provider-settings-panel.tsx`: Provider 设置入口与视图装配
- `provider-settings-api.ts`: 私有与公共 Provider API 族选择
- `workspace/`: Provider 列表资源、请求代次和纯状态迁移
- `use-provider-settings-controller.ts`: Workspace、配置动作、模型动作、纯展示投影装配，以及 CC Switch 默认选择后的运行时偏好刷新
- `actions/`: 唯一命令互斥；`config/` 管理配置事务，`model/` 管理模型交互与命令
- `model/`: 预设、配置、模型列表和展示映射的纯模型；展示模型统一产出标题、格式能力、端点和徽标状态
- `components/`: 侧栏、按字段组拆分的配置表单、共享禁用状态的详情头和按 Header/Row/Toggle 分层的模型列表
- `dialogs/`: 新增模型、模型参数和删除占用确认

Provider 列表、选中项、表单模式和草稿属于同一个 workspace，刷新时必须原子替换；
CC Switch 返回默认选择后必须刷新运行时偏好快照，让对话与后台任务模型立即生效；
Workspace 刷新必须按请求代次提交，过期结果不得写状态、反馈或全局可用性缓存；
删除弹窗使用带类型的单一状态，不增加目标、确认框和占用框的平行布尔状态；
Provider 表单弹窗使用 plain chrome：添加模型只显示 Model ID 与启用开关；模型能力使用设置行而非图标卡；占用删除只显示目标、真实后果与 Agent 名称，不暴露内部 Agent ID。
所有异步命令共享一个基于 ref 的互斥入口，不增加镜像 busy/submitting 状态。
模型与测试动作只依赖 `PersistProvider` 窄命令，不读取配置动作控制器的完整状态。
模型同步、添加、更新和测试分别声明所需的 API 子集，不依赖完整模型 API 门面。
侧栏目录、格式选项、标题和能力标志只由纯展示模型推导，控制器与面板不得重复解释 Provider 规则。
Provider 图标按资源表解析；没有已知资源时统一回退名称首字母，不渲染空蒙版。
Provider 预设用 `endpoint_mode` 区分固定目录端点、资源级 Base URL 与完全自定义端点；Azure 只开放资源 Base URL，deployment name 通过手工添加模型进入模型卡。
内置 Provider 侧栏按英文显示名排序；目录声明顺序不承担展示顺序语义。
