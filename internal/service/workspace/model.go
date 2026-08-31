// INPUT: workspace 文件系统中已经校验的路径、正文与内容 revision。
// OUTPUT: HTTP/服务层共享的文件树、正文与修改结果模型。
// POS: workspace 传输模型；revision 只描述正文版本，不承担业务身份。
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
	Path     string `json:"path"`
	Content  string `json:"content"`
	Revision string `json:"revision"`
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
