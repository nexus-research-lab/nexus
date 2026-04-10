/**
 * AgentOptions Dialog 主容器
 *
 * 双栏布局：左侧导航 + 右侧内容区
 * 管理所有状态并分发给子组件
 */

"use client";

import { useEffect, useState } from "react";
import { Settings, X } from "lucide-react";
import { cn } from "@/lib/utils";
import type {
  AgentNameValidationResult,
  AgentOptions as AgentConfigOptions,
} from "@/types/agent";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import {
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
} from "@/shared/ui/dialog/dialog-styles";

import { AgentOptionsNav, type TabKey } from "./agent-options-nav";
import { AgentOptionsIdentityTab } from "./agent-options-identity-tab";
import { AgentOptionsPersonaTab } from "./agent-options-persona-tab";
import { AgentOptionsSkillsTab } from "./agent-options-skills-tab";
import { AgentOptionsAdvancedTab } from "./agent-options-advanced-tab";

// ==================== 类型定义 ====================

interface AgentOptionsProps {
  agent_id?: string;
  mode: "create" | "edit";
  is_open: boolean;
  on_close: () => void;
  on_delete?: (agent_id: string) => void;
  on_save: (title: string, options: AgentConfigOptions) => void;
  on_validate_name?: (name: string) => Promise<AgentNameValidationResult>;
  initial_title?: string;
  initial_options?: Partial<AgentConfigOptions>;
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
}: AgentOptionsProps) {
  const sourceOptions = initial_options as AgentDialogInitialOptions;

  // ---- 导航状态 ----
  const [activeTab, setActiveTab] = useState<TabKey>("identity");

  // ---- Identity 状态 ----
  const [title, setTitle] = useState(initial_title || "Agent");
  const [description, setDescription] = useState("");
  const [vibeTags, setVibeTags] = useState<string[]>([]);
  const [model, setModel] = useState(sourceOptions.model || "glm-5");

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
    setTitle(initial_title || "Agent");
    setDescription("");
    setVibeTags([]);
    setModel(opts.model || "glm-5");
    setSystemPrompt(opts.system_prompt || "");
    setPermissionMode(opts.permission_mode || "default");
    setAllowedTools(opts.allowed_tools || []);
    setDisallowedTools(opts.disallowed_tools || []);
    setNameValidation(null);
    setIsValidatingName(false);
  }, [is_open, initial_title, initial_options]);

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
              error instanceof Error ? error.message : "名称校验失败",
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
  }, [title, is_open, on_validate_name]);

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
    on_save(trimmedTitle, options);
    on_close();
  };

  const isNameInvalid = !!(
    nameValidation &&
    (!nameValidation.is_valid || !nameValidation.is_available)
  );
  const canSave = !!title.trim() && !isValidatingName && !isNameInvalid;
  const canDelete = mode === "edit" && Boolean(agent_id) && Boolean(on_delete);

  if (!is_open) return null;

  return (
    <div className="dialog-backdrop animate-in fade-in duration-200">
      <div className="dialog-shell radius-shell-xl flex h-[85vh] w-full max-w-[980px] flex-col overflow-hidden animate-in zoom-in-95 duration-200">
        <div className="dialog-header">
          <div className={cn(DIALOG_HEADER_LEADING_CLASS_NAME, "min-w-0 flex-1 items-center")}>
            <div className={cn(DIALOG_HEADER_ICON_CLASS_NAME, "h-14 w-14 rounded-[20px] text-primary")}>
              <Settings className="h-7 w-7" />
            </div>
            <div className="min-w-0">
              <h2 className="dialog-title truncate" data-size="hero">
                {mode === "create" ? "创建 Agent" : title}
              </h2>
              {mode === "edit" && agent_id ? (
                <p className="dialog-subtitle">ID: {agent_id}</p>
              ) : (
                <p className="dialog-subtitle">配置身份、技能、权限和行为方式</p>
              )}
            </div>
          </div>
          <WorkspacePillButton
            aria-label="关闭对话框"
            onClick={on_close}
            density="compact"
            size="icon"
            variant="icon"
          >
            <X className="h-5 w-5" />
          </WorkspacePillButton>
        </div>

        {/* 主体：左导航 + 右内容 */}
        <div className="flex flex-1 overflow-hidden">
          {/* 左侧图标导航 */}
          <AgentOptionsNav
            activeTab={activeTab}
            onTabChange={setActiveTab}
          />

          {/* 中间内容区 */}
          <div className="flex-1 overflow-y-auto bg-transparent p-8">
            {activeTab === "identity" && (
              <AgentOptionsIdentityTab
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
        <div className="dialog-footer">
          {canDelete ? (
            <WorkspacePillButton
              onClick={() => {
                if (!agent_id || !on_delete) {
                  return;
                }
                on_delete(agent_id);
              }}
              size="md"
              tone="danger"
              variant="tonal"
              class_name="mr-auto"
            >
              删除 Agent
            </WorkspacePillButton>
          ) : null}
          <WorkspacePillButton
            onClick={on_close}
            size="md"
            variant="tonal"
          >
            取消
          </WorkspacePillButton>
          <WorkspacePillButton
            onClick={handleSave}
            disabled={!canSave}
            size="md"
            variant={canSave ? "primary" : "tonal"}
          >
            {mode === "create" ? "创建 Agent" : "保存更改"}
          </WorkspacePillButton>
        </div>
      </div>
    </div>
  );
}
