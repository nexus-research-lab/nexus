# default-models/

L4 | 父级: web/src/features/settings

## 职责

- `settings-default-models-section.tsx`: 模型设置独立入口与纯视图，集中展示默认对话、生图、视觉理解、后台任务模型。
- `use-default-model-preferences.ts`: 当前 owner 与 runtime 的 Provider 模型目录读取、显式重试及模型选择保存编排；复用 `general/use-user-preferences.ts` 的版本化事务。
- `default-model-preferences-model.ts`: 按用途投影可选模型、推荐排序和显示值，编码与解码精确 Provider / Model 选择。
- `settings-default-model-row.tsx`: 复用共享选择器的四类模型行；模型行不维护业务快照或恢复状态。

用户偏好是保存选择的唯一状态源。当前目录未包含已保存的精确 Provider / Model 时，显示值为空；目录清空后四类选择器均展示空态，不回填失效模型，不自动改写偏好，不借用其他 Provider 的同名模型。原选择重新进入可用目录时恢复显示。无显式对话或生图偏好时，仅采用后端返回且仍在可用目录中的对应默认选择。

模型目录随 owner、运行时类型和显式重试加载，迟到响应须匹配当前 owner 代次；同一作用域读取失败保留最后成功快照，并提供分区级重试。Provider 启停、换 Key、清除 Key 与模型卡管理属于相邻 `provider-settings/`，模型页不复制凭据动作。

选择保存复用 Preferences 的版本、首次读取门禁与未知结果对账，不发布第二份选择镜像；加载、保存、未对账写入期间禁用选择，目录读取期间也禁用选择及重复重试。所有保存操作共用模型角色锁，避免同页并发覆盖。

模型行只显示一次标题和说明，不重复提供“服务 / 模型”标签。桌面端选择器占 300px 列宽，窄屏为整行；菜单布局、键盘、焦点及忙碌样式由共享控件拥有。

共置行为测试覆盖中英文可访问名称、四类用途的精确选择分派、加载与保存锁、目录重试防重，以及目录清空和恢复时保留偏好的空值展示。
