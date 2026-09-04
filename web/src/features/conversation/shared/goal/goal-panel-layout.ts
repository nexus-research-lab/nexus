// INPUT: Goal 状态/可靠性表面所在的 Conversation 密度模式。
// OUTPUT: 对齐正文与紧凑 Composer 的两种稳定 lane recipe。
// POS: Goal 局部布局所有者；不包含状态、动作、颜色、圆角或阴影。

import { COMPOSER_COMPACT_LANE_CLASS_NAME } from "../composer/composer-styles";
import { CONVERSATION_CONTENT_LANE_CLASS_NAME } from "../conversation-panel-styles";

export const GOAL_PANEL_STRIP_CLASS_NAME =
  `${CONVERSATION_CONTENT_LANE_CLASS_NAME} px-3 sm:px-5 xl:px-6`;

export const GOAL_PANEL_COMPACT_CLASS_NAME =
  `${COMPOSER_COMPACT_LANE_CLASS_NAME} px-4`;
