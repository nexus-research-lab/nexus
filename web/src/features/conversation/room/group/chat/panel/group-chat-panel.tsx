"use client";

import { GroupChatPanelView } from "./group-chat-panel-view";
import type { GroupChatPanelProps } from "./group-chat-panel-types";
import { useGroupChatPanelModel } from "./use-group-chat-panel-model";

export type { GroupChatPanelProps } from "./group-chat-panel-types";

/** Room 群聊入口只组合领域控制器，具体会话与渲染阶段由子模块负责。 */
export function GroupChatPanel(props: GroupChatPanelProps) {
  const model = useGroupChatPanelModel(props);
  return <GroupChatPanelView model={model} />;
}
