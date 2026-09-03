# Scheduled Pickers

- `picker-popover.tsx` 只定义 Picker 浮层样式和内容边界，锚点生命周期复用 `shared/ui/overlay/`。
- `picker-trigger.tsx` 统一日期/时间字段触发器，必须复用 `UiButton` 并暴露 `aria-expanded`/`aria-haspopup`；禁止页面复制大号加号触发器。
- `time-picker-column.tsx` 统一时段、小时、分钟和秒的选项列。
- `picker-types.ts` 保存有限值和有序选项描述，格式转换归 `picker-formatters.ts`。

Daily 与 SingleRun 只组合字段，不复制时间选项按钮；禁用规则由消费者通过窄函数传入。
月份导航统一使用 `UiIconButton`，日期和时间选项统一使用 `UiChoiceButton`；业务 Picker 不得直接导入共享样式投影或手写 button 字号、字重与圆角。
