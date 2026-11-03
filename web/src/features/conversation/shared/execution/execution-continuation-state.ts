/**
 * INPUT: Conversation-scoped authoritative resource and continuation action owner.
 * OUTPUT: Shared context consumed by timeline and graph controls.
 * POS: Context identity only; mutations remain in use-execution-recovery.
 */
import { createContext, useContext } from "react";
import type { ExecutionResource } from "./use-execution-resource";
import type { ExecutionRecoveryAction } from "./use-execution-recovery";
export const ContinuationContext = createContext<{ resource: ExecutionResource; action: ExecutionRecoveryAction } | null>(null);
export function useSharedExecutionContinuation() { return useContext(ContinuationContext); }
