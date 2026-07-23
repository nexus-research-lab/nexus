/**
 * AgentOptions Advanced Tab
 *
 * 权限控制 + 工具授权
 */

"use client";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import {
  AGENT_PERMISSION_MODES,
  AVAILABLE_AGENT_TOOLS,
  countVisibleAgentPreauthorizedTools,
} from "@/lib/agent-options";

interface AgentOptionsAdvancedTabProps {
  permissionMode: string;
  onPermissionModeChange: (mode: string) => void;
  allowedTools: string[];
  onToggleTool: (toolName: string, type: "allowed" | "disallowed") => void;
}

/** Advanced Tab 组件 — 权限控制与工具授权 */
export function AgentOptionsAdvancedTab({
  permissionMode: permissionMode,
  onPermissionModeChange: onPermissionModeChange,
  allowedTools: allowedTools,
  onToggleTool: onToggleTool,
}: AgentOptionsAdvancedTabProps) {
  const { t } = useI18n();
  const isBypassPermissionMode = permissionMode === "bypassPermissions";
  const preauthorizedToolCount = countVisibleAgentPreauthorizedTools(allowedTools);

  return (
    <div className="space-y-7 animate-in slide-in-from-right-4 duration-300 [overflow-anchor:none]">
      {/* 权限模式 */}
      <div className="space-y-4">
        <div className="flex flex-col items-start justify-between gap-2 sm:flex-row sm:items-end sm:gap-5">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-(--text-soft)">
              {t("agent_options.advanced.runtime_policy")}
            </p>
            <h3 className="mt-1 text-[15px] font-semibold text-(--text-strong)">
              {t("agent_options.advanced.permission_control")}
            </h3>
          </div>
          <p className="max-w-[280px] text-left text-xs leading-5 text-(--text-soft) sm:text-right">
            {t("agent_options.advanced.permission_control_hint")}
          </p>
        </div>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {AGENT_PERMISSION_MODES.map((pm) => (
            <UiChoiceButton
              active={permissionMode === pm.value}
              className="relative min-h-[96px] w-full flex-col items-stretch overflow-hidden px-4 py-3.5 text-left"
              choiceSize="lg"
              key={pm.value}
              onClick={() => onPermissionModeChange(pm.value)}
            >
              <div className="mb-1.5 flex items-center justify-between">
                <span className="text-[13.5px] font-semibold">{t(pm.labelKey)}</span>
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
              <p className="text-[12px] leading-[1.55] text-muted-foreground">
                {t(pm.descriptionKey)}
              </p>
            </UiChoiceButton>
          ))}
        </div>

        {/* bypassPermissions 警告 */}
        {isBypassPermissionMode ? (
          <div className="surface-radius-md border border-[color:color-mix(in_srgb,var(--warning)_20%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] px-4 py-3.5 text-[12px] leading-[1.6] text-(--warning)">
            {t("agent_options.advanced.bypass_warning")}
          </div>
        ) : null}
      </div>

      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-(--text-soft)">
              {t("agent_options.advanced.tool_access")}
            </p>
            <h3 className="mt-1 text-[15px] font-semibold text-(--text-strong)">
              {t("agent_options.advanced.tool_access")}
            </h3>
          </div>
          <span className="min-w-[92px] text-right text-[11px] tabular-nums text-(--text-soft)">
            {t("agent_options.advanced.enabled_tools", { count: preauthorizedToolCount })}
          </span>
        </div>

        {/* 安全提示 */}
        <div className="surface-radius-md flex gap-3 border border-[color:color-mix(in_srgb,var(--warning)_20%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] px-4 py-3.5">
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
            <p className="text-[13px] font-medium text-[color-mix(in_srgb,var(--warning)_80%,white)]">{t("agent_options.advanced.security_title")}</p>
            <p className="mt-1 text-[12px] leading-[1.55] text-[color-mix(in_srgb,var(--warning)_70%,white)]">
              {t("agent_options.advanced.security_hint")}
            </p>
          </div>
        </div>

        {/* 工具列表 */}
        <div className="grid grid-cols-1 gap-2.5 [overflow-anchor:none]">
          {AVAILABLE_AGENT_TOOLS.map((tool) => {
            const isChecked = allowedTools.includes(tool.name);
            return (
              <div
                key={tool.name}
                className={cn(
                  "surface-radius-md flex min-h-[72px] items-center justify-between gap-4 border px-4 py-3 transition-[background,border-color] duration-(--motion-duration-fast)",
                  isChecked
                    ? "border-[color-mix(in_srgb,var(--primary)_20%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_5%,transparent)]"
                    : "border-(--divider-subtle-color) bg-transparent hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)"
                )}
              >
                <div className="min-w-0 flex-1">
                  <div className="text-[13.5px] font-semibold leading-[1.35]">{tool.name}</div>
                  <div className="mt-1 text-[12px] leading-[1.5] text-muted-foreground">
                    {t(tool.descriptionKey)}
                  </div>
                </div>
                <div className="flex h-8 w-[64px] shrink-0 items-center justify-end">
                  <GlassSwitch
                    checked={isChecked}
                    onChange={() => onToggleTool(tool.name, "allowed")}
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
