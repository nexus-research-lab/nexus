/**
 * AgentOptions Persona Tab
 *
 * System Prompt / Instructions 编辑器
 * 从原 prompt tab 拆分而来
 */

"use client";

interface AgentOptionsPersonaTabProps {
  systemPrompt: string;
  onSystemPromptChange: (value: string) => void;
}

/** Persona Tab 组件 — 系统提示词编辑 */
export function AgentOptionsPersonaTab({
  systemPrompt,
  onSystemPromptChange,
}: AgentOptionsPersonaTabProps) {
  return (
    <div className="flex h-full flex-col space-y-5 animate-in slide-in-from-right-4 duration-300">
      {/* 系统提示词编辑器 */}
      <div className="flex flex-1 flex-col space-y-2">
        <label className="flex items-center justify-between text-[11px] font-semibold text-[color:var(--text-muted)]">
          <span>系统提示词 (System Prompt)</span>
          <span className="font-normal text-[color:var(--text-soft)]">
            支持 Markdown
          </span>
        </label>
        <div className="relative flex-1">
          <textarea
            value={systemPrompt}
            onChange={(e) => onSystemPromptChange(e.target.value)}
            className="dialog-input absolute inset-0 h-full w-full resize-none rounded-2xl px-3.5 py-3 text-sm font-mono leading-relaxed text-[color:var(--text-strong)] placeholder:text-[color:var(--text-soft)] focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-[var(--disabled-opacity)]"
            placeholder="在此输入自定义系统提示词，它将决定 Agent 的行为模式、角色设定和限制条件..."
          />
        </div>
        <p className="text-xs text-muted-foreground">
          💡 提示：自定义系统提示词将覆盖默认的 Agent 设定。
        </p>
      </div>
    </div>
  );
}
