package logx

import (
	"strconv"
	"strings"
)

func pickScope(fields []field) (string, []field) {
	service, component := "", ""
	rest := make([]field, 0, len(fields))
	for _, f := range fields {
		switch f.key {
		case "service":
			service = f.value
		case "component":
			component = f.value
		default:
			rest = append(rest, f)
		}
	}
	switch {
	case service != "" && component != "":
		return service + "/" + component, rest
	case service != "":
		return service, rest
	default:
		return component, rest
	}
}

// pickAccess 识别 method/status/path 都齐的 HTTP access log，折叠成 accessLog。
func pickAccess(fields []field) (*accessLog, []field) {
	var method, path, durationMs, bytesWritten, remoteIP string
	var status int
	hasMethod, hasStatus, hasPath := false, false, false
	rest := make([]field, 0, len(fields))
	for _, f := range fields {
		switch f.key {
		case "method":
			method = f.value
			hasMethod = true
		case "status":
			if value, err := strconv.Atoi(f.value); err == nil {
				status = value
				hasStatus = true
			} else {
				rest = append(rest, f)
			}
		case "path":
			path = f.value
			hasPath = true
		case "duration_ms":
			durationMs = f.value
		case "bytes":
			bytesWritten = f.value
		case "remote_ip":
			remoteIP = f.value
		default:
			rest = append(rest, f)
		}
	}
	if !(hasMethod && hasStatus && hasPath) {
		// 不是 access log，把抽出来的字段补回去。
		if method != "" {
			rest = append(rest, field{key: "method", value: method})
		}
		if path != "" {
			rest = append(rest, field{key: "path", value: path})
		}
		if durationMs != "" {
			rest = append(rest, field{key: "duration_ms", value: durationMs})
		}
		if bytesWritten != "" {
			rest = append(rest, field{key: "bytes", value: bytesWritten})
		}
		if remoteIP != "" {
			rest = append(rest, field{key: "remote_ip", value: remoteIP})
		}
		return nil, rest
	}
	if remoteIP != "" && remoteIP != "127.0.0.1" && remoteIP != "::1" {
		rest = append(rest, field{key: "ip", value: remoteIP})
	}
	return &accessLog{
		method:   method,
		status:   status,
		duration: formatDuration(durationMs),
		bytes:    formatBytes(bytesWritten),
		path:     path,
	}, rest
}

func pickRequestID(fields []field) (string, []field) {
	rest := make([]field, 0, len(fields))
	requestID := ""
	for _, f := range fields {
		if f.key == "request_id" {
			requestID = f.value
			continue
		}
		rest = append(rest, f)
	}
	if len(requestID) > 8 {
		requestID = requestID[:8]
	}
	return requestID, rest
}

func pickSDKDebug(fields []field) (*sdkDebugLog, []field) {
	debugLog := &sdkDebugLog{}
	rest := make([]field, 0, len(fields))
	hasSummary := false
	for _, f := range fields {
		switch f.key {
		case "sdk_summary":
			debugLog.summary = f.value
			hasSummary = strings.TrimSpace(f.value) != ""
		case "session_key":
			debugLog.sessionKey = f.value
		case "agent_id":
			debugLog.agentID = f.value
		case "round_id":
			debugLog.roundID = f.value
		default:
			rest = append(rest, f)
		}
	}
	if !hasSummary {
		return nil, fields
	}
	return debugLog, rest
}
