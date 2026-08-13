---
name: docx
title: Word 文档读取
version: 1.0.0
category_key: content-docs
category_name: 内容与文档
tags:
  - word
  - docx
  - document-reading
description: >-
  读取和分析 Word 文档，包括标准 .docx、旧版 .doc，以及扩展名为 .docx
  但内容实际是旧版 .doc 的文件。用户上传、引用或要求读取、总结、审阅、
  提取或分析任何 Word 文件时使用；不要用通用 Read 直接读取 Word 二进制文件。
recommendation: 适合读取、提取和分析 Word 文档，也能识别后缀与实际格式不一致的文件。
---

# Word 文档读取

Word 文件是二进制容器，不能直接用通用 `Read` 工具读取。先按文件内容识别格式，
再提取文本；不要只相信 `.doc` 或 `.docx` 后缀。

## 读取流程

1. 找到上传附件在 workspace 中的真实路径。
2. 运行随 Skill 提供的脚本：

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/read_word.py" "<word-file>"
```

3. 文档较长时，把结果写入新的文本文件，再分段读取：

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/read_word.py" "<word-file>" --output "<new-output.txt>"
```

脚本会按文件签名处理两类内容：

- 标准 DOCX：直接用 Python 标准库读取 OOXML 正文和表格，不依赖 `python-docx` 或
  `pandoc`。
- 旧版 DOC/OLE：优先用 LibreOffice 转为 DOCX；macOS 没有 LibreOffice 时回退到
  系统自带的 `textutil`。扩展名伪装成 `.docx` 也走这条路径。

## 边界

- 转换器缺失时，明确请用户安装 LibreOffice，或把文件另存为真正的 DOCX/PDF；
  不要把依赖缺失说成文档损坏。
- 未提取到文字时，文档可能只有扫描图片。说明当前没有可验证的文本，并请用户
  提供可搜索版本或 PDF；不要猜测图片里的内容。
- 本 Skill 只负责读取和提取，不承诺保持 Word 排版，也不直接覆盖原文件。
