// Package appdelta 基于 Yovoice 的 bsdiff/qsufsort 重建完整安装包。
// 分块独立生成以限制发布机内存，重建结果必须通过完整安装包的 SHA-256 校验。
package appdelta

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
)

const magic = "Nexus package delta 1\n"
const blockSize = 4 << 20
const maxPackage = 2 << 30
const literal = ^uint32(0)

func Create(old, target *os.File, dst io.Writer) error {
	info, err := target.Stat()
	if err != nil {
		return err
	}
	if info.Size() <= 0 || info.Size() > maxPackage {
		return errors.New("unsupported package size")
	}
	if _, err = io.WriteString(dst, magic); err != nil {
		return err
	}
	if err = binary.Write(dst, binary.LittleEndian, uint64(info.Size())); err != nil {
		return err
	}
	for offset := int64(0); offset < info.Size(); offset += blockSize {
		data := make([]byte, min(int64(blockSize), info.Size()-offset))
		if _, err = target.ReadAt(data, offset); err != nil {
			return err
		}
		base := make([]byte, blockSize)
		n, err := old.ReadAt(base, offset)
		if err != nil && err != io.EOF {
			return err
		}
		base = base[:n]
		var payload []byte
		var length uint32
		if !bytes.Equal(base, data) {
			payload = data
			length = literal
			if n > 0 {
				if patch := Diff(base, data); len(patch) < len(data) {
					payload = patch
					length = uint32(len(patch))
				}
			}
		}
		if err = binary.Write(dst, binary.LittleEndian, length); err != nil {
			return err
		}
		if _, err = dst.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

func Apply(old *os.File, patch io.Reader, dst io.Writer) error {
	header := make([]byte, len(magic))
	if _, err := io.ReadFull(patch, header); err != nil || string(header) != magic {
		return errDamagedPatch
	}
	var size int64
	if err := binary.Read(patch, binary.LittleEndian, &size); err != nil || size <= 0 || size > maxPackage {
		return errDamagedPatch
	}
	info, err := old.Stat()
	if err != nil {
		return err
	}
	for offset := int64(0); offset < size; offset += blockSize {
		n := min(int64(blockSize), size-offset)
		baseSize := max(int64(0), min(int64(blockSize), info.Size()-offset))
		base := io.NewSectionReader(old, offset, baseSize)
		var length uint32
		if err = binary.Read(patch, binary.LittleEndian, &length); err != nil {
			return err
		}
		switch length {
		case 0:
			_, err = io.CopyN(dst, base, n)
		case literal:
			_, err = io.CopyN(dst, patch, n)
		default:
			if int64(length) >= n {
				return errDamagedPatch
			}
			data := make([]byte, length)
			if _, err = io.ReadFull(patch, data); err == nil {
				err = Patch(dst, n, base, baseSize, bytes.NewReader(data), int64(length))
			}
		}
		if err != nil {
			return err
		}
	}
	var end [1]byte
	if _, err = io.ReadFull(patch, end[:]); err != io.EOF {
		return errDamagedPatch
	}
	return nil
}
