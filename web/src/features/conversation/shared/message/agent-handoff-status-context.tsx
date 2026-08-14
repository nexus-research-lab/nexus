/**
 * INPUT: 当前 Room 会话按 handoff_id 投影出的单调交接阶段。
 * OUTPUT: 消息正文中的 Agent mention 可读取的窄状态上下文。
 * POS: Room 面板与共享 mention chip 之间不穿透 feed props 的桥接边界。
 */
"use client";

import {
  createContext,
  type ReactNode,
  useContext,
} from "react";

export type AgentHandoffPhase =
  | "preparing"
  | "queued"
  | "active"
  | "responded";
export type AgentHandoffStatusMap = Readonly<Record<string, AgentHandoffPhase>>;

const EMPTY_HANDOFF_STATUSES: AgentHandoffStatusMap = Object.freeze({});
const AGENT_HANDOFF_STATUS_CONTEXT = createContext<AgentHandoffStatusMap>(
  EMPTY_HANDOFF_STATUSES,
);

export function AgentHandoffStatusProvider({
  children,
  statuses,
}: {
  children: ReactNode;
  statuses: AgentHandoffStatusMap;
}) {
  return (
    <AGENT_HANDOFF_STATUS_CONTEXT.Provider value={statuses}>
      {children}
    </AGENT_HANDOFF_STATUS_CONTEXT.Provider>
  );
}

// Provider 与消费 Hook 必须共享同一个私有 Context，避免公开可变写入口。
// eslint-disable-next-line react-refresh/only-export-components
export function useAgentHandoffStatus(
  handoffId?: string | null,
): AgentHandoffPhase | null {
  const statuses = useContext(AGENT_HANDOFF_STATUS_CONTEXT);
  const normalizedHandoffId = handoffId?.trim();
  return normalizedHandoffId
    ? statuses[normalizedHandoffId] ?? null
    : null;
}
