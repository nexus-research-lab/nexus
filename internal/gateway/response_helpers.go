// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：response_helpers.go
// @Date   ：2026/04/17 10:30:00
// @Author ：leemysw
// 2026/04/17 10:30:00   Create
// =====================================================

package gateway

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Server) handleNotImplemented(group string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		s.writeJSON(writer, http.StatusNotImplemented, map[string]any{
			"code": 1,
			"msg":  "not_implemented",
			"data": map[string]any{
				"group": group,
				"path":  request.URL.Path,
			},
		})
	}
}

func (s *Server) writeJSON(writer http.ResponseWriter, status int, payload map[string]any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}

func (s *Server) writeSuccess(writer http.ResponseWriter, data any) {
	s.writeJSON(writer, http.StatusOK, map[string]any{
		"code":    "0000",
		"message": "success",
		"success": true,
		"data":    data,
	})
}

func (s *Server) writeFailure(writer http.ResponseWriter, status int, detail string) {
	clientDetail := strings.TrimSpace(detail)
	if clientDetail != "" {
		s.baseLogger().Warn("网关请求失败", "status", status, "detail", clientDetail)
	}
	clientDetail = gatewayClientErrorDetail(status, clientDetail)
	s.writeJSON(writer, status, map[string]any{
		"code":    fmtStatusCode(status),
		"message": "failed",
		"success": false,
		"data": map[string]any{
			"detail": clientDetail,
		},
	})
}

func gatewayClientErrorDetail(status int, detail string) string {
	switch status {
	case http.StatusBadRequest:
		if isClientMessageText(detail) {
			return detail
		}
		return "请求参数错误"
	case http.StatusUnauthorized:
		return "未授权"
	case http.StatusForbidden:
		return "禁止访问"
	case http.StatusNotFound:
		return "资源不存在"
	case http.StatusConflict:
		return "请求冲突"
	case http.StatusUnprocessableEntity:
		if isClientMessageText(detail) {
			return detail
		}
		return "请求无效"
	default:
		if status >= http.StatusInternalServerError {
			return "服务内部错误"
		}
		if isClientMessageText(detail) {
			return detail
		}
		return "请求失败"
	}
}

func (s *Server) bindJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	return s.bindJSONWithOptions(writer, request, target, false)
}

func (s *Server) bindJSONAllowEmpty(writer http.ResponseWriter, request *http.Request, target any) bool {
	return s.bindJSONWithOptions(writer, request, target, true)
}

func (s *Server) bindJSONWithOptions(
	writer http.ResponseWriter,
	request *http.Request,
	target any,
	allowEmpty bool,
) bool {
	if err := decodeJSONBody(request.Body, target, allowEmpty); err != nil {
		if allowEmpty && errors.Is(err, io.EOF) {
			return true
		}
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return false
	}
	return true
}

func decodeJSONBody(body io.Reader, target any, allowEmpty bool) error {
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(target); err != nil {
		if allowEmpty && errors.Is(err, io.EOF) {
			return io.EOF
		}
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
	}
	return errors.New("json body must contain a single top-level value")
}

func fmtStatusCode(status int) string {
	return strings.TrimSpace(strconv.Itoa(status))
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func isClientMessageError(err error) bool {
	if err == nil {
		return false
	}
	return isClientMessageText(err.Error())
}

func isClientMessageText(message string) bool {
	return strings.Contains(message, "不能为空") ||
		strings.Contains(message, "不一致") ||
		strings.Contains(message, "已存在") ||
		strings.Contains(message, "至少") ||
		strings.Contains(message, "不支持") ||
		strings.Contains(message, "不能作为") ||
		strings.Contains(message, " is required") ||
		strings.Contains(message, " must be ") ||
		strings.Contains(message, "正在运行中")
}

func isStructuredSessionKeyError(err error) bool {
	if err == nil {
		return false
	}
	var target protocol.StructuredSessionKeyError
	return errors.As(err, &target)
}

func stringValue(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
}

func boolValue(value any) (bool, bool) {
	typed, ok := value.(bool)
	if ok {
		return typed, true
	}
	return false, false
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	default:
		return 0
	}
}
