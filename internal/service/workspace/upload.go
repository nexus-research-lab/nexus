package workspace

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

const maxUploadSize = 20 * 1024 * 1024

// UploadFile 上传单个文件到 workspace。
func (s *Service) UploadFile(ctx context.Context, agentID string, filename string, destination string, reader io.Reader) (*UploadResult, error) {
	agentValue, err := s.ensureAgentWorkspace(ctx, agentID)
	if err != nil {
		return nil, err
	}
	confinedRoot, err := s.openAgentWorkspace(agentValue, false)
	if err != nil {
		return nil, err
	}
	defer confinedRoot.Close()
	result, content, err := uploadFileAtRoot(
		agentValue.WorkspacePath,
		confinedRoot,
		filename,
		destination,
		reader,
		uploadFileOptions{dedupeRoots: []string{"tmp/attachments"}},
		func(path string) {
			if s.live != nil {
				s.live.SuppressWatcher(agentValue.AgentID, path)
			}
		},
	)
	if err != nil {
		return nil, err
	}
	if s.live != nil {
		if snapshot, ok := tryDecodeTextSnapshot(result.Path, content); ok {
			s.live.EmitAPIWrite(agentValue.AgentID, result.Path, snapshot)
		}
	}
	return result, nil
}

// UploadFileToRoot 上传单个文件到指定根目录，调用方负责保证根目录归属。
func UploadFileToRoot(root string, filename string, destination string, reader io.Reader) (*UploadResult, error) {
	result, _, err := uploadFileToRoot(
		root,
		filename,
		destination,
		reader,
		uploadFileOptions{dedupeRoots: []string{"attachments"}},
		nil,
	)
	return result, err
}

// UploadFileWithRoot 在调用方已固定的 workspace fd 内上传文件。
func UploadFileWithRoot(
	rootPath string,
	confinedRoot *confinedfs.Root,
	filename string,
	destination string,
	reader io.Reader,
) (*UploadResult, error) {
	if confinedRoot == nil {
		return nil, errors.New("workspace 根句柄不能为空")
	}
	result, _, err := uploadFileAtRoot(
		rootPath,
		confinedRoot,
		filename,
		destination,
		reader,
		uploadFileOptions{dedupeRoots: []string{"attachments"}},
		nil,
	)
	return result, err
}

func uploadFileToRoot(
	root string,
	filename string,
	destination string,
	reader io.Reader,
	options uploadFileOptions,
	beforeWrite func(string),
) (*UploadResult, []byte, error) {
	confinedRoot, err := confinedfs.Open(root)
	if err != nil {
		return nil, nil, err
	}
	defer confinedRoot.Close()
	return uploadFileAtRoot(
		root,
		confinedRoot,
		filename,
		destination,
		reader,
		options,
		beforeWrite,
	)
}

func uploadFileAtRoot(
	root string,
	confinedRoot *confinedfs.Root,
	filename string,
	destination string,
	reader io.Reader,
	options uploadFileOptions,
	beforeWrite func(string),
) (*UploadResult, []byte, error) {
	safeName := normalizeUploadName(filename)
	if safeName == "" {
		safeName = "uploaded_file"
	}
	content, err := io.ReadAll(io.LimitReader(reader, maxUploadSize+1))
	if err != nil {
		return nil, nil, err
	}
	if len(content) > maxUploadSize {
		return nil, nil, invalidWorkspaceMutation(errors.New("文件大小超过限制 (20MB)"))
	}
	contentMD5 := md5Hex(content)

	relativePath := buildUploadTargetPath(strings.TrimSpace(destination), safeName)
	_, normalizedPath, err := resolveWorkspacePath(root, relativePath)
	if err != nil {
		return nil, nil, invalidWorkspaceMutation(err)
	}
	if matched, err := fileMatchesMD5(confinedRoot, normalizedPath, contentMD5, int64(len(content))); err != nil {
		return nil, nil, err
	} else if matched {
		return &UploadResult{
			Path: normalizedPath,
			Name: filepath.Base(normalizedPath),
			Size: int64(len(content)),
		}, content, nil
	}
	if existingPath, matched, err := findDuplicateUploadedFile(
		confinedRoot,
		normalizedPath,
		contentMD5,
		int64(len(content)),
		options.dedupeRoots,
	); err != nil {
		return nil, nil, err
	} else if matched {
		return &UploadResult{
			Path: existingPath,
			Name: filepath.Base(existingPath),
			Size: int64(len(content)),
		}, content, nil
	}
	if normalizedPath, err = ensureUniqueWorkspaceFile(confinedRoot, normalizedPath); err != nil {
		return nil, nil, err
	}
	if err = confinedRoot.MkdirAll(filepath.Dir(normalizedPath), workspaceDirectoryMode()); err != nil {
		return nil, nil, err
	}
	if beforeWrite != nil {
		beforeWrite(normalizedPath)
	}
	if err = confinedRoot.WriteFileAtomic(normalizedPath, content, workspaceFileMode()); err != nil {
		return nil, nil, err
	}
	return &UploadResult{
		Path: normalizedPath,
		Name: filepath.Base(normalizedPath),
		Size: int64(len(content)),
	}, content, nil
}
