// INPUT: Agent 标签集合、草稿重置键、本地化文案与集合更新命令。
// OUTPUT: 单一 Field 中可横向滚动的 Chip、IME 安全添加与移除后的输入焦点。
// POS: Agent 标签复合字段；独占草稿/去重，不拥有持久化或跨字段校验。

import { useCallback, useId, useRef, type KeyboardEvent } from "react";
import { Plus } from "lucide-react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiIconButton } from "@/shared/ui/button/button";
import { UiRemovableChip } from "@/shared/ui/form/removable-chip";
import { UiField } from "@/shared/ui/form/form-control";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface IdentityTagsProps {
  addLabel: string;
  label: string;
  onChange: (tags: string[]) => void;
  resetKey: string;
  tags: string[];
}

export function IdentityTags({
  addLabel,
  label,
  onChange,
  resetKey,
  tags,
}: IdentityTagsProps) {
  const { t } = useI18n();
  const [tagInput, setTagInput] = useResettableState("", resetKey);
  const inputId = useId();
  const inputRef = useRef<HTMLInputElement>(null);
  const canAdd = Boolean(tagInput.trim() && !tags.includes(tagInput.trim()));

  const addTag = useCallback(() => {
    const normalizedTag = tagInput.trim();
    if (normalizedTag && !tags.includes(normalizedTag)) {
      onChange([...tags, normalizedTag]);
    }
    setTagInput("");
    inputRef.current?.focus();
  }, [onChange, setTagInput, tagInput, tags]);

  const handleKeyDown = useCallback((event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== "Enter" || isImeKeyboardEvent(event.nativeEvent)) {
      return;
    }
    event.preventDefault();
    addTag();
  }, [addTag]);

  return (
    <UiField htmlFor={inputId} label={label}>
      <div
        className="dialog-input flex h-9 w-full min-w-0 items-center gap-1.5 pl-2 pr-1"
      >
        <div className="flex min-w-0 flex-1 items-center gap-1.5 overflow-x-auto overscroll-x-contain [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
          {tags.map((tag) => (
            <UiRemovableChip
              key={tag}
              onRemove={() => {
                onChange(tags.filter((item) => item !== tag));
                inputRef.current?.focus();
              }}
              removeLabel={t("agent_options.identity.remove_tag", { tag })}
            >
              {tag}
            </UiRemovableChip>
          ))}
          <input
            className={cn(
              "h-7 min-w-[120px] flex-1 bg-transparent px-1 outline-none placeholder:text-(--text-muted)",
              getUiTypographyClassName({ role: "control", tone: "strong" }),
            )}
            id={inputId}
            ref={inputRef}
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
          disabled={!canAdd}
          onClick={addTag}
          size="sm"
          type="button"
          variant="ghost"
        >
          <Plus className="h-3.5 w-3.5" />
        </UiIconButton>
      </div>
    </UiField>
  );
}
