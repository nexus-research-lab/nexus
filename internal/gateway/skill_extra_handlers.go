// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：skill_extra_handlers.go
// @Date   ：2026/04/16 21:02:00
// @Author ：leemysw
// 2026/04/16 21:02:00   Create
// =====================================================

package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleImportGitSkill(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		URL    string `json:"url"`
		Branch string `json:"branch"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.skills.ImportGit(request.Context(), payload.URL, payload.Branch)
	if err != nil {
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleSearchExternalSkills(writer http.ResponseWriter, request *http.Request) {
	item, err := s.skills.SearchExternalSkills(
		request.Context(),
		request.URL.Query().Get("q"),
		strings.EqualFold(request.URL.Query().Get("include_readme"), "true"),
	)
	if err != nil {
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handlePreviewExternalSkill(writer http.ResponseWriter, request *http.Request) {
	item, err := s.skills.GetExternalSkillPreview(request.Context(), request.URL.Query().Get("detail_url"))
	if err != nil {
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleImportSkillsShSkill(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		PackageSpec string `json:"package_spec"`
		SkillSlug   string `json:"skill_slug"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		s.writeFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := s.skills.ImportSkillsSh(request.Context(), payload.PackageSpec, payload.SkillSlug)
	if err != nil {
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleUpdateImportedSkills(writer http.ResponseWriter, request *http.Request) {
	item, err := s.skills.UpdateImportedSkills(request.Context())
	if err != nil {
		s.writeFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) handleUpdateSingleSkill(writer http.ResponseWriter, request *http.Request) {
	item, err := s.skills.UpdateSingleSkill(request.Context(), chi.URLParam(request, "skill_name"))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			s.writeFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		s.writeFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	s.writeSuccess(writer, item)
}

func (s *Server) parseLocalSkillImportRequest(request *http.Request) ([]byte, string, string, error) {
	contentType := strings.ToLower(strings.TrimSpace(request.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		file, header, err := request.FormFile("file")
		if err == nil {
			defer file.Close()
			payload, readErr := io.ReadAll(file)
			return payload, header.Filename, "", readErr
		}
		localPath := strings.TrimSpace(request.FormValue("local_path"))
		return nil, "", localPath, nil
	}
	var payload struct {
		LocalPath string `json:"local_path"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		return nil, "", "", err
	}
	return nil, "", payload.LocalPath, nil
}
