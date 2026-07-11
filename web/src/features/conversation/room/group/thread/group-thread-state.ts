import { createContext, useContext } from "react";

export interface ThreadTarget {
  roundId: string;
  agentId: string;
}

export interface ThreadControlState {
  activeThread: ThreadTarget | null;
  openThread: (roundId: string, agentId: string) => void;
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
