package protocol

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
	return append([]byte(nil), id.raw...), nil
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

type AppServerJSONRPCNotification struct {
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

func NewAppServerRPCError(code int64, message string) AppServerRPCErrorBody {
	return AppServerRPCErrorBody{Code: code, Message: message}
}
