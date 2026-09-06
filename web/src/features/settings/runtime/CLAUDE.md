# runtime/

运行时设置分区只展示按内核隔离的设置。当前 nxs 暴露 ToolSearch 和 SDK 内置 WebSearch，其他 runtime 没有可配置项时保持空状态。

- `use-runtime-settings-controller.ts` 复用 General 的 version CAS 偏好事务，负责 runtime 切换和 nxs 设置更新；读取失败、冲突和未知结果共用持续的对账提示。
- `settings-runtime-section.tsx` 只负责运行时设置的展示，不直接调用 API；用户可见标题使用“运行引擎 / 工具发现 / 网页搜索”等任务语言，不把 ToolSearch、WebSearch、schema 或 bridge 当作页面说明；网页搜索首屏只显示服务与必填项，其余通用参数和服务参数放在“更多设置”中，密钥字段只作为更新请求输入，不展示服务端返回的明文。
- `model/` 只保存 runtime 选择项等纯展示模型，不依赖 General 分区。
- 高级配置中的分组标题保留真实 heading 语义，复用公共 supporting / medium；分割与分组间距只在本分区的组合组件持有，不再使用大字距微标签。
- 工具发现行复用 SettingsToggleRow，以当前可见标题命名并关联说明；Runtime 控制器继续独占当前值、加载/保存禁用及更新命令。
- 私有网络与服务商提取选项直接消费 `UiCheckboxRow density="compact"`，不保留只转发属性的 SettingsCheckSetting。业务值、Provider 能力条件、精确 patch 与保存锁继续由运行设置持有。
- 输入与 Provider 选择直接组合 UiField，并用实例级 ID 精确关联；密钥输入、清除动作和获取地址互为独立节点，禁止恢复包裹复合内容的私有 SettingsField。分段选项通过 showLabel 显示一次组名，JSON 校验保留原有 blur 提交语义，错误归公共 Field，不能给全部实例共用错误 ID。
- `settings-runtime-section.test.tsx` 使用实际页面和隔离控制器命令覆盖六种服务的标签/分组、失焦规范化保存、密钥替换/清除、禁用与多实例 JSON 错误；不替代 Preferences 事务或浏览器视觉验收。
