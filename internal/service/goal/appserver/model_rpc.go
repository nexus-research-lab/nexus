// INPUT: Codex app-server thread/goal JSON-RPC request, response and transport error classification.
// OUTPUT: stable request IDs plus invalid-request/internal/conflict envelopes with machine-readable reason codes.
// POS: Goal app-server wire contract; domain errors are classified by the WebSocket transport adapter.
package appserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	AppServerRPCInvalidRequestCode int64 = -32600
	AppServerRPCMethodNotFoundCode int64 = -32601
	AppServerRPCInternalErrorCode  int64 = -32603
	// AppServerRPCConflictCode belongs to the JSON-RPC server-error range and
	// represents a retryable Goal concurrency or binding conflict. It is not an
	// HTTP status code.
	AppServerRPCConflictCode int64 = -32009

	AppServerRPCReasonConflict                 = "conflict"
	AppServerRPCReasonVersionStale             = "version_stale"
	AppServerRPCReasonRevisionStale            = "revision_stale"
	AppServerRPCReasonExecutionBindingConflict = "execution_binding_conflict"
)

// AppServerRequestID 保留 Codex app-server JSON-RPC id 的原始 string/number 表示。
type AppServerRequestID struct {
	raw json.RawMessage
}

func (id *AppServerRequestID) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return errors.New("request id is required")
	}
	var probe any
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&probe); err != nil {
		return err
	}
	switch value := probe.(type) {
	case string:
		id.raw = append(id.raw[:0], trimmed...)
		return nil
	case json.Number:
		if _, err := value.Int64(); err != nil {
			return fmt.Errorf("unsupported request id number %q", value.String())
		}
		id.raw = append(id.raw[:0], trimmed...)
		return nil
	default:
		return fmt.Errorf("unsupported request id type %T", probe)
	}
}

func (id AppServerRequestID) MarshalJSON() ([]byte, error) {
	if len(id.raw) == 0 {
		return []byte("null"), nil
	}
	return bytes.Clone(id.raw), nil
}

func (id AppServerRequestID) IsZero() bool {
	return len(id.raw) == 0
}

// AppServerJSONRPCRequest 是 Codex app-server 使用的轻量 JSON-RPC 请求。
type AppServerJSONRPCRequest struct {
	JSONRPC string             `json:"jsonrpc,omitempty"`
	ID      AppServerRequestID `json:"id"`
	Method  string             `json:"method"`
	Params  json.RawMessage    `json:"params,omitempty"`
}

type AppServerJSONRPCResponse struct {
	ID     AppServerRequestID `json:"id"`
	Result any                `json:"result"`
}

type AppServerJSONRPCError struct {
	ID    AppServerRequestID    `json:"id"`
	Error AppServerRPCErrorBody `json:"error"`
}

type AppServerRPCErrorBody struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// AppServerRPCErrorData gives clients a stable branch key without requiring
// them to parse the human-readable error message.
type AppServerRPCErrorData struct {
	ReasonCode string `json:"reason_code"`
}

type AppServerJSONRPCNotification struct {
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

func NewAppServerRPCError(code int64, message string) AppServerRPCErrorBody {
	return AppServerRPCErrorBody{Code: code, Message: message}
}

func NewAppServerRPCConflictError(message, reasonCode string) AppServerRPCErrorBody {
	return AppServerRPCErrorBody{
		Code:    AppServerRPCConflictCode,
		Message: message,
		Data:    AppServerRPCErrorData{ReasonCode: reasonCode},
	}
}
