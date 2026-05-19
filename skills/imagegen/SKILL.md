---
name: imagegen
title: Image Generation
description: Generate or edit raster images when the task benefits from AI-created bitmap visuals such as photos, illustrations, textures, sprites, mockups, UI mockups, product shots, or transparent-background cutouts. Use when the result should be an image asset rather than repo-native SVG, HTML/CSS, or canvas.
scope: any
tags: [image, asset, generation, design]
---

# Image Generation Skill

为当前 workspace 生成或编辑位图资产。Nexus 只有一个执行入口：`nexusctl imagegen`。Provider、鉴权、接口兼容和响应解析全部由 Go 服务负责，skill 只负责组织 prompt、调用 CLI、返回结果。

## 快路径

普通单图生成必须走快路径。

- skill 内只调用 1 次 Bash。
- 不读取 references。
- 不创建 prompt 文件。
- 不读取输出图片。
- 不做目录探测。
- 不使用 Write、Read、LS、Glob、TaskOutput。
- 不主动用 high quality。

命令模板：

```bash
nexusctl imagegen generate \
  --prompt "A concise production prompt" \
  --size 1024x1024 \
  --quality low \
  --output-format png \
  --file-name stable-name
```

如果用户明确要横幅、封面或宽图，使用 `--size 1792x1024`，仍保持 `--quality low`。如果图片服务返回 429/过载，最多再调用 1 次 Bash，降低尺寸或保持 low 重试。

CLI JSON 的 `item.path` 和 `item.markdown` 是结果真相源。不要打开生成后的 PNG/JPG/WebP 文件验证，这会把二进制或 base64 塞回上下文。

## 判断

- 没有输入图片：默认 `generate`。
- 用户要求改图、局部替换、合成、mask：使用 `edit`。
- 用户提供图片只是做风格或构图参考：仍使用 `generate`，除非明确要求编辑原图。
- 多个不同资产使用多次 CLI 调用，每个资产一条 prompt；不要把多资产塞进一个 prompt。
- 需求更适合 SVG、HTML/CSS、Canvas 或现有矢量源时，不用本 skill。

## 生成

把用户描述整理成一条不超过 600 字符的 `--prompt`。用户描述已经具体时不要读取任何 reference。

```bash
nexusctl imagegen generate \
  --prompt "Nexus AI assistant promotional banner, futuristic dark blue background, glowing neural network core, large clean NEXUS text, premium technology style" \
  --size 1024x1024 \
  --quality low \
  --output-format png \
  --file-name nexus-promo-banner
```

只有 prompt 很长、包含大量引号/换行，或复杂编辑时才用 prompt 文件。必须在同一个 Bash 命令内写入文件，并对 `--workspace-path` 与 `--prompt-file` 使用绝对路径：

```bash
WORKSPACE="/absolute/workspace/path"
PROMPT_FILE="$WORKSPACE/tmp/imagegen/prompt.txt"
mkdir -p "$WORKSPACE/tmp/imagegen"
printf '%s\n' "Long production prompt" > "$PROMPT_FILE"
nexusctl imagegen generate \
  --workspace-path "$WORKSPACE" \
  --prompt-file "$PROMPT_FILE" \
  --size 1024x1024 \
  --quality low \
  --output-format png \
  --file-name asset-name
```

不要用 Write 写 prompt 文件；它可能要求先 Read，容易引入无效轮次。

## 编辑

```bash
nexusctl imagegen edit \
  --image-path input.png \
  --mask-path mask.png \
  --prompt "Make this black and white" \
  --output-format png \
  --file-name edited-image
```

`--image-path` 和 `--mask-path` 使用 workspace 相对路径。只有路径不明确时才做一次必要的文件查找。

## 回复

最终回复保持极简，只给 CLI 返回的 markdown 和路径。不要输出尺寸、大小、配色说明、过程说明或下一步建议，除非用户明确询问。

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
