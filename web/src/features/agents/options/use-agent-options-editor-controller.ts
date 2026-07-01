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
  agent_id,
  mode,
  is_active,
  on_delete,
  on_save,
  on_validate_name,
  initial_title = "",
  initial_options = {},
  initial_avatar = "",
  initial_description = "",
  initial_vibe_tags = [],
  on_cancel,
  close_after_save = false,
  show_cancel_button = true,
  show_delete_button = true,
  variant = "dialog",
  content_max_width_class_name = "max-w-[920px]",
  active_tab,
  on_tab_change,
  hide_inline_nav = false,
}: AgentOptionsEditorProps) {
  const { t } = useI18n();
  const sourceOptions = initial_options as AgentDialogInitialOptions;
  const initial_resolved_title = useMemo(
    () => initial_title || t("agent_options.default_name"),
    [initial_title, t],
  );
  const initial_vibe_tags_signature = initial_vibe_tags.join("\x1f");
  const sourceModel = sourceOptions.model?.trim() || DEFAULT_AGENT_OPTION_MODEL;
  const initial_provider = sourceModel
    ? normalize_agent_option_provider(sourceOptions.provider) || DEFAULT_AGENT_OPTION_PROVIDER
    : DEFAULT_AGENT_OPTION_PROVIDER;
  const initial_permission_mode = sourceOptions.permission_mode || DEFAULT_AGENT_PERMISSION_MODE;
  const initial_allowed_tools = sourceOptions.allowed_tools || [];
  const initial_disallowed_tools = sourceOptions.disallowed_tools || [];
  const initial_allowed_tools_signature = initial_allowed_tools.join("\x1f");
  const initial_disallowed_tools_signature = initial_disallowed_tools.join("\x1f");
  const editor_reset_key = [
    is_active ? "active" : "inactive",
    initial_resolved_title,
    initial_avatar,
    initial_description,
    initial_vibe_tags_signature,
    initial_provider,
    sourceModel,
    initial_permission_mode,
    initial_allowed_tools_signature,
    initial_disallowed_tools_signature,
  ].join("\x1e");

  const [uncontrolledActiveTab, setUncontrolledActiveTab] = useResettableState<TabKey>("identity", editor_reset_key);
  const activeTab = active_tab ?? uncontrolledActiveTab;
  const setActiveTab = on_tab_change ?? setUncontrolledActiveTab;

  const [title, setTitle] = useResettableState(initial_resolved_title, editor_reset_key);
  const [avatar, setAvatar] = useResettableState(initial_avatar, editor_reset_key);
  const [description, setDescription] = useResettableState(initial_description, editor_reset_key);
  const [vibeTags, setVibeTags] = useResettableState<string[]>(initial_vibe_tags, editor_reset_key);
  const [provider, setProvider] = useResettableState<AgentProvider>(initial_provider, editor_reset_key);
  const [model, setModel] = useResettableState<string>(sourceModel, editor_reset_key);
  const [defaultProvider, setDefaultProvider] = useResettableState<AgentProvider>("", editor_reset_key);
  const [defaultModel, setDefaultModel] = useResettableState<string>("", editor_reset_key);
  const [providerOptions, setProviderOptions] = useState<ProviderOption[]>([]);
  const [providerOptionsLoading, setProviderOptionsLoading] = useState(false);
  const [providerOptionsError, setProviderOptionsError] = useResettableState<string | null>(null, editor_reset_key);
  const [saveFeedback, setSaveFeedback] = useResettableState<SaveFeedback | null>(null, `${is_active ? "active" : "inactive"}\x1f${agent_id}`);
  const saveFeedbackTimerRef = useRef<number | null>(null);

  const [permissionMode, setPermissionMode] = useResettableState(initial_permission_mode, editor_reset_key);
  const [allowedTools, setAllowedTools] = useResettableState<string[]>(initial_allowed_tools, editor_reset_key);
  const [disallowedTools, setDisallowedTools] = useResettableState<string[]>(initial_disallowed_tools, editor_reset_key);

  const [nameValidation, setNameValidation] =
    useResettableState<AgentNameValidationResult | null>(null, editor_reset_key);
  const [isValidatingName, setIsValidatingName] = useResettableState(false, editor_reset_key);
  const [isSaving, setIsSaving] = useResettableState(false, editor_reset_key);
  const trimmed_title = title.trim();
  const has_title_changed = trimmed_title !== initial_resolved_title.trim();

  useEffect(() => {
    return () => {
      if (saveFeedbackTimerRef.current !== null) {
        window.clearTimeout(saveFeedbackTimerRef.current);
      }
    };
  }, []);

  const clear_save_feedback = () => {
    if (saveFeedbackTimerRef.current !== null) {
      window.clearTimeout(saveFeedbackTimerRef.current);
      saveFeedbackTimerRef.current = null;
    }
    setSaveFeedback(null);
  };

  useEffect(() => {
    if (!is_active) {
      return;
    }

    let cancelled = false;

    const load_provider_options = async () => {
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

    void load_provider_options();
    return () => {
      cancelled = true;
    };
  }, [is_active, t]);

  useEffect(() => {
    if (!is_active) return;
    if (!on_validate_name) {
      setNameValidation(null);
      return;
    }
    if (!trimmed_title) {
      setNameValidation(null);
      setIsValidatingName(false);
      return;
    }
    if (!has_title_changed) {
      setNameValidation(null);
      setIsValidatingName(false);
      return;
    }

    let cancelled = false;
    const timer = window.setTimeout(async () => {
      try {
        setIsValidatingName(true);
        const result = await on_validate_name(trimmed_title);
        if (!cancelled) setNameValidation(result);
      } catch (error) {
        if (!cancelled) {
          setNameValidation({
            name: trimmed_title,
            normalized_name: trimmed_title,
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
  }, [trimmed_title, has_title_changed, is_active, on_validate_name, t]);

  const toggle_tool = (
    toolName: string,
    type: "allowed" | "disallowed"
  ) => {
    clear_save_feedback();
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

  const handle_save = async () => {
    if (!trimmed_title) return;
    if (isValidatingName || isSaving) return;
    if (saveFeedbackTimerRef.current !== null) {
      window.clearTimeout(saveFeedbackTimerRef.current);
      saveFeedbackTimerRef.current = null;
    }
    setSaveFeedback(null);
    const requires_final_name_validation = Boolean(on_validate_name) && (mode === "create" || has_title_changed);
    let latest_name_validation = nameValidation;

    if (requires_final_name_validation) {
      const has_current_valid_result = latest_name_validation?.name === trimmed_title;
      if (!has_current_valid_result) {
        setIsValidatingName(true);
        try {
          latest_name_validation = await on_validate_name!(trimmed_title);
          setNameValidation(latest_name_validation);
        } catch (error) {
          latest_name_validation = {
            name: trimmed_title,
            normalized_name: trimmed_title,
            is_valid: false,
            is_available: false,
            reason:
              error instanceof Error
                ? error.message
                : t("agent_options.identity.validation_failed"),
            workspace_path: null,
          };
          setNameValidation(latest_name_validation);
        } finally {
          setIsValidatingName(false);
        }
      }

      if (
        latest_name_validation &&
        (!latest_name_validation.is_valid || !latest_name_validation.is_available)
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
      await on_save(trimmed_title, options, {
        avatar,
        description: description.trim(),
        vibe_tags: vibeTags,
      });
      if (close_after_save) {
        on_cancel?.();
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
  const canSave = !!trimmed_title && !isValidatingName && !isNameInvalid && !isSaving;
  const canDelete = show_delete_button && mode === "edit" && Boolean(agent_id) && Boolean(on_delete);
  const saveButtonLabel = isSaving
    ? t("common.saving")
    : saveFeedback?.tone === "success"
      ? t("agent_options.save_success")
      : saveFeedback?.tone === "error"
        ? t("agent_options.save_failed")
        : mode === "create"
          ? t("agent_options.title_create")
          : t("agent_options.save_changes");

  const handle_delete = () => {
    if (!agent_id || !on_delete) {
      return;
    }
    on_delete(agent_id);
  };

  return {
    active_tab: activeTab,
    set_active_tab: setActiveTab,
    advanced_props: {
      permission_mode: permissionMode,
      on_permission_mode_change: (value: string) => {
        clear_save_feedback();
        setPermissionMode(value);
      },
      allowed_tools: allowedTools,
      on_toggle_tool: toggle_tool,
    },
    can_delete: canDelete,
    can_save: canSave,
    cancel_label: t("common.cancel"),
    content_max_width_class_name,
    delete_agent_label: t("agent_options.delete_agent"),
    handle_delete,
    handle_save,
    hide_inline_nav,
    identity_props: {
      avatar,
      on_avatar_change: (value: string) => {
        clear_save_feedback();
        setAvatar(value);
      },
      title,
      on_title_change: (value: string) => {
        clear_save_feedback();
        setTitle(value);
      },
      description,
      on_description_change: (value: string) => {
        clear_save_feedback();
        setDescription(value);
      },
      vibe_tags: vibeTags,
      on_vibe_tags_change: (value: string[]) => {
        clear_save_feedback();
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
        clear_save_feedback();
        setProvider(value);
      },
      on_model_change: (value: string) => {
        clear_save_feedback();
        setModel(value);
      },
      name_validation: nameValidation,
      is_validating_name: isValidatingName,
      variant,
    },
    is_active,
    mode,
    on_cancel,
    save_button_label: saveButtonLabel,
    save_feedback: saveFeedback,
    show_cancel_button,
    skills_agent_id: mode === "edit" ? agent_id : undefined,
    variant,
  };
}
