# general/

L4 | 父级: web/src/features/settings

## 职责

- `settings-general-section.tsx`: 按设置导航分区装配 General、外观、工作区与权限视图
- `use-general-settings-controller.ts`: 常规行为、默认模型与权限动作编排
- `use-user-preferences.ts`: 用户偏好首次读取门禁、version CAS、未知结果对账与草稿重应用
- `use-echo-settings.ts`: 主动跟进开关的同 aggregate CAS、未知结果对账与停用后在途跟进收口
- `use-default-model-preferences.ts`: Provider 模型目录请求与默认模型保存事务
- `use-desktop-settings.ts`、`use-workspace-settings.ts`: 各自独占 Section 所需的资源和桌面 Bridge 命令生命周期
- `model/`: 分别组装完整偏好、默认模型目录展示和桌面状态根快照；跨 Config 的值清洗规则归 `lib/settings/`
- `sections/`: 桌面应用、外观、行为、工作区和权限纯视图，不直接调用 API 或 Desktop Bridge
- `components/`: 默认模型与引导复位行

布局常量和分段控件来自 `../shared/settings-panel-ui.tsx`，General 不拥有跨域共享 UI。

设置行在桌面与窄屏都保留影响用户判断的短说明；说明不解释 bridge、SDK 或请求链路等内部实现。

用户偏好是默认模型值的唯一状态源，不维护选择值镜像。Provider 目录只随运行时类型变化加载，运行时动作不得主动触发第二次加载；目录响应严格按当前协议投影，不保留旧字段兜底。

Preferences 首次 GET 成功并取得持久 version 前禁止写入，不得用缓存或默认快照覆盖服务端。PATCH 必须带该 version 的 If-Match；冲突或传输结果未知时保留本页草稿、不发布全局 runtime 默认值，且只能通过精确 GET 对账后显式选择服务端版本或把本页叶子改动叠加后再保存；不自动重放 PATCH。

情绪系统是用户级显式偏好，默认关闭；常规设置只负责持久化开关，是否注入每轮情绪上下文由后端 DM/Room 运行时边界决定。

主动跟进开关复用 Preferences 单调 version，但通过独立 Echo 服务完成停用后的 attempt 收口。PUT 必须携带读取到的 Echo ETag；同页其他 Preferences 写入成功后只同步已证明的 aggregate revision，不猜测或改写 Echo 开关。冲突和结果未知先 GET 对账，关闭请求丢失回执时仍需显式重跑幂等的“停止在途跟进”阶段，不能仅凭开关已为关闭就宣告整个流程完成。

自动记忆与自动整理记忆是 nxs 的用户级显式偏好，默认开启；前者控制新对话的长期记忆抽取，后者作为 AutoDream 总开关阻止宿主后台唤醒，均不删除已有记忆或会话摘要。

默认模型行在桌面端为 Provider / Model 控件保留 300–360px 列宽，窄屏回落为整行宽度；触发器保持单行并使用足以容纳英文下行字形的行盒，展开菜单通过锚点定位的 `menuMinWidth` 保证完整名称可读。

常规页只展示桌面宿主 Bridge 返回的应用版本与构建号，并在同一区域提供日志导出；Go sidecar 的服务版本不再投影成用户可见的应用版本。Web 模式没有 Desktop Bridge 时隐藏整个桌面应用区域。

工作区分区只属于桌面应用：它通过原生宿主选择目标目录、迁移完整状态根并直接重启，Web/服务端不展示或持久化部署 workspace 路径。目录选择只回填草稿路径，必须由用户再次确认“迁移并重启”才产生数据变更。
