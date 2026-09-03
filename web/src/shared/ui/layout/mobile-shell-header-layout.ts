// INPUT: 窄窗应用页头与 Room 页头需要共享的客户区几何。
// OUTPUT: 浏览器/Windows 保持 52px，macOS 可由宿主实测窗口控件中心覆盖的布局 class。
// POS: shared/ui 窄窗壳层布局真相；不包含返回行为、标题内容或业务操作。

/** 窄窗页头统一读取平台投影变量，避免 Feature 各自写死高度。 */
export const MOBILE_SHELL_HEADER_HEIGHT_CLASS_NAME =
  "h-[var(--mobile-shell-header-height,52px)]";

/** 窄窗页头两侧留白保持一致，页面动作不得另建自己的内容轴。 */
export const MOBILE_SHELL_HEADER_GUTTER_CLASS_NAME = "px-2 sm:px-3";
