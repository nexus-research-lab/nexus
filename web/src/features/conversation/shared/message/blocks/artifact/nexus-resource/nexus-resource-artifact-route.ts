/**
 * [INPUT]: Agent/Room 资源卡片协议，含可选 Room 主持开场内容与目标 Agent。
 * [OUTPUT]: 资源详情路由；有开场协议时生成一次性 Room 自动发送参数。
 * [POS]: Nexus 资源卡片到应用内导航意图的纯投影。
 */

import { AppRouteBuilders } from "@/app/router/route-paths";
import {
  buildRoomInitialActionKey,
  isRoomInitialActionConsumed,
} from "@/lib/conversation/room-initial-action";
import type { NexusResourceArtifactContent } from "@/types/conversation/message/content";

export function buildNexusResourceArtifactRoute(
  artifact: NexusResourceArtifactContent,
): string {
  if (artifact.resource_kind === "agent") {
    return AppRouteBuilders.contactAgent(artifact.resource_id);
  }

  const route = artifact.conversation_id
    ? AppRouteBuilders.roomConversation(
        artifact.resource_id,
        artifact.conversation_id,
      )
    : AppRouteBuilders.room(artifact.resource_id);
  const initialMessage = artifact.initial_message?.trim();
  const initialActionKey = buildRoomInitialActionKey(
    artifact.resource_id,
    artifact.conversation_id,
  );
  if (!initialMessage || isRoomInitialActionConsumed(initialActionKey)) {
    return route;
  }

  const params = new URLSearchParams({
    initial: initialMessage,
    initial_action_key: initialActionKey,
    scripted_host_message: "1",
  });
  const targetAgentIDs = (artifact.initial_target_agent_ids ?? [])
    .map((agentID) => agentID.trim())
    .filter(Boolean);
  if (targetAgentIDs.length > 0) {
    params.set("initial_target_agent_ids", targetAgentIDs.join(","));
  }
  return `${route}?${params.toString()}`;
}
