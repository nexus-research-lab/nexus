/**
 * AgentOptions Dialog 主容器
 *
 * 双栏布局：左侧导航 + 右侧内容区
 * 管理所有状态并分发给子组件
 */

"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { Settings, X } from "lucide-react";
import { cn } from "@/lib/utils";
import type {
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions as AgentConfigOptions,
} from "@/types/agent";
import {
  DIALOG_ICON_BUTTON_CLASS_NAME,
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
  getDialogActionClassName,
} from "@/shared/ui/dialog/dialog-styles";
import { useI18n } from "@/shared/i18n/i18n-context";

import { AgentOptionsNav, type TabKey } from "./agent-options-nav";
import { AgentOptionsIdentityTab } from "./agent-options-identity-tab";
import { AgentOptionsPersonaTab } from "./agent-options-persona-tab";
import { AgentOptionsSkillsTab } from "./agent-options-skills-tab";
import { AgentOptionsAdvancedTab } from "./agent-options-advanced-tab";

interface AgentOptionsProps {
  agent_id?: string;
  mode: "create" | "edit";
  is_open: boolean;
  on_close: () => void;
  on_delete?: (agent_id: string) => void;
  on_save: (title: string, options: AgentConfigOptions, identity: AgentIdentityDraft) => void;
  on_validate_name?: (name: string) => Promise<AgentNameValidationResult>;
  initial_title?: string;
  initial_options?: Partial<AgentConfigOptions>;
  initial_avatar?: string;
  initial_description?: string;
  initial_vibe_tags?: string[];
}

/** 扩展选项 */
interface AgentDialogInitialOptions extends Partial<AgentConfigOptions> {
  permission_mode?: string;
  allowed_tools?: string[];
  disallowed_tools?: string[];
}

// ==================== 主组件 ====================

/** AgentOptions 对话框 */
export function AgentOptions({
  agent_id,
  mode,
  is_open,
  on_close,
  on_delete,
  on_save,
  on_validate_name,
  initial_title = "",
  initial_options = {},
  initial_avatar = "",
  initial_description = "",
  initial_vibe_tags = [],
}: AgentOptionsProps) {
  const { t } = useI18n();
  const sourceOptions = initial_options as AgentDialogInitialOptions;

  // ---- 导航状态 ----
  const [activeTab, setActiveTab] = useState<TabKey>("identity");

  // ---- Identity 状态 ----
  const [title, setTitle] = useState(initial_title || t("agent_options.default_name"));
  const [avatar, setAvatar] = useState(initial_avatar);
  const [description, setDescription] = useState(initial_description);
  const [vibeTags, setVibeTags] = useState<string[]>(initial_vibe_tags);
  const [model, setModel] = useState(sourceOptions.model || "glm-5.1");

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

  // ---- 对话框打开时重置状态 ----
  useEffect(() => {
    if (!is_open) return;
    const opts = initial_options as AgentDialogInitialOptions;
    setActiveTab("identity");
    setTitle(initial_title || t("agent_options.default_name"));
    setAvatar(initial_avatar);
    setDescription(initial_description);
    setVibeTags(initial_vibe_tags);
    setModel(opts.model || "glm-5.1");
    setSystemPrompt(opts.system_prompt || "");
    setPermissionMode(opts.permission_mode || "default");
    setAllowedTools(opts.allowed_tools || []);
    setDisallowedTools(opts.disallowed_tools || []);
    setNameValidation(null);
    setIsValidatingName(false);
  }, [initial_avatar, initial_description, initial_options, initial_title, initial_vibe_tags, is_open, t]);

  useEffect(() => {
    if (!is_open) {
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        on_close();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [is_open, on_close]);

  // ---- 名称校验 debounce ----
  useEffect(() => {
    if (!is_open) return;
    if (!on_validate_name) {
      setNameValidation(null);
      return;
    }
    const trimmed = title.trim();
    if (!trimmed) {
      setNameValidation(null);
      setIsValidatingName(false);
      return;
    }

    let cancelled = false;
    const timer = window.setTimeout(async () => {
      try {
        setIsValidatingName(true);
        const result = await on_validate_name(trimmed);
        if (!cancelled) setNameValidation(result);
      } catch (error) {
        if (!cancelled) {
          setNameValidation({
            name: trimmed,
            normalized_name: trimmed,
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
  }, [title, is_open, on_validate_name, t]);

  // ---- 切换工具授权 ----
  const toggleTool = (
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
  const handleSave = () => {
    const trimmedTitle = title.trim();
    if (!trimmedTitle) return;
    if (isValidatingName) return;
    if (
      nameValidation &&
      (!nameValidation.is_valid || !nameValidation.is_available)
    )
      return;

    const options: AgentConfigOptions = {
      model,
      permission_mode: permissionMode,
      allowed_tools: allowedTools,
      disallowed_tools: disallowedTools,
      system_prompt: systemPrompt || undefined,
      setting_sources: ["project"],
    };
    on_save(trimmedTitle, options, {
      avatar,
      description: description.trim(),
      vibe_tags: vibeTags,
    });
    on_close();
  };

  const isNameInvalid = !!(
    nameValidation &&
    (!nameValidation.is_valid || !nameValidation.is_available)
  );
  const canSave = !!title.trim() && !isValidatingName && !isNameInvalid;
  const canDelete = mode === "edit" && Boolean(agent_id) && Boolean(on_delete);

  if (!is_open || typeof document === "undefined") return null;

  const dialog = (
    <div className="dialog-backdrop z-[9999]" role="dialog" aria-modal="true">
      <div className="dialog-shell radius-shell-xl flex h-[80vh] w-full max-w-[920px] flex-col overflow-hidden">
        <div className="dialog-header px-5 py-4">
          <div className={cn(DIALOG_HEADER_LEADING_CLASS_NAME, "min-w-0 flex-1 items-center")}>
            <div className={cn(DIALOG_HEADER_ICON_CLASS_NAME, "h-11 w-11 rounded-[16px] text-primary")}>
              <Settings className="h-5 w-5" />
            </div>
            <div className="min-w-0">
              <h2 className="dialog-title truncate text-[22px] font-black tracking-[-0.04em]">
                {mode === "create" ? t("agent_options.title_create") : title}
              </h2>
              {mode === "edit" && agent_id ? (
                <p className="dialog-subtitle">{t("agent_options.id_prefix")}: {agent_id}</p>
              ) : (
                <p className="dialog-subtitle">{t("agent_options.subtitle_create")}</p>
              )}
            </div>
          </div>
          <button
            className={DIALOG_ICON_BUTTON_CLASS_NAME}
            aria-label={t("agent_options.close_dialog")}
            onClick={on_close}
            type="button"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* 主体：左导航 + 右内容 */}
        <div className="flex flex-1 overflow-hidden">
          {/* 左侧图标导航 */}
          <AgentOptionsNav
            activeTab={activeTab}
            onTabChange={setActiveTab}
          />

          {/* 中间内容区 */}
          <div className="flex-1 overflow-y-auto bg-transparent p-6">
            {activeTab === "identity" && (
              <AgentOptionsIdentityTab
                avatar={avatar}
                onAvatarChange={setAvatar}
                title={title}
                onTitleChange={setTitle}
                description={description}
                onDescriptionChange={setDescription}
                vibeTags={vibeTags}
                onVibeTagsChange={setVibeTags}
                model={model}
                onModelChange={setModel}
                nameValidation={nameValidation}
                isValidatingName={isValidatingName}
              />
            )}

            {activeTab === "persona" && (
              <AgentOptionsPersonaTab
                systemPrompt={systemPrompt}
                onSystemPromptChange={setSystemPrompt}
              />
            )}

            {activeTab === "advanced" && (
              <AgentOptionsAdvancedTab
                permissionMode={permissionMode}
                onPermissionModeChange={setPermissionMode}
                allowedTools={allowedTools}
                onToggleTool={toggleTool}
              />
            )}

            {activeTab === "skills" && (
              <AgentOptionsSkillsTab
                agent_id={mode === "edit" ? agent_id : undefined}
                is_visible={activeTab === "skills"}
              />
            )}
          </div>
        </div>

        {/* 底部按钮 */}
        <div className="dialog-footer px-5 py-3.5">
          {canDelete ? (
            <button
              className={cn(getDialogActionClassName("danger"), "mr-auto")}
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
          <button
            className={getDialogActionClassName("default")}
            onClick={on_close}
            type="button"
          >
            {t("common.cancel")}
          </button>
          <button
            className={getDialogActionClassName(canSave ? "primary" : "default")}
            onClick={handleSave}
            disabled={!canSave}
            type="button"
          >
            {mode === "create" ? t("agent_options.title_create") : t("agent_options.save_changes")}
          </button>
        </div>
      </div>
    </div>
  );

  return createPortal(dialog, document.body);
}
