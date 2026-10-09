// 测试专用入口：生产路径已不再调用，仅供本包测试复用。
package workspace

func (s *SessionFileStore) readJSONL(path string) ([]map[string]any, error) {
	root, relative, err := s.openStorePath(path, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return readJSONLAtRoot(root, relative)
}
