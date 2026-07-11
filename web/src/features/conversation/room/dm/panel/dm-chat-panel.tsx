"use client";

import { DmChatPanelView } from "./dm-chat-panel-view";
import type { DmChatPanelProps } from "./dm-chat-panel-types";
import { useDmChatPanelModel } from "./use-dm-chat-panel-model";

export type { DmChatPanelProps } from "./dm-chat-panel-types";

/** DM 入口只组合领域控制器，具体会话和渲染阶段由子模块负责。 */
export function DmChatPanel(props: DmChatPanelProps) {
  const model = useDmChatPanelModel(props);
  return <DmChatPanelView model={model} />;
}
