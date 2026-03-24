// !/usr/bin/env python
// -*- coding: utf-8 -*-
// =====================================================
// @File   :session-options-dialog.tsx
// @Date   :2025-12-01 22:56
// @Author :leemysw
// 2025-12-01 22:56   Create
// =====================================================

/**
 * Session Options Dialog Component
 *
 * 用于创建和编辑会话配置的对话框组件，支持多标签页配置界面
 */

"use client";

import { useEffect, useState } from "react";
import { MessageSquare, Settings, Sparkles, Wrench, X, Zap } from "lucide-react";
import { cn } from "@/lib/utils";
import { SessionOptions } from "@/types/session";
import { AgentNameValidationResult } from "@/types/agent";
import { AgentDialogInitialOptions, AgentOptionsProps } from "@/types/shared-ui";

type TabKey = 'basic' | 'prompt' | 'tools' | 'skills' | 'advanced';

// 预定义的模型列表
const AVAILABLE_MODELS = [
  {value: 'glm-5', label: 'GLM 5'},
  {value: 'deepseek-chat', label: 'DeepSeek Chat | 深度求索'},
  {value: 'claude-3-5-sonnet', label: 'Claude 3.5 Sonnet'},
  {value: 'claude-3-opus', label: 'Claude 3 Opus'},
  {value: 'claude-3-haiku', label: 'Claude 3 Haiku'},
];

// 权限模式选项
const PERMISSION_MODES = [
  {value: 'default', label: '默认（继续前询问）', description: '只读工具会自动预先授权，其它操作仍需权限。'},
  {value: 'plan', label: '规划模式', description: '继承默认的只读工具集，并会在执行行为前呈现计划。'},
  {value: 'acceptEdits', label: '自动授权文件编辑', description: '默认的只读工具会自动预先授权，但执行仍被禁用。'},
  {value: 'bypassPermissions', label: '跳过所有权限检查', description: '所有工具都会在无审批情况下执行。'},
] as const;

// 常用工具列表（硬编码，后续可从API获取）
// 'Task', 'TaskOutput', 'Bash', 'Glob', 'Grep', 'ExitPlanMode', 'Read', 'Edit', 'Write', 'NotebookEdit', 'WebFetch', 'TodoWrite', 'WebSearch', 'KillShell', 'AskUserQuestion', 'Skill', 'EnterPlanMode'
const AVAILABLE_TOOLS = [
  {name: 'Task', description: 'Executes tasks'},
  {name: 'TaskOutput', description: 'Displays task output'},
  {name: 'Bash', description: 'Executes shell commands in your environment'},
  {name: 'Glob', description: 'Matches file names and patterns'},
  {name: 'Grep', description: 'Searches for patterns in files'},
  {name: 'ExitPlanMode', description: 'Exits planning mode'},
  {name: 'Read', description: 'Reads files'},
  {name: 'Edit', description: 'Edits files'},
  {name: 'Write', description: 'Creates or overwrites files'},
  {name: 'NotebookEdit', description: 'Edits Jupyter Notebooks'},
  {name: 'WebFetch', description: 'Fetches web pages'},
  {name: 'TodoWrite', description: 'Creates or updates to-do lists'},
  {name: 'WebSearch', description: 'Performs web searches with domain filtering'},
  {name: 'KillShell', description: 'Kills the shell process'},
  {name: 'AskUserQuestion', description: 'Asks the user a question'},
  {name: 'Skill', description: 'Executes a skill'},
  {name: 'EnterPlanMode', description: 'Enters planning mode'}
];

// ==================== 主组件 ====================

export function AgentOptions(
  {
    mode,
    is_open,
    on_close,
    on_save,
    on_validate_name,
    initial_title = '',
    initial_options = {},
  }: AgentOptionsProps) {
  const sourceOptions = initial_options as AgentDialogInitialOptions;

  // 状态管理
  const [activeTab, setActiveTab] = useState<TabKey>('basic');
  const [title, setTitle] = useState(initial_title || 'Agent');
  const [model, setModel] = useState(sourceOptions.model || 'glm-5');
  const [permissionMode, setPermissionMode] = useState(
    sourceOptions.permission_mode || sourceOptions.permission_mode || 'default'
  );
  const [allowedTools, setAllowedTools] = useState<string[]>(
    sourceOptions.allowed_tools || sourceOptions.allowed_tools || []
  );
  const [disallowedTools, setDisallowedTools] = useState<string[]>(
    sourceOptions.disallowed_tools || sourceOptions.disallowed_tools || []
  );
  const [systemPrompt, setSystemPrompt] = useState(sourceOptions.system_prompt || '');

  // 技能配置状态
  const [skillsEnabled, setSkillsEnabled] = useState(
    sourceOptions.skills_enabled ?? sourceOptions.skills_enabled ?? true
  );
  const [settingSources, setSettingSources] = useState<('user' | 'project' | 'local')[]>(
    sourceOptions.setting_sources || sourceOptions.setting_sources || ['user', 'project']
  );
  const [nameValidation, setNameValidation] = useState<AgentNameValidationResult | null>(null);
  const [isValidatingName, setIsValidatingName] = useState(false);

  useEffect(() => {
    if (!is_open) return;
    const nextOptions = initial_options as AgentDialogInitialOptions;
    setActiveTab('basic');
    setTitle(initial_title || 'Agent');
    setModel(nextOptions.model || 'glm-5');
    setPermissionMode(nextOptions.permission_mode || nextOptions.permission_mode || 'default');
    setAllowedTools(nextOptions.allowed_tools || nextOptions.allowed_tools || []);
    setDisallowedTools(nextOptions.disallowed_tools || nextOptions.disallowed_tools || []);
    setSystemPrompt(nextOptions.system_prompt || '');
    setSkillsEnabled(nextOptions.skills_enabled ?? nextOptions.skills_enabled ?? true);
    setSettingSources(
      nextOptions.setting_sources || nextOptions.setting_sources || ['user', 'project']
    );
    setNameValidation(null);
    setIsValidatingName(false);
  }, [is_open, initial_title, initial_options]);

  // 切换技能来源
  const toggleSettingSource = (source: 'user' | 'project' | 'local') => {
    setSettingSources(prev =>
      prev.includes(source)
        ? prev.filter(s => s !== source)
        : [...prev, source]
    );
  };

  // 标签页配置
  const tabs = [
    {key: 'basic' as TabKey, label: '基础设置', icon: Settings},
    {key: 'prompt' as TabKey, label: '提示词设置', icon: MessageSquare},
    {key: 'tools' as TabKey, label: '工具与权限', icon: Wrench},
    {key: 'skills' as TabKey, label: 'SKILLS 配置', icon: Sparkles},
    {key: 'advanced' as TabKey, label: '高级设置', icon: Zap},
  ];

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
        if (!cancelled) {
          setNameValidation(result);
        }
      } catch (error) {
        if (!cancelled) {
          setNameValidation({
            name: trimmed,
            normalized_name: trimmed,
            is_valid: false,
            is_available: false,
            reason: error instanceof Error ? error.message : '名称校验失败',
            workspace_path: null,
          });
        }
      } finally {
        if (!cancelled) {
          setIsValidatingName(false);
        }
      }
    }, 300);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [title, is_open, on_validate_name]);

  // 处理工具选择
  const toggleTool = (toolName: string, type: 'allowed' | 'disallowed') => {
    if (type === 'allowed') {
      setAllowedTools(prev =>
        prev.includes(toolName)
          ? prev.filter(t => t !== toolName)
          : [...prev, toolName]
      );
    } else {
      setDisallowedTools(prev =>
        prev.includes(toolName)
          ? prev.filter(t => t !== toolName)
          : [...prev, toolName]
      );
    }
  };

  // 处理保存
  const handleSave = () => {
    const trimmedTitle = title.trim();
    if (!trimmedTitle) return;
    if (isValidatingName) return;
    if (nameValidation && (!nameValidation.is_valid || !nameValidation.is_available)) return;

    // 如果启用技能，自动添加 "Skill" 到 allowedTools
    const finalAllowedTools = [...allowedTools];
    if (skillsEnabled && !finalAllowedTools.includes('Skill')) {
      finalAllowedTools.push('Skill');
    }

    const options: SessionOptions = {
      model,
      permission_mode: permissionMode,
      allowed_tools: finalAllowedTools,
      disallowed_tools: disallowedTools,
      system_prompt: systemPrompt || undefined,
      // Skills 配置
      skills_enabled: skillsEnabled,
      setting_sources: settingSources.length > 0 ? settingSources : undefined,
    };
    on_save(trimmedTitle, options);
    on_close();
  };

  const isNameInvalid = !!(nameValidation && (!nameValidation.is_valid || !nameValidation.is_available));
  const canSave = !!title.trim() && !isValidatingName && !isNameInvalid;

  if (!is_open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm animate-in fade-in duration-200">
      <div
        className="soft-ring radius-shell-xl panel-surface flex h-[85vh] w-full max-w-4xl flex-col overflow-hidden animate-in zoom-in-95 duration-200">
        {/* 头部 */}
        <div className="flex items-center justify-between border-b border-white/55 px-6 py-5">
          <div className="flex items-center gap-3">
            <div className="neo-pill radius-shell-sm flex h-10 w-10 items-center justify-center text-primary">
              <Settings className="w-4 h-4"/>
            </div>
            <div>
              <h2 className="text-lg font-semibold text-foreground tracking-tight">
                {mode === 'create' ? '创建 Agent' : 'Agent 设置'}
              </h2>
              <p className="text-xs text-muted-foreground">
                {mode === 'create' ? '配置 Agent 能力与行为策略' : `正在编辑: ${title}`}
              </p>
            </div>
          </div>
          <button
            aria-label="关闭对话框"
            onClick={on_close}
            className="neo-pill radius-shell-sm p-2 text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary/50"
          >
            <X className="w-5 h-5"/>
          </button>
        </div>

        {/* 主体：左侧标签 + 右侧内容 */}
        <div className="flex flex-1 overflow-hidden">
          {/* 左侧标签页 */}
          <div className="flex w-56 flex-col gap-2 border-r border-white/55 bg-white/10 p-3">
            {tabs.map(tab => {
              const Icon = tab.icon;
              const isActive = activeTab === tab.key;
              return (
                <button
                  key={tab.key}
                  onClick={() => setActiveTab(tab.key)}
                  className={cn(
                    "radius-shell-md flex items-center gap-3 px-4 py-3 text-sm transition-all duration-200",
                    isActive
                      ? "neo-card-flat text-primary font-medium shadow-[0_14px_24px_rgba(133,119,255,0.12)]"
                      : "text-muted-foreground hover:bg-white/25 hover:text-foreground"
                  )}
                >
                  <Icon className={cn("w-4 h-4", isActive ? "text-primary" : "opacity-70")}/>
                  <span>{tab.label}</span>
                  {isActive && (
                    <div className="ml-auto w-1.5 h-1.5 rounded-full bg-primary"/>
                  )}
                </button>
              );
            })}
          </div>

          {/* 右侧内容区 */}
          <div className="flex-1 overflow-y-auto bg-transparent p-8">
            {/* 基础设置 */}
            {activeTab === 'basic' && (
              <div className="space-y-8 max-w-2xl animate-in slide-in-from-right-4 duration-300">
                <div className="space-y-4">
                  <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">基本信息</h3>

                  <div className="space-y-2">
                    <label
                      className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                      Agent 名称 <span className="text-red-500">*</span>
                    </label>
                    <input
                      type="text"
                      value={title}
                      onChange={(e) => setTitle(e.target.value)}
                      className="neo-inset radius-shell-sm flex h-11 w-full px-4 py-2 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40 disabled:cursor-not-allowed disabled:opacity-50 transition-all"
                      placeholder="例如：Coding Assistant"
                    />
                    <div className="min-h-5 text-xs">
                      {isValidatingName && (
                        <span className="text-muted-foreground">正在校验名称...</span>
                      )}
                      {!isValidatingName && nameValidation?.reason && (
                        <span className="text-red-500">{nameValidation.reason}</span>
                      )}
                      {!isValidatingName && nameValidation?.is_valid && nameValidation?.is_available && (
                        <span className="text-emerald-600">
                          名称可用，工作区将自动创建到：{nameValidation.workspace_path}
                        </span>
                      )}
                    </div>
                  </div>

                  <div className="space-y-2">
                    <label className="text-sm font-medium leading-none">
                      模型选择 <span className="text-red-500">*</span>
                    </label>
                    <div className="relative">
                      <select
                        value={model}
                        onChange={(e) => setModel(e.target.value)}
                        className="neo-inset radius-shell-sm flex h-11 w-full appearance-none px-4 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40 disabled:cursor-not-allowed disabled:opacity-50 transition-all"
                      >
                        {AVAILABLE_MODELS.map(m => (
                          <option key={m.value} value={m.value}>{m.label}</option>
                        ))}
                      </select>
                      <div
                        className="absolute right-3 top-1/2 -translate-y-1/2 pointer-events-none text-muted-foreground">
                        <svg width="10" height="6" viewBox="0 0 10 6" fill="none" xmlns="http://www.w3.org/2000/svg">
                          <path d="M1 1L5 5L9 1" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"
                                strokeLinejoin="round"/>
                        </svg>
                      </div>
                    </div>
                  </div>
                </div>

                <div className="space-y-4">
                  <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">工作区策略</h3>
                  <div className="neo-card-flat radius-shell-md space-y-1 p-4 text-sm text-muted-foreground">
                    <p>工作目录由系统自动托管，不再支持手动输入。</p>
                    <p>目录规则：`~/.nexus-core/workspace/&lt;agent_name_slug&gt;`。</p>
                    <p>首次创建时会自动初始化 `AGENTS.md`、`MEMORY.md` 等模板。</p>
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-medium leading-none">描述</label>
                    <textarea
                      className="neo-inset radius-shell-md flex min-h-[96px] w-full resize-y px-4 py-3 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40 disabled:cursor-not-allowed disabled:opacity-50 transition-all"
                      rows={3}
                      placeholder="描述此会话的目标或背景信息..."
                    />
                  </div>
                </div>
              </div>
            )}

            {/* 提示词设置 */}
            {activeTab === 'prompt' && (
              <div className="space-y-6 h-full flex flex-col animate-in slide-in-from-right-4 duration-300">
                <div className="flex flex-col h-full space-y-2">
                  <label className="text-sm font-medium leading-none flex items-center justify-between">
                    <span>系统提示词 (System Prompt)</span>
                    <span className="text-xs font-normal text-muted-foreground">支持 Markdown</span>
                  </label>
                  <div className="flex-1 relative">
                    <textarea
                      value={systemPrompt}
                      onChange={(e) => setSystemPrompt(e.target.value)}
                      className="absolute inset-0 h-full w-full resize-none neo-inset radius-shell-md px-4 py-3 text-sm font-mono leading-relaxed placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40 disabled:cursor-not-allowed disabled:opacity-50"
                      placeholder="在此输入自定义系统提示词，它将决定 Agent 的行为模式、角色设定和限制条件..."
                    />
                  </div>
                  <p className="text-xs text-muted-foreground">
                    💡 提示：自定义系统提示词将覆盖默认的 Agent 设定。
                  </p>
                </div>
              </div>
            )}

            {/* 工具与权限 */}
            {activeTab === 'tools' && (
              <div className="space-y-8 animate-in slide-in-from-right-4 duration-300">
                {/* 权限模式 */}
                <div className="space-y-4">
                  <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">权限控制</h3>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {PERMISSION_MODES.map(pm => (
                      <button
                        key={pm.value}
                        onClick={() => setPermissionMode(pm.value)}
                        className={cn(
                          "radius-shell-md relative p-4 text-left transition-all duration-200",
                          permissionMode === pm.value
                            ? "neo-card bg-primary/5 ring-1 ring-primary/40"
                            : "neo-card-flat hover:border-primary/30"
                        )}
                      >
                        <div className="flex items-center justify-between mb-2">
                          <span className="font-semibold text-sm">{pm.label}</span>
                          {permissionMode === pm.value && (
                            <div className="w-4 h-4 rounded-full bg-primary flex items-center justify-center">
                              <svg width="10" height="8" viewBox="0 0 10 8" fill="none"
                                   xmlns="http://www.w3.org/2000/svg">
                                <path d="M1 4L3.5 6.5L9 1" stroke="white" strokeWidth="1.5" strokeLinecap="round"
                                      strokeLinejoin="round"/>
                              </svg>
                            </div>
                          )}
                        </div>
                        <p className="text-xs text-muted-foreground leading-relaxed">{pm.description}</p>
                      </button>
                    ))}
                  </div>
                  {permissionMode === 'bypassPermissions' && allowedTools.length > 0 && (
                    <div className="radius-shell-md border border-amber-500/20 bg-amber-500/10 p-4 text-xs leading-relaxed text-amber-700">
                      `bypassPermissions` 会放行所有工具，`allowed_tools` 只代表预授权集合，并不能限制其它工具。
                      如果你想在全放行模式下屏蔽个别危险工具，请改用 `disallowed_tools`。
                    </div>
                  )}
                </div>

                {/* 预先授权工具 */}
                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">工具授权</h3>
                    <span className="text-xs text-muted-foreground">
                      已启用 {allowedTools.length} 个工具
                    </span>
                  </div>

                  <div className="radius-shell-md flex gap-3 border border-orange-500/20 bg-orange-500/10 p-4">
                    <div className="text-orange-600 mt-0.5">
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
                           strokeLinecap="round" strokeLinejoin="round">
                        <path
                          d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path>
                        <line x1="12" y1="9" x2="12" y2="13"></line>
                        <line x1="12" y1="17" x2="12.01" y2="17"></line>
                      </svg>
                    </div>
                    <div>
                      <p className="text-sm font-medium text-orange-700">安全提示</p>
                      <p className="text-xs text-orange-600/90 mt-1 leading-relaxed">
                        被选中的工具将被`预先授权`，Agent 调用这些工具时将不会请求您的确认。请仅为您完全信任的工具开启此选项。
                      </p>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 gap-3">
                    {AVAILABLE_TOOLS.map(tool => {
                      const isChecked = allowedTools.includes(tool.name);
                      return (
                        <div
                          key={tool.name}
                          className={cn(
                            "radius-shell-md flex items-center justify-between p-4 transition-all duration-200",
                            isChecked
                              ? "neo-card bg-primary/5"
                              : "neo-card-flat hover:border-primary/20"
                          )}
                        >
                          <div className="flex-1 mr-4">
                            <div className="font-medium text-sm flex items-center gap-2">
                              {tool.name}
                              {isChecked && <span
                                className="text-[10px] px-1.5 py-0.5 bg-primary/10 text-primary rounded font-medium">已授权</span>}
                            </div>
                            <div className="text-xs text-muted-foreground mt-1">{tool.description}</div>
                          </div>

                          {/* 自定义 Switch 样式 */}
                          <label className="relative inline-flex items-center cursor-pointer">
                            <input
                              type="checkbox"
                              checked={isChecked}
                              onChange={() => toggleTool(tool.name, 'allowed')}
                              className="sr-only peer"
                            />
                            <div
                              className="w-11 h-6 bg-muted rounded-full peer peer-focus:ring-2 peer-focus:ring-primary/20 dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-0.5 after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-primary"></div>
                          </label>
                        </div>
                      );
                    })}
                  </div>
                </div>
              </div>
            )}

            {/* 技能配置 */}
            {activeTab === 'skills' && (
              <div className="space-y-8 max-w-2xl animate-in slide-in-from-right-4 duration-300">
                <div className="space-y-4">
                  <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">Agent Skills</h3>

                  {/* 技能启用开关 */}
                  <div
                    className="neo-card-flat radius-shell-md flex items-center justify-between p-4 transition-all hover:border-primary/20">
                    <div className="flex-1">
                      <label className="text-sm font-medium leading-none flex items-center gap-2">
                        启用技能系统
                        {skillsEnabled && <span
                          className="text-[10px] px-1.5 py-0.5 bg-green-500/10 text-green-600 rounded font-medium">已启用</span>}
                      </label>
                      <p className="text-xs text-muted-foreground mt-1 leading-relaxed">
                        技能是可重用的专业能力模块，Claude 会根据任务上下文自动调用相关技能。
                      </p>
                    </div>
                    <label className="relative inline-flex items-center cursor-pointer ml-4">
                      <input
                        type="checkbox"
                        checked={skillsEnabled}
                        onChange={(e) => setSkillsEnabled(e.target.checked)}
                        className="sr-only peer"
                      />
                      <div
                        className="w-11 h-6 bg-muted rounded-full peer peer-focus:ring-2 peer-focus:ring-primary/20 dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-0.5 after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-primary"></div>
                    </label>
                  </div>
                </div>

                {/* 设置来源选择 */}
                <div className="space-y-4">
                  <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">设置加载来源</h3>

                  <div className="radius-shell-md flex gap-3 border border-orange-500/20 bg-orange-500/10 p-4">
                    <div className="text-orange-600 mt-0.5">
                      <Sparkles className="w-4 h-4"/>
                    </div>
                    <div>
                      <p className="text-sm font-medium text-orange-700">来源同时影响技能与权限规则</p>
                      <p className="text-xs text-orange-600/90 mt-1 leading-relaxed">
                        Nexus 会从这些来源读取配置。项目 / 本地设置里的权限规则只有在对应来源启用后，后续会话才会自动生效。
                      </p>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 gap-3">
                    {/* 用户设置 */}
                    <div
                      className={cn(
                        "radius-shell-md flex items-center justify-between p-4 transition-all duration-200",
                        settingSources.includes('user')
                          ? "neo-card bg-primary/5"
                          : "neo-card-flat hover:border-primary/20"
                      )}
                    >
                      <div className="flex-1 mr-4">
                        <div className="font-medium text-sm flex items-center gap-2">
                          用户设置
                          {settingSources.includes('user') && <span
                            className="text-[10px] px-1.5 py-0.5 bg-primary/10 text-primary rounded font-medium">已启用</span>}
                        </div>
                        <div className="text-xs text-muted-foreground mt-1">
                          读取全局技能和权限设置。
                        </div>
                      </div>
                      <label className="relative inline-flex items-center cursor-pointer">
                        <input
                          type="checkbox"
                          checked={settingSources.includes('user')}
                          onChange={() => toggleSettingSource('user')}
                          className="sr-only peer"
                        />
                        <div
                          className="w-11 h-6 bg-muted rounded-full peer peer-focus:ring-2 peer-focus:ring-primary/20 dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-0.5 after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-primary"></div>
                      </label>
                    </div>

                    {/* 项目设置 */}
                    <div
                      className={cn(
                        "radius-shell-md flex items-center justify-between p-4 transition-all duration-200",
                        settingSources.includes('project')
                          ? "neo-card bg-primary/5"
                          : "neo-card-flat hover:border-primary/20"
                      )}
                    >
                      <div className="flex-1 mr-4">
                        <div className="font-medium text-sm flex items-center gap-2">
                          项目设置
                          {settingSources.includes('project') && <span
                            className="text-[10px] px-1.5 py-0.5 bg-primary/10 text-primary rounded font-medium">已启用</span>}
                        </div>
                        <div className="text-xs text-muted-foreground mt-1">
                          从 workspace 读取。
                        </div>
                      </div>
                      <label className="relative inline-flex items-center cursor-pointer">
                        <input
                          type="checkbox"
                          checked={settingSources.includes('project')}
                          onChange={() => toggleSettingSource('project')}
                          className="sr-only peer"
                        />
                        <div
                          className="w-11 h-6 bg-muted rounded-full peer peer-focus:ring-2 peer-focus:ring-primary/20 dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-0.5 after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-primary"></div>
                      </label>
                    </div>
                  </div>
                </div>
              </div>
            )}

            {/* 高级设置 */}
            {activeTab === 'advanced' && (
              <div className="space-y-8 max-w-2xl animate-in slide-in-from-right-4 duration-300">

              </div>
            )}
          </div>
        </div>

        {/* 底部按钮 */}
        <div className="flex items-center justify-end gap-3 border-t border-white/55 px-6 py-5">
          <button
            onClick={on_close}
            className="neo-pill radius-shell-sm px-5 py-2.5 text-sm font-medium transition-colors hover:text-accent"
          >
            取消
          </button>
          <button
            onClick={handleSave}
            disabled={!canSave}
            className={cn(
              "radius-shell-sm px-5 py-2.5 text-sm font-medium transition-colors",
              canSave
                ? "bg-primary text-primary-foreground shadow-[0_18px_34px_rgba(133,119,255,0.2)] hover:bg-primary/90"
                : "neo-pill text-muted-foreground cursor-not-allowed"
            )}
          >
            {mode === 'create' ? '创建 Agent' : '保存更改'}
          </button>
        </div>
      </div>
    </div>
  );
}
