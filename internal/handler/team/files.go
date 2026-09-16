package team

import (
	"context"
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
	"time"
)

func (h *Handlers) HandleRoomFiles(w http.ResponseWriter, r *http.Request) {
	h.noStore(w)
	if r.Method != http.MethodGet && !h.requireMutationOrigin(w, r) {
		return
	}
	token, ok := h.exchangeToken(w, r, false)
	if !ok {
		return
	}
	roomID, fileID := chi.URLParam(r, "room_id"), chi.URLParam(r, "file_id")
	if !validResourceID(roomID) || (fileID != "" && !validResourceID(fileID)) || r.ContentLength > 32<<20 {
		h.writeRequestError(w, r, "team.file_invalid", "文件请求无效，单文件最大 32 MiB", false)
		return
	}
	client, ok := h.relay.(interface {
		RoomFiles(context.Context, string, string, string, string, http.Header, int64, io.Reader) (*http.Response, error)
	})
	if !ok {
		h.api.WriteFailure(w, http.StatusServiceUnavailable, "共享文件服务不可用")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	response, err := client.RoomFiles(ctx, token, roomID, fileID, r.Method, r.Header, r.ContentLength, r.Body)
	if err != nil {
		h.api.WriteFailure(w, http.StatusBadGateway, "共享文件请求未确认，请重试原操作")
		return
	}
	defer response.Body.Close()
	for _, name := range []string{"Content-Type", "Content-Disposition", "X-Content-Type-Options"} {
		if value := response.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(response.Body, 33<<20))
}
