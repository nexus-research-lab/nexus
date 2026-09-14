// INPUT: 会话正文、Composer、消息 frame 与人工决策行的领域布局角色。
// OUTPUT: 各布局消费者共用的宽度与决策命中区 className。
// POS: 会话布局所有者；按钮材质、字体与状态仍由共享 UI 持有。

export const CONVERSATION_CONTENT_LANE_CLASS_NAME =
  "mx-auto w-full max-w-[844px]";

// 中文注释：Composer 列比正文列（800px）略宽，让主输入面在视觉上容纳正文而不是被正文压住。
export const CONVERSATION_COMPOSER_LANE_CLASS_NAME =
  "mx-auto w-full max-w-[880px]";

// 中文注释：消息 frame 覆盖头像的两个外边缘，正文跨列后与它们精确对齐。
export const CONVERSATION_ASSISTANT_FRAME_WIDTH_CLASS_NAME =
  "max-w-[820px]";

// 只用于人工决策行，不能通过祖先选择器放大菜单或其他后代按钮。
export const CONVERSATION_DECISION_ACTION_CLASS_NAME =
  "min-h-11 min-w-11 sm:min-h-8 sm:min-w-0";
