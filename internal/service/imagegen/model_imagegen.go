package imagegen

// GenerateInput 表示图片生成请求。
type GenerateInput struct {
	Provider          string
	Prompt            string
	WorkspacePath     string
	Size              string
	Quality           string
	Background        string
	OutputFormat      string
	OutputCompression *int
	FileName          string
}

// EditInput 表示图片编辑请求。
type EditInput struct {
	Provider          string
	Prompt            string
	WorkspacePath     string
	ImagePath         string
	MaskPath          string
	Size              string
	Quality           string
	OutputFormat      string
	OutputCompression *int
	FileName          string
}

// Result 表示已落盘的图片生成结果。
type Result struct {
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	Path          string `json:"path"`
	MIMEType      string `json:"mime_type"`
	Size          string `json:"size,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
	Markdown      string `json:"markdown"`
}
