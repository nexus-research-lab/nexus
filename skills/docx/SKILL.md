---
name: docx
description: >-
  只要 Word 文档或模板（.docx、.dotx、旧版 .doc）是任务的输入或输出，或者需要被打开、
  读取、创建、修改，就使用本 Skill。也适用于 Word 内的表格、图片、批注、修订和格式调整，
  以及要求以 Word 交付的报告、备忘录、信函和模板。不用于 PDF、电子表格或不涉及 Word 文件的普通写作。
metadata:
  title: Word 文档
  version: 2.0.0
  category_key: content-docs
  category_name: 内容与文档
  recommendation: 适合读取、创建和编辑 Word 文档，并保留既有格式和可编辑结构。
  tags:
    - word
    - docx
    - document-reading
    - document-creation
    - document-editing
---

# Word 文档

按任务选择最短可靠路径：

| 任务 | 路径 |
|---|---|
| 读取、总结、问答 | 运行自带读取脚本；版式相关问题再渲染页面 |
| 新建文档 | 用 `python-docx` 生成可编辑 DOCX |
| 编辑现有文档 | 先读内容和版式，再用 `python-docx` 做局部修改 |
| 批注、修订、内容控件等高级结构 | 仅在库不能安全保留时做定点 OOXML 修改 |
| 旧版 `.doc` | 用 LibreOffice 转为 `.docx`，保留原件 |

## 工作契约

- 只读问题不重新保存或转换文件，除非读取确实需要转换。
- 写操作默认另存新文件；用户明确要求覆盖时才覆盖。
- 用户提供的模板、样式和版式优先于本 Skill 的默认建议。
- 先直接使用当前环境。`import docx` 失败且任务需要写文件时，取得用户同意后在隔离环境安装
  `${CLAUDE_SKILL_DIR}/requirements.txt`，不要静默修改系统 Python。

## 读取

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/read_word.py" "<input.docx>"
python3 "${CLAUDE_SKILL_DIR}/scripts/read_word.py" "<input.docx>" --output "<new-output.txt>"
```

脚本按文件签名识别 OOXML 与旧版 OLE，不只相信扩展名。它提取正文和表格；页眉页脚、图片、
批注、修订或版式决定答案时，还要查看渲染页面或相关 OOXML，不能从纯文本猜测。

## 创建与编辑

在 workspace 或临时目录写一个可重复运行的短 Python 脚本，再用 `python-docx` 输出文件。

- 新建前确认文档用途、读者和目标格式；信息足够时直接判断，不为装饰性选择阻塞任务。
- 使用 Word 样式、编号、分页符、分节、表格和页眉页脚表达结构，不用空格、手工圆点或连续空行伪造布局。
- 编辑已有文本时优先修改目标 run；给 `paragraph.text` 整体赋值会丢失段内格式。
- 表格使用明确列宽并处理重复表头；图片保持纵横比并设置可读尺寸。
- 遇到宏、内容控件、复杂域、批注或修订时，先确认往返保存是否会丢数据。无法安全保留就停止写入，
  说明限制或改用定点 OOXML 修改，不能静默降级。

## 完成条件

1. 用读取脚本回读最终文件，确认内容、顺序和关键表格正确。
2. 新建或涉及版式的编辑，用 LibreOffice 转成 PDF，再用 `pdftoppm` 生成页面图片并逐页检查：

```bash
soffice --headless --convert-to pdf --outdir "<empty-qa-dir>" "<output.docx>"
pdftoppm -png -r 144 "<empty-qa-dir>/<output.pdf>" "<empty-qa-dir>/page"
```

3. 修复截字、重叠、异常空白页、断裂表格、字体替换和错误页眉页脚后重新渲染。
4. 若 LibreOffice 或 Poppler 不可用，仍可完成纯内容读取；写任务要明确说明未完成视觉检查，不能声称版式已验证。
5. 最终只交付用户要求的 DOCX，不附带构建脚本和预览文件。
