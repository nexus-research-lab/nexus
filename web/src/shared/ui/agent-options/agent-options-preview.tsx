/**
 * AgentOptions Live Preview 面板
 *
 * 右侧实时预览 Agent 卡片效果
 * 反映 Identity Tab 中编辑的名称、描述、头像、标签等
 */

"use client";

import { User, Cpu, Wrench } from "lucide-react";
import { AVAILABLE_MODELS } from "./agent-options-constants";

interface AgentOptionsPreviewProps {
  title: string;
  description: string;
  vibeTags: string[];
  model: string;
  skillsEnabled: boolean;
}

/** 右侧 Live Preview 面板组件 */
export function AgentOptionsPreview({
  title,
  description,
  vibeTags,
  model,
  skillsEnabled,
}: AgentOptionsPreviewProps) {
  /** 获取模型显示名称 */
  const modelLabel =
    AVAILABLE_MODELS.find((m) => m.value === model)?.label ?? model;

  return (
    <div className="flex w-80 flex-col border-l border-white/22 bg-white/6 p-4">
      <h4 className="mb-4 text-xs font-medium uppercase tracking-wider text-muted-foreground">
        Live Preview
      </h4>

      {/* Agent 卡片预览 */}
      <div className="workspace-card radius-shell-md flex flex-col items-center gap-3 p-6">
        {/* 头像 */}
        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-primary/10 text-primary">
          <User className="h-8 w-8" />
        </div>

        {/* 名称 */}
        <h3 className="text-center text-base font-semibold text-foreground">
          {title || "Untitled Agent"}
        </h3>

        {/* 描述 */}
        {description ? (
          <p className="line-clamp-3 text-center text-xs leading-relaxed text-muted-foreground">
            {description}
          </p>
        ) : (
          <p className="text-center text-xs italic text-muted-foreground/50">
            暂无描述
          </p>
        )}

        {/* Vibe Tags */}
        {vibeTags.length > 0 && (
          <div className="flex flex-wrap justify-center gap-1.5">
            {vibeTags.map((tag) => (
              <span
                key={tag}
                className="rounded-full bg-primary/10 px-2 py-0.5 text-[10px] font-medium text-primary"
              >
                {tag}
              </span>
            ))}
          </div>
        )}

        {/* 分隔线 */}
        <div className="my-1 w-full border-t border-white/30" />

        {/* 元信息 */}
        <div className="w-full space-y-2 text-xs text-muted-foreground">
          <div className="flex items-center gap-2">
            <Cpu className="h-3.5 w-3.5 shrink-0" />
            <span className="truncate">Model: {modelLabel}</span>
          </div>
          <div className="flex items-center gap-2">
            <Wrench className="h-3.5 w-3.5 shrink-0" />
            <span>
              Skills: {skillsEnabled ? "enabled" : "disabled"}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
