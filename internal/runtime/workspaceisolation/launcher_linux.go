//go:build linux

package workspaceisolation

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func validateLauncherBinary(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("读取 runtime launcher 元数据: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 ||
		info.Mode().Perm()&0o022 != 0 {
		return errors.New("runtime launcher 必须是 root-owned 且不可被 group/other 写入的可执行文件")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 {
		return errors.New("runtime launcher 必须由 root 持有")
	}
	if info.Mode()&os.ModeSetuid == 0 {
		return errors.New("非 root Nexus server 必须使用 setuid runtime launcher")
	}
	return nil
}
