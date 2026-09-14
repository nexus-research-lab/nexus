// INPUT: Stable Room member identities for offline Goal regressions.
// OUTPUT: Two real Agent-shaped fixtures with distinct current permissions.
// POS: Room Goal test support; not a runtime candidate directory.
import type { Agent } from "@/types/agent/agent";

export const ROOM_GOAL_MEMBERS: Agent[] = [
  { agent_id: "alpha", name: "Alpha", created_at: 1, options: {}, status: "idle", workspace_path: "/alpha" },
  { agent_id: "beta", name: "Beta", created_at: 2, options: { permission_mode: "plan" }, status: "idle", workspace_path: "/beta" },
];
