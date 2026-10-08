//go:build windows

// INPUT: 普通 Windows 用户可创建的目录 junction 与根外哨兵文件。
// OUTPUT: 读取、目录打开与复制拒绝 reparse 跳转，删除链接不修改根外内容。
// POS: 宿主资源目录边界原生回归，不依赖符号链接特权或跳过测试。
package confinedfs

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// TestWindowsConfinedFSRejectsJunction 使用真实 mount-point reparse point 验证普通用户路径攻击。
func TestWindowsConfinedFSRejectsJunction(t *testing.T) {
	rootPath, outside := t.TempDir(), t.TempDir()
	sentinel := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(sentinel, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	createWindowsTestJunction(t, filepath.Join(rootPath, "junction"), outside)
	root, err := Open(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := root.ReadFile("junction/secret.txt"); err == nil {
		t.Fatalf("read through junction: %v", err)
	}
	if opened, err := root.OpenRootNoSymlink("junction"); err == nil {
		if opened != nil {
			_ = opened.Close()
		}
		t.Fatalf("open junction: %v", err)
	}
	destination, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close()
	if err := destination.CopyTreeFrom(root); err == nil {
		t.Fatalf("copy junction: %v", err)
	}
	if err := root.RemoveAll("junction"); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(sentinel); err != nil || string(body) != "outside" {
		t.Fatalf("junction removal modified outside sentinel: %q, %v", body, err)
	}
}

// createWindowsTestJunction 只为测试自有的空目录设置 mount-point buffer，不提升权限。
func createWindowsTestJunction(t *testing.T, link, target string) {
	t.Helper()
	if err := os.Mkdir(link, 0o700); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(link)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	substitute, err := windows.UTF16FromString(`\??\` + target)
	if err != nil {
		t.Fatal(err)
	}
	printed, err := windows.UTF16FromString(target)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 16+2*(len(substitute)+len(printed)))
	binary.LittleEndian.PutUint32(buffer[0:4], windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buffer[4:6], uint16(len(buffer)-8))
	binary.LittleEndian.PutUint16(buffer[10:12], uint16(2*(len(substitute)-1)))
	binary.LittleEndian.PutUint16(buffer[12:14], uint16(2*len(substitute)))
	binary.LittleEndian.PutUint16(buffer[14:16], uint16(2*(len(printed)-1)))
	for index, unit := range append(substitute, printed...) {
		binary.LittleEndian.PutUint16(buffer[16+index*2:], unit)
	}
	var returned uint32
	if err := windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &buffer[0], uint32(len(buffer)), nil, 0, &returned, nil); err != nil {
		t.Fatal(err)
	}
}
