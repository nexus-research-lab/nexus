//go:build windows

// INPUT: Windows 宿主 Skill 顶层目录项的路径与元数据。
// OUTPUT: 该目录项是否为需要受控解析的 reparse point。
// POS: 宿主 Skill 安全快照的 Windows junction 适配层。
package workspace

import (
	"os"

	"golang.org/x/sys/windows"
)

func isHostSkillDirectoryLink(path string, info os.FileInfo) (bool, error) {
	if info.Mode()&os.ModeSymlink != 0 {
		return true, nil
	}
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	attributes, err := windows.GetFileAttributes(pathPointer)
	if err != nil {
		return false, err
	}
	return attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}
