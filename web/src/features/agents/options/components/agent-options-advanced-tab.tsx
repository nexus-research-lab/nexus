/**
 * AgentOptions Advanced Tab
 *
 * 权限控制 + 工具授权
 */

"use client";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { GlassSwitch } from "@/shared/ui/liquid-glass";

/** 权限模式选项 */
const PERMISSION_MODES: ReadonlyArray<{
  value: string;
  label_key: TranslationKey;
  description_key: TranslationKey;
}> = [
  {
    value: "default",
    label_key: "agent_options.advanced.permission.default.label",
    description_key: "agent_options.advanced.permission.default.description",
  },
  {
    value: "plan",
    label_key: "agent_options.advanced.permission.plan.label",
    description_key: "agent_options.advanced.permission.plan.description",
  },
  {
    value: "acceptEdits",
    label_key: "agent_options.advanced.permission.accept_edits.label",
    description_key: "agent_options.advanced.permission.accept_edits.description",
  },
  {
    value: "bypassPermissions",
    label_key: "agent_options.advanced.permission.bypass.label",
    description_key: "agent_options.advanced.permission.bypass.description",
  },
] as const;

/** 常用工具列表 */
const AVAILABLE_TOOLS: ReadonlyArray<{
  name: string;
  description_key: TranslationKey;
}> = [
    { name: "Task", description_key: "agent_options.advanced.tool.task" },
    { name: "TaskOutput", description_key: "agent_options.advanced.tool.task_output" },
    { name: "Bash", description_key: "agent_options.advanced.tool.bash" },
    { name: "Glob", description_key: "agent_options.advanced.tool.glob" },
    { name: "Grep", description_key: "agent_options.advanced.tool.grep" },
    { name: "ExitPlanMode", description_key: "agent_options.advanced.tool.exit_plan_mode" },
    { name: "Read", description_key: "agent_options.advanced.tool.read" },
    { name: "Edit", description_key: "agent_options.advanced.tool.edit" },
    { name: "Write", description_key: "agent_options.advanced.tool.write" },
    { name: "NotebookEdit", description_key: "agent_options.advanced.tool.notebook_edit" },
    { name: "WebFetch", description_key: "agent_options.advanced.tool.web_fetch" },
    { name: "TodoWrite", description_key: "agent_options.advanced.tool.todo_write" },
    {
      name: "WebSearch",
      description_key: "agent_options.advanced.tool.web_search",
    },
    { name: "KillShell", description_key: "agent_options.advanced.tool.kill_shell" },
    { name: "AskUserQuestion", description_key: "agent_options.advanced.tool.ask_user_question" },
    { name: "Skill", description_key: "agent_options.advanced.tool.skill" },
    { name: "EnterPlanMode", description_key: "agent_options.advanced.tool.enter_plan_mode" },
  ];

interface AgentOptionsAdvancedTabProps {
  permission_mode: string;
  on_permission_mode_change: (mode: string) => void;
  allowed_tools: string[];
  on_toggle_tool: (tool_name: string, type: "allowed" | "disallowed") => void;
}

/** Advanced Tab 组件 — 权限控制与工具授权 */
export function AgentOptionsAdvancedTab({
  permission_mode,
  on_permission_mode_change,
  allowed_tools,
  on_toggle_tool,
}: AgentOptionsAdvancedTabProps) {
  const { t } = useI18n();

  return (
    <div className="space-y-4 animate-in slide-in-from-right-4 duration-300">
      {/* 权限模式 */}
      <div className="space-y-2.5">
        <div className="flex items-end justify-between gap-4">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-(--text-soft)">
              {t("agent_options.advanced.runtime_policy")}
            </p>
            <h3 className="mt-1 text-[15px] font-semibold text-(--text-strong)">
              {t("agent_options.advanced.permission_control")}
            </h3>
          </div>
          <p className="max-w-[240px] text-right text-xs leading-5 text-(--text-soft)">
            {t("agent_options.advanced.permission_control_hint")}
          </p>
        </div>
        <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
          {PERMISSION_MODES.map((pm) => (
            <button
              key={pm.value}
              onClick={() => on_permission_mode_change(pm.value)}
              className={cn(
                "relative overflow-hidden rounded-[15px] border px-3 py-2.5 text-left transition-[background,border-color,color] duration-(--motion-duration-normal)",
                permission_mode === pm.value
                  ? "border-[color:color-mix(in_srgb,var(--primary)_24%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_6%,transparent)] text-(--text-strong)"
                  : "border-(--divider-subtle-color) bg-transparent text-(--text-strong) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)"
              )}
            >
              <div className="mb-1 flex items-center justify-between">
                <span className="text-[13px] font-semibold">{t(pm.label_key)}</span>
                {permission_mode === pm.value && (
                  <div className="flex h-4 w-4 items-center justify-center rounded-full bg-primary">
                    <svg
                      width="10"
                      height="8"
                      viewBox="0 0 10 8"
                      fill="none"
                      xmlns="http://www.w3.org/2000/svg"
                    >
                      <path
                        d="M1 4L3.5 6.5L9 1"
                        stroke="white"
                        strokeWidth="1.5"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </div>
                )}
              </div>
              <p className="text-[11.5px] leading-[1.5] text-muted-foreground">
                {t(pm.description_key)}
              </p>
            </button>
          ))}
        </div>

        {/* bypassPermissions 警告 */}
        {permission_mode === "bypassPermissions" &&
          allowed_tools.length > 0 && (
            <div className="rounded-[15px] border border-[color:color-mix(in_srgb,var(--warning)_20%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] px-3.5 py-3 text-[11.5px] leading-[1.55] text-(--warning)">
              {t("agent_options.advanced.bypass_warning")}
            </div>
          )}
      </div>

      <div className="space-y-2.5">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-(--text-soft)">
              {t("agent_options.advanced.tool_access")}
            </p>
            <h3 className="mt-1 text-[15px] font-semibold text-(--text-strong)">
              {t("agent_options.advanced.tool_access")}
            </h3>
          </div>
          <span className="text-[11px] text-(--text-soft)">
            {t("agent_options.advanced.enabled_tools", { count: allowed_tools.length })}
          </span>
        </div>

        {/* 安全提示 */}
        <div className="flex gap-2.5 rounded-[15px] border border-[color-mix(in_srgb,var(--warning)_20%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] px-3 py-2.5">
          <div className="mt-0.5 text-(--warning)">
            <svg
              width="15"
              height="15"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
              <line x1="12" y1="9" x2="12" y2="13" />
              <line x1="12" y1="17" x2="12.01" y2="17" />
            </svg>
          </div>
          <div>
            <p className="text-[12.5px] font-medium text-[color-mix(in_srgb,var(--warning)_80%,white)]">{t("agent_options.advanced.security_title")}</p>
            <p className="mt-0.5 text-[11.5px] leading-normal text-[color-mix(in_srgb,var(--warning)_70%,white)]">
              {t("agent_options.advanced.security_hint")}
            </p>
          </div>
        </div>

        {/* 工具列表 */}
        <div className="grid grid-cols-1 gap-1.5">
          {AVAILABLE_TOOLS.map((tool) => {
            const isChecked = allowed_tools.includes(tool.name);
            return (
              <div
                key={tool.name}
                className={cn(
                  "flex items-center justify-between gap-3 rounded-[15px] border px-3 py-2.5 transition-[background,border-color] duration-(--motion-duration-fast)",
                  isChecked
                    ? "border-[color-mix(in_srgb,var(--primary)_20%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_5%,transparent)]"
                    : "border-(--divider-subtle-color) bg-transparent hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)"
                )}
              >
                <div className="min-w-0 flex-1">
                  <div className="text-[12.5px] font-semibold leading-[1.35]">{tool.name}</div>
                  <div className="mt-0.5 text-[11.5px] leading-[1.45] text-muted-foreground">
                    {t(tool.description_key)}
                  </div>
                </div>
                <div className="origin-right scale-[0.84]">
                  <GlassSwitch
                    checked={isChecked}
                    on_change={() => on_toggle_tool(tool.name, "allowed")}
                  />
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
