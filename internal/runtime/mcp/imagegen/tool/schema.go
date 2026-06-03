package tool

func generateSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "图片生成提示词。包含主体、风格、构图、文字约束和禁止项。",
			},
			"size": map[string]any{
				"type":        "string",
				"description": "可选图片尺寸，例如 1024x1024、1792x1024；留空使用默认值。",
			},
			"quality": map[string]any{
				"type":        "string",
				"description": "可选质量参数，由当前 Provider 支持情况决定，例如 low、medium、high、auto。",
			},
			"background": map[string]any{
				"type":        "string",
				"description": "可选背景参数，OpenAI 兼容 Provider 可使用 auto、transparent 或 opaque。",
			},
			"output_format": outputFormatSchema(),
			"output_compression": map[string]any{
				"type":        "integer",
				"minimum":     0,
				"maximum":     100,
				"description": "可选输出压缩质量，0 到 100。",
			},
			"file_name": map[string]any{
				"type":        "string",
				"description": "可选稳定文件名，不需要扩展名。工具会保存到 output/imagegen/。",
			},
		},
		"required":             []string{"prompt"},
		"additionalProperties": false,
	}
}

func editSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "图片编辑提示词。必须明确修改范围和保持不变的内容。",
			},
			"image_path": map[string]any{
				"type":        "string",
				"description": "当前 workspace 内待编辑图片的相对路径。",
			},
			"mask_path": map[string]any{
				"type":        "string",
				"description": "可选 mask 图片的 workspace 相对路径。",
			},
			"size": map[string]any{
				"type":        "string",
				"description": "可选图片尺寸，例如 1024x1024。",
			},
			"quality": map[string]any{
				"type":        "string",
				"description": "可选质量参数，由当前 Provider 支持情况决定。",
			},
			"output_format":      outputFormatSchema(),
			"output_compression": compressionSchema(),
			"file_name": map[string]any{
				"type":        "string",
				"description": "可选稳定文件名，不需要扩展名。工具会保存到 output/imagegen/。",
			},
		},
		"required":             []string{"prompt", "image_path"},
		"additionalProperties": false,
	}
}

func outputFormatSchema() map[string]any {
	return map[string]any{
		"type":        "string",
		"enum":        []string{"png", "jpeg", "webp"},
		"description": "输出图片格式；留空默认 png。",
	}
}

func compressionSchema() map[string]any {
	return map[string]any{
		"type":        "integer",
		"minimum":     0,
		"maximum":     100,
		"description": "可选输出压缩质量，0 到 100。",
	}
}
