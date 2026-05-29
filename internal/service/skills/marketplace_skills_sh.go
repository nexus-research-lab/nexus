package skills

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type skillsShImportTarget struct {
	RepositoryURL string
	SourceRef     string
	Identifier    string
	SkillPath     string
	SkillSlug     string
}

func parseSkillsShImportTarget(packageSpec string, skillSlug string) (skillsShImportTarget, error) {
	rawSpec := strings.TrimSpace(packageSpec)
	rawSlug := strings.TrimSpace(skillSlug)
	if rawSpec == "" {
		return skillsShImportTarget{}, errors.New("package_spec 不能为空")
	}
	rawSpec = strings.TrimPrefix(rawSpec, "skills.sh:")
	sourceRef, sourcePath := splitSkillsShPackageSpec(rawSpec)
	if sourceRef == "" {
		return skillsShImportTarget{}, errors.New("skills.sh 来源缺少 GitHub 仓库")
	}
	sourceParts := splitSkillsShID(sourceRef)
	if len(sourceParts) != 2 || !isSafeGitHubRepoSegment(sourceParts[0]) || !isSafeGitHubRepoSegment(sourceParts[1]) {
		return skillsShImportTarget{}, errors.New("skills.sh 来源 GitHub 仓库不合法")
	}
	sourceRef = strings.Join(sourceParts, "/")
	cleanSkillPath := firstNonEmpty(sourcePath, rawSlug)
	cleanSkillPath, err := cleanSkillSubdirPath(cleanSkillPath)
	if err != nil {
		return skillsShImportTarget{}, err
	}
	slug := firstNonEmpty(filepath.Base(filepath.FromSlash(cleanSkillPath)), rawSlug)
	if slug == "." || slug == string(os.PathSeparator) {
		slug = rawSlug
	}
	if strings.TrimSpace(slug) == "" {
		return skillsShImportTarget{}, errors.New("skill_slug 不能为空")
	}
	identifier := sourceRef
	if cleanSkillPath != "" {
		identifier += "/" + filepath.ToSlash(cleanSkillPath)
	}
	return skillsShImportTarget{
		RepositoryURL: "https://github.com/" + sourceRef,
		SourceRef:     sourceRef,
		Identifier:    identifier,
		SkillPath:     filepath.ToSlash(cleanSkillPath),
		SkillSlug:     strings.TrimSpace(slug),
	}, nil
}

func isSafeGitHubRepoSegment(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	return !strings.ContainsAny(value, `/\`)
}

func splitSkillsShPackageSpec(rawSpec string) (string, string) {
	if parsed, err := url.Parse(rawSpec); err == nil && parsed.Host != "" {
		host := strings.ToLower(parsed.Host)
		parts := splitSkillsShID(parsed.Path)
		switch host {
		case "github.com", "www.github.com":
			if len(parts) < 2 {
				return "", ""
			}
			if len(parts) >= 5 && parts[2] == "tree" {
				return strings.Join(parts[:2], "/"), strings.Join(parts[4:], "/")
			}
			return strings.Join(parts[:2], "/"), strings.Join(parts[2:], "/")
		case "skills.sh", "www.skills.sh":
			if len(parts) < 2 {
				return "", ""
			}
			return strings.Join(parts[:2], "/"), strings.Join(parts[2:], "/")
		}
	}
	if atIndex := strings.LastIndex(rawSpec, "@"); atIndex > 0 {
		return strings.Trim(strings.TrimSpace(rawSpec[:atIndex]), "/"), strings.Trim(strings.TrimSpace(rawSpec[atIndex+1:]), "/")
	}
	parts := splitSkillsShID(rawSpec)
	if len(parts) < 2 {
		return "", strings.Join(parts, "/")
	}
	return strings.Join(parts[:2], "/"), strings.Join(parts[2:], "/")
}

func findSkillsShSourceDir(root string, skillPath string, skillSlug string) (string, error) {
	cleanSkillPath, err := cleanSkillSubdirPath(skillPath)
	if err != nil {
		return "", err
	}
	if cleanSkillPath != "" {
		sourceDir := filepath.Join(root, cleanSkillPath)
		if _, statErr := os.Stat(filepath.Join(sourceDir, "SKILL.md")); statErr == nil {
			return sourceDir, nil
		}
	}
	type candidate struct {
		path  string
		score int
	}
	candidates := make([]candidate, 0)
	allSkillDirs := make([]string, 0)
	normalizedSlug := strings.ToLower(strings.TrimSpace(skillSlug))
	normalizedPath := strings.ToLower(filepath.ToSlash(cleanSkillPath))
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || info.Name() != "SKILL.md" {
			return nil
		}
		sourceDir := filepath.Dir(path)
		allSkillDirs = append(allSkillDirs, sourceDir)
		relativeDir, relErr := filepath.Rel(root, sourceDir)
		if relErr != nil {
			return relErr
		}
		relativeSlash := strings.ToLower(filepath.ToSlash(relativeDir))
		baseName := strings.ToLower(filepath.Base(sourceDir))
		score := 100
		if normalizedPath != "" && relativeSlash == normalizedPath {
			score = 0
		} else if normalizedPath != "" && (strings.HasSuffix(relativeSlash, "/"+normalizedPath) || relativeSlash == "skills/"+normalizedPath) {
			score = 1
		} else if normalizedSlug != "" && baseName == normalizedSlug {
			score = 2
		} else if normalizedSlug != "" {
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			parsed := parseSkillFrontmatter(string(content), filepath.Base(sourceDir))
			if strings.EqualFold(parsed.Name, skillSlug) || strings.EqualFold(parsed.Title, skillSlug) {
				score = 3
			}
		}
		if score < 100 {
			candidates = append(candidates, candidate{path: sourceDir, score: score})
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 && len(allSkillDirs) == 1 {
		return allSkillDirs[0], nil
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("未找到 skills.sh skill 目录: %s", firstNonEmpty(skillPath, skillSlug))
	}
	sort.Slice(candidates, func(i int, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score < candidates[j].score
		}
		return len(candidates[i].path) < len(candidates[j].path)
	})
	return candidates[0].path, nil
}

func buildSkillsPackageSpec(source string, slug string, name string) string {
	base := firstNonEmpty(strings.TrimSpace(source), strings.TrimSpace(slug))
	if base == "" {
		return name
	}
	if strings.Contains(base, "@") {
		return base
	}
	cleanBase := strings.Trim(base, "/")
	cleanSlug := firstNonEmpty(strings.Trim(strings.TrimSpace(slug), "/"), strings.Trim(strings.TrimSpace(name), "/"))
	if cleanSlug == "" || strings.HasSuffix(cleanBase, "/"+cleanSlug) {
		return cleanBase
	}
	return cleanBase + "/" + cleanSlug
}

func skillsShSourceFromID(id string) string {
	parts := splitSkillsShID(id)
	if len(parts) < 3 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], "/")
}

func skillsShSkillFromID(id string) string {
	parts := splitSkillsShID(id)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func splitSkillsShID(id string) []string {
	rawParts := strings.Split(strings.Trim(strings.TrimSpace(id), "/"), "/")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func skillsShDetailURL(apiURL string, sourceRef string, skillSlug string) string {
	sourceRef = strings.Trim(strings.TrimSpace(sourceRef), "/")
	skillSlug = strings.Trim(strings.TrimSpace(skillSlug), "/")
	if sourceRef == "" || skillSlug == "" {
		return strings.TrimRight(strings.TrimSpace(apiURL), "/") + "/" + skillSlug
	}
	parsed, err := url.Parse(strings.TrimSpace(apiURL))
	if err != nil || parsed.Host == "" {
		return "https://www.skills.sh/" + sourceRef + "/" + skillSlug
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	if strings.EqualFold(parsed.Host, "skills.sh") {
		parsed.Host = "www.skills.sh"
	}
	parsed.Path = "/" + sourceRef + "/" + skillSlug
	return parsed.String()
}
