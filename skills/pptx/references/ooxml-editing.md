# OOXML 定点编辑

PPTX 是一个 ZIP 容器。既有模板中的母版、主题、动画、媒体和关系通常比纯文本更重要，因此复杂编辑应尽量只修改确认过的 XML 部件。

## 编辑前

- 先保留原文件，运行 `read_presentation.py` 和 `inventory_presentation.py`。
- 用 `unzip -l` 或 ZIP API 确认目标部件存在，记录目标 slide、shape、relationship ID。
- 只允许从容器内部解析关系目标；目标越出根目录、指向不存在部件或使用外部关系时停止。

## 编辑中

- 优先修改单个文本 run、已知图片关系或 `presentation.xml` 中的页面顺序。
- 不要用整体替换清空未知段落格式，也不要删除未识别的扩展、媒体或备注关系。
- 每次修改只处理一个语义批次；输出写入临时文件，完成验证后再移动到新路径。

## 编辑后

运行：

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/validate_presentation.py" "<output.pptx>"
python3 "${CLAUDE_SKILL_DIR}/scripts/read_presentation.py" "<output.pptx>"
python3 "${CLAUDE_SKILL_DIR}/scripts/render_presentation.py" "<output.pptx>" --output-dir "<empty-qa-dir>"
```

关系和 XML 验证通过后仍必须看渲染结果。`validate_presentation.py` 只证明容器和关系可继续处理，不证明 PowerPoint 的布局、动画或字体呈现正确。
