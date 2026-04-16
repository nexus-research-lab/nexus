/**
 * =====================================================
 * @File   : agent-options-editor.tsx
 * @Date   : 2026-04-15 17:35
 * @Author : leemysw
 * 2026-04-15 17:35   Create
 * =====================================================
 */

"use client";

import { useEffect, useMemo, useState } from "react";
import { cn } from "@/lib/utils";
import { list_provider_options_api } from "@/lib/api/provider-config-api";
import type {
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions as AgentConfigOptions,
  AgentProvider,
} from "@/types/agent/agent";
import type { ProviderOption } from "@/types/capability/provider";
import {
  get_dialog_action_class_name,
} from "@/shared/ui/dialog/dialog-styles";
import { useI18n } from "@/shared/i18n/i18n-context";
import { set_default_agent_provider } from "@/config/options";
import {
  AgentOptionsNav,
  type TabKey,
} from "@/features/agents/options/components/agent-options-nav";
import { AgentOptionsIdentityTab } from "@/features/agents/options/components/agent-options-identity-tab";
import { AgentOptionsPersonaTab } from "@/features/agents/options/components/agent-options-persona-tab";
import { AgentOptionsSkillsTab } from "@/features/agents/options/components/agent-options-skills-tab";
import { AgentOptionsAdvancedTab } from "@/features/agents/options/components/agent-options-advanced-tab";
import {
  build_agent_option_provider_options,
  DEFAULT_AGENT_OPTION_PROVIDER,
  normalize_agent_option_provider,
} from "@/features/agents/options/agent-options-constants";

export interface AgentOptionsEditorProps {
  agent_id?: string;
  mode: "create" | "edit";
  is_active: boolean;
  on_delete?: (agent_id: string) => void;
  on_save: (title: string, options: AgentConfigOptions, identity: AgentIdentityDraft) => void | Promise<void>;
  on_validate_name?: (name: string) => Promise<AgentNameValidationResult>;
  initial_title?: string;
  initial_options?: Partial<AgentConfigOptions>;
  initial_avatar?: string;
  initial_description?: string;
  initial_vibe_tags?: string[];
  on_cancel?: () => void;
  close_after_save?: boolean;
  show_cancel_button?: boolean;
  show_delete_button?: boolean;
  variant?: "dialog" | "inline";
  content_max_width_class_name?: string;
}

/** 扩展选项 */
interface AgentDialogInitialOptions extends Partial<AgentConfigOptions> {
  permission_mode?: string;
  allowed_tools?: string[];
  disallowed_tools?: string[];
}

// ==================== 主组件 ====================

/** AgentOptions 表单主体 */
export function AgentOptionsEditor({
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
}: AgentOptionsEditorProps) {
  const { t } = useI18n();
  const sourceOptions = initial_options as AgentDialogInitialOptions;
  const initial_resolved_title = useMemo(
    () => initial_title || t("agent_options.default_name"),
    [initial_title, t],
  );

  // ---- 导航状态 ----
  const [activeTab, setActiveTab] = useState<TabKey>("identity");

  // ---- Identity 状态 ----
  const [title, setTitle] = useState(initial_title || t("agent_options.default_name"));
  const [avatar, setAvatar] = useState(initial_avatar);
  const [description, setDescription] = useState(initial_description);
  const [vibeTags, setVibeTags] = useState<string[]>(initial_vibe_tags);
  const [provider, setProvider] = useState<AgentProvider>(
    normalize_agent_option_provider(sourceOptions.provider) || DEFAULT_AGENT_OPTION_PROVIDER
  );
  const [defaultProvider, setDefaultProvider] = useState<AgentProvider>("");
  const [providerOptions, setProviderOptions] = useState<ProviderOption[]>([]);
  const [providerOptionsLoading, setProviderOptionsLoading] = useState(false);
  const [providerOptionsError, setProviderOptionsError] = useState<string | null>(null);

  // ---- Persona 状态 ----
  const [systemPrompt, setSystemPrompt] = useState(
    sourceOptions.system_prompt || ""
  );

  // ---- Advanced 状态 ----
  const [permissionMode, setPermissionMode] = useState(
    sourceOptions.permission_mode || "default"
  );
  const [allowedTools, setAllowedTools] = useState<string[]>(
    sourceOptions.allowed_tools || []
  );
  const [disallowedTools, setDisallowedTools] = useState<string[]>(
    sourceOptions.disallowed_tools || []
  );

  // ---- 名称校验 ----
  const [nameValidation, setNameValidation] =
    useState<AgentNameValidationResult | null>(null);
  const [isValidatingName, setIsValidatingName] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const trimmed_title = title.trim();
  const has_title_changed = trimmed_title !== initial_resolved_title.trim();

  // ---- 对话框打开时重置状态 ----
  useEffect(() => {
    if (!is_active) return;
    const opts = initial_options as AgentDialogInitialOptions;
    setActiveTab("identity");
    setTitle(initial_resolved_title);
    setAvatar(initial_avatar);
    setDescription(initial_description);
    setVibeTags(initial_vibe_tags);
    setProvider(normalize_agent_option_provider(opts.provider));
    setDefaultProvider("");
    setProviderOptionsError(null);
    setSystemPrompt(opts.system_prompt || "");
    setPermissionMode(opts.permission_mode || "default");
    setAllowedTools(opts.allowed_tools || []);
    setDisallowedTools(opts.disallowed_tools || []);
    setNameValidation(null);
    setIsValidatingName(false);
    setIsSaving(false);
  }, [initial_avatar, initial_description, initial_options, initial_resolved_title, initial_vibe_tags, is_active]);

  useEffect(() => {
    if (!is_active) {
      return;
    }

    let cancelled = false;

    const load_provider_options = async () => {
      try {
        setProviderOptionsLoading(true);
        const payload = await list_provider_options_api();
        if (cancelled) {
          return;
        }
        setProviderOptions(payload.items);
        setDefaultProvider(normalize_agent_option_provider(payload.default_provider));
        set_default_agent_provider(payload.default_provider);
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

  // ---- 名称校验 debounce ----
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

  // ---- 切换工具授权 ----
  const toggle_tool = (
    toolName: string,
    type: "allowed" | "disallowed"
  ) => {
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

  // ---- 保存逻辑 ----
  const handle_save = async () => {
    if (!trimmed_title) return;
    if (isValidatingName || isSaving) return;
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

    const options: AgentConfigOptions = {
      provider: provider.trim() || undefined,
      permission_mode: permissionMode,
      allowed_tools: allowedTools,
      disallowed_tools: disallowedTools,
      system_prompt: systemPrompt || undefined,
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
      }
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

  const content = (
    <>
      {activeTab === "identity" && (
        <AgentOptionsIdentityTab
          avatar={avatar}
          on_avatar_change={setAvatar}
          title={title}
          on_title_change={setTitle}
          description={description}
          on_description_change={setDescription}
          vibe_tags={vibeTags}
          on_vibe_tags_change={setVibeTags}
          provider={provider}
          default_provider={defaultProvider}
          provider_options={build_agent_option_provider_options(providerOptions, provider)}
          provider_options_error={providerOptionsError}
          provider_options_loading={providerOptionsLoading}
          on_provider_change={setProvider}
          name_validation={nameValidation}
          is_validating_name={isValidatingName}
          variant={variant}
        />
      )}

      {activeTab === "persona" && (
        <AgentOptionsPersonaTab
          system_prompt={systemPrompt}
          on_system_prompt_change={setSystemPrompt}
          variant={variant}
        />
      )}

      {activeTab === "advanced" && (
        <AgentOptionsAdvancedTab
          permission_mode={permissionMode}
          on_permission_mode_change={setPermissionMode}
          allowed_tools={allowedTools}
          on_toggle_tool={toggle_tool}
        />
      )}

      {activeTab === "skills" && (
        <AgentOptionsSkillsTab
          agent_id={mode === "edit" ? agent_id : undefined}
          is_visible={is_active && activeTab === "skills"}
        />
      )}
    </>
  );

  if (variant === "inline") {
    const inline_content_width_class_name =
      activeTab === "identity" ? content_max_width_class_name : "max-w-none";

    return (
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <AgentOptionsNav
          active_tab={activeTab}
          on_tab_change={setActiveTab}
          variant="inline"
          trailing={(
            <button
              className={get_dialog_action_class_name(canSave ? "primary" : "default", "compact")}
              onClick={() => {
                void handle_save();
              }}
              disabled={!canSave}
              type="button"
            >
              {isSaving
                ? t("common.saving")
                : mode === "create"
                  ? t("agent_options.title_create")
                  : t("agent_options.save_changes")}
            </button>
          )}
        />

        <div className="min-h-0 flex-1 overflow-y-auto">
          <div
            className={cn(
              "w-full px-6 py-5",
              inline_content_width_class_name,
              activeTab === "identity" && "mx-auto"
            )}
          >
            {content}
          </div>
        </div>

        {canDelete || (show_cancel_button && on_cancel) ? (
          <div className="flex items-center justify-end border-t dialog-divider px-6 py-3">
            {canDelete ? (
              <button
                className={cn(get_dialog_action_class_name("danger"), "mr-auto")}
                onClick={() => {
                  if (!agent_id || !on_delete) {
                    return;
                  }
                  on_delete(agent_id);
                }}
                type="button"
              >
                {t("agent_options.delete_agent")}
              </button>
            ) : null}
            {show_cancel_button && on_cancel ? (
              <button
                className={get_dialog_action_class_name("default")}
                onClick={on_cancel}
                type="button"
              >
                {t("common.cancel")}
              </button>
            ) : null}
          </div>
        ) : null}
      </div>
    );
  }

  return (
    <>
      <div className="flex min-h-0 flex-1 overflow-hidden">
        <AgentOptionsNav
          active_tab={activeTab}
          on_tab_change={setActiveTab}
        />

        <div className="flex-1 overflow-y-auto bg-transparent p-6">
          {content}
        </div>
      </div>

      <div className="dialog-footer px-5 py-3.5">
        {canDelete ? (
          <button
            className={cn(get_dialog_action_class_name("danger"), "mr-auto")}
            onClick={() => {
              if (!agent_id || !on_delete) {
                return;
              }
              on_delete(agent_id);
            }}
            type="button"
          >
            {t("agent_options.delete_agent")}
          </button>
        ) : null}
        {show_cancel_button && on_cancel ? (
          <button
            className={get_dialog_action_class_name("default")}
            onClick={on_cancel}
            type="button"
          >
            {t("common.cancel")}
          </button>
        ) : null}
        <button
          className={get_dialog_action_class_name(canSave ? "primary" : "default")}
          onClick={() => {
            void handle_save();
          }}
          disabled={!canSave}
          type="button"
        >
          {isSaving
            ? t("common.saving")
            : mode === "create"
              ? t("agent_options.title_create")
              : t("agent_options.save_changes")}
        </button>
      </div>
    </>
  );
}
