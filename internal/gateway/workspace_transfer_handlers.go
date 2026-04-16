// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：workspace_transfer_handlers.go
// @Date   ：2026/04/16 20:48:00
// @Author ：leemysw
// 2026/04/16 20:48:00   Create
// =====================================================

package gateway

import (
	"errors"
	"net/http"
	"strings"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/workspace"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleUploadWorkspaceFile(writer http.ResponseWriter, request *http.Request) {
	file, header, err := request.FormFile("file")
	if err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "缺少上传文件")
		return
	}
	defer file.Close()

	item, err := s.workspace.UploadFile(
		request.Context(),
		chi.URLParam(request, "agent_id"),
		header.Filename,
		request.FormValue("path"),
		file,
	)
	if errors.Is(err, agent2.ErrAgentNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "路径") || strings.Contains(err.Error(), "限制") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleDownloadWorkspaceFile(writer http.ResponseWriter, request *http.Request) {
	filePath, fileName, err := s.workspace.GetFileForDownload(
		request.Context(),
		chi.URLParam(request, "agent_id"),
		request.URL.Query().Get("path"),
	)
	if errors.Is(err, agent2.ErrAgentNotFound) || errors.Is(err, workspacepkg.ErrFileNotFound) {
		s.writeFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "路径") || strings.Contains(err.Error(), "目录") {
			s.writeFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	writer.Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
	http.ServeFile(writer, request, filePath)
}
