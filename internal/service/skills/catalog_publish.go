// INPUT: 已固定 owner root 内通过完整校验的 staging Skill 与目标目录。
// OUTPUT: 可提交、可回滚的原子目录发布/移除句柄，旧版本在数据库提交前保留。
// POS: Skill 文件系统与 catalog transaction 之间的补偿边界。
package skills

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

type skillPublication struct {
	root           *confinedfs.Root
	targetRelative string
	backupRelative string
	hadPrior       bool
	targetHasNew   bool
	finished       bool
}

func validateStagedSkill(
	root *confinedfs.Root,
	stagingRelative string,
	expectedName string,
) error {
	staging, err := root.OpenRootNoSymlink(stagingRelative)
	if err != nil {
		return err
	}
	defer staging.Close()
	content, err := readConfinedRegularFile(staging, "SKILL.md")
	if err != nil {
		return err
	}
	parsed := parseSkillFrontmatter(string(content), expectedName)
	if strings.TrimSpace(parsed.Name) != strings.TrimSpace(expectedName) {
		return fmt.Errorf(
			"staged SKILL.md name=%q 与目标 name=%q 不一致",
			parsed.Name,
			expectedName,
		)
	}
	manifestPayload, err := readConfinedRegularFile(staging, ".nexus-skill.json")
	if err != nil {
		return err
	}
	var manifest externalManifest
	if err = json.Unmarshal(manifestPayload, &manifest); err != nil {
		return err
	}
	if strings.TrimSpace(manifest.Name) != strings.TrimSpace(expectedName) ||
		!strings.EqualFold(strings.TrimSpace(manifest.SourceType), sourceTypeExternal) {
		return errors.New("staged Skill manifest 身份无效")
	}
	return nil
}

func publishStagedSkill(
	root *confinedfs.Root,
	stagingRelative string,
	targetRelative string,
) (*skillPublication, error) {
	publication := &skillPublication{
		root:           root,
		targetRelative: path.Clean(targetRelative),
	}
	if _, err := root.Lstat(publication.targetRelative); err == nil {
		publication.hadPrior = true
		backupRelative, allocateErr := allocateSkillBackup(root)
		if allocateErr != nil {
			return nil, allocateErr
		}
		publication.backupRelative = backupRelative
		if err = root.Rename(publication.targetRelative, backupRelative); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := root.Rename(stagingRelative, publication.targetRelative); err != nil {
		if publication.hadPrior {
			restoreErr := root.Rename(publication.backupRelative, publication.targetRelative)
			if restoreErr == nil {
				publication.finished = true
				return nil, err
			}
			return publication, errors.Join(err, restoreErr)
		}
		return nil, err
	}
	publication.targetHasNew = true
	return publication, nil
}

func stageSkillRemoval(
	root *confinedfs.Root,
	targetRelative string,
) (*skillPublication, error) {
	backupRelative, err := allocateSkillBackup(root)
	if err != nil {
		return nil, err
	}
	publication := &skillPublication{
		root:           root,
		targetRelative: path.Clean(targetRelative),
		backupRelative: backupRelative,
		hadPrior:       true,
	}
	if err = root.Rename(publication.targetRelative, backupRelative); err != nil {
		return nil, err
	}
	return publication, nil
}

func allocateSkillBackup(root *confinedfs.Root) (string, error) {
	backupRelative, err := root.MkdirTemp(
		privateSkillStagingRoot,
		".previous-skill-",
		0o700,
	)
	if err != nil {
		return "", err
	}
	if err = root.RemoveAll(backupRelative); err != nil {
		return "", err
	}
	return backupRelative, nil
}

func (p *skillPublication) rollback() error {
	if p == nil || p.finished {
		return nil
	}
	p.finished = true
	var result error
	if p.targetHasNew {
		result = errors.Join(result, p.root.RemoveAll(p.targetRelative))
	}
	if p.hadPrior {
		result = errors.Join(result, p.root.Rename(p.backupRelative, p.targetRelative))
	}
	return result
}

func (p *skillPublication) finalize() error {
	if p == nil || p.finished {
		return nil
	}
	p.finished = true
	if !p.hadPrior {
		return nil
	}
	return p.root.RemoveAll(p.backupRelative)
}
