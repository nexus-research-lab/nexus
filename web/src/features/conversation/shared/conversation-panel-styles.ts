// INPUT: 会话正文、Composer 与消息 frame 的领域宽度角色。
// OUTPUT: 各真实布局消费者共用的宽度 className。
// POS: 会话内容轴所有者；不保留没有消费者的近似宽度配方。

export const CONVERSATION_CONTENT_LANE_CLASS_NAME =
  "mx-auto w-full max-w-[844px]";

// 中文注释：Composer 列比正文列（800px）略宽，让主输入面在视觉上容纳正文而不是被正文压住。
export const CONVERSATION_COMPOSER_LANE_CLASS_NAME =
  "mx-auto w-full max-w-[880px]";

// 中文注释：消息 frame 覆盖头像的两个外边缘，正文跨列后与它们精确对齐。
export const CONVERSATION_ASSISTANT_FRAME_WIDTH_CLASS_NAME =
  "max-w-[820px]";
