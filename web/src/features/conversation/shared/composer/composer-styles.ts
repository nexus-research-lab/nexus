// INPUT: Composer 各子视图需要共享的局部几何与表面名称。
// OUTPUT: 只描述 Conversation Composer 领域结构的稳定 class 和测量常量。
// POS: Composer 视觉 pattern；按钮 DOM 与状态样式由 shared UiButton 原语负责。

export const COMPOSER_ATTACHMENT_CLASS_NAME =
  "chip-default group relative inline-flex items-center gap-2 rounded-[8px] px-2.5 py-1.5";

export const COMPOSER_ATTACHMENT_PREVIEW_CLASS_NAME =
  "group/preview inline-flex min-w-0 items-center gap-2 rounded-[6px] outline-none transition-colors hover:text-(--text-strong) focus-visible:ring-2 focus-visible:ring-primary/50";

export const COMPOSER_ATTACHMENT_ROW_CLASS_NAME =
  "flex flex-wrap gap-2 border-b border-(--divider-subtle-color) px-2.5 py-2";

export const COMPOSER_ATTACHMENT_REMOVE_CLASS_NAME =
  "ml-1 rounded-full p-0.5 text-(--destructive) opacity-60 transition-[background,opacity] duration-(--motion-duration-fast) hover:bg-[color:color-mix(in_srgb,var(--destructive)_10%,transparent)] focus-visible:ring-2 focus-visible:ring-primary/50";

export const COMPOSER_IMAGE_ATTACHMENT_CLASS_NAME =
  "group relative flex h-12 w-12 shrink-0 items-center justify-center rounded-[10px] border border-(--divider-subtle-color) bg-(--surface-panel-subtle-background) p-0.5 shadow-(--surface-control-field-shadow)";

export const COMPOSER_IMAGE_ATTACHMENT_PREVIEW_CLASS_NAME =
  "group/preview relative flex h-full w-full cursor-zoom-in items-center justify-center overflow-hidden rounded-[8px] outline-none focus-visible:ring-2 focus-visible:ring-primary/60";

export const COMPOSER_IMAGE_ATTACHMENT_REMOVE_CLASS_NAME =
  "absolute -right-1.5 -top-1.5 inline-flex h-[18px] w-[18px] items-center justify-center rounded-full border border-(--surface-control-border) bg-(--surface-raised-background) text-(--destructive) opacity-85 shadow-(--surface-control-shadow) transition-[background,opacity,transform] duration-(--motion-duration-fast) hover:scale-105 hover:bg-(--surface-interactive-hover-background) hover:opacity-100 focus-visible:ring-2 focus-visible:ring-primary/50";

export const COMPOSER_SHELL_CLASS_NAME =
  "input-shell nexus-chat-composer-shell workbench-input-shell overflow-hidden rounded-[20px]";

export const COMPOSER_TEXTAREA_MAX_HEIGHT_PX = 120;

export const COMPOSER_COMPACT_LANE_CLASS_NAME =
  "mx-auto w-full max-w-[720px]";

export const COMPOSER_FOOTER_CLASS_NAME =
  "nexus-chat-composer-footer text-(--text-soft)";
