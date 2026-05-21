package syslimit

// OpenFilesLimitSnapshot 描述当前进程文件句柄限制状态。
type OpenFilesLimitSnapshot struct {
	Soft   uint64
	Hard   uint64
	Raised bool
}

// EnsureOpenFilesLimit 默认不调整限制；支持的平台会在 init 中替换为真实实现。
var EnsureOpenFilesLimit = func(uint64) (OpenFilesLimitSnapshot, error) {
	return OpenFilesLimitSnapshot{}, nil
}
