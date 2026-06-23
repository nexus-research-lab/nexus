package tool

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/mcp/imagegen/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	imagegensvc "github.com/nexus-research-lab/nexus/internal/service/imagegen"
)

var errImagegenServiceMissing = errors.New("图片生成服务未初始化")

const editDescription = "编辑当前 workspace 内已有位图图片，并把结果另存到 output/imagegen/。" +
	"适合背景替换、局部修改、风格调整和 mask 编辑；image_path 与 mask_path 必须是 workspace 相对路径。" +
	"Provider 和模型默认使用 Settings 中的默认生图模型。"

func edit(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "edit_image",
		Description: editDescription,
		SearchHint:  searchHintEditImage,
		AlwaysLoad:  true,
		InputSchema: editSchema(),
		Annotations: &sdktool.ToolAnnotations{OpenWorld: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			if svc == nil {
				return errorResult(errImagegenServiceMissing), nil
			}
			workspacePath, err := requireWorkspacePath(sctx)
			if err != nil {
				return errorResult(err), nil
			}
			result, payload, err := svc.EditImage(scopedToolContext(ctx, sctx), imagegensvc.EditInput{
				Prompt:            stringArg(args, "prompt"),
				WorkspacePath:     workspacePath,
				ImagePath:         stringArg(args, "image_path"),
				MaskPath:          stringArg(args, "mask_path"),
				Size:              stringArg(args, "size"),
				Quality:           stringArg(args, "quality"),
				OutputFormat:      stringArg(args, "output_format"),
				OutputCompression: intPointerArg(args, "output_compression"),
				FileName:          stringArg(args, "file_name"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return imageResult("edit_image", result, len(payload)), nil
		},
	}
}

func imageResult(action string, result *imagegensvc.Result, payloadBytes int) sdktool.ToolResult {
	item := map[string]any{}
	if result != nil {
		item = map[string]any{
			"provider":  result.Provider,
			"model":     result.Model,
			"path":      result.Path,
			"mime_type": result.MIMEType,
			"markdown":  result.Markdown,
		}
		if result.Size != "" {
			item["size"] = result.Size
		}
		if result.RevisedPrompt != "" {
			item["revised_prompt"] = result.RevisedPrompt
		}
	}
	return jsonResult(map[string]any{
		"domain":        "imagegen",
		"action":        action,
		"item":          item,
		"payload_bytes": payloadBytes,
	})
}
