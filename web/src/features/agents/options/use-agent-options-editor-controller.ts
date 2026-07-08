"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { listProviderOptionsApi } from "@/lib/api/provider-config-api";
import type {
  AgentNameValidationResult,
  AgentOptions as AgentConfigOptions,
  AgentProvider,
} from "@/types/agent/agent";
import type { ProviderOption } from "@/types/capability/provider";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  getDefaultAgentRuntimeKind,
  setDefaultAgentModel,
  setDefaultAgentProvider,
} from "@/config/options";
import type { TabKey } from "@/features/agents/options/components/agent-options-nav";
import {
  DEFAULT_AGENT_OPTION_MODEL,
  DEFAULT_AGENT_PERMISSION_MODE,
  DEFAULT_AGENT_OPTION_PROVIDER,
  normalizeAgentAllowedToolsForEditor,
  normalizeAgentOptionProvider,
} from "@/features/agents/options/agent-options-constants";
import type {
  AgentDialogInitialOptions,
  AgentOptionsEditorProps,
  SaveFeedback,
} from "@/features/agents/options/agent-options-editor-model";

export function useAgentOptionsEditorController({
  agentId: agentId,
  mode,
  isActive: isActive,
  onDelete: onDelete,
  onSave: onSave,
  onValidateName: onValidateName,
  initialTitle: initialTitle = "",
  initialOptions: initialOptions = {},
  initialAvatar: initialAvatar = "",
  initialDescription: initialDescription = "",
  initialVibeTags: initialVibeTags = [],
  onCancel: onCancel,
  closeAfterSave: closeAfterSave = false,
  showCancelButton: showCancelButton = true,
  showDeleteButton: showDeleteButton = true,
  variant = "dialog",
  contentMaxWidthClassName: contentMaxWidthClassName = "max-w-[920px]",
  activeTab: controlledActiveTab,
  onTabChange: onTabChange,
  hideInlineNav: hideInlineNav = false,
}: AgentOptionsEditorProps) {
  const { t } = useI18n();
  const sourceOptions = initialOptions as AgentDialogInitialOptions;
  const initialResolvedTitle = useMemo(
    () => initialTitle || t("agent_options.default_name"),
    [initialTitle, t],
  );
  const initialVibeTagsSignature = initialVibeTags.join("\x1f");
  const sourceModel = sourceOptions.model?.trim() || DEFAULT_AGENT_OPTION_MODEL;
  const initialProvider = sourceModel
    ? normalizeAgentOptionProvider(sourceOptions.provider) || DEFAULT_AGENT_OPTION_PROVIDER
    : DEFAULT_AGENT_OPTION_PROVIDER;
  const initialPermissionMode = sourceOptions.permission_mode || DEFAULT_AGENT_PERMISSION_MODE;
  const initialAllowedTools = normalizeAgentAllowedToolsForEditor(sourceOptions.allowed_tools);
  const initialDisallowedTools = sourceOptions.disallowed_tools || [];
  const initialAllowedToolsSignature = initialAllowedTools.join("\x1f");
  const initialDisallowedToolsSignature = initialDisallowedTools.join("\x1f");
  const editorResetKey = [
    isActive ? "active" : "inactive",
    initialResolvedTitle,
    initialAvatar,
    initialDescription,
    initialVibeTagsSignature,
    initialProvider,
    sourceModel,
    initialPermissionMode,
    initialAllowedToolsSignature,
    initialDisallowedToolsSignature,
  ].join("\x1e");

  const [uncontrolledActiveTab, setUncontrolledActiveTab] = useResettableState<TabKey>("identity", editorResetKey);
  const activeTab = controlledActiveTab ?? uncontrolledActiveTab;
  const setActiveTab = onTabChange ?? setUncontrolledActiveTab;

  const [title, setTitle] = useResettableState(initialResolvedTitle, editorResetKey);
  const [avatar, setAvatar] = useResettableState(initialAvatar, editorResetKey);
  const [description, setDescription] = useResettableState(initialDescription, editorResetKey);
  const [vibeTags, setVibeTags] = useResettableState<string[]>(initialVibeTags, editorResetKey);
  const [provider, setProvider] = useResettableState<AgentProvider>(initialProvider, editorResetKey);
  const [model, setModel] = useResettableState<string>(sourceModel, editorResetKey);
  const [defaultProvider, setDefaultProvider] = useResettableState<AgentProvider>("", editorResetKey);
  const [defaultModel, setDefaultModel] = useResettableState<string>("", editorResetKey);
  const [providerOptions, setProviderOptions] = useState<ProviderOption[]>([]);
  const [providerOptionsLoading, setProviderOptionsLoading] = useState(false);
  const [providerOptionsError, setProviderOptionsError] = useResettableState<string | null>(null, editorResetKey);
  const [saveFeedback, setSaveFeedback] = useResettableState<SaveFeedback | null>(null, `${isActive ? "active" : "inactive"}\x1f${agentId}`);
  const saveFeedbackTimerRef = useRef<number | null>(null);

  const [permissionMode, setPermissionMode] = useResettableState(initialPermissionMode, editorResetKey);
  const [allowedTools, setAllowedTools] = useResettableState<string[]>(initialAllowedTools, editorResetKey);
  const [disallowedTools, setDisallowedTools] = useResettableState<string[]>(initialDisallowedTools, editorResetKey);

  const [nameValidation, setNameValidation] =
    useResettableState<AgentNameValidationResult | null>(null, editorResetKey);
  const [isValidatingName, setIsValidatingName] = useResettableState(false, editorResetKey);
  const [isSaving, setIsSaving] = useResettableState(false, editorResetKey);
  const trimmedTitle = title.trim();
  const hasTitleChanged = trimmedTitle !== initialResolvedTitle.trim();

  useEffect(() => {
    return () => {
      if (saveFeedbackTimerRef.current !== null) {
        window.clearTimeout(saveFeedbackTimerRef.current);
      }
    };
  }, []);

  const clearSaveFeedback = () => {
    if (saveFeedbackTimerRef.current !== null) {
      window.clearTimeout(saveFeedbackTimerRef.current);
      saveFeedbackTimerRef.current = null;
    }
    setSaveFeedback(null);
  };

  useEffect(() => {
    if (!isActive) {
      return;
    }

    let cancelled = false;

    const loadProviderOptions = async () => {
      try {
        setProviderOptionsLoading(true);
        const payload = await listProviderOptionsApi(getDefaultAgentRuntimeKind());
        if (cancelled) {
          return;
        }
        setProviderOptions(payload.items);
        setDefaultProvider(normalizeAgentOptionProvider(payload.default_provider));
        setDefaultModel(payload.default_model?.trim() || "");
        setDefaultAgentProvider(payload.default_provider);
        setDefaultAgentModel(payload.default_model);
        setProviderOptionsError(null);
      } catch (error) {
        if (!cancelled) {
          setProviderOptionsError(
            error instanceof Error
              ? error.message
              : t("agent_options.identity.provider_load_failed")
          );
        }
      } finally {
        if (!cancelled) {
          setProviderOptionsLoading(false);
        }
      }
    };

    void loadProviderOptions();
    return () => {
      cancelled = true;
    };
  }, [
    isActive,
    setDefaultModel,
    setDefaultProvider,
    setProviderOptionsError,
    t,
  ]);

  useEffect(() => {
    if (!isActive) return;
    if (!onValidateName) {
      setNameValidation(null);
      return;
    }
    if (!trimmedTitle) {
      setNameValidation(null);
      setIsValidatingName(false);
      return;
    }
    if (!hasTitleChanged) {
      setNameValidation(null);
      setIsValidatingName(false);
      return;
    }

    let cancelled = false;
    const timer = window.setTimeout(async () => {
      try {
        setIsValidatingName(true);
        const result = await onValidateName(trimmedTitle);
        if (!cancelled) setNameValidation(result);
      } catch (error) {
        if (!cancelled) {
          setNameValidation({
            name: trimmedTitle,
            normalized_name: trimmedTitle,
            is_valid: false,
            is_available: false,
            reason:
              error instanceof Error
                ? error.message
                : t("agent_options.identity.validation_failed"),
            workspace_path: null,
          });
        }
      } finally {
        if (!cancelled) setIsValidatingName(false);
      }
    }, 300);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [
    trimmedTitle,
    hasTitleChanged,
    isActive,
    onValidateName,
    setIsValidatingName,
    setNameValidation,
    t,
  ]);

  const toggleTool = (
    toolName: string,
    type: "allowed" | "disallowed"
  ) => {
    clearSaveFeedback();
    if (type === "allowed") {
      setAllowedTools((prev) =>
        prev.includes(toolName)
          ? prev.filter((t) => t !== toolName)
          : [...prev, toolName]
      );
    } else {
      setDisallowedTools((prev) =>
        prev.includes(toolName)
          ? prev.filter((t) => t !== toolName)
          : [...prev, toolName]
      );
    }
  };

  const handleSave = async () => {
    if (!trimmedTitle) return;
    if (isValidatingName || isSaving) return;
    if (saveFeedbackTimerRef.current !== null) {
      window.clearTimeout(saveFeedbackTimerRef.current);
      saveFeedbackTimerRef.current = null;
    }
    setSaveFeedback(null);
    const requiresFinalNameValidation = Boolean(onValidateName) && (mode === "create" || hasTitleChanged);
    let latestNameValidation = nameValidation;

    if (requiresFinalNameValidation) {
      const hasCurrentValidResult = latestNameValidation?.name === trimmedTitle;
      if (!hasCurrentValidResult) {
        setIsValidatingName(true);
        try {
          latestNameValidation = await onValidateName!(trimmedTitle);
          setNameValidation(latestNameValidation);
        } catch (error) {
          latestNameValidation = {
            name: trimmedTitle,
            normalized_name: trimmedTitle,
            is_valid: false,
            is_available: false,
            reason:
              error instanceof Error
                ? error.message
                : t("agent_options.identity.validation_failed"),
            workspace_path: null,
          };
          setNameValidation(latestNameValidation);
        } finally {
          setIsValidatingName(false);
        }
      }

      if (
        latestNameValidation &&
        (!latestNameValidation.is_valid || !latestNameValidation.is_available)
      ) {
        return;
      }
    }

    const selectedProvider = provider.trim();
    const selectedModel = model.trim();
    const hasExplicitModel = Boolean(selectedProvider && selectedModel);
    const options: AgentConfigOptions = {
      provider: hasExplicitModel ? selectedProvider : DEFAULT_AGENT_OPTION_PROVIDER,
      model: hasExplicitModel ? selectedModel : DEFAULT_AGENT_OPTION_MODEL,
      permission_mode: permissionMode,
      allowed_tools: normalizeAgentAllowedToolsForEditor(allowedTools),
      disallowed_tools: disallowedTools,
      max_turns: sourceOptions.max_turns,
      max_thinking_tokens: sourceOptions.max_thinking_tokens,
      mcp_servers: sourceOptions.mcp_servers,
      setting_sources: ["project"],
    };
    setIsSaving(true);
    try {
      await onSave(trimmedTitle, options, {
        avatar,
        description: description.trim(),
        vibe_tags: vibeTags,
      });
      if (closeAfterSave) {
        onCancel?.();
      } else {
        setSaveFeedback({
          tone: "success",
          message: t("agent_options.save_success"),
        });
        saveFeedbackTimerRef.current = window.setTimeout(() => {
          setSaveFeedback((current) => current?.tone === "success" ? null : current);
          saveFeedbackTimerRef.current = null;
        }, 1800);
      }
    } catch (error) {
      setSaveFeedback({
        tone: "error",
        message: error instanceof Error ? error.message : t("agent_options.save_failed"),
      });
    } finally {
      setIsSaving(false);
    }
  };

  const isNameInvalid = !!(
    nameValidation &&
    (!nameValidation.is_valid || !nameValidation.is_available)
  );
  const canSave = !!trimmedTitle && !isValidatingName && !isNameInvalid && !isSaving;
  const canDelete = showDeleteButton && mode === "edit" && Boolean(agentId) && Boolean(onDelete);
  const saveButtonLabel = isSaving
    ? t("common.saving")
    : saveFeedback?.tone === "success"
      ? t("agent_options.save_success")
      : saveFeedback?.tone === "error"
        ? t("agent_options.save_failed")
        : mode === "create"
          ? t("agent_options.title_create")
          : t("agent_options.save_changes");

  const handleDelete = () => {
    if (!agentId || !onDelete) {
      return;
    }
    onDelete(agentId);
  };

  return {
    activeTab,
    setActiveTab,
    advancedProps: {
      permissionMode,
      onPermissionModeChange: (value: string) => {
        clearSaveFeedback();
        setPermissionMode(value);
      },
      allowedTools,
      onToggleTool: toggleTool,
    },
    canDelete,
    canSave,
    cancelLabel: t("common.cancel"),
    contentMaxWidthClassName,
    deleteAgentLabel: t("agent_options.delete_agent"),
    handleDelete,
    handleSave,
    hideInlineNav,
    identityProps: {
      avatar,
      onAvatarChange: (value: string) => {
        clearSaveFeedback();
        setAvatar(value);
      },
      title,
      onTitleChange: (value: string) => {
        clearSaveFeedback();
        setTitle(value);
      },
      description,
      onDescriptionChange: (value: string) => {
        clearSaveFeedback();
        setDescription(value);
      },
      vibeTags,
      onVibeTagsChange: (value: string[]) => {
        clearSaveFeedback();
        setVibeTags(value);
      },
      provider,
      model,
      defaultProvider,
      defaultModel,
      providerOptions,
      providerOptionsError,
      providerOptionsLoading,
      onProviderChange: (value: AgentProvider) => {
        clearSaveFeedback();
        setProvider(value);
      },
      onModelChange: (value: string) => {
        clearSaveFeedback();
        setModel(value);
      },
      nameValidation,
      isValidatingName,
      variant,
    },
    isActive,
    mode,
    onCancel,
    saveButtonLabel,
    saveFeedback,
    showCancelButton,
    skillsAgentId: mode === "edit" ? agentId : undefined,
    variant,
  };
}
