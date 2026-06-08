---
name: imagegen
title: Image Generation
description: Generate or edit raster images when the task benefits from AI-created bitmap visuals such as photos, illustrations, textures, sprites, mockups, UI mockups, product shots, or transparent-background cutouts. Use when the result should be an image asset rather than repo-native SVG, HTML/CSS, or canvas.
scope: any
tags: [image, asset, generation, design]
---

# Image Generation Skill

为当前 workspace 生成或编辑位图资产。默认使用内置 `nexus_imagegen` MCP 工具；Provider、鉴权、接口兼容和响应解析全部由 Go 服务负责。`nexusctl imagegen` 只作为显式 CLI 兜底和手工调试入口。

## 快路径

普通单图生成必须走内置工具快路径。

- 调用 `generate_image`，不要调用 Bash。
- 不读取 references。
- 不创建 prompt 文件。
- 不读取输出图片。
- 不做目录探测。
- 不使用 Write、Read、LS、Glob、TaskOutput。
- 不主动用 high quality。

工具入参模板：

```json
{
  "prompt": "A concise production prompt",
  "size": "1024x1024",
  "quality": "low",
  "output_format": "png",
  "file_name": "stable-name"
}
```

如果用户明确要横幅、封面或宽图，使用 `size=1792x1024`，仍保持 `quality=low`。如果图片服务返回 429/过载，最多再调用 1 次工具，降低尺寸或保持 low 重试。

工具返回的 `item.path` 和 `item.markdown` 是结果真相源。不要打开生成后的 PNG/JPG/WebP 文件验证，这会把二进制或 base64 塞回上下文。
不要在工具入参里选择 Provider 或模型，让 Settings 里的默认生图模型决定；只有用户明确要求 CLI/provider/model 控制时，才使用 `nexusctl imagegen` 兜底。

## 判断

- 没有输入图片：默认 `generate`。
- 用户要求改图、局部替换、合成、mask：使用 `edit`。
- 用户提供图片只是做风格或构图参考：仍使用 `generate`，除非明确要求编辑原图。
- 多个不同资产使用多次 `generate_image` 调用，每个资产一条 prompt；不要把多资产塞进一个 prompt。
- 需求更适合 SVG、HTML/CSS、Canvas 或现有矢量源时，不用本 skill。

## 生成

把用户描述整理成一条不超过 600 字符的 `prompt`。用户描述已经具体时不要读取任何 reference。

```json
{
  "prompt": "Nexus AI assistant promotional banner, futuristic dark blue background, glowing neural network core, large clean NEXUS text, premium technology style",
  "size": "1024x1024",
  "quality": "low",
  "output_format": "png",
  "file_name": "nexus-promo-banner"
}
```

只有用户明确选择 CLI 模式，或需要 provider/model 覆盖时，才参考 `references/cli.md` 使用 `nexusctl imagegen`。

## 编辑

```json
{
  "image_path": "input.png",
  "mask_path": "mask.png",
  "prompt": "Make this black and white",
  "output_format": "png",
  "file_name": "edited-image"
}
```

调用 `edit_image`；`image_path` 和 `mask_path` 使用 workspace 相对路径。只有路径不明确时才做一次必要的文件查找。

## 回复

最终回复保持极简，只给工具返回的 markdown 和路径。不要输出尺寸、大小、配色说明、过程说明或下一步建议，除非用户明确询问。

```markdown
已生成：`output/imagegen/nexus-promo-banner.png`

![Nexus 宣传图](output/imagegen/nexus-promo-banner.png)
```

## 复杂任务参考

普通单图生成不要读取 references。只有复杂多资产、透明背景、强文字约束或编辑失败排查时再打开对应文件：

- `references/prompting.md`
- `references/sample-prompts.md`
- `references/cli.md`
- `scripts/remove_chroma_key.py`
