/**
 * AgentOptions Identity Tab
 *
 * 包含 Avatar、Name、Description、Vibe Tags、Model Provider
 * 从原 basic tab 拆分并增强
 */

"use client";

import { useState, useCallback } from "react";
import { Plus, X as XIcon, User } from "lucide-react";
import { cn } from "@/lib/utils";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import type { AgentNameValidationResult } from "@/types/agent";
import { AVAILABLE_MODELS } from "./agent-options-constants";

interface AgentOptionsIdentityTabProps {
  title: string;
  onTitleChange: (value: string) => void;
  description: string;
  onDescriptionChange: (value: string) => void;
  vibeTags: string[];
  onVibeTagsChange: (tags: string[]) => void;
  model: string;
  onModelChange: (value: string) => void;
  nameValidation: AgentNameValidationResult | null;
  isValidatingName: boolean;
}

/** Identity Tab 组件 */
export function AgentOptionsIdentityTab({
  title,
  onTitleChange,
  description,
  onDescriptionChange,
  vibeTags,
  onVibeTagsChange,
  model,
  onModelChange,
  nameValidation,
  isValidatingName,
}: AgentOptionsIdentityTabProps) {
  const [tagInput, setTagInput] = useState("");

  /** 添加标签 */
  const handleAddTag = useCallback(() => {
    const trimmed = tagInput.trim();
    if (trimmed && !vibeTags.includes(trimmed)) {
      onVibeTagsChange([...vibeTags, trimmed]);
    }
    setTagInput("");
  }, [tagInput, vibeTags, onVibeTagsChange]);

  /** 按回车添加标签 */
  const handleTagKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLInputElement>) => {
      if (e.key === "Enter") {
        e.preventDefault();
        handleAddTag();
      }
    },
    [handleAddTag]
  );

  /** 删除标签 */
  const handleRemoveTag = useCallback(
    (tag: string) => {
      onVibeTagsChange(vibeTags.filter((t) => t !== tag));
    },
    [vibeTags, onVibeTagsChange]
  );

  return (
    <div className="space-y-6 animate-in slide-in-from-right-4 duration-300">
      {/* Avatar 区域 */}
      <div className="space-y-2">
        <label className="text-[11px] font-semibold text-slate-600">Avatar</label>
        <div className="flex items-center gap-4">
          <div className="flex h-16 w-16 items-center justify-center rounded-full bg-primary/10 text-primary">
            <User className="h-8 w-8" />
          </div>
          <div className="flex gap-2">
            <WorkspacePillButton size="sm" variant="tonal">
              Upload
            </WorkspacePillButton>
            <WorkspacePillButton size="sm" variant="tonal">
              Remove
            </WorkspacePillButton>
          </div>
        </div>
      </div>

      {/* Name 输入 */}
      <div className="space-y-2">
        <label className="text-[11px] font-semibold text-slate-600">
          Name <span className="text-red-500">*</span>
        </label>
        <input
          type="text"
          value={title}
          onChange={(e) => onTitleChange(e.target.value)}
          className="dialog-input rounded-xl flex h-11 w-full px-4 py-2 text-sm text-slate-800 placeholder:text-slate-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 transition-all"
          placeholder="例如：Coding Assistant"
        />
        {/* 名称校验反馈 */}
        <div className="min-h-5 text-xs">
          {isValidatingName && (
            <span className="text-muted-foreground">正在校验名称...</span>
          )}
          {!isValidatingName && nameValidation?.reason && (
            <span className="text-red-500">{nameValidation.reason}</span>
          )}
          {!isValidatingName &&
            nameValidation?.is_valid &&
            nameValidation?.is_available && (
              <span className="text-emerald-600">
                名称可用，工作区将自动创建到：{nameValidation.workspace_path}
              </span>
            )}
        </div>
      </div>

      {/* Description 文本域 */}
      <div className="space-y-2">
        <label className="text-[11px] font-semibold text-slate-600">Description</label>
        <textarea
          value={description}
          onChange={(e) => onDescriptionChange(e.target.value)}
          className="dialog-input rounded-2xl flex min-h-[80px] w-full resize-y px-4 py-3 text-sm text-slate-800 placeholder:text-slate-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 transition-all"
          rows={3}
          placeholder="描述此 Agent 的目标或背景信息..."
        />
      </div>

      {/* Vibe Tags 标签输入 */}
      <div className="space-y-2">
        <label className="text-[11px] font-semibold text-slate-600">Vibe Tags</label>
        <div className="flex flex-wrap items-center gap-2">
          {vibeTags.map((tag) => (
            <span
              key={tag}
              className={cn(
                "inline-flex items-center gap-1 rounded-full bg-primary/10 px-2.5 py-1 text-xs font-medium text-primary"
              )}
            >
              {tag}
              <button
                type="button"
                onClick={() => handleRemoveTag(tag)}
                className="ml-0.5 rounded-full p-0.5 transition-colors hover:bg-primary/20"
              >
                <XIcon className="h-3 w-3" />
              </button>
            </span>
          ))}
          {/* 添加标签输入 */}
          <div className="flex items-center gap-1">
            <input
              type="text"
              value={tagInput}
              onChange={(e) => setTagInput(e.target.value)}
              onKeyDown={handleTagKeyDown}
              className="dialog-input rounded-lg h-7 w-24 px-2 text-xs text-slate-800 placeholder:text-slate-400 focus-visible:outline-none transition-all"
              placeholder="添加标签"
            />
            <button
              type="button"
              onClick={handleAddTag}
              className="flex h-7 w-7 items-center justify-center rounded-md text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
            >
              <Plus className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      </div>

      {/* Model Provider 下拉 */}
      <div className="space-y-2">
        <label className="text-[11px] font-semibold text-slate-600">
          Model Provider <span className="text-red-500">*</span>
        </label>
        <div className="relative">
          <select
            value={model}
            onChange={(e) => onModelChange(e.target.value)}
            className="dialog-input rounded-xl flex h-11 w-full appearance-none px-4 py-2 text-sm text-slate-800 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 transition-all"
          >
            {AVAILABLE_MODELS.map((m) => (
              <option key={m.value} value={m.value}>
                {m.label}
              </option>
            ))}
          </select>
          {/* 下拉箭头 */}
          <div className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground">
            <svg
              width="10"
              height="6"
              viewBox="0 0 10 6"
              fill="none"
              xmlns="http://www.w3.org/2000/svg"
            >
              <path
                d="M1 1L5 5L9 1"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </div>
        </div>
      </div>
    </div>
  );
}
