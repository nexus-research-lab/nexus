import { useCallback, type KeyboardEvent } from "react";
import { Plus, X } from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { cn } from "@/shared/ui/class-name";
import { UiIconButton } from "@/shared/ui/button/button";
import { UiInput } from "@/shared/ui/form/form-control";

import type { AgentIdentityVariant } from "./identity-layout";

interface VibeTagLayout {
  addButtonSize: "md" | "sm";
  inputClassName: string;
  inputSize: "sm" | "xs";
  labelClassName: string;
  rowGapClassName: string;
}

const VIBE_TAG_LAYOUTS: Record<AgentIdentityVariant, VibeTagLayout> = {
  dialog: {
    addButtonSize: "md",
    inputClassName: "w-[132px] rounded-lg",
    inputSize: "sm",
    labelClassName: "text-[11px] font-semibold text-(--text-muted)",
    rowGapClassName: "gap-2",
  },
  inline: {
    addButtonSize: "md",
    inputClassName: "w-[112px] rounded-full",
    inputSize: "sm",
    labelClassName:
      "text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)",
    rowGapClassName: "gap-2",
  },
};

interface IdentityVibeTagsProps {
  addLabel: string;
  label: string;
  onChange: (tags: string[]) => void;
  resetKey: string;
  tags: string[];
  variant: AgentIdentityVariant;
}

export function IdentityVibeTags({
  addLabel,
  label,
  onChange,
  resetKey,
  tags,
  variant,
}: IdentityVibeTagsProps) {
  const [tagInput, setTagInput] = useResettableState("", resetKey);
  const layout = VIBE_TAG_LAYOUTS[variant];

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
    <div className="space-y-3">
      <label className={layout.labelClassName}>{label}</label>
      <div className="flex flex-wrap items-center gap-2">
        {tags.map((tag) => (
          <span
            className="inline-flex min-h-8 shrink-0 items-center gap-1 rounded-[8px] border border-[color:color-mix(in_srgb,var(--primary)_16%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_4%,transparent)] px-2.5 py-1 text-[12px] font-medium text-primary"
            key={tag}
          >
            {tag}
            <UiIconButton
              aria-label={`移除 ${tag}`}
              className="ml-0.5 h-6 w-6 rounded-full"
              onClick={() => onChange(tags.filter((item) => item !== tag))}
              size="xs"
              type="button"
              variant="ghost"
            >
              <X className="h-3 w-3" />
            </UiIconButton>
          </span>
        ))}
        <div className={cn("flex shrink-0 items-center", layout.rowGapClassName)}>
          <UiInput
            className={layout.inputClassName}
            controlSize={layout.inputSize}
            onChange={(event) => setTagInput(event.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={addLabel}
            type="text"
            value={tagInput}
          />
          <UiIconButton
            aria-label={addLabel}
            onClick={addTag}
            size={layout.addButtonSize}
            type="button"
            variant="ghost"
          >
            <Plus className="h-3.5 w-3.5" />
          </UiIconButton>
        </div>
      </div>
    </div>
  );
}
