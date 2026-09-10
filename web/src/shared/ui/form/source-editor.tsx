// INPUT: Native textarea value/events, read-only or disabled state and optional exact Field identity.
// OUTPUT: A borderless source surface with shared code metrics, scrolling and an inset keyboard focus ring.
// POS: Source editing primitive; owns no draft, file identity, save command, shortcut or preview mode.

import { forwardRef, type TextareaHTMLAttributes } from "react";
import { cn } from "@/shared/ui/class-name";
import { useFieldControlAttributes } from "./field-accessibility";
import { UI_SOURCE_TEXT_CLASS_NAME } from "./source-text-styles";

export const UiSourceEditor = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(
  function UiSourceEditor({ className, spellCheck = false, autoCapitalize = "off", autoCorrect = "off", ...props }, ref) {
    const fieldAttributes = useFieldControlAttributes(props);
    return <textarea
      {...props}
      {...fieldAttributes}
      autoCapitalize={autoCapitalize}
      autoCorrect={autoCorrect}
      className={cn(
        "soft-scrollbar min-h-0 min-w-0 w-full resize-none overflow-auto overscroll-contain border-0 bg-transparent p-0 text-(--text-default) outline-none read-only:text-(--text-muted) focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[color:var(--ring)] disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
        UI_SOURCE_TEXT_CLASS_NAME,
        className,
      )}
      ref={ref}
      spellCheck={spellCheck}
    />;
  },
);
