package imagegen

import (
	"errors"
	"path/filepath"
	"strings"
)

func normalizeInput(input GenerateInput) (GenerateInput, error) {
	input.Provider = strings.TrimSpace(input.Provider)
	input.Model = strings.TrimSpace(input.Model)
	input.Prompt = strings.TrimSpace(input.Prompt)
	input.WorkspacePath = strings.TrimSpace(input.WorkspacePath)
	input.Size = strings.TrimSpace(input.Size)
	input.Quality = strings.TrimSpace(input.Quality)
	input.Background = strings.TrimSpace(input.Background)
	input.OutputFormat = strings.ToLower(strings.TrimSpace(input.OutputFormat))
	input.FileName = strings.TrimSpace(input.FileName)
	if input.OutputCompression != nil {
		if *input.OutputCompression < 0 || *input.OutputCompression > 100 {
			return GenerateInput{}, errors.New("output_compression 必须在 0 到 100 之间")
		}
	}
	if input.Prompt == "" {
		return GenerateInput{}, errors.New("prompt 不能为空")
	}
	if input.WorkspacePath == "" {
		return GenerateInput{}, errors.New("workspace_path 不能为空")
	}
	if input.Size == "" {
		input.Size = defaultSize
	}
	if input.OutputFormat == "" {
		input.OutputFormat = defaultOutputFormat
	}
	switch input.OutputFormat {
	case "png", "jpeg", "jpg", "webp":
	default:
		return GenerateInput{}, errors.New("output_format 只支持 png、jpeg、jpg、webp")
	}
	if input.OutputFormat == "jpg" {
		input.OutputFormat = "jpeg"
	}
	return input, nil
}

func normalizeEditInput(input EditInput) (EditInput, error) {
	input.Provider = strings.TrimSpace(input.Provider)
	input.Model = strings.TrimSpace(input.Model)
	input.Prompt = strings.TrimSpace(input.Prompt)
	input.WorkspacePath = strings.TrimSpace(input.WorkspacePath)
	input.ImagePath = strings.TrimSpace(input.ImagePath)
	input.MaskPath = strings.TrimSpace(input.MaskPath)
	input.Size = strings.TrimSpace(input.Size)
	input.Quality = strings.TrimSpace(input.Quality)
	input.OutputFormat = strings.ToLower(strings.TrimSpace(input.OutputFormat))
	input.FileName = strings.TrimSpace(input.FileName)
	if input.Prompt == "" {
		return EditInput{}, errors.New("prompt 不能为空")
	}
	if input.WorkspacePath == "" {
		return EditInput{}, errors.New("workspace_path 不能为空")
	}
	if input.ImagePath == "" {
		return EditInput{}, errors.New("image_path 不能为空")
	}
	if input.OutputFormat == "" {
		input.OutputFormat = defaultOutputFormat
	}
	switch input.OutputFormat {
	case "png", "jpeg", "jpg", "webp":
	default:
		return EditInput{}, errors.New("output_format 只支持 png、jpeg、jpg、webp")
	}
	if input.OutputFormat == "jpg" {
		input.OutputFormat = "jpeg"
	}
	if input.OutputCompression != nil {
		if *input.OutputCompression < 0 || *input.OutputCompression > 100 {
			return EditInput{}, errors.New("output_compression 必须在 0 到 100 之间")
		}
	}
	return input, nil
}

func resolveWorkspaceFile(workspacePath string, relativePath string) (string, error) {
	cleanWorkspace := filepath.Clean(strings.TrimSpace(workspacePath))
	cleanRelative := filepath.Clean(strings.TrimSpace(relativePath))
	if cleanRelative == "." || cleanRelative == "" {
		return "", errors.New("图片路径不能为空")
	}
	if filepath.IsAbs(cleanRelative) || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) || cleanRelative == ".." {
		return "", errors.New("图片路径必须在当前 workspace 内")
	}
	fullPath := filepath.Join(cleanWorkspace, cleanRelative)
	rel, err := filepath.Rel(cleanWorkspace, fullPath)
	if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", errors.New("图片路径必须在当前 workspace 内")
	}
	return fullPath, nil
}
