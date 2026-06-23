package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadFileToRootReusesIdenticalTargetByMD5(t *testing.T) {
	root := t.TempDir()

	first, err := UploadFileToRoot(root, "demo.txt", "docs/", strings.NewReader("same content"))
	if err != nil {
		t.Fatalf("首次上传失败: %v", err)
	}
	second, err := UploadFileToRoot(root, "demo.txt", "docs/", strings.NewReader("same content"))
	if err != nil {
		t.Fatalf("重复上传失败: %v", err)
	}
	if second.Path != first.Path {
		t.Fatalf("相同内容应复用目标文件: first=%+v second=%+v", first, second)
	}
	entries, err := os.ReadDir(filepath.Join(root, "docs"))
	if err != nil {
		t.Fatalf("读取上传目录失败: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("相同内容不应生成重复文件: %v", entries)
	}

	changed, err := UploadFileToRoot(root, "demo.txt", "docs/", strings.NewReader("changed content"))
	if err != nil {
		t.Fatalf("不同内容上传失败: %v", err)
	}
	if changed.Path == first.Path {
		t.Fatalf("不同内容不应复用原文件: first=%+v changed=%+v", first, changed)
	}
}

func TestUploadFileToRootReusesAttachmentByMD5(t *testing.T) {
	root := t.TempDir()

	first, err := UploadFileToRoot(root, "demo.txt", "attachments/batch-1/", strings.NewReader("same content"))
	if err != nil {
		t.Fatalf("首次附件上传失败: %v", err)
	}
	second, err := UploadFileToRoot(root, "demo.txt", "attachments/batch-2/", strings.NewReader("same content"))
	if err != nil {
		t.Fatalf("重复附件上传失败: %v", err)
	}
	if second.Path != first.Path {
		t.Fatalf("附件相同内容应复用已有文件: first=%+v second=%+v", first, second)
	}
	if _, err = os.Stat(filepath.Join(root, "attachments", "batch-2", "demo.txt")); !os.IsNotExist(err) {
		t.Fatalf("重复附件不应落盘到新目录: %v", err)
	}

	changed, err := UploadFileToRoot(root, "demo.txt", "attachments/batch-3/", strings.NewReader("changed content"))
	if err != nil {
		t.Fatalf("不同附件上传失败: %v", err)
	}
	if changed.Path == first.Path {
		t.Fatalf("附件不同内容不应复用原文件: first=%+v changed=%+v", first, changed)
	}
}
