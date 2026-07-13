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
	candidate := accessCandidate{}
	rest := make([]field, 0, len(fields))
	for _, f := range fields {
		setter := accessFieldSetters[f.key]
		if setter == nil || !setter(&candidate, f.value) {
			rest = append(rest, f)
		}
	}
	if !candidate.complete() {
		// 不完整的访问字段仍是普通日志上下文，保持原顺序交给通用渲染器。
		return nil, fields
	}
	if candidate.hasRemoteAddress() {
		rest = append(rest, field{key: "ip", value: candidate.remoteIP})
	}
	return &accessLog{
		method:   candidate.method,
		status:   candidate.status,
		duration: formatDuration(candidate.durationMs),
		bytes:    formatBytes(candidate.bytesWritten),
		path:     candidate.path,
	}, rest
}

type accessCandidate struct {
	method       string
	path         string
	durationMs   string
	bytesWritten string
	remoteIP     string
	status       int
	hasMethod    bool
	hasStatus    bool
	hasPath      bool
}

type accessFieldSetter func(*accessCandidate, string) bool

var accessFieldSetters = map[string]accessFieldSetter{
	"method":      setAccessMethod,
	"status":      setAccessStatus,
	"path":        setAccessPath,
	"duration_ms": setAccessDuration,
	"bytes":       setAccessBytes,
	"remote_ip":   setAccessRemoteIP,
}

func setAccessMethod(candidate *accessCandidate, value string) bool {
	candidate.method = value
	candidate.hasMethod = true
	return true
}

func setAccessStatus(candidate *accessCandidate, value string) bool {
	status, err := strconv.Atoi(value)
	if err != nil {
		return false
	}
	candidate.status = status
	candidate.hasStatus = true
	return true
}

func setAccessPath(candidate *accessCandidate, value string) bool {
	candidate.path = value
	candidate.hasPath = true
	return true
}

func setAccessDuration(candidate *accessCandidate, value string) bool {
	candidate.durationMs = value
	return true
}

func setAccessBytes(candidate *accessCandidate, value string) bool {
	candidate.bytesWritten = value
	return true
}

func setAccessRemoteIP(candidate *accessCandidate, value string) bool {
	candidate.remoteIP = value
	return true
}

func (c accessCandidate) complete() bool {
	return c.hasMethod && c.hasStatus && c.hasPath
}

func (c accessCandidate) hasRemoteAddress() bool {
	return c.remoteIP != "" && c.remoteIP != "127.0.0.1" && c.remoteIP != "::1"
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
