# CLI Reference

本文件只描述 `nexusctl imagegen` 的 Agent 调用方式。Provider 配置、鉴权、接口兼容和响应解析都由 Nexus Go 服务处理，不属于 skill 的判断职责。

## 命令

- `generate`：根据 prompt 生成新图片。
- `edit`：根据 prompt 编辑现有图片，可选 mask。

`nexusctl imagegen` 不会把凭据放进命令行参数。它会从 Settings 的图片生成 Provider 读取配置，并把请求交给后端图片生成服务。

## 生成图片

```bash
nexusctl imagegen generate \
  --prompt "A red fox in an autumn forest" \
  --size 1024x1024 \
  --quality low \
  --output-format png \
  --file-name fox
```

普通生成优先使用 `--prompt`。只有非常长或包含大量引号/换行的 prompt 才写入文件；写文件时必须使用绝对 workspace 和绝对 prompt 文件路径：

```bash
WORKSPACE="/absolute/workspace/path"
PROMPT_FILE="$WORKSPACE/tmp/imagegen/prompt.txt"
mkdir -p "$WORKSPACE/tmp/imagegen"
printf '%s\n' "A long production prompt" > "$PROMPT_FILE"
nexusctl imagegen generate \
  --workspace-path "$WORKSPACE" \
  --prompt-file "$PROMPT_FILE" \
  --size 1024x1024 \
  --quality low \
  --output-format png \
  --file-name fox
```

## 编辑图片

```bash
nexusctl imagegen edit \
  --image-path image_to_edit.png \
  --mask-path mask.png \
  --prompt "Make this black and white" \
  --output-format png \
  --file-name edited
```

## 常用参数

- `--prompt` / `--prompt-file`：二选一。
- `--provider`：可选；不传则使用默认图片 Provider。
- `--workspace-path`：可选；不传则使用当前目录。
- `--size`：例如 `1024x1024`。
- `--quality`：由 Provider 支持情况决定，常用 `low` 做草稿。
- `--output-format`：常用 `png`，透明后处理资产优先 `png` 或 `webp`。
- `--output-compression`：可选，0 到 100。
- `--file-name`：不含路径的稳定文件名；不传时服务会根据 prompt 生成。
- `--image-path`：编辑目标，workspace 相对路径。
- `--mask-path`：编辑 mask，workspace 相对路径，可选。

## 输出

CLI 输出 JSON，其中：

- `item.path`：最终 workspace 相对路径。
- `item.markdown`：可直接展示的 Markdown 图片语法。
- `payload_bytes`：图片字节数。

最终回复必须使用 Markdown 图片语法展示图片，不能只返回代码格式路径。
不要再读取 `item.path` 指向的二进制图片文件；CLI 输出已经足够用于展示和回复。
