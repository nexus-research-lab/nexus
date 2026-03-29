/**
 * AgentOptions Skills Tab
 *
 * 技能启用开关 + 设置来源选择
 * 从原 skills tab 拆分而来
 */

"use client";

import { Sparkles } from "lucide-react";
import { cn } from "@/lib/utils";

/** 设置来源类型 */
type SettingSource = "user" | "project" | "local";

/** 来源配置项 */
interface SourceItem {
  key: SettingSource;
  label: string;
  description: string;
}

/** 可选来源列表 */
const SOURCE_ITEMS: SourceItem[] = [
  { key: "user", label: "用户设置", description: "读取全局技能和权限设置。" },
  { key: "project", label: "项目设置", description: "从 workspace 读取。" },
];

interface AgentOptionsSkillsTabProps {
  skillsEnabled: boolean;
  onSkillsEnabledChange: (enabled: boolean) => void;
  settingSources: SettingSource[];
  onToggleSettingSource: (source: SettingSource) => void;
}

/** Skills Tab 组件 — 技能系统配置 */
export function AgentOptionsSkillsTab({
  skillsEnabled,
  onSkillsEnabledChange,
  settingSources,
  onToggleSettingSource,
}: AgentOptionsSkillsTabProps) {
  return (
    <div className="space-y-8 animate-in slide-in-from-right-4 duration-300">
      {/* 技能启用开关 */}
      <div className="space-y-4">
        <h3 className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
          Agent Skills
        </h3>

        <div className="workspace-card radius-shell-md flex items-center justify-between p-4 transition-all hover:border-primary/20">
          <div className="flex-1">
            <label className="flex items-center gap-2 text-sm font-medium leading-none">
              启用技能系统
              {skillsEnabled && (
                <span className="rounded bg-green-500/10 px-1.5 py-0.5 text-[10px] font-medium text-green-600">
                  已启用
                </span>
              )}
            </label>
            <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
              技能是可重用的专业能力模块，Claude
              会根据任务上下文自动调用相关技能。
            </p>
          </div>
          {/* 自定义 Switch */}
          <label className="relative ml-4 inline-flex cursor-pointer items-center">
            <input
              type="checkbox"
              checked={skillsEnabled}
              onChange={(e) => onSkillsEnabledChange(e.target.checked)}
              className="peer sr-only"
            />
            <div className="h-6 w-11 rounded-full bg-muted peer peer-checked:bg-primary peer-focus:ring-2 peer-focus:ring-primary/20 after:absolute after:left-[2px] after:top-0.5 after:h-5 after:w-5 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:after:translate-x-full peer-checked:after:border-white dark:bg-gray-700 dark:border-gray-600" />
          </label>
        </div>
      </div>

      {/* 设置来源选择 */}
      <div className="space-y-4">
        <h3 className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
          设置加载来源
        </h3>

        {/* 提示信息 */}
        <div className="radius-shell-md flex gap-3 border border-orange-500/20 bg-orange-500/10 p-4">
          <div className="mt-0.5 text-orange-600">
            <Sparkles className="h-4 w-4" />
          </div>
          <div>
            <p className="text-sm font-medium text-orange-700">
              来源同时影响技能与权限规则
            </p>
            <p className="mt-1 text-xs leading-relaxed text-orange-600/90">
              Nexus
              会从这些来源读取配置。项目/本地设置里的权限规则只有在对应来源启用后，后续会话才会自动生效。
            </p>
          </div>
        </div>

        {/* 来源列表 */}
        <div className="grid grid-cols-1 gap-3">
          {SOURCE_ITEMS.map((item) => {
            const isEnabled = settingSources.includes(item.key);
            return (
              <div
                key={item.key}
                className={cn(
                  "radius-shell-md flex items-center justify-between p-4 transition-all duration-200",
                  isEnabled
                    ? "workspace-card-strong bg-primary/5"
                    : "workspace-card hover:border-primary/20"
                )}
              >
                <div className="mr-4 flex-1">
                  <div className="flex items-center gap-2 text-sm font-medium">
                    {item.label}
                    {isEnabled && (
                      <span className="rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium text-primary">
                        已启用
                      </span>
                    )}
                  </div>
                  <div className="mt-1 text-xs text-muted-foreground">
                    {item.description}
                  </div>
                </div>
                {/* 自定义 Switch */}
                <label className="relative inline-flex cursor-pointer items-center">
                  <input
                    type="checkbox"
                    checked={isEnabled}
                    onChange={() => onToggleSettingSource(item.key)}
                    className="peer sr-only"
                  />
                  <div className="h-6 w-11 rounded-full bg-muted peer peer-checked:bg-primary peer-focus:ring-2 peer-focus:ring-primary/20 after:absolute after:left-[2px] after:top-0.5 after:h-5 after:w-5 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:after:translate-x-full peer-checked:after:border-white dark:bg-gray-700 dark:border-gray-600" />
                </label>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
