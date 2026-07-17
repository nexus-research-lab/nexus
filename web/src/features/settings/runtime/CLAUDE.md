# runtime/

运行时设置分区只展示按内核隔离的设置。当前 nxs 暴露 ToolSearch 和 SDK 内置 WebSearch，其他 runtime 没有可配置项时保持空状态。

- `use-runtime-settings-controller.ts` 负责用户偏好加载、runtime 切换和 nxs 设置更新。
- `settings-runtime-section.tsx` 只负责运行时设置的展示，不直接调用 API；WebSearch 首屏只显示 provider 与必填项，其余 SDK 支持的通用参数和 provider 参数放在“更多设置”中，密钥字段只作为更新请求输入，不展示服务端返回的明文。
- `model/` 只保存 runtime 选择项等纯展示模型，不依赖 General 分区。
