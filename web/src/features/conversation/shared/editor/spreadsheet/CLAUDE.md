# Spreadsheet Preview

- `spreadsheet-preview-model.ts` 只把 Workbook 投影为 Sheet、Row、Cell、Merge 和样式索引结构。
- `spreadsheet-cell-value.ts` 独占 ExcelJS `CellValue` 分类和文本格式化，不将封闭联合退化成 `unknown` 字段袋。
- `spreadsheet-cell-style.ts` 独占 ExcelJS 到预览样式、预览样式到 CSS 的两段外观投影，值格式化不得解释样式。
- 字号按源文件 pt 转 CSS px（96/72），不套 App 字号下限；单元格拥有自然行高、默认单行/显式换行及真实 flex 纵向对齐，不能用对普通 div 无效的 vertical-align。内容面使用 paper 背景/文字/边界 token 保持源文件颜色关系，外部工作表选择与坐标栏继续使用 App 材质和 Typography。
- `spreadsheet-grid-model.ts` 按视口解析、普通单元格投影、合并锚点投影和布局计算四个阶段生成可见网格，不重新读取 ExcelJS 对象。
- 网格输出按源行列顺序排列的行；跨出视窗的合并锚点仍只出现一次并携带受工作表范围约束的 span。只读视图使用 table/row/cell 与总行列数、源索引和 span，不声明未实现的交互 grid；滚动 region 复用公共预览配方且允许原生键盘滚动，视觉坐标栏不重复进入可访问树。DOM 语义遵循 [WAI table pattern](https://www.w3.org/WAI/ARIA/apg/patterns/table/) 与 [虚拟表格属性](https://www.w3.org/WAI/ARIA/apg/practices/grid-and-table-properties/)；宿主/辅助技术验收另行记录。
- `spreadsheet-readonly-workbook.tsx` 只编排工作表选择与虚拟网格；多工作表切换必须消费全局底线式 `UiTabs`，不得恢复表格专属胶囊或选中底色。
- 新增 CellValue 成员时扩展格式化规则表；默认规则只负责暴露空文本边界，不在视图猜测对象结构。
- `spreadsheet-file-preview.tsx` 组合上层统一加载/失败面，成功后仅向 Header 投影工作表数量，失败面独立滚动；工作簿读取与重试继续归资源 Hook。`use-spreadsheet-preview.ts` 消费上层 Office scope，文件/账号变化重置工作簿与工作表选择，旧读入或解析结果不得回写。
