# Scheduled Pickers

- `picker-popover.tsx` 只定义 Picker 浮层样式和内容边界，锚点生命周期复用 `shared/ui/overlay/`。
- `time-picker-column.tsx` 统一时段、小时、分钟和秒的选项列。
- `picker-types.ts` 保存有限值和有序选项描述，格式转换归 `picker-formatters.ts`。

Daily 与 SingleRun 只组合字段，不复制时间选项按钮；禁用规则由消费者通过窄函数传入。
