# Task Schedule

- 调度草稿与选择器展开状态分别使用一个对象维护。
- 时间转换和 cron 解析保持纯函数；日/周与每月 cron 只映射完整匹配的可视化形状，其他标准五段表达式必须原样保留到自定义 Cron 编辑态。
- 打开单次执行选择器不得覆盖已初始化的未来时间。
- 计划类型以 UiSegmentedControl 的可见组名和填满栏宽布局展示，选项按内容分配宽度；计划字段用实例级身份关联 UiField，星期组由 Field 关联说明，间隔数值/单位各自具名并并排对齐。周期分组使用 UiPanel，输入沿用公共默认尺寸，原始 Cron 使用 code 文字角色；不在计划视图重新声明字号、字重或任意圆角。

mutation 提示保留 effect 区别：not_applied 使用 danger，accepted/committed/unknown 使用 warning，不把待核对或已提交结果渲染为明确失败。
