// INPUT: v0.1.27 的 Skill registry、Agent workspace 与 runtime 记录。
// OUTPUT: 将旧版外部 Skill 源迁移到当前 owner workspace，并恢复稳定引用。
// POS: 只负责上一发布版本到当前共享 Skill 存储的单次升级迁移。
package migration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

const (
	// 该迁移只对应 v0.1.27，未发布过的中间目录不纳入兼容面。
	legacySkillStorageMigrationName = "20260723_migrate_v0_1_27_skill_storage"

	legacyRegistryUsersDirName      = "users"
	legacyRegistryMigratedDirName   = "legacy-migrated"
	legacyRegistryUnassignedDirName = "legacy-unassigned"
	legacySkillConflictBackupDir    = ".migration-backups"
)

type legacySkillStorageMigration struct {
	ctx                context.Context
	cfg                config.Config
	db                 *sql.DB
	dialect            storage.SQLDialect
	logger             *slog.Logger
	conflictBackupRoot string
}

type legacySkillAgent struct {
	id            string
	runtimeID     string
	ownerUserID   string
	workspacePath string
	skillIDs      []string
}

type legacySkillSource struct {
	name string
	path string
}

type legacySkillManifest struct {
	Name       string `json:"name"`
	SourceType string `json:"source_type"`
}

type ownerSkillNames map[string]map[string]string
type skillUsageOwners map[string]map[string]struct{}

// RunLegacySkillStorage 将 v0.1.27 的 Skill 数据迁移到当前 owner workspace。
//
// 文件和数据库引用全部成功后才写完成标记。目标若已有完整外部 Skill 则复用；
// 若是同名的无效旧目录，则先移入宿主备份区，再迁移旧源，避免单个 Skill
// 冲突阻断整个服务启动，也不静默覆盖或丢弃用户内容。
func RunLegacySkillStorage(
	ctx context.Context,
	cfg config.Config,
	configRoot string,
	logger *slog.Logger,
) error {
	if logger == nil {
		logger = slog.Default()
	}
	configRoot = filepath.Clean(configRoot)
	markerPath := workspaceFileMigrationMarker(configRoot, legacySkillStorageMigrationName)
	applied, err := workspaceFileMigrationApplied(markerPath)
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	db, err := storage.OpenDB(cfg)
	if err != nil {
		return fmt.Errorf("打开 Skill 存储迁移数据库: %w", err)
	}
	defer db.Close()

	migration := legacySkillStorageMigration{
		ctx:                ctx,
		cfg:                cfg,
		db:                 db,
		dialect:            storage.NewSQLDialect(cfg.DatabaseDriver),
		logger:             logger,
		conflictBackupRoot: filepath.Join(configRoot, legacySkillConflictBackupDir, legacySkillStorageMigrationName),
	}
	affected, err := migration.apply()
	if err != nil {
		return fmt.Errorf("执行 v0.1.27 Skill 存储迁移: %w", err)
	}
	if err = writeWorkspaceFileMigrationMarker(markerPath); err != nil {
		return err
	}
	logger.Info("v0.1.27 Skill 存储迁移完成",
		"migration", legacySkillStorageMigrationName,
		"affected_paths", affected,
	)
	return nil
}

func (m *legacySkillStorageMigration) apply() (int, error) {
	agents, err := m.loadAgents()
	if err != nil {
		return 0, err
	}
	owners, usage, err := collectLegacySkillUsage(agents)
	if err != nil {
		return 0, err
	}
	userIDs, err := m.loadUserIDs()
	if err != nil {
		return 0, err
	}
	for _, ownerUserID := range userIDs {
		owners[ownerUserID] = struct{}{}
	}
	imported, err := m.loadImportedSkills()
	if err != nil {
		return 0, err
	}
	for ownerUserID, names := range imported {
		owners[ownerUserID] = struct{}{}
		for _, name := range names {
			addSkillUsage(usage, name, ownerUserID)
		}
	}
	owners[authctx.SystemUserID] = struct{}{}

	externalNames := ownerSkillNames{}
	affected, err := m.migrateOwnerRegistries(owners, externalNames)
	if err != nil {
		return affected, err
	}
	migrated, err := m.migrateGlobalRegistries(owners, usage, externalNames)
	affected += migrated
	if err != nil {
		return affected, err
	}
	for index := range agents {
		migrated, migrateErr := m.migrateAgent(&agents[index], externalNames)
		affected += migrated
		if migrateErr != nil {
			return affected, migrateErr
		}
	}
	return affected, nil
}

func (m *legacySkillStorageMigration) loadAgents() ([]legacySkillAgent, error) {
	rows, err := m.db.QueryContext(m.ctx, `
SELECT a.id,
       COALESCE(rt.id, ''),
       COALESCE(a.owner_user_id, ''),
       a.workspace_path,
       COALESCE(rt.skill_ids_json, '[]')
FROM agents a
LEFT JOIN runtimes rt ON rt.agent_id = a.id
ORDER BY a.id ASC`)
	if err != nil {
		return nil, fmt.Errorf("读取 Agent Skill 迁移记录: %w", err)
	}
	defer rows.Close()

	result := make([]legacySkillAgent, 0)
	for rows.Next() {
		var agent legacySkillAgent
		var skillIDsJSON string
		if err = rows.Scan(
			&agent.id,
			&agent.runtimeID,
			&agent.ownerUserID,
			&agent.workspacePath,
			&skillIDsJSON,
		); err != nil {
			return nil, fmt.Errorf("扫描 Agent Skill 迁移记录: %w", err)
		}
		agent.ownerUserID = normalizedOwnerID(agent.ownerUserID)
		agent.workspacePath = strings.TrimSpace(agent.workspacePath)
		if err = json.Unmarshal([]byte(skillIDsJSON), &agent.skillIDs); err != nil {
			return nil, fmt.Errorf("解析 Agent %s Skill 引用: %w", agent.id, err)
		}
		result = append(result, agent)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 Agent Skill 迁移记录: %w", err)
	}
	return result, nil
}

func (m *legacySkillStorageMigration) loadUserIDs() ([]string, error) {
	rows, err := m.db.QueryContext(m.ctx, `
SELECT user_id
FROM users
ORDER BY user_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("读取 Skill 迁移用户: %w", err)
	}
	defer rows.Close()

	result := make([]string, 0)
	for rows.Next() {
		var ownerUserID string
		if err = rows.Scan(&ownerUserID); err != nil {
			return nil, fmt.Errorf("扫描 Skill 迁移用户: %w", err)
		}
		result = append(result, normalizedOwnerID(ownerUserID))
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 Skill 迁移用户: %w", err)
	}
	return result, nil
}

func (m *legacySkillStorageMigration) loadImportedSkills() (ownerSkillNames, error) {
	rows, err := m.db.QueryContext(m.ctx, `
SELECT owner_user_id, skill_name
FROM imported_skills
ORDER BY owner_user_id ASC, skill_name ASC`)
	if err != nil {
		return nil, fmt.Errorf("读取已导入 Skill 归属: %w", err)
	}
	defer rows.Close()

	result := ownerSkillNames{}
	for rows.Next() {
		var ownerUserID string
		var skillName string
		if err = rows.Scan(&ownerUserID, &skillName); err != nil {
			return nil, fmt.Errorf("扫描已导入 Skill 归属: %w", err)
		}
		addOwnerSkill(result, normalizedOwnerID(ownerUserID), skillName)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历已导入 Skill 归属: %w", err)
	}
	return result, nil
}

func (m *legacySkillStorageMigration) migrateOwnerRegistries(
	owners map[string]struct{},
	externalNames ownerSkillNames,
) (int, error) {
	affected := 0
	ownersBySegment := make(map[string][]string, len(owners))
	for _, ownerUserID := range sortedStringSet(owners) {
		segment := legacyOwnerSegment(ownerUserID)
		ownersBySegment[segment] = append(ownersBySegment[segment], ownerUserID)
	}
	knownSegments := make(map[string]struct{}, len(ownersBySegment))
	for segment := range ownersBySegment {
		knownSegments[segment] = struct{}{}
	}
	registryRoot := m.legacyRegistryRoot()
	for _, segment := range sortedStringSet(knownSegments) {
		sourceRoot := filepath.Join(registryRoot, legacyRegistryUsersDirName, segment)
		sources, err := scanLegacySkillRoot(sourceRoot, false)
		if err != nil {
			return affected, err
		}
		for _, source := range sources {
			for _, ownerUserID := range ownersBySegment[segment] {
				migrated, migrateErr := m.copySkillToOwner(source, ownerUserID)
				if migrateErr != nil {
					return affected, migrateErr
				}
				addOwnerSkill(externalNames, ownerUserID, source.name)
				affected += migrated
			}
			if removeErr := os.RemoveAll(source.path); removeErr != nil {
				return affected, fmt.Errorf("清理旧 owner Skill %q: %w", source.path, removeErr)
			}
		}
	}

	// 已删除用户无法再由数据库反解；放入系统 owner，避免唯一源变成孤儿数据。
	usersRoot := filepath.Join(registryRoot, legacyRegistryUsersDirName)
	entries, err := os.ReadDir(usersRoot)
	if os.IsNotExist(err) {
		return affected, nil
	}
	if err != nil {
		return affected, fmt.Errorf("读取旧 owner Skill 根 %q: %w", usersRoot, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, known := knownSegments[entry.Name()]; known {
			continue
		}
		sources, scanErr := scanLegacySkillRoot(filepath.Join(usersRoot, entry.Name()), false)
		if scanErr != nil {
			return affected, scanErr
		}
		for _, source := range sources {
			migrated, migrateErr := m.copySkillToOwner(source, authctx.SystemUserID)
			if migrateErr != nil {
				return affected, migrateErr
			}
			addOwnerSkill(externalNames, authctx.SystemUserID, source.name)
			if removeErr := os.RemoveAll(source.path); removeErr != nil {
				return affected, fmt.Errorf("清理未知 owner Skill %q: %w", source.path, removeErr)
			}
			affected += migrated
		}
	}
	return affected, nil
}

func (m *legacySkillStorageMigration) migrateGlobalRegistries(
	owners map[string]struct{},
	usage skillUsageOwners,
	externalNames ownerSkillNames,
) (int, error) {
	affected := 0
	registryRoot := m.legacyRegistryRoot()
	for _, bucket := range []string{"", legacyRegistryMigratedDirName, legacyRegistryUnassignedDirName} {
		sourceRoot := registryRoot
		if bucket != "" {
			sourceRoot = filepath.Join(sourceRoot, bucket)
		}
		sources, err := scanLegacySkillRoot(sourceRoot, bucket == "")
		if err != nil {
			return affected, err
		}
		for _, source := range sources {
			targetOwners := usage[strings.ToLower(source.name)]
			if len(targetOwners) == 0 {
				targetOwners = map[string]struct{}{authctx.SystemUserID: {}}
			}
			for _, ownerUserID := range sortedStringSet(targetOwners) {
				if _, known := owners[ownerUserID]; !known {
					ownerUserID = authctx.SystemUserID
				}
				migrated, migrateErr := m.copySkillToOwner(source, ownerUserID)
				if migrateErr != nil {
					return affected, migrateErr
				}
				addOwnerSkill(externalNames, ownerUserID, source.name)
				affected += migrated
			}
			if removeErr := os.RemoveAll(source.path); removeErr != nil {
				return affected, fmt.Errorf("清理旧全局 Skill %q: %w", source.path, removeErr)
			}
		}
	}
	return affected, nil
}

func (m *legacySkillStorageMigration) migrateAgent(
	agent *legacySkillAgent,
	externalNames ownerSkillNames,
) (int, error) {
	if agent == nil || agent.workspacePath == "" {
		return 0, nil
	}
	deployedNames, err := listLegacyV027WorkspaceSkills(agent.workspacePath)
	if err != nil {
		return 0, fmt.Errorf("读取 Agent %s 已部署 Skill: %w", agent.id, err)
	}
	ownerNames := externalNames[agent.ownerUserID]
	migratedNames := map[string]string{}
	deployedExternalNames := map[string]string{}
	for _, deployedName := range deployedNames {
		source, external := findLegacyWorkspaceSkill(agent.workspacePath, deployedName)
		if !external {
			continue
		}
		canonical, known := ownerNames[strings.ToLower(source.name)]
		if !known {
			canonical, known = ownerNames[strings.ToLower(deployedName)]
		}
		if !known {
			canonical = source.name
			_, migrateErr := m.copySkillToOwner(source, agent.ownerUserID)
			if migrateErr != nil {
				return 0, migrateErr
			}
			addOwnerSkill(externalNames, agent.ownerUserID, canonical)
			ownerNames = externalNames[agent.ownerUserID]
			ownerNames[strings.ToLower(deployedName)] = canonical
		}
		if !isLegacyExternalSkillDir(m.ownerSkillPath(agent.ownerUserID, canonical)) {
			return 0, fmt.Errorf("Agent %s 的外部 Skill %s 缺少有效 owner 源", agent.id, canonical)
		}
		migratedNames[strings.ToLower(canonical)] = canonical
		deployedExternalNames[deployedName] = canonical
	}

	updatedIDs, changed := normalizeLegacySkillReferences(agent.skillIDs, migratedNames)
	if changed && agent.runtimeID != "" {
		if err = m.updateAgentSkillIDs(agent.id, updatedIDs); err != nil {
			return 0, err
		}
	}
	if agent.runtimeID == "" {
		// 没有 runtime 记录时保留 workspace 副本，避免清理后失去启用关系。
		return 0, nil
	}
	affected := 0
	for deployedName := range deployedExternalNames {
		if err = removeLegacyV027WorkspaceSkill(agent.workspacePath, deployedName); err != nil {
			return affected, fmt.Errorf("清理 Agent %s 的旧 Skill %s: %w", agent.id, deployedName, err)
		}
		affected++
	}
	if changed {
		affected++
	}
	return affected, nil
}

func (m *legacySkillStorageMigration) copySkillToOwner(
	source legacySkillSource,
	ownerUserID string,
) (int, error) {
	target := m.ownerSkillPath(ownerUserID, source.name)
	copied, err := m.copySkillIfMissing(source.path, target, ownerUserID, source.name)
	if err != nil {
		return 0, fmt.Errorf("迁移 Skill %s 到 owner %s: %w", source.name, ownerUserID, err)
	}
	if err = workspacepkg.EnsureUserSkillLibrary(m.cfg, ownerUserID); err != nil {
		return 0, fmt.Errorf("准备 owner %s 的 Skill 发现入口: %w", ownerUserID, err)
	}
	if copied {
		return 1, nil
	}
	return 0, nil
}

func (m *legacySkillStorageMigration) updateAgentSkillIDs(agentID string, skillIDs []string) error {
	payload, err := json.Marshal(skillIDs)
	if err != nil {
		return fmt.Errorf("序列化 Agent %s Skill 引用: %w", agentID, err)
	}
	query := fmt.Sprintf(`
UPDATE runtimes
SET skill_ids_json = %s, updated_at = %s
WHERE agent_id = %s`,
		m.dialect.Bind(1),
		m.dialect.CurrentTimestamp(),
		m.dialect.Bind(2),
	)
	if _, err = m.db.ExecContext(m.ctx, query, string(payload), agentID); err != nil {
		return fmt.Errorf("更新 Agent %s Skill 引用: %w", agentID, err)
	}
	return nil
}

func (m *legacySkillStorageMigration) ownerSkillPath(ownerUserID string, skillName string) string {
	return filepath.Join(
		agentpkg.UserWorkspaceBasePath(m.cfg, normalizedOwnerID(ownerUserID)),
		".agents",
		"skills",
		skillName,
	)
}

func (m *legacySkillStorageMigration) legacyRegistryRoot() string {
	cacheRoot := strings.TrimSpace(m.cfg.CacheFileDir)
	if cacheRoot == "" {
		cacheRoot = "cache"
	}
	return filepath.Join(filepath.Clean(cacheRoot), "skills", "registry")
}

func collectLegacySkillUsage(
	agents []legacySkillAgent,
) (map[string]struct{}, skillUsageOwners, error) {
	owners := map[string]struct{}{authctx.SystemUserID: {}}
	usage := skillUsageOwners{}
	for _, agent := range agents {
		ownerUserID := normalizedOwnerID(agent.ownerUserID)
		owners[ownerUserID] = struct{}{}
		for _, reference := range agent.skillIDs {
			if name, ok := protocol.ParseExternalSkillReference(reference); ok {
				addSkillUsage(usage, name, ownerUserID)
			}
		}
		deployedNames, err := listLegacyV027WorkspaceSkills(agent.workspacePath)
		if err != nil {
			return nil, nil, fmt.Errorf("读取 Agent %s 已部署 Skill: %w", agent.id, err)
		}
		for _, name := range deployedNames {
			if source, ok := findLegacyWorkspaceSkill(agent.workspacePath, name); ok {
				addSkillUsage(usage, source.name, ownerUserID)
			}
		}
	}
	return owners, usage, nil
}

func scanLegacySkillRoot(root string, skipReserved bool) ([]legacySkillSource, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取旧 Skill 根 %q: %w", root, err)
	}
	result := make([]legacySkillSource, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || (skipReserved && isLegacyRegistryReservedDir(entry.Name())) {
			continue
		}
		path := filepath.Join(root, entry.Name())
		name, ok := readLegacyExternalSkillName(path)
		if ok {
			result = append(result, legacySkillSource{name: name, path: path})
		}
	}
	sort.Slice(result, func(left, right int) bool {
		return strings.ToLower(result[left].name) < strings.ToLower(result[right].name)
	})
	return result, nil
}

func findLegacyWorkspaceSkill(workspacePath string, skillName string) (legacySkillSource, bool) {
	for _, root := range legacyV027WorkspaceSkillRoots(workspacePath) {
		path := filepath.Join(root, skillName)
		if name, ok := readLegacyExternalSkillName(path); ok {
			return legacySkillSource{name: name, path: path}, true
		}
	}
	return legacySkillSource{}, false
}

func listLegacyV027WorkspaceSkills(workspacePath string) ([]string, error) {
	if strings.TrimSpace(workspacePath) == "" {
		return nil, nil
	}
	result := make([]string, 0)
	seen := map[string]struct{}{}
	for _, root := range legacyV027WorkspaceSkillRoots(workspacePath) {
		entries, err := os.ReadDir(root)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			info, statErr := os.Stat(filepath.Join(root, entry.Name()))
			if statErr != nil || !info.IsDir() {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(entry.Name()))
			if key == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, entry.Name())
		}
	}
	sort.Strings(result)
	return result, nil
}

func removeLegacyV027WorkspaceSkill(workspacePath string, skillName string) error {
	paths := make([]string, 0, 2)
	for _, root := range legacyV027WorkspaceSkillRoots(workspacePath) {
		path := filepath.Join(root, skillName)
		if _, ok := readLegacyExternalSkillName(path); ok {
			paths = append(paths, path)
		}
	}
	for _, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func legacyV027WorkspaceSkillRoots(workspacePath string) []string {
	return []string{
		filepath.Join(workspacePath, ".agents", "skills"),
		filepath.Join(workspacePath, ".claude", "skills"),
	}
}

func readLegacyExternalSkillName(path string) (string, bool) {
	if info, err := os.Stat(filepath.Join(path, "SKILL.md")); err != nil || info.IsDir() {
		return "", false
	}
	payload, err := os.ReadFile(filepath.Join(path, ".nexus-skill.json"))
	if err != nil {
		return "", false
	}
	var manifest legacySkillManifest
	if json.Unmarshal(payload, &manifest) != nil {
		return "", false
	}
	sourceType := strings.ToLower(strings.TrimSpace(manifest.SourceType))
	if sourceType != "" && sourceType != "external" {
		return "", false
	}
	name := strings.TrimSpace(manifest.Name)
	if !validLegacySkillName(name) {
		name = filepath.Base(path)
	}
	return name, validLegacySkillName(name)
}

func isLegacyExternalSkillDir(path string) bool {
	_, ok := readLegacyExternalSkillName(path)
	return ok
}

func (m *legacySkillStorageMigration) copySkillIfMissing(
	sourcePath string,
	targetPath string,
	ownerUserID string,
	skillName string,
) (bool, error) {
	if isLegacyExternalSkillDir(targetPath) {
		return false, nil
	}
	if !isLegacyExternalSkillDir(sourcePath) {
		return false, fmt.Errorf("源目录不是有效外部 Skill: %s", sourcePath)
	}
	if _, err := os.Lstat(targetPath); err == nil {
		backupPath, backupErr := m.backupInvalidSkillTarget(ownerUserID, skillName, targetPath)
		if backupErr != nil {
			return false, backupErr
		}
		if m.logger != nil {
			m.logger.Warn("旧版 Skill 迁移保留同名无效目标",
				"owner_user_id", ownerUserID,
				"skill", skillName,
				"target", targetPath,
				"backup", backupPath,
			)
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	parent := filepath.Dir(targetPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return false, err
	}
	temporaryPath, err := os.MkdirTemp(parent, ".skill-migration-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(temporaryPath)
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return false, err
	}
	if err = os.Chmod(temporaryPath, sourceInfo.Mode().Perm()); err != nil {
		return false, err
	}
	if err = copyLegacySkillTree(sourcePath, temporaryPath); err != nil {
		return false, err
	}
	if err = os.Rename(temporaryPath, targetPath); err != nil {
		return false, err
	}
	return true, nil
}

// backupInvalidSkillTarget 把无法识别为外部 Skill 的同名目标移出发现根。
// 备份先完整落盘才删除原路径；这样即使旧 registry 源与现有目录内容不同，
// 两份数据都保留，后续可按日志中的路径人工比较或恢复。
func (m *legacySkillStorageMigration) backupInvalidSkillTarget(
	ownerUserID string,
	skillName string,
	targetPath string,
) (string, error) {
	backupBasePath := filepath.Join(
		m.conflictBackupRoot,
		appfs.UserPathSegment(normalizedOwnerID(ownerUserID)),
		skillName,
	)
	backupPath, err := nextAvailableLegacySkillBackupPath(backupBasePath)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(backupPath), 0o700); err != nil {
		return "", fmt.Errorf("创建 Skill 冲突备份目录: %w", err)
	}
	if err = moveLegacySkillConflict(targetPath, backupPath); err != nil {
		return "", fmt.Errorf("备份同名无效 Skill 目标 %q: %w", targetPath, err)
	}
	return backupPath, nil
}

func nextAvailableLegacySkillBackupPath(basePath string) (string, error) {
	for index := 1; index < 10000; index++ {
		candidate := basePath
		if index > 1 {
			candidate = fmt.Sprintf("%s.%d", basePath, index)
		}
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("无法为 Skill 迁移生成唯一备份路径 %q", basePath)
}

func moveLegacySkillConflict(sourcePath string, targetPath string) error {
	renameErr := os.Rename(sourcePath, targetPath)
	if renameErr == nil {
		return nil
	}
	if err := copyLegacySkillEntry(sourcePath, targetPath); err != nil {
		return fmt.Errorf("重命名失败: %v；跨文件系统复制也失败: %w", renameErr, err)
	}
	if err := os.RemoveAll(sourcePath); err != nil {
		return fmt.Errorf("重命名失败: %v；备份复制成功但清理原路径失败: %w", renameErr, err)
	}
	return nil
}

func copyLegacySkillEntry(sourcePath string, targetPath string) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		linkTarget, readErr := os.Readlink(sourcePath)
		if readErr != nil {
			return readErr
		}
		return os.Symlink(linkTarget, targetPath)
	}
	if info.IsDir() {
		if err = os.Mkdir(targetPath, info.Mode().Perm()); err != nil {
			return err
		}
		return copyLegacySkillTree(sourcePath, targetPath)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("Skill 冲突目标包含不支持的文件类型: %s", sourcePath)
	}
	return copyLegacySkillFile(sourcePath, targetPath, info.Mode().Perm())
}

func copyLegacySkillTree(sourceRoot string, targetRoot string) error {
	return filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if relativePath == "." {
			return nil
		}
		targetPath := filepath.Join(targetRoot, relativePath)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			linkTarget, readErr := os.Readlink(path)
			if readErr != nil {
				return readErr
			}
			if err = os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			return os.Symlink(linkTarget, targetPath)
		}
		if entry.IsDir() {
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Skill 包含不支持的文件类型: %s", path)
		}
		return copyLegacySkillFile(path, targetPath, info.Mode().Perm())
	})
}

func copyLegacySkillFile(sourcePath string, targetPath string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func normalizeLegacySkillReferences(
	selected []string,
	migratedNames map[string]string,
) ([]string, bool) {
	result := make([]string, 0, len(selected)+len(migratedNames))
	seen := make(map[string]struct{}, cap(result))
	changed := false
	appendReference := func(reference string) {
		key := strings.ToLower(strings.TrimSpace(reference))
		if key == "" {
			changed = true
			return
		}
		if _, exists := seen[key]; exists {
			changed = true
			return
		}
		seen[key] = struct{}{}
		result = append(result, reference)
	}
	for _, reference := range selected {
		value := strings.TrimSpace(reference)
		name := legacySkillReferenceName(value)
		canonical := value
		if externalName, ok := migratedNames[strings.ToLower(name)]; ok {
			canonical = protocol.BuildExternalSkillReference(externalName)
		} else if externalName, ok := protocol.ParseExternalSkillReference(value); ok {
			canonical = protocol.BuildExternalSkillReference(externalName)
		}
		if canonical != value {
			changed = true
		}
		appendReference(canonical)
	}
	names := make([]string, 0, len(migratedNames))
	for _, name := range migratedNames {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		reference := protocol.BuildExternalSkillReference(name)
		if _, exists := seen[strings.ToLower(reference)]; !exists {
			changed = true
		}
		appendReference(reference)
	}
	return result, changed
}

func legacySkillReferenceName(reference string) string {
	if name, ok := protocol.ParseExternalSkillReference(reference); ok {
		return name
	}
	return strings.TrimSpace(reference)
}

func addOwnerSkill(target ownerSkillNames, ownerUserID string, skillName string) {
	name := strings.TrimSpace(skillName)
	if !validLegacySkillName(name) {
		return
	}
	ownerUserID = normalizedOwnerID(ownerUserID)
	if target[ownerUserID] == nil {
		target[ownerUserID] = map[string]string{}
	}
	target[ownerUserID][strings.ToLower(name)] = name
}

func addSkillUsage(target skillUsageOwners, skillName string, ownerUserID string) {
	name := strings.TrimSpace(skillName)
	if !validLegacySkillName(name) {
		return
	}
	key := strings.ToLower(name)
	if target[key] == nil {
		target[key] = map[string]struct{}{}
	}
	target[key][normalizedOwnerID(ownerUserID)] = struct{}{}
}

func normalizedOwnerID(ownerUserID string) string {
	if value := strings.TrimSpace(ownerUserID); value != "" {
		return value
	}
	return authctx.SystemUserID
}

func legacyOwnerSegment(ownerUserID string) string {
	value := strings.TrimSpace(ownerUserID)
	if value == "" {
		return authctx.SystemUserID
	}
	var builder strings.Builder
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z',
			character >= 'A' && character <= 'Z',
			character >= '0' && character <= '9',
			character == '-',
			character == '_',
			character == '.',
			character == '@':
			builder.WriteRune(character)
		default:
			builder.WriteRune('_')
		}
	}
	if builder.Len() == 0 {
		return authctx.SystemUserID
	}
	return builder.String()
}

func validLegacySkillName(name string) bool {
	value := strings.TrimSpace(name)
	return value != "" &&
		value != "." &&
		value != ".." &&
		!strings.ContainsAny(value, `/\`+"\x00") &&
		!strings.HasPrefix(strings.ToLower(value), "external:")
}

func isLegacyRegistryReservedDir(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case legacyRegistryUsersDirName,
		legacyRegistryMigratedDirName,
		legacyRegistryUnassignedDirName:
		return true
	default:
		return false
	}
}

func sortedStringSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
