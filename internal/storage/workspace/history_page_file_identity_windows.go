//go:build windows

// INPUT: 已通过 confinedfs 打开的 canonical history 普通文件。
// OUTPUT: 跨 append 保持稳定、替换文件时变化的 volume/file-index 身份。
// POS: history page source 替换检测的 Windows 文件身份边界。
package workspace

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func historyPageFileIdentity(file *os.File, _ os.FileInfo) string {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &info); err != nil {
		return ""
	}
	return fmt.Sprintf(
		"%d:%d:%d",
		info.VolumeSerialNumber,
		info.FileIndexHigh,
		info.FileIndexLow,
	)
}
