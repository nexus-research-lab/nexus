/**
 * INPUT: Room root、Agent 与可选 agent_round_id。
 * OUTPUT: 精确到一次 Agent 执行轮的 Thread 选择态与开关协议。
 * POS: Room Thread 控制上下文的身份契约。
 */
import { createContext, useContext } from "react";

export interface ThreadTarget {
  roundId: string;
  agentId: string;
  agentRoundId: string | null;
}

export interface ThreadControlState {
  activeThread: ThreadTarget | null;
  openThread: (
    roundId: string,
    agentId: string,
    agentRoundId?: string | null,
  ) => void;
  closeThread: () => void;
}

export const ThreadControlContext = createContext<ThreadControlState | null>(null);

export function useGroupThread(): ThreadControlState {
  const context = useContext(ThreadControlContext);
  if (!context) {
    throw new Error("useGroupThread must be used within GroupThreadContextProvider");
  }
  return context;
}
