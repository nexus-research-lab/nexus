// INPUT: Read-only preview surfaces that need bounded native keyboard scrolling.
// OUTPUT: Shared overflow, overscroll containment and inset focus-ring geometry.
// POS: Neutral preview viewport recipe; consumers own content, names, semantics and focusability.
export const UI_PREVIEW_VIEWPORT_CLASS_NAME = "soft-scrollbar min-h-0 min-w-0 overflow-auto overscroll-contain focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[color:var(--ring)]";
