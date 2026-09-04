// INPUT: Agent 标签集合、复合字段变体、草稿重置键与集合更新命令。
// OUTPUT: 使用共享可移除 Chip 和图标动作的标签输入组合。
// POS: Agent identity 领域表单组合；不拥有标签持久化或跨字段校验。

import { useCallback, useId, type KeyboardEvent } from "react";
import { Plus } from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { cn } from "@/shared/ui/class-name";
import { UiIconButton } from "@/shared/ui/button/button";
import { UiRemovableChip } from "@/shared/ui/form/removable-chip";

import {
  IDENTITY_FIELD_LABEL_CLASS_NAMES,
  type AgentIdentityVariant,
} from "./identity-layout";

interface IdentityTagsProps {
  addLabel: string;
  label: string;
  onChange: (tags: string[]) => void;
  resetKey: string;
  tags: string[];
  variant: AgentIdentityVariant;
}

export function IdentityTags({
  addLabel,
  label,
  onChange,
  resetKey,
  tags,
  variant,
}: IdentityTagsProps) {
  const [tagInput, setTagInput] = useResettableState("", resetKey);
  const inputId = useId();
  const labelClassName = IDENTITY_FIELD_LABEL_CLASS_NAMES[variant];

  const addTag = useCallback(() => {
    const normalizedTag = tagInput.trim();
    if (normalizedTag && !tags.includes(normalizedTag)) {
      onChange([...tags, normalizedTag]);
    }
    setTagInput("");
  }, [onChange, setTagInput, tagInput, tags]);

  const handleKeyDown = useCallback((event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== "Enter") {
      return;
    }
    event.preventDefault();
    addTag();
  }, [addTag]);

  return (
    <div className="space-y-2">
      <label className={labelClassName} htmlFor={inputId}>
        {label}
      </label>
      <div
        className={cn(
          "dialog-input flex w-full min-w-0 items-center gap-1.5 pl-2 pr-1",
          variant === "dialog" ? "min-h-10 py-1.5" : "min-h-9 py-1",
        )}
      >
        <div className="flex min-w-0 flex-1 items-center gap-1.5 overflow-x-auto overscroll-x-contain [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
          {tags.map((tag) => (
            <UiRemovableChip
              key={tag}
              onRemove={() => onChange(tags.filter((item) => item !== tag))}
              removeLabel={`移除 ${tag}`}
            >
              {tag}
            </UiRemovableChip>
          ))}
          <input
            className="h-7 min-w-[120px] flex-1 bg-transparent px-1 text-sm text-(--text-strong) outline-none placeholder:text-(--text-soft)"
            id={inputId}
            onChange={(event) => setTagInput(event.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={addLabel}
            type="text"
            value={tagInput}
          />
        </div>
        <UiIconButton
          aria-label={addLabel}
          className="shrink-0"
          onClick={addTag}
          size="sm"
          type="button"
          variant="ghost"
        >
          <Plus className="h-3.5 w-3.5" />
        </UiIconButton>
      </div>
    </div>
  );
}
