/**
 * AgentOptions Advanced Tab
 *
 * 权限控制 + 工具授权
 */

"use client";

import { cn } from "@/lib/utils";
import { GlassSwitch } from "@/shared/ui/liquid-glass";

/** 权限模式选项 */
const PERMISSION_MODES = [
  {
    value: "default",
    label: "默认（继续前询问）",
    description: "只读工具会自动预先授权，其它操作仍需权限。",
  },
  {
    value: "plan",
    label: "规划模式",
    description: "继承默认的只读工具集，并会在执行行为前呈现计划。",
  },
  {
    value: "acceptEdits",
    label: "自动授权文件编辑",
    description: "默认的只读工具会自动预先授权，但执行仍被禁用。",
  },
  {
    value: "bypassPermissions",
    label: "跳过所有权限检查",
    description: "所有工具都会在无审批情况下执行。",
  },
] as const;

/** 常用工具列表 */
const AVAILABLE_TOOLS = [
  { name: "Task", description: "Executes tasks" },
  { name: "TaskOutput", description: "Displays task output" },
  { name: "Bash", description: "Executes shell commands in your environment" },
  { name: "Glob", description: "Matches file names and patterns" },
  { name: "Grep", description: "Searches for patterns in files" },
  { name: "ExitPlanMode", description: "Exits planning mode" },
  { name: "Read", description: "Reads files" },
  { name: "Edit", description: "Edits files" },
  { name: "Write", description: "Creates or overwrites files" },
  { name: "NotebookEdit", description: "Edits Jupyter Notebooks" },
  { name: "WebFetch", description: "Fetches web pages" },
  { name: "TodoWrite", description: "Creates or updates to-do lists" },
  {
    name: "WebSearch",
    description: "Performs web searches with domain filtering",
  },
  { name: "KillShell", description: "Kills the shell process" },
  { name: "AskUserQuestion", description: "Asks the user a question" },
  { name: "Skill", description: "Executes a skill" },
  { name: "EnterPlanMode", description: "Enters planning mode" },
];

interface AgentOptionsAdvancedTabProps {
  permissionMode: string;
  onPermissionModeChange: (mode: string) => void;
  allowedTools: string[];
  onToggleTool: (toolName: string, type: "allowed" | "disallowed") => void;
}

/** Advanced Tab 组件 — 权限控制与工具授权 */
export function AgentOptionsAdvancedTab({
  permissionMode,
  onPermissionModeChange,
  allowedTools,
  onToggleTool,
}: AgentOptionsAdvancedTabProps) {
  return (
    <div className="space-y-6 animate-in slide-in-from-right-4 duration-300">
      {/* 权限模式 */}
      <div className="space-y-3.5">
        <div className="flex items-end justify-between gap-4">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-(--text-soft)">
              Runtime Policy
            </p>
            <h3 className="mt-1 text-[15px] font-semibold text-(--text-strong)">
              权限控制
            </h3>
          </div>
          <p className="max-w-[240px] text-right text-xs leading-5 text-(--text-soft)">
            这里决定 Agent 在执行命令、编辑文件和联网操作时的审批边界。
          </p>
        </div>
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {PERMISSION_MODES.map((pm) => (
            <button
              key={pm.value}
              onClick={() => onPermissionModeChange(pm.value)}
              className={cn(
                "radius-shell-md relative overflow-hidden p-3.5 text-left transition-[background,border-color,color] duration-(--motion-duration-normal)",
                permissionMode === pm.value
                  ? "dialog-card-active"
                  : "dialog-card hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)"
              )}
            >
              <div className="mb-2 flex items-center justify-between">
                <span className="text-[13px] font-semibold">{pm.label}</span>
                {permissionMode === pm.value && (
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
              <p className="text-[12px] leading-5 text-muted-foreground">
                {pm.description}
              </p>
            </button>
          ))}
        </div>

        {/* bypassPermissions 警告 */}
        {permissionMode === "bypassPermissions" &&
          allowedTools.length > 0 && (
            <div className="radius-shell-md border border-[color:color-mix(in_srgb,var(--warning)_20%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] p-4 text-xs leading-relaxed text-(--warning)">
              `bypassPermissions` 会放行所有工具，`allowed_tools`
              只代表预授权集合，并不能限制其它工具。
              如果你想在全放行模式下屏蔽个别危险工具，请改用
              `disallowed_tools`。
            </div>
          )}
      </div>

      <div className="space-y-3.5">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-(--text-soft)">
              Tool Access
            </p>
            <h3 className="mt-1 text-[15px] font-semibold text-(--text-strong)">
              工具预授权
            </h3>
          </div>
          <span className="text-[11px] text-(--text-soft)">
            已启用 {allowedTools.length} 个工具
          </span>
        </div>

        {/* 安全提示 */}
        <div className="radius-shell-md flex gap-3 border border-[color:color-mix(in_srgb,var(--warning)_20%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] p-3.5">
          <div className="mt-0.5 text-(--warning)">
            <svg
              width="16"
              height="16"
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
            <p className="text-sm font-medium text-[color:color-mix(in_srgb,var(--warning)_80%,white)]">安全提示</p>
            <p className="mt-1 text-xs leading-relaxed text-[color:color-mix(in_srgb,var(--warning)_70%,white)]">
              被选中的工具将被`预先授权`，Agent
              调用这些工具时将不会请求您的确认。请仅为您完全信任的工具开启此选项。
            </p>
          </div>
        </div>

        {/* 工具列表 */}
        <div className="grid grid-cols-1 gap-2.5">
          {AVAILABLE_TOOLS.map((tool) => {
            const isChecked = allowedTools.includes(tool.name);
            return (
              <div
                key={tool.name}
                className="radius-shell-md flex items-center justify-between gap-4 border border-(--divider-subtle-color) bg-(--surface-card-background) p-3.5"
              >
                <div className="mr-4 flex-1">
                  <div className="text-[13px] font-semibold">{tool.name}</div>
                  <div className="mt-1 text-[12px] leading-5 text-muted-foreground">
                    {tool.description}
                  </div>
                </div>
                <GlassSwitch
                  checked={isChecked}
                  on_change={() => onToggleTool(tool.name, "allowed")}
                />
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
