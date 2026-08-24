---
name: xlsx
description: >-
  只要电子表格文件（.xlsx、.xlsm、旧版 .xls、.csv、.tsv）是任务的主要输入或输出，或者需要
  被打开、读取、创建、修改，就使用本 Skill。也适用于公式、格式、图表、数据清洗和表格格式转换。
  不用于既不读取也不产出表格文件的一般数据分析、数据库流程、独立脚本或普通文档制作。
metadata:
  title: Excel 工作簿
  version: 2.0.0
  category_key: data-automation
  category_name: 数据与自动化
  recommendation: 适合读取、创建和编辑 Excel 工作簿，并保留公式、格式与可审计结构。
  tags:
    - excel
    - xlsx
    - spreadsheet-reading
    - spreadsheet-creation
    - spreadsheet-editing
---

# Excel 工作簿

| 任务 | 路径 |
|---|---|
| 快速读取 XLSX | 运行自带读取脚本，保留工作表、行列和公式信息 |
| 读取 CSV/TSV | 用 Python `csv` 或通用文本工具；保留原始字段类型约束 |
| 新建或编辑工作簿 | 用 `openpyxl` 处理公式、格式、表格和图表 |
| 大批量数据清洗 | 标准库足够时直接使用；确有需要再用 pandas |
| 旧版 `.xls` | 用 LibreOffice 转为 `.xlsx`，保留原件 |

## 工作契约

- 只读问题不保存、重算或导出工作簿。
- 写操作默认另存新文件；模板和既有格式优先于默认样式。
- 先直接使用当前环境。`import openpyxl` 失败且任务需要写文件时，取得用户同意后在隔离环境安装
  `${CLAUDE_SKILL_DIR}/requirements.txt`，不要静默修改系统 Python。

## 读取

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/read_spreadsheet.py" "<input.xlsx>"
python3 "${CLAUDE_SKILL_DIR}/scripts/read_spreadsheet.py" "<input.xlsx>" --sheet "<sheet-name>" --max-rows 1000 --max-columns 100
python3 "${CLAUDE_SKILL_DIR}/scripts/read_spreadsheet.py" "<input.xlsx>" --output "<new-output.md>"
```

脚本同时显示公式文本和文件内缓存值。关键结论要定位到工作表与单元格，并沿公式引用核对输入；
缓存可能过期，不能把它当成刚刚重算的结果。

## 创建与编辑

在 workspace 或临时目录写一个可重复运行的短 Python 脚本，再用 `openpyxl` 输出文件。

- 数值、日期、百分比和货币写成真实类型，格式代码只负责显示；ID、邮编等标识符才写文本。
- 派生结果使用可读公式并引用输入单元格，不把 Python 计算值或魔法数字写死在计算区。
- 跨表引用始终给工作表名加单引号，例如 `='Assumptions'!B5`；正确使用绝对和相对引用。
- 读取公式与缓存值时分别打开两次。绝不能保存以 `data_only=True` 打开的工作簿，否则公式会丢失。
- 编辑时延续相邻格式、公式模式、条件格式、表格和图表范围；不要对成熟模板做全表重排。
- `.xlsm` 使用 `keep_vba=True`，并保持宏格式输出。外部链接、透视表、切片器和高级图表可能在往返保存时变化，
  只修改必要区域并明确验证范围。

## 完成条件

1. 含公式时先在单独目录用 LibreOffice 重算，检查转换结果后再作为最终文件，不能直接覆盖未验证的源文件。
2. 用读取脚本回读关键范围，确认公式仍存在、缓存值已更新，并扫描 `#REF!`、`#DIV/0!`、
   `#VALUE!`、`#NAME?`、`#N/A` 等错误。
3. 用 LibreOffice 导出 PDF 或工作表预览，检查每张非空工作表的标题、表头、关键数字、图表、分页和打印区域。
4. 缺少 LibreOffice 时可以继续不依赖重算的读取；含公式的写任务必须说明结果尚未由表格引擎重算。
5. 最终只交付用户要求的 XLSX/XLSM/CSV/TSV，不附带构建脚本和预览文件。
