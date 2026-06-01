package clientopts

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

const runtimeArgFileMaxAge = 24 * time.Hour

var runtimeArgFilesRoot = defaultRuntimeArgFilesRoot

func defaultRuntimeArgFilesRoot() string {
	return filepath.Join(appfs.ConfigDir(), "runtime", "arg-files")
}

func materializeProcessArgFiles(options *agentclient.Options) error {
	return materializeProcessArgFilesForOS(runtime.GOOS, options)
}

func materializeProcessArgFilesForOS(goos string, options *agentclient.Options) error {
	if goos != "windows" || options == nil {
		return nil
	}
	if err := cleanupRuntimeArgFiles(); err != nil {
		return err
	}
	if options.System.Append != "" {
		path, err := writeRuntimeArgFile("append-system-prompt", ".txt", []byte(options.System.Append))
		if err != nil {
			return fmt.Errorf("write append system prompt arg file: %w", err)
		}
		if options.ExtraArgs == nil {
			options.ExtraArgs = map[string]string{}
		}
		options.ExtraArgs["append-system-prompt-file"] = path
		options.System.Append = ""
	}
	if len(options.MCP.Servers) > 0 && strings.TrimSpace(options.MCP.Config) == "" {
		payload, err := marshalRuntimeMCPConfig(options.MCP.Servers)
		if err != nil {
			return err
		}
		path, err := writeRuntimeArgFile("mcp-config", ".json", payload)
		if err != nil {
			return fmt.Errorf("write MCP config arg file: %w", err)
		}
		options.MCP.Config = path
	}
	return nil
}

func cleanupRuntimeArgFiles() error {
	root := runtimeArgFilesRoot()
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("create runtime arg file dir: %w", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	expiredBefore := time.Now().Add(-runtimeArgFileMaxAge)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(expiredBefore) {
			continue
		}
		_ = os.Remove(filepath.Join(root, entry.Name()))
	}
	return nil
}

func writeRuntimeArgFile(prefix string, extension string, payload []byte) (string, error) {
	root := runtimeArgFilesRoot()
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(root, prefix+"-*"+extension)
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := os.Chmod(path, 0o600); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if _, err := file.Write(payload); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func marshalRuntimeMCPConfig(servers map[string]sdkmcp.ServerConfig) ([]byte, error) {
	mcpServers := map[string]any{}
	for name, config := range servers {
		if config == nil {
			continue
		}
		payload, err := marshalRuntimeMCPServer(name, config)
		if err != nil {
			return nil, err
		}
		if stringMapValue(payload, "scope") == "" {
			payload["scope"] = "dynamic"
		}
		mcpServers[name] = payload
	}
	return json.Marshal(map[string]any{
		"mcpServers": mcpServers,
	})
}

func marshalRuntimeMCPServer(name string, config sdkmcp.ServerConfig) (map[string]any, error) {
	switch typed := config.(type) {
	case sdkmcp.StdioServerConfig:
		if strings.TrimSpace(typed.Command) == "" {
			return nil, fmt.Errorf("mcp: server %q command is empty", name)
		}
		payload := map[string]any{
			"command": typed.Command,
		}
		if len(typed.Args) > 0 {
			payload["args"] = typed.Args
		}
		if len(typed.Env) > 0 {
			payload["env"] = typed.Env
		}
		return payload, nil
	case sdkmcp.SSEServerConfig:
		return marshalHTTPRuntimeMCPServer(name, "sse", typed.URL, typed.Headers, typed.HeadersHelper, typed.OAuth)
	case sdkmcp.HTTPServerConfig:
		return marshalHTTPRuntimeMCPServer(name, "http", typed.URL, typed.Headers, typed.HeadersHelper, typed.OAuth)
	case sdkmcp.SDKServerConfig:
		if typed.Instance == nil {
			return nil, fmt.Errorf("mcp: sdk server %q instance is nil", name)
		}
		serverName := strings.TrimSpace(typed.Name)
		if serverName == "" {
			serverName = name
		}
		return map[string]any{
			"type": "sdk",
			"name": serverName,
		}, nil
	default:
		return nil, fmt.Errorf("mcp: server %q has unsupported config type %T", name, config)
	}
}

func marshalHTTPRuntimeMCPServer(
	name string,
	serverType string,
	serverURL string,
	headers map[string]string,
	headersHelper string,
	oauth *sdkmcp.OAuthConfig,
) (map[string]any, error) {
	if strings.TrimSpace(serverURL) == "" {
		return nil, fmt.Errorf("mcp: server %q url is empty", name)
	}
	payload := map[string]any{
		"type": serverType,
		"url":  serverURL,
	}
	if len(headers) > 0 {
		payload["headers"] = headers
	}
	if helper := strings.TrimSpace(headersHelper); helper != "" {
		payload["headersHelper"] = helper
	}
	if oauth != nil {
		oauthPayload, err := marshalRuntimeMCPOAuth(name, *oauth)
		if err != nil {
			return nil, err
		}
		if len(oauthPayload) > 0 {
			payload["oauth"] = oauthPayload
		}
	}
	return payload, nil
}

func marshalRuntimeMCPOAuth(name string, config sdkmcp.OAuthConfig) (map[string]any, error) {
	payload := map[string]any{}
	if clientID := strings.TrimSpace(config.ClientID); clientID != "" {
		payload["clientId"] = clientID
	}
	if config.CallbackPort < 0 {
		return nil, fmt.Errorf("mcp: server %q oauth callback port must be positive", name)
	}
	if config.CallbackPort > 0 {
		payload["callbackPort"] = config.CallbackPort
	}
	if metadataURL := strings.TrimSpace(config.AuthServerMetadataURL); metadataURL != "" {
		parsed, err := url.Parse(metadataURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return nil, fmt.Errorf("mcp: server %q oauth authServerMetadataUrl must be an https URL", name)
		}
		payload["authServerMetadataUrl"] = metadataURL
	}
	if config.XAA != nil {
		payload["xaa"] = *config.XAA
	}
	return payload, nil
}

func stringMapValue(values map[string]any, key string) string {
	if value, ok := values[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}
