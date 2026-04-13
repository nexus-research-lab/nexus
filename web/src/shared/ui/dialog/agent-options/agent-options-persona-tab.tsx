/**
 * AgentOptions Persona Tab
 *
 * System Prompt / Instructions 编辑器
 * 从原 prompt tab 拆分而来
 */

"use client";

import { useI18n } from "@/shared/i18n/i18n-context";

interface AgentOptionsPersonaTabProps {
  systemPrompt: string;
  onSystemPromptChange: (value: string) => void;
}

/** Persona Tab 组件 — 系统提示词编辑 */
export function AgentOptionsPersonaTab({
  systemPrompt,
  onSystemPromptChange,
}: AgentOptionsPersonaTabProps) {
  const { t } = useI18n();

  return (
    <div className="flex h-full flex-col space-y-5 animate-in slide-in-from-right-4 duration-300">
      {/* 系统提示词编辑器 */}
      <div className="flex flex-1 flex-col space-y-2">
        <label className="flex items-center justify-between text-[11px] font-semibold text-(--text-muted)">
          <span>{t("agent_options.persona.system_prompt")}</span>
          <span className="font-normal text-(--text-soft)">
            {t("agent_options.persona.supports_markdown")}
          </span>
        </label>
        <div className="relative flex-1">
          <textarea
            value={systemPrompt}
            onChange={(e) => onSystemPromptChange(e.target.value)}
            className="dialog-input absolute inset-0 h-full w-full resize-none rounded-2xl px-3.5 py-3 text-sm font-mono leading-relaxed text-(--text-strong) placeholder:text-(--text-soft) focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
            placeholder={t("agent_options.persona.placeholder")}
          />
        </div>
        <p className="text-xs text-muted-foreground">
          {t("agent_options.persona.hint")}
        </p>
      </div>
    </div>
  );
}
