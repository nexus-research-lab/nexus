# runtime/

运行时设置分区只展示按内核隔离的开关。当前 nxs 仅暴露 ToolSearch，其他 runtime 没有可配置项时保持空状态。

- `use-runtime-settings-controller.ts` 负责用户偏好加载、runtime 切换和 ToolSearch 更新。
- `settings-runtime-section.tsx` 只负责运行时设置的展示，不直接调用 API。
- `model/` 只保存 runtime 选择项等纯展示模型，不依赖 General 分区。
