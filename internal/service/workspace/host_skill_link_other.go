//go:build !windows

// INPUT: 非 Windows 宿主 Skill 顶层目录项的路径与元数据。
// OUTPUT: 该目录项是否需要按受控目录链接解析。
// POS: 宿主 Skill 安全快照的系统适配层。
package workspace

import "os"

func isHostSkillDirectoryLink(_ string, info os.FileInfo) (bool, error) {
	return info.Mode()&os.ModeSymlink != 0, nil
}
