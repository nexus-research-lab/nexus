// Package appdelta 生成和应用桌面应用的差分更新包：发布时对比两个打包好的应用目录写出补丁，
// 客户端用已安装的旧版目录重建出全新的新版目录，再交给原生宿主做签名校验和安装。
package appdelta

import (
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// 补丁格式：格式头、索引长度（uvarint）、DEFLATE 压缩的 JSON 索引，随后按索引顺序排列各文件的数据。
// 新版的每个文件从旧版复制、对旧版文件应用二进制差分，或从压缩数据解出。
// 客户端用已安装旧版自带的服务程序重建，格式只能向后兼容地扩展，不兼容时换格式头。
const deltaMagic = "nexus directory delta 1\n"

type deltaIndex struct {
	// 源版本和目标版本都必须与更新请求匹配。
	From    string       `json:"from"`
	Version string       `json:"version"`
	Entries []deltaEntry `json:"entries"`
}

// 条目使用相对安装根目录的路径。
type deltaEntry struct {
	Path string `json:"path"`
	Dir  bool   `json:"dir,omitzero"`
	// 符号链接目标。
	Link string `json:"link,omitzero"`
	// 普通文件权限。
	Mode   uint32 `json:"mode,omitzero"`
	Size   int64  `json:"size,omitzero"`
	SHA256 string `json:"sha256,omitzero"`
	// 旧文件路径为空时，数据为压缩后的完整文件。
	From string `json:"from,omitzero"`
	Data int64  `json:"data,omitzero"`
}

func (e *deltaEntry) isFile() bool { return !e.Dir && e.Link == "" }

// 生成二进制差分约需文件大小十倍的内存，超过此上限的变化文件直接压缩。
const maxDiff = 256 << 20

// 公证票据在 DMG 安装副本中可能缺失，始终完整写入。
const stapledTicket = "Contents/CodeResources"

// WriteDelta 为指定条目生成补丁，未指定条目时处理整个目录。
func WriteDelta(w io.Writer, from, to, oldDir, newDir string, entries []string) error {
	// 记录各路径的摘要及相同内容的首个文件。
	type oldFile struct {
		sum  string // SHA-256
		size int64
	}
	old := map[string]oldFile{}
	bySum := map[string]string{}
	err := filepath.WalkDir(oldDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		if relPath(oldDir, p) == stapledTicket {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		h := sha256.New()
		size, err := io.Copy(h, f)
		if err != nil {
			return err
		}
		rel, sum := relPath(oldDir, p), hex.EncodeToString(h.Sum(nil))
		old[rel] = oldFile{sum, size}
		if _, ok := bySum[sum]; !ok {
			bySum[sum] = rel
		}
		return nil
	})
	if err != nil {
		return err
	}
	if entries == nil {
		all, err := os.ReadDir(newDir)
		if err != nil {
			return err
		}
		for _, e := range all {
			entries = append(entries, e.Name())
		}
	}

	index := deltaIndex{From: from, Version: to}
	var data [][]byte
	for _, top := range entries {
		err := filepath.WalkDir(filepath.Join(newDir, top), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			e := deltaEntry{Path: relPath(newDir, p)}
			switch {
			case d.IsDir():
				e.Dir = true
			case d.Type()&fs.ModeSymlink != 0:
				if e.Link, err = os.Readlink(p); err != nil {
					return err
				}
				e.Link = filepath.ToSlash(e.Link)
			case d.Type().IsRegular():
				info, err := d.Info()
				if err != nil {
					return err
				}
				b, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				sum := sha256.Sum256(b)
				e.Mode, e.Size, e.SHA256 = uint32(info.Mode().Perm()), int64(len(b)), hex.EncodeToString(sum[:])
				o := old[e.Path]
				if o.sum == e.SHA256 {
					e.From = e.Path
				} else if src, ok := bySum[e.SHA256]; ok {
					e.From = src // 复用移动或复制的文件
				} else {
					blob := deflate(b)
					if o.size > 0 && o.size <= maxDiff && len(b) <= maxDiff {
						prev, err := os.ReadFile(filepath.Join(oldDir, filepath.FromSlash(e.Path)))
						if err != nil {
							return err
						}
						if patch := Diff(prev, b); len(patch) < len(blob) {
							e.From, blob = e.Path, patch
						}
					}
					e.Data = int64(len(blob))
					data = append(data, blob)
				}
			default:
				return fmt.Errorf("%s is not a file, directory or link", p)
			}
			index.Entries = append(index.Entries, e)
			return nil
		})
		if err != nil {
			return err
		}
	}

	js, err := json.Marshal(index)
	if err != nil {
		return err
	}
	js = deflate(js)
	head := binary.AppendUvarint([]byte(deltaMagic), uint64(len(js)))
	if _, err := w.Write(append(head, js...)); err != nil {
		return err
	}
	for _, b := range data {
		if _, err := w.Write(b); err != nil {
			return err
		}
	}
	return nil
}

func relPath(dir, p string) string {
	rel, _ := filepath.Rel(dir, p)
	return filepath.ToSlash(rel)
}

// 限制客户端解压后的索引大小。
const maxDeltaIndex = 16 << 20

var errDamagedDelta = errors.New("damaged delta update")

// ApplyDelta 从旧目录重建到全新目录，逐文件核对大小和 SHA-256。
func ApplyDelta(r io.ReaderAt, size int64, from, to, oldDir, newDir string) (err error) {
	var head [len(deltaMagic) + binary.MaxVarintLen64]byte
	n, _ := r.ReadAt(head[:min(size, int64(len(head)))], 0)
	if !bytes.HasPrefix(head[:n], []byte(deltaMagic)) {
		return errors.New("not a delta update")
	}
	var indexLen uint64
	k, ok := uvarints(head[len(deltaMagic):n], &indexLen)
	start := int64(len(deltaMagic)) + k
	if !ok || indexLen > uint64(size-start) {
		return errDamagedDelta
	}
	js, err := io.ReadAll(io.LimitReader(flate.NewReader(io.NewSectionReader(r, start, int64(indexLen))), maxDeltaIndex+1))
	if err != nil || len(js) > maxDeltaIndex {
		return errDamagedDelta
	}
	var index deltaIndex
	if err := json.Unmarshal(js, &index); err != nil {
		return errDamagedDelta
	}
	if index.From != from || index.Version != to {
		return fmt.Errorf("the delta updates %s to %s, not %s to %s", index.From, index.Version, from, to)
	}

	// 创建文件前先验证目录索引及数据范围。
	offset := start + int64(indexLen)
	seen := map[string]bool{}
	for _, e := range index.Entries {
		if !fs.ValidPath(e.Path) || e.Path == "." || seen[e.Path] || e.Size < 0 || e.Data < 0 || e.Data > size-offset ||
			e.Dir && e.Link != "" || !e.isFile() && (e.Size != 0 || e.Data != 0 || e.From != "") ||
			e.From != "" && !fs.ValidPath(e.From) || e.isFile() && e.From == "" && e.Data == 0 {
			return errDamagedDelta
		}
		if e.Link != "" {
			if target := path.Join(path.Dir(e.Path), e.Link); path.IsAbs(e.Link) || !fs.ValidPath(target) {
				return fmt.Errorf("delta update link %q points outside the app", e.Path)
			}
		}
		seen[e.Path] = true
		offset += e.Data
	}
	if offset != size {
		return errDamagedDelta
	}

	if err := os.Mkdir(newDir, 0o755); err != nil {
		return err
	}
	// 仅删除本次创建的暂存目录，失败时不留下可被误用的半成品。
	defer func() {
		if err != nil {
			_ = os.RemoveAll(newDir)
		}
	}()
	oldRoot, err := os.OpenRoot(oldDir)
	if err != nil {
		return err
	}
	defer oldRoot.Close()
	newRoot, err := os.OpenRoot(newDir)
	if err != nil {
		return err
	}
	defer newRoot.Close()
	offset = start + int64(indexLen)
	for _, e := range index.Entries {
		name := filepath.FromSlash(e.Path)
		if dir := filepath.Dir(name); dir != "." {
			if err := newRoot.MkdirAll(dir, 0o755); err != nil {
				return err
			}
		}
		switch {
		case e.Dir:
			err = newRoot.MkdirAll(name, 0o755)
		case e.Link != "":
			err = newRoot.Symlink(filepath.FromSlash(e.Link), name)
		default:
			err = applyFile(e, io.NewSectionReader(r, offset, e.Data), oldRoot, newRoot)
			offset += e.Data
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// 根据条目重建普通文件。
func applyFile(e deltaEntry, data *io.SectionReader, oldRoot, newRoot *os.Root) error {
	f, err := newRoot.OpenFile(filepath.FromSlash(e.Path), os.O_CREATE|os.O_EXCL|os.O_WRONLY, fs.FileMode(e.Mode).Perm()|0o200)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	out := &limitWriter{w: io.MultiWriter(f, h), n: e.Size}
	if e.From == "" {
		_, err = io.Copy(out, flate.NewReader(data))
	} else {
		var src *os.File
		if src, err = oldRoot.Open(filepath.FromSlash(e.From)); err != nil {
			return err
		}
		defer src.Close()
		var info fs.FileInfo
		switch info, err = src.Stat(); {
		case err != nil:
		case !info.Mode().IsRegular():
			err = fmt.Errorf("%s is not a file", e.From)
		case e.Data == 0:
			_, err = io.Copy(out, src)
		default:
			err = Patch(out, e.Size, src, info.Size(), data, e.Data)
		}
	}
	if err == nil {
		err = f.Close()
	}
	if err == errDamagedPatch || err == errTooLong || err == nil && (out.n != 0 || hex.EncodeToString(h.Sum(nil)) != e.SHA256) {
		// 旧文件与发布基准不一致。
		return fmt.Errorf("the delta update made %s wrong", e.Path)
	}
	return err
}

var errTooLong = errors.New("too long")

// 限制写出的字节数。
type limitWriter struct {
	w io.Writer
	n int64
}

func (l *limitWriter) Write(b []byte) (int, error) {
	if int64(len(b)) > l.n {
		return 0, errTooLong
	}
	l.n -= int64(len(b))
	return l.w.Write(b)
}

// ApplyFile 在新建目录中重建目标版本，失败时清理本次暂存。
func ApplyFile(filename, from, to, oldDir, newDir string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	return ApplyDelta(f, st.Size(), from, to, oldDir, newDir)
}
