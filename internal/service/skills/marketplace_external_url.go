package skills

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

func externalSearchURL(sourceURL string, defaultPath string, queryValues map[string]string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("skill 来源 URL 不正确")
	}
	if strings.Trim(parsed.Path, "/") == "" {
		parsed.Path = defaultPath
	}
	values := parsed.Query()
	for key, value := range queryValues {
		if strings.TrimSpace(value) != "" {
			values.Set(key, value)
		}
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func githubTreeURL(gitURL string, gitPath string) string {
	gitURL = strings.TrimSpace(gitURL)
	gitPath = strings.TrimSpace(gitPath)
	if gitURL == "" || gitPath == "" {
		return gitURL
	}
	return strings.TrimRight(gitURL, "/") + "/tree/main/" + strings.Trim(gitPath, "/")
}

func githubIdentifierFromRepoPath(repo string, skillPath string) string {
	repo = strings.Trim(strings.TrimSpace(repo), "/")
	skillPath = strings.Trim(strings.TrimSpace(skillPath), "/")
	if repo == "" || skillPath == "" {
		return ""
	}
	return repo + "/" + skillPath
}

func splitGitHubIdentifier(identifier string) (string, string) {
	parts := strings.SplitN(strings.Trim(strings.TrimSpace(identifier), "/"), "/", 3)
	if len(parts) < 3 {
		return "", ""
	}
	repo := parts[0] + "/" + parts[1]
	return "https://github.com/" + repo, parts[2]
}

func githubBlobToRawURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || !strings.EqualFold(parsed.Host, "github.com") {
		return ""
	}
	parts := strings.SplitN(strings.Trim(parsed.Path, "/"), "/", 5)
	if len(parts) < 5 || parts[2] != "blob" {
		return ""
	}
	return "https://raw.githubusercontent.com/" + parts[0] + "/" + parts[1] + "/" + parts[3] + "/" + parts[4]
}

func normalizeExternalTrust(trust string) string {
	switch strings.ToLower(strings.TrimSpace(trust)) {
	case "official", "builtin", "trusted":
		return externalSourceTrustOfficial
	case "private":
		return externalSourceTrustPrivate
	default:
		return externalSourceTrustCommunity
	}
}

func clawhubDownloadURL(sourceURL string, slug string) string {
	parsed, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil || parsed.Host == "" || strings.TrimSpace(slug) == "" {
		return ""
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	path := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(path, "/search") {
		path = strings.TrimSuffix(path, "/search")
	}
	if path == "" || path == "/" || !strings.Contains(path, "/api/") {
		path = "/api/v1"
	}
	parsed.Path = strings.TrimRight(path, "/") + "/download"
	values := parsed.Query()
	values.Set("slug", strings.TrimSpace(slug))
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func clawhubDetailURL(sourceURL string, owner string, slug string) string {
	parsed, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil || parsed.Host == "" {
		return "https://clawhub.ai/skills/" + strings.TrimSpace(slug)
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	if strings.TrimSpace(owner) != "" {
		parsed.Path = "/" + strings.Trim(strings.TrimSpace(owner), "/") + "/" + strings.Trim(strings.TrimSpace(slug), "/")
	} else {
		parsed.Path = "/skills/" + strings.Trim(strings.TrimSpace(slug), "/")
	}
	return parsed.String()
}

func inferExternalImportMode(item ExternalSkillSearchItem) string {
	if strings.TrimSpace(item.GitURL) != "" {
		return externalSourceKindGit
	}
	if strings.TrimSpace(item.RawURL) != "" || externalURLLooksImportable(item.DetailURL) {
		return externalSourceKindURL
	}
	if strings.TrimSpace(item.PackageSpec) != "" {
		return externalSourceKindSkillsSh
	}
	return ""
}

func normalizeImportMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "claude-plugins", "claude_plugins", "claude-plugins.dev":
		return externalSourceKindClaudePlugins
	case "skills.sh", "skillssh", "skills_sh":
		return externalSourceKindSkillsSh
	case "clawhub", "clawhub.ai":
		return externalSourceKindClawhub
	case "hermes", "hermes-index", "hermes_index":
		return externalSourceKindHermesIndex
	case "browse.sh", "browsesh", "browse_sh", "browse-sh":
		return externalSourceKindBrowseSh
	case "github", "git":
		return externalSourceKindGit
	case "direct", "direct_url", "url", "zip":
		return externalSourceKindURL
	default:
		return strings.TrimSpace(mode)
	}
}

func externalURLLooksImportable(rawURL string) bool {
	path := strings.ToLower(strings.TrimSpace(rawURL))
	return strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".zip")
}

func (s *Service) validateExternalURL(ctx context.Context, rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", errors.New("skills 外部链接非法")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("skills 外部链接协议非法")
	}
	if parsed.Host == "" {
		return "", errors.New("skills 外部链接域名为空")
	}
	canonicalizeSkillsShExternalURL(parsed)
	allowedHosts := map[string]struct{}{
		"claude-plugins.dev":        {},
		"skills.sh":                 {},
		"www.skills.sh":             {},
		"clawhub.ai":                {},
		"github.com":                {},
		"raw.githubusercontent.com": {},
	}
	for _, source := range s.externalSkillSources(ctx) {
		sourceURL, parseErr := url.Parse(source.URL)
		if parseErr == nil && sourceURL.Host != "" {
			allowedHosts[strings.ToLower(sourceURL.Host)] = struct{}{}
		}
	}
	if _, ok := allowedHosts[strings.ToLower(parsed.Host)]; !ok {
		return "", errors.New("skills 外部链接域名未在来源白名单中")
	}
	return parsed.String(), nil
}

func canonicalizeSkillsShExternalURL(parsed *url.URL) {
	if parsed == nil || !strings.EqualFold(parsed.Host, "skills.sh") {
		return
	}
	path := strings.Trim(strings.ToLower(parsed.Path), "/")
	if path == "" || strings.HasPrefix(path, "api/") {
		return
	}
	parsed.Host = "www.skills.sh"
}

func isSkillsShPreviewURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Host)
	return host == "skills.sh" || host == "www.skills.sh"
}
