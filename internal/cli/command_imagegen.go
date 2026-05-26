package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/service/imagegen"

	"github.com/spf13/cobra"
)

const nexusctlWorkspacePathEnvName = "NEXUSCTL_WORKSPACE_PATH"

func newImagegenCommand(services *cliServiceProvider) *cobra.Command {
	command := &cobra.Command{
		Use:   "imagegen",
		Short: "图片生成 CLI",
		Long:  "通过 Settings 中的图片生成 Provider 调用系统 imagegen skill，输出 workspace 相对路径与元数据。",
	}
	command.AddCommand(newImagegenGenerateCommand(services))
	command.AddCommand(newImagegenEditCommand(services))
	return command
}

func newImagegenGenerateCommand(services *cliServiceProvider) *cobra.Command {
	var input imagegen.GenerateInput
	var promptFile string
	command := &cobra.Command{
		Use:   "generate",
		Short: "生成图片",
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt, err := resolveImagegenPrompt(input.Prompt, promptFile)
			if err != nil {
				return err
			}
			input.Prompt = prompt
			workspacePath, err := resolveImagegenWorkspacePath(input.WorkspacePath)
			if err != nil {
				return err
			}
			input.WorkspacePath = workspacePath
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			result, payload, err := appServices.Imagegen.GenerateImage(commandContext(cmd), input)
			if err != nil {
				return err
			}
			return emitImagegenResult("generate", result, len(payload))
		},
	}
	bindImagegenCommonFlags(command, &input.Provider, &input.Model, &input.WorkspacePath, &input.Prompt, &promptFile, &input.Size, &input.Quality, &input.OutputFormat, &input.OutputCompression, &input.FileName)
	command.Flags().StringVar(&input.Background, "background", "", "OpenAI 兼容背景参数")
	return command
}

func newImagegenEditCommand(services *cliServiceProvider) *cobra.Command {
	var input imagegen.EditInput
	var promptFile string
	command := &cobra.Command{
		Use:   "edit",
		Short: "编辑图片",
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt, err := resolveImagegenPrompt(input.Prompt, promptFile)
			if err != nil {
				return err
			}
			input.Prompt = prompt
			workspacePath, err := resolveImagegenWorkspacePath(input.WorkspacePath)
			if err != nil {
				return err
			}
			input.WorkspacePath = workspacePath
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			result, payload, err := appServices.Imagegen.EditImage(commandContext(cmd), input)
			if err != nil {
				return err
			}
			return emitImagegenResult("edit", result, len(payload))
		},
	}
	bindImagegenCommonFlags(command, &input.Provider, &input.Model, &input.WorkspacePath, &input.Prompt, &promptFile, &input.Size, &input.Quality, &input.OutputFormat, &input.OutputCompression, &input.FileName)
	command.Flags().StringVar(&input.ImagePath, "image-path", "", "workspace 内待编辑图片路径")
	command.Flags().StringVar(&input.MaskPath, "mask-path", "", "workspace 内 mask 图片路径")
	_ = command.MarkFlagRequired("image-path")
	return command
}

func bindImagegenCommonFlags(
	command *cobra.Command,
	provider *string,
	model *string,
	workspacePath *string,
	prompt *string,
	promptFile *string,
	size *string,
	quality *string,
	outputFormat *string,
	outputCompression **int,
	fileName *string,
) {
	command.Flags().StringVar(provider, "provider", "", "可选图片生成 Provider key")
	command.Flags().StringVar(model, "model", "", "可选图片生成模型；指定时需同时指定 --provider")
	command.Flags().StringVar(workspacePath, "workspace-path", "", "workspace 绝对路径；缺省使用当前目录")
	command.Flags().StringVar(prompt, "prompt", "", "图片生成/编辑 prompt")
	command.Flags().StringVar(promptFile, "prompt-file", "", "从文件读取 prompt")
	command.Flags().StringVar(size, "size", "", "图片尺寸，例如 1024x1024")
	command.Flags().StringVar(quality, "quality", "", "质量参数，例如 low、medium、high、auto")
	command.Flags().StringVar(outputFormat, "output-format", "png", "输出图片格式：png|jpeg|webp")
	command.Flags().Var(newIntPointerFlag(outputCompression), "output-compression", "输出压缩质量，0 到 100")
	command.Flags().StringVar(fileName, "file-name", "", "可选文件名，不需要扩展名")
}

func resolveImagegenPrompt(prompt string, promptFile string) (string, error) {
	if prompt != "" && promptFile != "" {
		return "", usageErrorf("--prompt 与 --prompt-file 不能同时使用")
	}
	if promptFile == "" {
		return prompt, nil
	}
	payload, err := os.ReadFile(promptFile)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func resolveImagegenWorkspacePath(value string) (string, error) {
	if value != "" {
		return filepath.Abs(value)
	}
	if runtimeWorkspacePath := strings.TrimSpace(os.Getenv(nexusctlWorkspacePathEnvName)); runtimeWorkspacePath != "" {
		return filepath.Abs(runtimeWorkspacePath)
	}
	return os.Getwd()
}

func emitImagegenResult(action string, result *imagegen.Result, payloadBytes int) error {
	return emitJSON(map[string]any{
		"domain":        "imagegen",
		"action":        action,
		"item":          result,
		"payload_bytes": payloadBytes,
	})
}
