// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：middleware.go
// @Date   ：2026/04/11 20:21:00
// @Author ：leemysw
// 2026/04/11 20:21:00   Create
// =====================================================

package gateway

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/nexus-research-lab/nexus-core/internal/logx"
	"io"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// 中文注释：responseRecorder 负责在不破坏 websocket/hijack 能力的前提下记录状态码和字节数。
type responseRecorder struct {
	http.ResponseWriter
	status       int
	bytesWritten int
	wroteHeader  bool
}

func newResponseRecorder(writer http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: writer,
		status:         http.StatusOK,
	}
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(payload []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	size, err := r.ResponseWriter.Write(payload)
	r.bytesWritten += size
	return size, err
}

func (r *responseRecorder) ReadFrom(reader io.Reader) (int64, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	if source, ok := r.ResponseWriter.(io.ReaderFrom); ok {
		size, err := source.ReadFrom(reader)
		r.bytesWritten += int(size)
		return size, err
	}
	return io.Copy(r, reader)
}

func (r *responseRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (r *responseRecorder) Push(target string, options *http.PushOptions) error {
	pusher, ok := r.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, options)
}

func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (r *responseRecorder) HeaderWritten() bool {
	return r.wroteHeader
}

func (s *Server) requestContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := strings.TrimSpace(request.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = generateRequestID()
		}
		writer.Header().Set("X-Request-ID", requestID)

		requestLogger := s.baseLogger().With("request_id", requestID)
		ctx := logx.WithRequestID(request.Context(), requestID)
		ctx = logx.WithLogger(ctx, requestLogger)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func (s *Server) accessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		startedAt := time.Now()
		recorder := newResponseRecorder(writer)
		next.ServeHTTP(recorder, request)

		requestLogger := logx.FromContext(request.Context())
		duration := time.Since(startedAt)
		fields := []any{
			"method", request.Method,
			"path", request.URL.Path,
			"status", recorder.status,
			"duration_ms", duration.Milliseconds(),
			"bytes", recorder.bytesWritten,
			"remote_ip", clientIP(request),
		}
		if userAgent := strings.TrimSpace(request.UserAgent()); userAgent != "" {
			fields = append(fields, "user_agent", userAgent)
		}
		if rawQuery := strings.TrimSpace(request.URL.RawQuery); rawQuery != "" {
			fields = append(fields, "query", rawQuery)
		}

		switch {
		case recorder.status >= http.StatusInternalServerError:
			requestLogger.Error("HTTP 请求完成", fields...)
		case recorder.status >= http.StatusBadRequest:
			requestLogger.Warn("HTTP 请求完成", fields...)
		default:
			requestLogger.Info("HTTP 请求完成", fields...)
		}
	})
}

func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				requestLogger := logx.FromContext(request.Context())
				requestLogger.Error("HTTP 请求 panic",
					"method", request.Method,
					"path", request.URL.Path,
					"remote_ip", clientIP(request),
					"panic", fmt.Sprint(recovered),
					"stack", string(debug.Stack()),
				)

				if recorder, ok := writer.(*responseRecorder); ok && recorder.HeaderWritten() {
					return
				}
				s.writeFailure(writer, http.StatusInternalServerError, "服务内部错误")
			}
		}()
		next.ServeHTTP(writer, request)
	})
}

func (s *Server) baseLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return logx.NewDiscardLogger()
}

func clientIP(request *http.Request) string {
	if forwarded := strings.TrimSpace(request.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if realIP := strings.TrimSpace(request.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(request.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(request.RemoteAddr)
}

func generateRequestID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}
