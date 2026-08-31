// INPUT: 核心 HTTP 请求、owner 身份、Preferences/Provider/runtime 服务。
// OUTPUT: 健康、运行选项、带 ETag/CAS 的偏好设置与主机设置响应。
// POS: Web Preferences 写入口；RMW 委托 Service owner 锁，热同步失败仅做 version 条件回滚。
package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	clientopts "github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	nxsruntimesvc "github.com/nexus-research-lab/nexus/internal/service/nxsruntime"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
	versionpkg "github.com/nexus-research-lab/nexus/internal/version"
)

// Handlers 封装核心 HTTP handlers。
type Handlers struct {
	api       *handlershared.API
	agents    *agentpkg.Service
	providers *providercfg.Service
	prefs     *preferencessvc.Service
	nxs       *nxsruntimesvc.Service
	runtime   *runtimectx.Manager
}

// SetRuntimeManager 绑定活跃 Agent runtime 管理器。
func (h *Handlers) SetRuntimeManager(manager *runtimectx.Manager) {
	h.runtime = manager
}

// New 创建核心 handlers。
func New(
	api *handlershared.API,
	agents *agentpkg.Service,
	providers *providercfg.Service,
	prefs ...*preferencessvc.Service,
) *Handlers {
	var prefService *preferencessvc.Service
	if len(prefs) > 0 {
		prefService = prefs[0]
	}
	return &Handlers{
		api:       api,
		agents:    agents,
		providers: providers,
		prefs:     prefService,
		nxs:       nxsruntimesvc.NewService(),
	}
}

// HandleHealth 返回健康检查。
func (h *Handlers) HandleHealth(writer http.ResponseWriter, request *http.Request) {
	h.api.WriteJSON(writer, http.StatusOK, map[string]any{
		"code": 0,
		"msg":  "ok",
		"data": map[string]any{
			"status": "ok",
		},
	})
}

// HandleSystemVersion 返回当前二进制版本信息。
func (h *Handlers) HandleSystemVersion(writer http.ResponseWriter, request *http.Request) {
	h.api.WriteSuccess(writer, versionpkg.Current())
}

// HandleRuntimeOptions 返回前端启动所需运行时选项。
func (h *Handlers) HandleRuntimeOptions(writer http.ResponseWriter, request *http.Request) {
	defaultAgent, err := h.agents.GetDefaultAgent(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	prefs, err := h.currentPreferences(request)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	providerOptions, err := h.providers.ListOptionsForRuntime(request.Context(), prefs.AgentRuntimeKind)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	prefs, _ = applyImagegenDefaultTool(prefs, providerOptions)
	defaultProvider := providerOptions.DefaultProvider
	defaultModel := providerOptions.DefaultModel
	providerValue := strings.TrimSpace(prefs.DefaultAgentOptions.Provider)
	modelValue := strings.TrimSpace(prefs.DefaultAgentOptions.Model)
	if providerValue != "" && modelValue != "" {
		defaultProvider = &providerValue
		defaultModel = &modelValue
	}
	h.api.WriteJSON(writer, http.StatusOK, map[string]any{
		"code":    "0000",
		"message": "success",
		"success": true,
		"data": map[string]any{
			"default_agent_id":       defaultAgent.AgentID,
			"default_agent_avatar":   defaultAgent.Avatar,
			"default_agent_provider": defaultProvider,
			"default_agent_model":    defaultModel,
			"preferences":            prefs,
		},
	})
}

// HandleGetPreferences 返回当前用户偏好。
func (h *Handlers) HandleGetPreferences(writer http.ResponseWriter, request *http.Request) {
	writePreferencesNoStore(writer)
	prefs, err := h.currentPreferences(request)
	if err != nil {
		h.writePreferencesReadFailure(writer, request, err)
		return
	}
	prefs, err = h.withProviderPreferenceDefaults(request, prefs)
	if err != nil {
		h.writePreferencesReadFailure(writer, request, err)
		return
	}
	writePreferencesETag(writer, prefs.Version)
	h.api.WriteSuccess(writer, prefs)
}

// HandleUpdatePreferences 更新当前用户偏好。
func (h *Handlers) HandleUpdatePreferences(writer http.ResponseWriter, request *http.Request) {
	writePreferencesNoStore(writer)
	if h.prefs == nil {
		defaults := preferencessvc.DefaultPreferences()
		writePreferencesETag(writer, defaults.Version)
		h.api.WriteSuccess(writer, defaults)
		return
	}
	var payload preferencessvc.UpdateRequest
	if !h.api.BindJSONError(writer, request, &payload, handlershared.FailureSpec{
		Code:     "preferences.request_invalid",
		Category: protocol.FailureCategoryValidation,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "偏好设置格式不正确",
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "preferences.review_values",
		},
	}) {
		return
	}
	expectedVersion, err := parsePreferencesIfMatch(request.Header.Get("If-Match"))
	if err != nil {
		h.api.WriteError(writer, request, http.StatusBadRequest, handlershared.FailureSpec{
			Code:     "preferences.precondition_invalid",
			Category: protocol.FailureCategoryValidation,
			Effect:   protocol.FailureEffectNotApplied,
			Detail:   "偏好设置版本条件无效",
			Cause:    err,
			Resolution: &protocol.FailureResolution{
				Actor:  protocol.FailureRecoveryActorUser,
				Action: "preferences.reload",
			},
		})
		return
	}
	ownerUserID := currentOwnerUserID(request)
	webSearchChanged := payload.WebSearch != nil || payload.WebSearchAPIKey != nil
	var previous preferencessvc.Preferences
	var defaultSelection providercfg.DefaultAgentSelection
	var defaultSelectionChanged bool
	var preparationErr error
	var validationErr error
	prepare := func(current preferencessvc.Preferences) (preferencessvc.UpdateRequest, error) {
		previous = current
		prepared, prepareErr := h.prepareProviderPreferenceDefaults(request, current, payload)
		if prepareErr != nil {
			preparationErr = prepareErr
			return preferencessvc.UpdateRequest{}, prepareErr
		}
		defaultSelection, defaultSelectionChanged = updatedDefaultAgentSelection(current, prepared)
		if defaultSelectionChanged && h.providers != nil {
			if validateErr := h.providers.ValidateDefaultAgentSelection(request.Context(), defaultSelection); validateErr != nil {
				validationErr = validateErr
				return preferencessvc.UpdateRequest{}, validateErr
			}
		}
		return prepared, nil
	}
	var item preferencessvc.Preferences
	if expectedVersion == nil {
		item, err = h.prefs.UpdatePrepared(request.Context(), ownerUserID, prepare)
	} else {
		item, err = h.prefs.UpdatePreparedAtVersion(
			request.Context(), ownerUserID, *expectedVersion, prepare,
		)
	}
	if err != nil {
		if errors.Is(err, preferencessvc.ErrVersionConflict) {
			h.writePreferencesVersionConflict(writer, request, err)
			return
		}
		status := http.StatusInternalServerError
		if validationErr != nil {
			status = http.StatusBadRequest
		}
		effect := protocol.FailureEffectUnknown
		category := protocol.FailureCategoryInternal
		code := "preferences.update_result_unknown"
		if validationErr != nil {
			effect = protocol.FailureEffectNotApplied
			category = protocol.FailureCategoryValidation
			code = "preferences.selection_invalid"
		} else if preparationErr != nil {
			effect = protocol.FailureEffectNotApplied
			code = "preferences.preparation_failed"
		}
		detail := "暂时无法确认偏好设置是否已经保存"
		if validationErr != nil {
			detail = "偏好设置的内容不符合要求"
		} else if preparationErr != nil {
			detail = "暂时无法准备偏好设置更新"
		}
		h.api.WriteError(writer, request, status, handlershared.FailureSpec{
			Code:     code,
			Category: category,
			Effect:   effect,
			Detail:   detail,
			Cause:    err,
			Resolution: &protocol.FailureResolution{
				Actor:  protocol.FailureRecoveryActorUser,
				Action: "preferences.reload",
			},
		})
		return
	}
	if defaultSelectionChanged && h.providers != nil {
		if _, err = h.providers.ReconcileDefaultAgentBindings(request.Context(), defaultSelection); err != nil {
			err = errors.Join(err, h.rollbackPreferencesMutation(
				request.Context(), ownerUserID, item, previous, defaultSelectionChanged, false,
			))
			h.writePreferencesPostCommitFailure(writer, request, err)
			return
		}
	}
	if webSearchChanged {
		if err = h.syncWebSearchRuntime(request.Context(), item); err != nil {
			err = errors.Join(err, h.rollbackPreferencesMutation(
				request.Context(), ownerUserID, item, previous, defaultSelectionChanged, true,
			))
			h.writePreferencesPostCommitFailure(writer, request, err)
			return
		}
	}
	writePreferencesETag(writer, item.Version)
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) rollbackPreferencesMutation(
	ctx context.Context,
	ownerUserID string,
	applied preferencessvc.Preferences,
	previous preferencessvc.Preferences,
	defaultSelectionChanged bool,
	restoreRuntime bool,
) error {
	rollbackValue, restored, restoreErr := h.prefs.RestoreIfVersion(
		ctx,
		ownerUserID,
		applied.Version,
		previous,
	)
	if restoreErr != nil {
		return restoreErr
	}
	if !restored {
		return errors.New("Preferences 回滚已跳过：已有后续写入")
	}
	var reconcileErr error
	if defaultSelectionChanged && h.providers != nil {
		previousSelection, _ := updatedDefaultAgentSelection(previous, preferencessvc.UpdateRequest{})
		_, reconcileErr = h.providers.ReconcileDefaultAgentBindings(ctx, previousSelection)
	}
	var runtimeRestoreErr error
	if restoreRuntime {
		runtimeRestoreErr = h.syncWebSearchRuntime(ctx, rollbackValue)
	}
	return errors.Join(reconcileErr, runtimeRestoreErr)
}

func (h *Handlers) syncWebSearchRuntime(ctx context.Context, preferences preferencessvc.Preferences) error {
	if h.runtime == nil || h.agents == nil {
		return nil
	}
	agents, err := h.agents.ListAgentRecords(ctx)
	if err != nil {
		return err
	}
	environment := clientopts.BuildWebSearchRuntimeEnv(
		"nxs",
		runtimeselectionsvc.WebSearchConfigFromPreferences(preferences.WebSearch),
	)
	for _, item := range agents {
		if err := h.runtime.UpdateEnvironmentForAgent(ctx, item.AgentID, environment); err != nil {
			return err
		}
	}
	return nil
}

// HandleNXSRuntimeStatus 返回当前主机上 nxs runtime 的本地可用状态。
func (h *Handlers) HandleNXSRuntimeStatus(writer http.ResponseWriter, request *http.Request) {
	h.api.WriteSuccess(writer, h.nxs.Status())
}

func (h *Handlers) currentPreferences(request *http.Request) (preferencessvc.Preferences, error) {
	if h.prefs == nil {
		return preferencessvc.DefaultPreferences(), nil
	}
	return h.prefs.Get(request.Context(), currentOwnerUserID(request))
}

func currentOwnerUserID(request *http.Request) string {
	return authsvc.OwnerUserID(request.Context())
}
