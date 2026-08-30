import { useCallback, type KeyboardEvent } from "react";
import { Plus, X } from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { cn } from "@/shared/ui/class-name";
import { UiIconButton } from "@/shared/ui/button/button";
import { UiInput } from "@/shared/ui/form/form-control";

import {
  IDENTITY_FIELD_LABEL_CLASS_NAMES,
  type AgentIdentityVariant,
} from "./identity-layout";

interface TagLayout {
  addButtonSize: "lg" | "md";
  inputClassName: string;
  inputSize: "md" | "sm";
  rowGapClassName: string;
}

const TAG_LAYOUTS: Record<AgentIdentityVariant, TagLayout> = {
  dialog: {
    addButtonSize: "lg",
    inputClassName: "h-10 min-w-[132px] flex-1 radius-control-md",
    inputSize: "md",
    rowGapClassName: "gap-2",
  },
  inline: {
    addButtonSize: "lg",
    inputClassName: "min-w-[128px] flex-1",
    inputSize: "md",
    rowGapClassName: "gap-1.5",
  },
};

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
  const layout = TAG_LAYOUTS[variant];
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
      <label className={labelClassName}>{label}</label>
      <div
        className={cn(
          "flex min-w-0 items-center",
          layout.rowGapClassName,
        )}
      >
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
      <div
        aria-label={label}
        className="h-7 min-w-0 overflow-x-auto overscroll-x-contain [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
      >
        <div className="flex h-7 min-w-max items-center gap-2">
          {tags.map((tag) => (
            <span
              className="chip-default inline-flex h-7 shrink-0 items-center gap-1 px-2 text-compact font-medium text-(--text-default)"
              key={tag}
            >
              {tag}
              <UiIconButton
                aria-label={`移除 ${tag}`}
                className="-mr-1 h-5 w-5 text-(--icon-muted)"
                onClick={() => onChange(tags.filter((item) => item !== tag))}
                size="xs"
                type="button"
                variant="ghost"
              >
                <X className="h-3 w-3" />
              </UiIconButton>
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}
