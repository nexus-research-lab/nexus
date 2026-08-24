---
name: pptx
description: >-
  只要 PowerPoint 演示文稿或模板（.pptx、.potx、旧版 .ppt）是任务的输入或输出，或者需要
  被打开、读取、创建、修改，就使用本 Skill。用户提到幻灯片、deck、演示文稿或 PowerPoint
  文件时也应触发，包括内容提取、模板套用、页面增删、图表、图片和演讲者备注。
metadata:
  title: PowerPoint 演示文稿
  version: 2.0.0
  category_key: content-docs
  category_name: 内容与文档
  recommendation: 适合读取、创建和编辑 PowerPoint，并延续模板、母版与页面结构。
  tags:
    - powerpoint
    - pptx
    - presentation-reading
    - presentation-creation
    - presentation-editing
---

# PowerPoint 演示文稿

| 任务 | 路径 |
|---|---|
| 读取、总结、问答 | 运行自带读取脚本；视觉信息再看逐页预览 |
| 新建演示文稿 | 用 `python-pptx` 生成可编辑 PPTX |
| 编辑现有演示文稿或套模板 | 先检查全部页面和模板结构，再局部修改 |
| 复杂母版、动画、SmartArt、媒体 | 优先保留现有 OOXML；只做已验证的定点修改 |
| 旧版 `.ppt` | 用 LibreOffice 转为 `.pptx`，保留原件 |

## 工作契约

- 只读问题不重新保存或导出演示文稿。
- 写操作默认另存新文件，不覆盖源稿或模板。
- 用户提供的模板是视觉和结构权威；不要擅自换母版、主题、字体或配色。
- 先直接使用当前环境。`import pptx` 失败且任务需要写文件时，取得用户同意后在隔离环境安装
  `${CLAUDE_SKILL_DIR}/requirements.txt`，不要静默修改系统 Python。

## 读取

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/read_presentation.py" "<input.pptx>"
python3 "${CLAUDE_SKILL_DIR}/scripts/read_presentation.py" "<input.pptx>" --output "<new-output.md>"
```

脚本按演示顺序提取正文、表格和演讲者备注。图表、图片、SmartArt、构图和只含图片的页面仍要
查看渲染结果；未提取到文字不等于页面为空。

## 创建与编辑

在 workspace 或临时目录写一个可重复运行的短 Python 脚本，再用 `python-pptx` 输出文件。

- 新建前确定受众、场景和主结论；信息足够时自行选择页数和叙事，不为装饰性偏好阻塞任务。
- 有模板时复用其母版、版式和占位符；编辑局部元素，不在每页重造共同结构。
- 每页表达一个清晰结论。内容过多时先精简或换布局，不靠缩小字号硬塞。
- 编辑现有文本时修改目标 run；给 `text_frame.text` 整体赋值会清除段落和字符格式。
- 图表尽量保持 PowerPoint 原生可编辑；图片保持纵横比并检查裁切，详细说明放演讲者备注。
- `python-pptx` 不能安全保留的动画、SmartArt、媒体或特殊母版结构，不得静默重建或丢弃。

## 完成条件

1. 用读取脚本回读最终文件，确认页序、正文、表格和备注完整，并搜索残留占位文字。
2. 用 LibreOffice 转成 PDF，再用 `pdftoppm` 渲染全部页面：

```bash
soffice --headless --convert-to pdf --outdir "<empty-qa-dir>" "<output.pptx>"
pdftoppm -png -r 144 "<empty-qa-dir>/<output.pdf>" "<empty-qa-dir>/slide"
```

3. 逐页检查标题换行、文字溢出、裁切、重叠、对齐、对比度、图表标签、页脚和图片清晰度。
4. 缺少 LibreOffice 或 Poppler 时可以继续内容读取；写任务要说明未完成视觉检查。
5. 最终只交付 PPTX，不附带构建脚本、提取文本和预览图片。
