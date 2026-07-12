package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/imagegen/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	imagegensvc "github.com/nexus-research-lab/nexus/internal/service/imagegen"
)

const generateDescription = "生成一张位图图片并保存到当前 Agent workspace 的 output/imagegen/。" +
	"适合照片、插画、产品图、UI mockup、hero 图、游戏素材等 raster asset。" +
	"Provider 和模型默认使用 Settings 中的默认生图模型；工具返回 workspace 相对路径、Markdown、MIME type、Provider 和模型。"

func generate(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "generate_image",
		Description: generateDescription,
		SearchHint:  searchHintGenerateImage,
		InputSchema: generateSchema(),
		Annotations: &sdktool.ToolAnnotations{OpenWorld: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			if svc == nil {
				return errorResult(errImagegenServiceMissing), nil
			}
			workspacePath, err := requireWorkspacePath(sctx)
			if err != nil {
				return errorResult(err), nil
			}
			result, payload, err := svc.GenerateImage(scopedToolContext(ctx, sctx), imagegensvc.GenerateInput{
				Prompt:            stringArg(args, "prompt"),
				WorkspacePath:     workspacePath,
				Size:              stringArg(args, "size"),
				Quality:           stringArg(args, "quality"),
				Background:        stringArg(args, "background"),
				OutputFormat:      stringArg(args, "output_format"),
				OutputCompression: intPointerArg(args, "output_compression"),
				FileName:          stringArg(args, "file_name"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return imageResult("generate_image", result, len(payload)), nil
		},
	}
}
