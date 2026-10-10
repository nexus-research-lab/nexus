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
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	response, err := h.relay.RoomFiles(ctx, token, roomID, fileID, r.Method, r.Header, r.ContentLength, r.Body)
	if err != nil {
		h.api.WriteFailure(w, http.StatusBadGateway, "共享文件请求未确认，请重试原操作")
		return
	}
	defer response.Body.Close()
	if fileID != "" {
		// 上传者可控内容不能以可渲染类型落在 Nexus 源上；浏览器只经 fetch→Blob 保存。
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "sandbox")
	} else if value := response.Header.Get("Content-Type"); value != "" {
		w.Header().Set("Content-Type", value)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(response.Body, 33<<20))
}
