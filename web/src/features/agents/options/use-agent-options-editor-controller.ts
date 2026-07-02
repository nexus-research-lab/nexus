"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { list_provider_options_api } from "@/lib/api/provider-config-api";
import type {
  AgentNameValidationResult,
  AgentOptions as AgentConfigOptions,
  AgentProvider,
} from "@/types/agent/agent";
import type { ProviderOption } from "@/types/capability/provider";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  get_default_agent_runtime_kind,
  set_default_agent_model,
  set_default_agent_provider,
} from "@/config/options";
import type { TabKey } from "@/features/agents/options/components/agent-options-nav";
import {
  DEFAULT_AGENT_OPTION_MODEL,
  DEFAULT_AGENT_PERMISSION_MODE,
  DEFAULT_AGENT_OPTION_PROVIDER,
  normalize_agent_option_provider,
} from "@/features/agents/options/agent-options-constants";
import type {
  AgentDialogInitialOptions,
  AgentOptionsEditorProps,
  SaveFeedback,
} from "@/features/agents/options/agent-options-editor-model";

export function useAgentOptionsEditorController({
  agent_id: agentId,
  mode,
  is_active: isActive,
  on_delete: onDelete,
  on_save: onSave,
  on_validate_name: onValidateName,
  initial_title: initialTitle = "",
  initial_options: initialOptions = {},
  initial_avatar: initialAvatar = "",
  initial_description: initialDescription = "",
  initial_vibe_tags: initialVibeTags = [],
  on_cancel: onCancel,
  close_after_save: closeAfterSave = false,
  show_cancel_button: showCancelButton = true,
  show_delete_button: showDeleteButton = true,
  variant = "dialog",
  content_max_width_class_name: contentMaxWidthClassName = "max-w-[920px]",
  active_tab: controlledActiveTab,
  on_tab_change: onTabChange,
  hide_inline_nav: hideInlineNav = false,
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
    ? normalize_agent_option_provider(sourceOptions.provider) || DEFAULT_AGENT_OPTION_PROVIDER
    : DEFAULT_AGENT_OPTION_PROVIDER;
  const initialPermissionMode = sourceOptions.permission_mode || DEFAULT_AGENT_PERMISSION_MODE;
  const initialAllowedTools = sourceOptions.allowed_tools || [];
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
        const payload = await list_provider_options_api(get_default_agent_runtime_kind());
        if (cancelled) {
          return;
        }
        setProviderOptions(payload.items);
        setDefaultProvider(normalize_agent_option_provider(payload.default_provider));
        setDefaultModel(payload.default_model?.trim() || "");
        set_default_agent_provider(payload.default_provider);
        set_default_agent_model(payload.default_model);
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
  }, [isActive, t]);

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
  }, [trimmedTitle, hasTitleChanged, isActive, onValidateName, t]);

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
      allowed_tools: allowedTools,
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
    active_tab: activeTab,
    set_active_tab: setActiveTab,
    advanced_props: {
      permission_mode: permissionMode,
      on_permission_mode_change: (value: string) => {
        clearSaveFeedback();
        setPermissionMode(value);
      },
      allowed_tools: allowedTools,
      on_toggle_tool: toggleTool,
    },
    can_delete: canDelete,
    can_save: canSave,
    cancel_label: t("common.cancel"),
    content_max_width_class_name: contentMaxWidthClassName,
    delete_agent_label: t("agent_options.delete_agent"),
    handle_delete: handleDelete,
    handle_save: handleSave,
    hide_inline_nav: hideInlineNav,
    identity_props: {
      avatar,
      on_avatar_change: (value: string) => {
        clearSaveFeedback();
        setAvatar(value);
      },
      title,
      on_title_change: (value: string) => {
        clearSaveFeedback();
        setTitle(value);
      },
      description,
      on_description_change: (value: string) => {
        clearSaveFeedback();
        setDescription(value);
      },
      vibe_tags: vibeTags,
      on_vibe_tags_change: (value: string[]) => {
        clearSaveFeedback();
        setVibeTags(value);
      },
      provider,
      model,
      default_provider: defaultProvider,
      default_model: defaultModel,
      provider_options: providerOptions,
      provider_options_error: providerOptionsError,
      provider_options_loading: providerOptionsLoading,
      on_provider_change: (value: AgentProvider) => {
        clearSaveFeedback();
        setProvider(value);
      },
      on_model_change: (value: string) => {
        clearSaveFeedback();
        setModel(value);
      },
      name_validation: nameValidation,
      is_validating_name: isValidatingName,
      variant,
    },
    is_active: isActive,
    mode,
    on_cancel: onCancel,
    save_button_label: saveButtonLabel,
    save_feedback: saveFeedback,
    show_cancel_button: showCancelButton,
    skills_agent_id: mode === "edit" ? agentId : undefined,
    variant,
  };
}
