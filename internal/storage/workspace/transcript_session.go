package workspace

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

var (
	transcriptSessionIDPattern         = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	subagentTranscriptSessionIDPattern = regexp.MustCompile(`^agent-[0-9a-f]{32}$`)
	subagentTranscriptMarker           = []byte("agent-")
)

// IsTranscriptSessionID 判断值是否符合 Claude/nxs transcript session id 形态。
func IsTranscriptSessionID(sessionID string) bool {
	return transcriptSessionIDPattern.MatchString(strings.ToLower(strings.TrimSpace(sessionID)))
}

// IsSubagentTranscriptSessionID 判断值是否是 nxs 独立 Agent thread 的 transcript id。
func IsSubagentTranscriptSessionID(sessionID string) bool {
	return subagentTranscriptSessionIDPattern.MatchString(strings.ToLower(strings.TrimSpace(sessionID)))
}

// TranscriptSessionExists 判断 workspace 下是否存在可恢复的 SDK transcript。
func (s *AgentHistoryStore) TranscriptSessionExists(workspacePath string, sessionID string) (bool, error) {
	return withRuntimePermissionRepair(s, func() (bool, error) {
		trimmedSessionID := strings.TrimSpace(sessionID)
		normalizedSessionID := strings.ToLower(trimmedSessionID)
		if normalizedSessionID == "" || !IsTranscriptSessionID(normalizedSessionID) {
			return false, nil
		}
		if _, err := s.resolveTranscriptPath(workspacePath, normalizedSessionID); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
}

type transcriptSessionArtifactCatalog struct {
	artifacts       map[string]map[string]map[string]struct{}
	edges           map[string]map[string]struct{}
	memoryArtifacts map[string]map[string]struct{}
}

func newTranscriptSessionArtifactCatalog() *transcriptSessionArtifactCatalog {
	return &transcriptSessionArtifactCatalog{
		artifacts:       make(map[string]map[string]map[string]struct{}),
		edges:           make(map[string]map[string]struct{}),
		memoryArtifacts: make(map[string]map[string]struct{}),
	}
}

// DeleteTranscriptSession 删除 SDK session 的 transcript、会话目录、摘要与
// 无其他 session 引用的独立 Subagent transcript。
func (s *AgentHistoryStore) DeleteTranscriptSession(workspacePath string, sessionID string) (bool, error) {
	normalizedSessionID := strings.ToLower(strings.TrimSpace(sessionID))
	if !IsTranscriptSessionID(normalizedSessionID) {
		return false, nil
	}
	return withRuntimePermissionRepair(s, func() (bool, error) {
		return s.deleteTranscriptSessionArtifacts(workspacePath, normalizedSessionID)
	})
}

func (s *AgentHistoryStore) deleteTranscriptSessionArtifacts(
	workspacePath string,
	sessionID string,
) (bool, error) {
	projectsRoot, projectsRootPath, err := s.openTranscriptProjectsRoot(workspacePath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer projectsRoot.Close()

	catalog, err := scanTranscriptSessionArtifacts(projectsRoot, sessionID)
	if err != nil {
		return false, err
	}

	deleteNodes := catalog.deletionNodes(sessionID)
	touchedProjects := make(map[string]struct{})
	deleted := false
	for projectName, nodes := range catalog.artifacts {
		projectRoot, openErr := projectsRoot.OpenRootNoSymlink(projectName)
		if errors.Is(openErr, os.ErrNotExist) {
			continue
		}
		if openErr != nil {
			return deleted, openErr
		}
		for nodeID, artifactNames := range nodes {
			if _, shouldDelete := deleteNodes[nodeID]; !shouldDelete {
				continue
			}
			for artifactName := range artifactNames {
				if removeErr := projectRoot.RemoveAll(artifactName); removeErr != nil {
					projectRoot.Close()
					return deleted, removeErr
				}
				deleted = true
				touchedProjects[projectName] = struct{}{}
			}
		}
		projectRoot.Close()
	}

	for projectName, artifactNames := range catalog.memoryArtifacts {
		projectRoot, openErr := projectsRoot.OpenRootNoSymlink(projectName)
		if errors.Is(openErr, os.ErrNotExist) {
			continue
		}
		if openErr != nil {
			return deleted, openErr
		}
		for artifactName := range artifactNames {
			if removeErr := projectRoot.RemoveAll(artifactName); removeErr != nil {
				projectRoot.Close()
				return deleted, removeErr
			}
			deleted = true
			touchedProjects[projectName] = struct{}{}
		}
		projectRoot.Close()
	}

	if deleted {
		s.invalidateTranscriptCachePrefix(projectsRootPath)
		removeEmptyTranscriptProjects(projectsRoot, touchedProjects)
	}
	return deleted, nil
}

func (s *AgentHistoryStore) openTranscriptProjectsRoot(
	workspacePath string,
) (*confinedfs.Root, string, error) {
	if strings.TrimSpace(s.ownerUserID) != "" {
		root, err := s.paths.openOwnerTranscriptProjectsRoot(s.ownerUserID, false)
		if err != nil {
			return nil, "", err
		}
		return root, root.Name(), nil
	}
	rootPath := s.transcriptProjectsRootForWorkspace(canonicalizeTranscriptPath(workspacePath))
	root, err := confinedfs.Open(rootPath)
	if err != nil {
		return nil, "", err
	}
	return root, rootPath, nil
}

func scanTranscriptSessionArtifacts(
	projectsRoot *confinedfs.Root,
	sessionID string,
) (*transcriptSessionArtifactCatalog, error) {
	catalog := newTranscriptSessionArtifactCatalog()
	projectEntries, err := fs.ReadDir(projectsRoot.FS(), ".")
	if err != nil {
		return nil, err
	}
	for _, projectEntry := range projectEntries {
		projectInfo, statErr := projectsRoot.Lstat(projectEntry.Name())
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return nil, statErr
		}
		if !projectInfo.IsDir() || projectInfo.Mode()&os.ModeSymlink != 0 {
			continue
		}
		projectRoot, openErr := projectsRoot.OpenRootNoSymlink(projectEntry.Name())
		if openErr != nil {
			return nil, openErr
		}
		scanErr := catalog.scanProject(projectRoot, projectEntry.Name(), sessionID)
		projectRoot.Close()
		if scanErr != nil {
			return nil, scanErr
		}
	}
	return catalog, nil
}

func (c *transcriptSessionArtifactCatalog) scanProject(
	projectRoot *confinedfs.Root,
	projectName string,
	sessionID string,
) error {
	entries, err := fs.ReadDir(projectRoot.FS(), ".")
	if err != nil {
		return err
	}
	memoryNodeID := "session-memory-" + sessionID
	for _, entry := range entries {
		artifactName := entry.Name()
		if artifactName == memoryNodeID || artifactName == memoryNodeID+".jsonl" {
			c.addMemoryArtifact(projectName, artifactName)
		}

		nodeID, isTranscript, ok := transcriptArtifactNodeID(artifactName)
		if !ok {
			continue
		}
		info, statErr := projectRoot.Lstat(artifactName)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return statErr
		}
		c.addArtifact(projectName, nodeID, artifactName)
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		var references map[string]struct{}
		switch {
		case isTranscript && info.Mode().IsRegular():
			references, err = readTranscriptSubagentReferences(projectRoot, artifactName)
		case !isTranscript && info.IsDir():
			references, err = readSessionDirectorySubagentReferences(projectRoot, artifactName)
		}
		if err != nil {
			return err
		}
		c.addEdges(nodeID, references)
	}
	return nil
}

func transcriptArtifactNodeID(artifactName string) (string, bool, bool) {
	isTranscript := strings.HasSuffix(artifactName, ".jsonl")
	nodeID := artifactName
	if isTranscript {
		nodeID = strings.TrimSuffix(nodeID, ".jsonl")
	}
	nodeID = strings.ToLower(strings.TrimSpace(nodeID))
	if !IsTranscriptSessionID(nodeID) && !IsSubagentTranscriptSessionID(nodeID) {
		return "", false, false
	}
	return nodeID, isTranscript, true
}

func readTranscriptSubagentReferences(
	root *confinedfs.Root,
	path string,
) (map[string]struct{}, error) {
	file, err := root.OpenFileNoSymlink(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	references := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, transcriptReadBufferBytes), transcriptScannerBufferBytes)
	for scanner.Scan() {
		if !bytes.Contains(scanner.Bytes(), subagentTranscriptMarker) {
			continue
		}
		payload := map[string]any{}
		if json.Unmarshal(scanner.Bytes(), &payload) != nil {
			continue
		}
		collectSubagentReferences(payload, "", references)
	}
	return references, scanner.Err()
}

func readSessionDirectorySubagentReferences(
	projectRoot *confinedfs.Root,
	artifactName string,
) (map[string]struct{}, error) {
	sessionRoot, err := projectRoot.OpenRootNoSymlink(artifactName)
	if err != nil {
		return nil, err
	}
	defer sessionRoot.Close()
	subagentsRoot, err := sessionRoot.OpenRootNoSymlink("subagents")
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer subagentsRoot.Close()

	entries, err := fs.ReadDir(subagentsRoot.FS(), ".")
	if err != nil {
		return nil, err
	}
	references := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".meta.json") {
			continue
		}
		payload, readErr := subagentsRoot.ReadFile(entry.Name())
		if readErr != nil {
			return nil, readErr
		}
		value := map[string]any{}
		if json.Unmarshal(payload, &value) != nil {
			continue
		}
		collectSubagentReferences(value, "", references)
	}
	return references, nil
}

func collectSubagentReferences(value any, fieldName string, references map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			collectSubagentReferences(child, key, references)
		}
	case []any:
		for _, child := range typed {
			collectSubagentReferences(child, fieldName, references)
		}
	case string:
		if !isTranscriptReferenceField(fieldName) {
			return
		}
		normalized := strings.ToLower(strings.TrimSpace(typed))
		if IsSubagentTranscriptSessionID(normalized) {
			references[normalized] = struct{}{}
		}
	}
}

func isTranscriptReferenceField(fieldName string) bool {
	switch fieldName {
	case "agent_id", "agentId", "child_session_id", "childSessionId", "session_id", "sessionId":
		return true
	default:
		return false
	}
}

func (c *transcriptSessionArtifactCatalog) addArtifact(
	projectName string,
	nodeID string,
	artifactName string,
) {
	if c.artifacts[projectName] == nil {
		c.artifacts[projectName] = make(map[string]map[string]struct{})
	}
	if c.artifacts[projectName][nodeID] == nil {
		c.artifacts[projectName][nodeID] = make(map[string]struct{})
	}
	c.artifacts[projectName][nodeID][artifactName] = struct{}{}
}

func (c *transcriptSessionArtifactCatalog) addMemoryArtifact(projectName string, artifactName string) {
	if c.memoryArtifacts[projectName] == nil {
		c.memoryArtifacts[projectName] = make(map[string]struct{})
	}
	c.memoryArtifacts[projectName][artifactName] = struct{}{}
}

func (c *transcriptSessionArtifactCatalog) addEdges(
	source string,
	targets map[string]struct{},
) {
	if len(targets) == 0 {
		return
	}
	if c.edges[source] == nil {
		c.edges[source] = make(map[string]struct{})
	}
	for target := range targets {
		c.edges[source][target] = struct{}{}
	}
}

func (c *transcriptSessionArtifactCatalog) deletionNodes(sessionID string) map[string]struct{} {
	candidates := c.reachableNodes(sessionID)
	preserved := make(map[string]struct{})
	for source, targets := range c.edges {
		if _, belongsToSession := candidates[source]; belongsToSession {
			continue
		}
		for target := range targets {
			if _, candidate := candidates[target]; candidate {
				c.addReachableNodes(target, preserved)
			}
		}
	}
	for nodeID := range preserved {
		delete(candidates, nodeID)
	}
	return candidates
}

func (c *transcriptSessionArtifactCatalog) reachableNodes(root string) map[string]struct{} {
	result := make(map[string]struct{})
	c.addReachableNodes(root, result)
	return result
}

func (c *transcriptSessionArtifactCatalog) addReachableNodes(
	root string,
	result map[string]struct{},
) {
	if _, exists := result[root]; exists {
		return
	}
	result[root] = struct{}{}
	for target := range c.edges[root] {
		c.addReachableNodes(target, result)
	}
}

func removeEmptyTranscriptProjects(
	projectsRoot *confinedfs.Root,
	projectNames map[string]struct{},
) {
	for projectName := range projectNames {
		projectRoot, err := projectsRoot.OpenRootNoSymlink(projectName)
		if err != nil {
			continue
		}
		entries, readErr := fs.ReadDir(projectRoot.FS(), ".")
		projectRoot.Close()
		if readErr == nil && len(entries) == 0 {
			_ = projectsRoot.Remove(projectName)
		}
	}
}

// DeleteTranscriptProject 删除整个 workspace 对应的 transcript 项目目录。
func (s *AgentHistoryStore) DeleteTranscriptProject(workspacePath string) (bool, error) {
	canonicalPath := canonicalizeTranscriptPath(workspacePath)
	projectsRoot := s.transcriptProjectsRootForWorkspace(canonicalPath)
	projectDir, err := s.findTranscriptProjectDirAt(projectsRoot, canonicalPath)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(projectDir) == "" {
		return false, nil
	}
	if strings.TrimSpace(s.ownerUserID) != "" {
		root, err := s.paths.openOwnerTranscriptProjectsRoot(
			s.ownerUserID,
			false,
		)
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		defer root.Close()
		relative, err := filepath.Rel(root.Name(), projectDir)
		if err != nil || relative == "." ||
			strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return false, errors.New("transcript project is outside owner root")
		}
		if err := root.RemoveAll(filepath.ToSlash(relative)); err != nil {
			return false, err
		}
		s.invalidateTranscriptCachePrefix(projectDir)
		return true, nil
	}
	root, relative, err := relativeStorePathWithCreate(projectsRoot, projectDir, false)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer root.Close()

	if err := root.RemoveAll(relative); err != nil {
		return false, err
	}
	s.invalidateTranscriptCachePrefix(projectDir)
	return true, nil
}
