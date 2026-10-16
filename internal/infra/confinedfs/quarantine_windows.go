//go:build windows

// INPUT: 固定目录根、持久原生对象身份与单段名称。
// OUTPUT: 原句柄不覆盖移入回收区，逐对象拒绝reparse/硬链并通过原句柄删除。
// POS: Windows宿主清理原语；不根据路径缺失推断已完成。
package confinedfs

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"runtime"
	"strings"
	"unsafe"
)

func windowsObjectIdentity(h windows.Handle) (string, bool, error) {
	var basic windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &basic); err != nil {
		return "", false, err
	}
	if basic.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return "", false, ErrSymlink
	}
	if basic.NumberOfLinks != 1 {
		return "", false, ErrHardlink
	}
	var id struct {
		Volume uint64
		ID     [16]byte
	}
	if err := windows.GetFileInformationByHandleEx(h, windows.FileIdInfo, (*byte)(unsafe.Pointer(&id)), uint32(unsafe.Sizeof(id))); err != nil {
		return "", false, err
	}
	if id.ID == ([16]byte{}) || basic.CreationTime == (windows.Filetime{}) {
		return "", false, errors.New("Windows persistent file identity unavailable")
	}
	return fmt.Sprintf("windows-v1:%016x:%x:%08x%08x", id.Volume, id.ID, basic.CreationTime.HighDateTime, basic.CreationTime.LowDateTime), basic.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0, nil
}

func (r *Root) WindowsDirectoryIdentity() (string, error) {
	f, err := r.Open(".")
	if err != nil {
		return "", err
	}
	defer f.Close()
	id, dir, err := windowsObjectIdentity(windows.Handle(f.Fd()))
	if err != nil {
		return "", err
	}
	if !dir {
		return "", errors.New("Windows identity is not a directory")
	}
	return id, nil
}

func windowsNativeChild(parent windows.Handle, name string, remove bool) (windows.Handle, error) {
	return windowsNativeOpen(parent, name, remove, false)
}

func windowsNativeOpen(parent windows.Handle, name string, remove, flush bool) (windows.Handle, error) {
	if name == "" || (name == "." && !flush) || name == ".." || strings.ContainsAny(name, "/\\:\x00") {
		return 0, ErrParentTraversal
	}
	units, err := windows.UTF16FromString(name)
	if err != nil {
		return 0, err
	}
	units = units[:len(units)-1]
	if len(units) > 32766 {
		return 0, ErrParentTraversal
	}
	type unicodeString struct {
		Length, MaximumLength uint16
		Buffer                *uint16
	}
	relative := unicodeString{uint16(len(units) * 2), uint16(len(units) * 2), &units[0]}
	attrs := struct {
		Length            uint32
		Root              windows.Handle
		Name              *unicodeString
		Flags             uint32
		Security, Quality uintptr
	}{Root: parent, Name: &relative}
	attrs.Length = uint32(unsafe.Sizeof(attrs))
	var result windows.Handle
	statusBlock := struct{ Status, Information uintptr }{}
	access := uintptr(windows.SYNCHRONIZE | windows.FILE_READ_ATTRIBUTES | windows.FILE_LIST_DIRECTORY)
	if remove {
		access |= windows.DELETE
	}
	if flush {
		access |= windows.FILE_GENERIC_WRITE
	}
	status, _, _ := windows.NewLazySystemDLL("ntdll.dll").NewProc("NtCreateFile").Call(uintptr(unsafe.Pointer(&result)), access, uintptr(unsafe.Pointer(&attrs)), uintptr(unsafe.Pointer(&statusBlock)), 0, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, 1, 0x200020, 0, 0)
	runtime.KeepAlive(units)
	runtime.KeepAlive(attrs)
	if int32(status) < 0 {
		return 0, fmt.Errorf("open fixed Windows child: NTSTATUS %#x", status)
	}
	return result, nil
}

// FlushWindowsDirectory refuses to publish durable removal when the filesystem
// cannot flush its directory metadata. There is no success fallback.
func (r *Root) FlushWindowsDirectory() error {
	file, err := r.Open(".")
	if err != nil {
		return err
	}
	defer file.Close()
	handle, err := windowsNativeOpen(windows.Handle(file.Fd()), ".", false, true)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	return windows.FlushFileBuffers(handle)
}

// WindowsQuarantinedDirectory retains the exact renamed root without delete
// sharing. Close never means deletion succeeded.
type WindowsQuarantinedDirectory struct {
	file     *os.File
	identity string
}

func (r *Root) OpenWindowsQuarantinedDirectory(name, expected string) (*WindowsQuarantinedDirectory, error) {
	parent, err := r.Open(".")
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	h, err := windowsNativeChild(windows.Handle(parent.Fd()), name, true)
	if err != nil {
		return nil, err
	}
	id, dir, err := windowsObjectIdentity(h)
	if err != nil || !dir || id != expected {
		windows.CloseHandle(h)
		return nil, errors.Join(ErrChanged, err)
	}
	return &WindowsQuarantinedDirectory{file: os.NewFile(uintptr(h), name), identity: id}, nil
}

func (r *Root) QuarantineWindowsDirectory(name string, destination *Root, target, expected string) (*WindowsQuarantinedDirectory, error) {
	if target == "" || target == "." || target == ".." || strings.ContainsAny(target, "/\\:\x00") {
		return nil, ErrParentTraversal
	}
	object, err := r.OpenWindowsQuarantinedDirectory(name, expected)
	if err != nil {
		return nil, err
	}
	parent, err := destination.Open(".")
	if err != nil {
		object.Close()
		return nil, err
	}
	defer parent.Close()
	units, err := windows.UTF16FromString(target)
	if err != nil {
		object.Close()
		return nil, err
	}
	units = units[:len(units)-1]
	header := struct {
		Replace uint32
		Root    windows.Handle
		Length  uint32
		Name    [1]uint16
	}{}
	offset := int(unsafe.Offsetof(header.Name))
	buffer := make([]byte, offset+len(units)*2)
	info := (*struct {
		Replace uint32
		Root    windows.Handle
		Length  uint32
	})(unsafe.Pointer(&buffer[0]))
	info.Root = windows.Handle(parent.Fd())
	info.Length = uint32(len(units) * 2)
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(&buffer[offset])), len(units)), units)
	ioStatus := struct{ Status, Information uintptr }{}
	status, _, _ := windows.NewLazySystemDLL("ntdll.dll").NewProc("NtSetInformationFile").Call(object.file.Fd(), uintptr(unsafe.Pointer(&ioStatus)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 10)
	runtime.KeepAlive(buffer)
	if int32(status) < 0 {
		object.Close()
		return nil, fmt.Errorf("quarantine fixed Windows directory: NTSTATUS %#x", status)
	}
	id, dir, err := windowsObjectIdentity(windows.Handle(object.file.Fd()))
	if err != nil || !dir || id != expected {
		object.Close()
		return nil, errors.Join(ErrChanged, err)
	}
	return object, nil
}

func (o *WindowsQuarantinedDirectory) Close() error {
	if o == nil || o.file == nil {
		return nil
	}
	err := o.file.Close()
	o.file = nil
	return err
}

// Delete returns success only after each real opened object accepted deletion
// and its owning handle closed. A retry finding no root cannot reproduce proof.
func (o *WindowsQuarantinedDirectory) Delete(ctx context.Context) error {
	if o == nil || o.file == nil {
		return errors.New("Windows quarantine owner unavailable")
	}
	budget := 100000
	if err := deleteWindowsObject(ctx, o.file, o.identity, 0, &budget); err != nil {
		return err
	}
	o.file = nil
	return nil
}

func deleteWindowsObject(ctx context.Context, file *os.File, expected string, depth int, budget *int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if depth > 128 || *budget <= 0 {
		return errors.New("Windows cleanup exceeds bounded tree")
	}
	*budget--
	id, dir, err := windowsObjectIdentity(windows.Handle(file.Fd()))
	if err != nil {
		return err
	}
	if expected != "" && id != expected {
		return ErrChanged
	}
	if dir {
		for {
			entries, readErr := file.ReadDir(128)
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				return readErr
			}
			for _, entry := range entries {
				h, err := windowsNativeChild(windows.Handle(file.Fd()), entry.Name(), true)
				if err != nil {
					return err
				}
				child := os.NewFile(uintptr(h), entry.Name())
				err = deleteWindowsObject(ctx, child, "", depth+1, budget)
				if err != nil {
					return errors.Join(err, child.Close())
				}
			}
			if errors.Is(readErr, io.EOF) {
				break
			}
		}
	}
	// FileDispositionInfoEx DELETE|POSIX_SEMANTICS removes the actual link
	// immediately. Legacy delete-pending may wait for unrelated query handles
	// and therefore cannot prove namespace removal when this owner closes.
	disposition := uint32(3)
	if err := windows.SetFileInformationByHandle(windows.Handle(file.Fd()), 21, (*byte)(unsafe.Pointer(&disposition)), 4); err != nil {
		return err
	}
	return file.Close()
}
