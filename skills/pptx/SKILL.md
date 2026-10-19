---
name: pptx
description: >-
  只要 PowerPoint 演示文稿或模板（.pptx、.potx、旧版 .ppt）是任务的输入或输出，或者需要
  被打开、读取、创建、修改，就使用本 Skill。用户提到幻灯片、deck、演示文稿或 PowerPoint
  文件时也应触发，包括内容提取、模板套用、页面增删、图表、图片和演讲者备注。
metadata:
  title: PowerPoint 演示文稿
  version: 2.1.0
  category_key: content-docs
  category_name: 内容与文档
  recommendation: 适合读取、创建和编辑 PowerPoint，支持模板盘点、HTML 布局生成和逐页验收。
  tags:
    - powerpoint
    - pptx
    - presentation-reading
    - presentation-creation
    - presentation-editing
---

# PowerPoint 演示文稿

| 任务 | 首选路径 |
|---|---|
| 读取、总结、问答 | `read_presentation.py` 提取正文、表格和备注；视觉信息再看渲染图 |
| 新建演示文稿 | 内容简单时用 `python-pptx`；版式密集时可用 HTML/CSS 布局再转 PPTX |
| 套用现有模板 | 先运行 `inventory_presentation.py`，再选择版式和占位符 |
| 编辑现有演示文稿 | 局部修改目标 run；结构调整使用 `reorder_slides.py` 或定点 OOXML 编辑 |
| 复杂母版、动画、SmartArt、媒体 | 保留原 OOXML；只做已验证的定点修改，不静默重建 |
| 结构与关系检查 | `validate_presentation.py`；它不替代渲染验收 |
| 旧版 `.ppt` | 用 LibreOffice 转为 `.pptx`，保留原件 |

## 工作模式

先根据输入和输出选择工作模式，不要把新建页面的 HTML 工作流套到既有模板编辑上。

### 新建演示文稿

- 内容简单、图表和版式不复杂时使用 `python-pptx`，保留文本和图表的可编辑性。
- 需要精确布局时，可在临时工作目录生成固定尺寸的 HTML 页面，再用环境中已有的 HTML-to-PPTX 工具转换；这条路径是可选的，不能静默安装全局 Node、浏览器或字体依赖。细节见 [HTML 布局生成](references/html-to-pptx.md)。
- HTML 页面使用 16:9 的统一画布（建议 720×405pt），所有可见内容必须在画布内；不支持的 CSS、复杂渐变和图标先转成图片，图表和表格需要在转换后确认是否仍可编辑。

### 套用或编辑现有模板

1. 用 `read_presentation.py` 读取正文、表格和备注，用 `inventory_presentation.py` 保存页面、形状、占位符和坐标清单。
2. 检查主题字体、色板、母版和页面缩略图；模板是视觉和结构权威，不擅自换主题或母版。
3. 只修改目标文本 run、图片或占位符。需要调整页面顺序时使用 `reorder_slides.py`，输出到新文件。
4. 修改后运行 `validate_presentation.py`，再回读内容并渲染全部页面。OOXML 定点修改的关系和回滚要求见 [OOXML 编辑](references/ooxml-editing.md)。

### 视觉检查

用 `render_presentation.py` 生成 PDF 和逐页 PNG；逐页检查标题换行、文字溢出、元素重叠、边界裁切、对齐、对比度、图表标签、页脚和图片清晰度。生成图片只是证据，不等于已经通过检查。

## 工作契约

- 只读问题不重新保存或导出演示文稿。
- 写操作默认另存新文件，不覆盖源稿或模板。
- 用户提供的模板是视觉和结构权威；不要擅自换母版、主题、字体或配色。
- 先直接使用当前环境。`import pptx` 失败且任务需要写文件时，取得用户同意后在隔离环境安装
  `${CLAUDE_SKILL_DIR}/requirements.txt`，不要静默修改系统 Python。
- 外部 HTML-to-PPTX 或字体工具只在当前环境已有、且依赖许可清楚时使用；缺失时回退到
  `python-pptx` 或 OOXML 定点修改，并在交付说明中标明降级。

## 读取

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/read_presentation.py" "<input.pptx>"
python3 "${CLAUDE_SKILL_DIR}/scripts/read_presentation.py" "<input.pptx>" --output "<new-output.md>"
python3 "${CLAUDE_SKILL_DIR}/scripts/inventory_presentation.py" "<input.pptx>" --output "<inventory.json>"
python3 "${CLAUDE_SKILL_DIR}/scripts/validate_presentation.py" "<output.pptx>"
python3 "${CLAUDE_SKILL_DIR}/scripts/render_presentation.py" "<output.pptx>" --output-dir "<empty-qa-dir>"
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
- 需要按模板批量替换时，先保存 inventory 并建立显式的页面/形状映射；未命中的映射应失败，不能猜测目标。
- 只修改已确认的 XML 部件；重打包前后都验证 ZIP、XML 和关系目标，禁止使用不受控的整目录解包。

## 完成条件

1. 用读取脚本回读最终文件，确认页序、正文、表格和备注完整，并搜索残留占位文字。
2. 用 `render_presentation.py`，或用 LibreOffice 转成 PDF，再用 `pdftoppm` 渲染全部页面：

```bash
soffice --headless --convert-to pdf --outdir "<empty-qa-dir>" "<output.pptx>"
pdftoppm -png -r 144 "<empty-qa-dir>/<output.pdf>" "<empty-qa-dir>/slide"
```

3. 逐页检查标题换行、文字溢出、裁切、重叠、对齐、对比度、图表标签、页脚和图片清晰度。
4. 缺少 LibreOffice 或 Poppler 时可以继续内容读取；写任务要说明未完成视觉检查。
5. 最终只交付 PPTX，不附带构建脚本、提取文本和预览图片。
