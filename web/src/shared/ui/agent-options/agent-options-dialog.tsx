/**
 * AgentOptions Dialog 主容器
 *
 * 三栏布局：左侧图标导航 (w-16) + 中间内容 (flex-1) + 右侧 Live Preview (w-80)
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

import { AgentOptionsNav, type TabKey } from "./agent-options-nav";
import { AgentOptionsIdentityTab } from "./agent-options-identity-tab";
import { AgentOptionsPersonaTab } from "./agent-options-persona-tab";
import { AgentOptionsSkillsTab } from "./agent-options-skills-tab";
import { AgentOptionsAdvancedTab } from "./agent-options-advanced-tab";
import { AgentOptionsPreview } from "./agent-options-preview";

// ==================== 类型定义 ====================

interface AgentOptionsProps {
  agent_id?: string;
  mode: "create" | "edit";
  is_open: boolean;
  on_close: () => void;
  on_save: (title: string, options: AgentConfigOptions) => void;
  on_validate_name?: (name: string) => Promise<AgentNameValidationResult>;
  initial_title?: string;
  initial_options?: Partial<AgentConfigOptions>;
}

/** 扩展选项（兼容旧字段） */
interface AgentDialogInitialOptions extends Partial<AgentConfigOptions> {
  permission_mode?: string;
  allowed_tools?: string[];
  disallowed_tools?: string[];
  skills_enabled?: boolean;
  setting_sources?: ("user" | "project" | "local")[];
}

// ==================== 主组件 ====================

/** AgentOptions 对话框 — 三栏布局 */
export function AgentOptions({
  agent_id,
  mode,
  is_open,
  on_close,
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

  // ---- Skills 状态 ----
  const [skillsEnabled, setSkillsEnabled] = useState(
    sourceOptions.skills_enabled ?? true
  );
  const [installedSkills, setInstalledSkills] = useState<string[]>(
    sourceOptions.installed_skills || []
  );
  const [settingSources, setSettingSources] = useState<
    ("user" | "project" | "local")[]
  >(sourceOptions.setting_sources || ["user", "project"]);

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
    setSkillsEnabled(opts.skills_enabled ?? true);
    setInstalledSkills(opts.installed_skills || []);
    setSettingSources(opts.setting_sources || ["user", "project"]);
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

  // ---- 切换技能来源 ----
  const toggleSettingSource = (source: "user" | "project" | "local") => {
    setSettingSources((prev) =>
      prev.includes(source)
        ? prev.filter((s) => s !== source)
        : [...prev, source]
    );
  };

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

    // 如果启用技能，自动添加 "Skill" 到 allowedTools
    const finalAllowedTools = [...allowedTools];
    if (skillsEnabled && !finalAllowedTools.includes("Skill")) {
      finalAllowedTools.push("Skill");
    }

    const options: AgentConfigOptions = {
      model,
      permission_mode: permissionMode,
      allowed_tools: finalAllowedTools,
      disallowed_tools: disallowedTools,
      system_prompt: systemPrompt || undefined,
      skills_enabled: skillsEnabled,
      installed_skills: installedSkills,
      setting_sources:
        settingSources.length > 0 ? settingSources : undefined,
    };
    on_save(trimmedTitle, options);
    on_close();
  };

  const isNameInvalid = !!(
    nameValidation &&
    (!nameValidation.is_valid || !nameValidation.is_available)
  );
  const canSave = !!title.trim() && !isValidatingName && !isNameInvalid;

  if (!is_open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="modal-dialog-surface radius-shell-xl flex h-[85vh] w-full max-w-5xl flex-col overflow-hidden animate-in zoom-in-95 duration-200">
        {/* 头部 */}
        <div className="flex items-center justify-between border-b modal-divider px-6 py-5">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl modal-card text-primary">
              <Settings className="h-4 w-4" />
            </div>
            <div>
              <h2 className="text-lg font-semibold tracking-tight text-slate-800">
                {mode === "create" ? "创建 Agent" : "Agent 设置"}
              </h2>
              <p className="text-xs text-slate-500">
                {mode === "create"
                  ? "配置 Agent 能力与行为策略"
                  : `正在编辑: ${title}`}
              </p>
            </div>
          </div>
          <button
            aria-label="关闭对话框"
            onClick={on_close}
            className="modal-btn-secondary rounded-xl p-2 text-slate-400 transition-colors hover:text-slate-700 focus-visible:ring-2 focus-visible:ring-primary/50"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* 三栏主体：左导航 + 中内容 + 右预览 */}
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

            {activeTab === "skills" && (
              <AgentOptionsSkillsTab
                agentId={agent_id}
                enabledSkillNames={installedSkills}
                skillsEnabled={skillsEnabled}
                onEnabledSkillNamesChange={setInstalledSkills}
                onSkillsEnabledChange={setSkillsEnabled}
                settingSources={settingSources}
                onToggleSettingSource={toggleSettingSource}
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
          </div>

          {/* 右侧 Live Preview */}
          <AgentOptionsPreview
            title={title}
            description={description}
            vibeTags={vibeTags}
            model={model}
            skillsEnabled={skillsEnabled}
          />
        </div>

        {/* 底部按钮 */}
        <div className="flex items-center justify-end gap-3 border-t modal-divider px-6 py-5">
          <button
            onClick={on_close}
            className="modal-btn-secondary rounded-xl px-5 py-2.5 text-sm font-medium text-slate-500 transition-colors hover:text-slate-800"
          >
            取消
          </button>
          <button
            onClick={handleSave}
            disabled={!canSave}
            className={cn(
              "rounded-xl px-5 py-2.5 text-sm font-medium transition-all",
              canSave
                ? "bg-primary text-primary-foreground shadow-[0_8px_24px_rgba(133,119,255,0.25)] hover:bg-primary/90 hover:shadow-[0_12px_32px_rgba(133,119,255,0.3)]"
                : "modal-card cursor-not-allowed text-slate-400"
            )}
          >
            {mode === "create" ? "创建 Agent" : "保存更改"}
          </button>
        </div>
      </div>
    </div>
  );
}
