// INPUT: 宿主 key mode、canonical state root、显式 active/legacy 配置与平台 Keychain loader。
// OUTPUT: 一把确定的 active key、去重 legacy keys 和不含秘密的来源标识。
// POS: nexus-server 启动期的统一宿主密钥解析器；Makefile 与业务 service 不接触平台密钥选择。
package credentials

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	hostKeyModeExplicit = "explicit"
	hostKeyModeFile     = "file"
	hostKeyModeKeychain = "keychain"
	hostKeyModeAuto     = "auto"
)

// HostKeys 是宿主解析后的 active/legacy 密钥配置；Source 可进入日志，keys 不得记录。
type HostKeys struct {
	Active string
	Legacy []string
	Source string
}

type hostKeyCandidate struct {
	raw    string
	source string
}

// ResolveHostKeys 按显式模式解析 Connector credentials key 来源。
func ResolveHostKeys(
	mode string,
	stateRoot string,
	explicitActive string,
	explicitLegacy []string,
) (HostKeys, error) {
	return resolveHostKeys(
		mode,
		stateRoot,
		explicitActive,
		explicitLegacy,
		loadHostKeychainConnectorKey,
	)
}

func resolveHostKeys(
	mode string,
	stateRoot string,
	explicitActive string,
	explicitLegacy []string,
	keychainLoader func() (string, error),
) (HostKeys, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = hostKeyModeExplicit
	}
	if mode == hostKeyModeExplicit {
		return selectHostKeys(
			[]hostKeyCandidate{{raw: explicitActive, source: "explicit"}},
			explicitLegacy,
		)
	}
	if mode != hostKeyModeFile && mode != hostKeyModeKeychain && mode != hostKeyModeAuto {
		return HostKeys{}, fmt.Errorf("不支持 CONNECTOR_CREDENTIALS_HOST_KEY_MODE=%q", mode)
	}

	if mode == hostKeyModeFile {
		fileKey, fileErr := readHostFallbackKey(stateRoot)
		if fileErr != nil && !errors.Is(fileErr, os.ErrNotExist) {
			return HostKeys{}, fileErr
		}
		if strings.TrimSpace(fileKey) == "" {
			return HostKeys{}, fmt.Errorf("Connector credentials fallback key 不存在: %w", fileErr)
		}
		return selectHostKeys(
			[]hostKeyCandidate{{raw: fileKey, source: "state_file"}, {raw: explicitActive, source: "explicit"}},
			explicitLegacy,
		)
	}
	if mode == hostKeyModeKeychain {
		keychainKey, keychainErr := keychainLoader()
		if keychainErr != nil || strings.TrimSpace(keychainKey) == "" {
			if keychainErr == nil {
				keychainErr = errors.New("Connector credentials Keychain item 为空")
			}
			return HostKeys{}, fmt.Errorf("读取 Connector credentials Keychain: %w", keychainErr)
		}
		fileKey, fileErr := readHostFallbackKey(stateRoot)
		if fileErr != nil && !errors.Is(fileErr, os.ErrNotExist) {
			return HostKeys{}, fileErr
		}
		return selectHostKeys(
			[]hostKeyCandidate{{raw: keychainKey, source: "keychain"}, {raw: fileKey, source: "state_file"}, {raw: explicitActive, source: "explicit"}},
			explicitLegacy,
		)
	}

	fileKey, fileErr := readHostFallbackKey(stateRoot)
	if fileErr != nil && !errors.Is(fileErr, os.ErrNotExist) {
		return HostKeys{}, fileErr
	}
	keychainKey, keychainErr := keychainLoader()
	candidates := make([]hostKeyCandidate, 0, 3)
	if keychainErr == nil {
		candidates = append(candidates, hostKeyCandidate{raw: keychainKey, source: "keychain"})
	}
	candidates = append(candidates,
		hostKeyCandidate{raw: fileKey, source: "state_file"},
		hostKeyCandidate{raw: explicitActive, source: "explicit"},
	)
	return selectHostKeys(candidates, explicitLegacy)
}

func selectHostKeys(
	candidates []hostKeyCandidate,
	explicitLegacy []string,
) (HostKeys, error) {
	ordered := make([]hostKeyCandidate, 0, len(candidates)+len(explicitLegacy))
	ordered = append(ordered, candidates...)
	for _, raw := range explicitLegacy {
		ordered = append(ordered, hostKeyCandidate{raw: raw, source: "explicit_legacy"})
	}
	seen := make(map[string]struct{}, len(ordered))
	valid := make([]hostKeyCandidate, 0, len(ordered))
	for _, candidate := range ordered {
		if strings.TrimSpace(candidate.raw) == "" {
			continue
		}
		key, err := DecodeKey(candidate.raw)
		if err != nil {
			return HostKeys{}, fmt.Errorf("解析 %s Connector credentials key: %w", candidate.source, err)
		}
		keyID := KeyID(key)
		if _, exists := seen[keyID]; exists {
			continue
		}
		seen[keyID] = struct{}{}
		valid = append(valid, candidate)
	}
	if len(valid) == 0 {
		return HostKeys{Source: "unconfigured"}, nil
	}
	result := HostKeys{Active: strings.TrimSpace(valid[0].raw), Source: valid[0].source}
	for _, candidate := range valid[1:] {
		result.Legacy = append(result.Legacy, strings.TrimSpace(candidate.raw))
	}
	return result, nil
}

func readHostFallbackKey(stateRoot string) (string, error) {
	stateRoot = strings.TrimSpace(stateRoot)
	if stateRoot == "" {
		return "", os.ErrNotExist
	}
	path := filepath.Join(filepath.Clean(stateRoot), "app", "config", "connector-credentials.key")
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(payload))
	if value == "" {
		return "", errors.New("Connector credentials fallback key 为空")
	}
	return value, nil
}
