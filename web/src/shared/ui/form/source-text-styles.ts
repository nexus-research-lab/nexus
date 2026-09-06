// INPUT: Source editing, streaming and plain-text preview surfaces.
// OUTPUT: Shared monospace metrics and a keyboard-scrollable preview viewport recipe.
// POS: Neutral source presentation; consumers own content, naming, padding and focusability.

export const UI_SOURCE_TEXT_CLASS_NAME = "font-mono text-sm leading-6";

export const UI_SOURCE_PREVIEW_SCROLL_CLASS_NAME = "soft-scrollbar min-h-0 min-w-0 overflow-auto overscroll-contain focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[color:var(--ring)]";
