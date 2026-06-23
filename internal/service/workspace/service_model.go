package workspace

// FileEntry 表示 workspace 文件树条目。
type FileEntry struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	IsDir      bool   `json:"is_dir"`
	Size       *int64 `json:"size,omitempty"`
	ModifiedAt string `json:"modified_at"`
	Depth      int    `json:"depth"`
}

// FileContent 表示 workspace 文件内容。
type FileContent struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// EntryMutationResponse 表示创建/删除返回。
type EntryMutationResponse struct {
	Path string `json:"path"`
}

// EntryRenameResponse 表示重命名返回。
type EntryRenameResponse struct {
	Path    string `json:"path"`
	NewPath string `json:"new_path"`
}

// UploadResult 表示上传文件结果。
type UploadResult struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}
