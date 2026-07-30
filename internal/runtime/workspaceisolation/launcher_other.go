//go:build !linux

package workspaceisolation

import (
	"errors"
	"os"
)

func validateLauncherBinary(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return errors.New("runtime launcher 必须是可执行普通文件")
	}
	return nil
}
