"use client";

import type { DmChatPanelProps } from "./dm-chat-panel-types";
import { useDmChatPanelModel } from "./controller/use-dm-chat-panel-model";
import { DmChatPanelView } from "./view/dm-chat-panel-view";

export type { DmChatPanelProps } from "./dm-chat-panel-types";

/** DM 入口只连接控制器与视图，领域阶段由子目录独立维护。 */
export function DmChatPanel(props: DmChatPanelProps) {
  const model = useDmChatPanelModel(props);
  return <DmChatPanelView model={model} />;
}
