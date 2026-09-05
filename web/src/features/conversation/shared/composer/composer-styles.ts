// INPUT: Composer 各子视图需要共享的局部几何与表面名称。
// OUTPUT: 只描述 Conversation Composer 领域结构的稳定 class 和测量常量。
// POS: Composer 视觉 pattern；按钮 DOM 与状态样式由 shared UiButton 原语负责。

export const COMPOSER_ATTACHMENT_CLASS_NAME =
  "chip-default group relative inline-flex items-center gap-2 px-2.5 py-1.5";

export const COMPOSER_ATTACHMENT_PREVIEW_CLASS_NAME =
  "group/preview min-h-0 min-w-0 gap-2 p-0";

export const COMPOSER_ATTACHMENT_ROW_CLASS_NAME =
  "flex flex-wrap gap-2 border-b border-(--divider-subtle-color) px-2.5 py-2";

export const COMPOSER_IMAGE_ATTACHMENT_CLASS_NAME =
  "group relative flex h-12 w-12 shrink-0 items-center justify-center radius-control-md border border-(--divider-subtle-color) bg-(--surface-panel-subtle-background) p-0.5 shadow-(--surface-control-field-shadow)";

export const COMPOSER_IMAGE_ATTACHMENT_PREVIEW_CLASS_NAME =
  "group/preview relative h-full min-h-0 w-full cursor-zoom-in overflow-hidden p-0";

export const COMPOSER_IMAGE_ATTACHMENT_REMOVE_CLASS_NAME =
  "absolute -right-1.5 -top-1.5";

export const COMPOSER_SHELL_CLASS_NAME =
  "input-shell nexus-chat-composer-shell overflow-hidden";

export const COMPOSER_TEXTAREA_MAX_HEIGHT_PX = 120;

export const COMPOSER_COMPACT_LANE_CLASS_NAME =
  "mx-auto w-full max-w-[720px]";

export const COMPOSER_FOOTER_CLASS_NAME =
  "nexus-chat-composer-footer text-(--text-soft)";
