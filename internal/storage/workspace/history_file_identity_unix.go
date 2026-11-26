//go:build !windows

// INPUT: 已通过 confinedfs 打开的 canonical history 普通文件。
// OUTPUT: 跨 append 保持稳定、替换文件时变化的设备与 inode 身份；lstat 结果同样可用。
// POS: history 翻页源替换检测的 Unix 文件身份边界。
package workspace

import (
	"fmt"
	"os"
	"syscall"
)

func historyPageFileIdentity(_ *os.File, info os.FileInfo) string {
	return statFileIdentity(info)
}

// statFileIdentity 只依赖 stat 结果，供未打开文件的 lstat 快速校验使用。
func statFileIdentity(info os.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return ""
	}
	return fmt.Sprintf("%d:%d", uint64(stat.Dev), uint64(stat.Ino))
}
