"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { UserRound } from "lucide-react";

import { cn } from "@/lib/utils";
import type { Agent } from "@/types/agent/agent";
import type { Goal } from "@/types/conversation/goal";
import {
  goal_continuation_hold_for_room_target,
  ROOM_GOAL_SCOPE_LABEL,
} from "@/features/conversation/shared/goal-continuation-hold";
import { GoalPanel } from "@/features/conversation/shared/goal-panel";

const ROOM_GOAL_LEAD_AGENT_ID_KEY = "room_goal_lead_agent_id";
const ROOM_GOAL_LEAD_AGENT_NAME_KEY = "room_goal_lead_agent_name";
const ROOM_GOAL_COLLABORATION_REQUIRED_KEY =
  "room_goal_collaboration_required";
const ROOM_GOAL_SCOPE_KEY = "room_goal_scope";

interface RoomGoalPanelProps {
  activity_key: string | number | null;
  can_control_session: boolean;
  is_loading: boolean;
  is_mobile_layout: boolean;
  room_host_agent_id?: string | null;
  room_members: Agent[];
  session_key: string | null;
}

function metadata_string(
  metadata: Record<string, unknown> | undefined,
  key: string,
): string {
  const value = metadata?.[key];
  return typeof value === "string" ? value.trim() : "";
}

function resolve_default_room_goal_lead(
  room_members: Agent[],
  host_agent_id: string | null | undefined,
): string {
  const normalized_host_agent_id = host_agent_id?.trim();
  if (
    normalized_host_agent_id &&
    room_members.some((agent) => agent.agent_id === normalized_host_agent_id)
  ) {
    return normalized_host_agent_id;
  }
  if (room_members.length === 1) {
    return room_members[0]?.agent_id ?? "";
  }
  return "";
}

function resolve_goal_lead_agent_id(
  goal: Goal | null,
  room_members: Agent[],
  fallback_agent_id: string,
): string {
  const metadata_agent_id = metadata_string(
    goal?.metadata,
    ROOM_GOAL_LEAD_AGENT_ID_KEY,
  );
  if (
    metadata_agent_id &&
    room_members.some((agent) => agent.agent_id === metadata_agent_id)
  ) {
    return metadata_agent_id;
  }
  return fallback_agent_id;
}

export function RoomGoalPanel({
  activity_key,
  can_control_session,
  is_loading,
  is_mobile_layout,
  room_host_agent_id,
  room_members,
  session_key,
}: RoomGoalPanelProps) {
  const default_lead_agent_id = useMemo(
    () => resolve_default_room_goal_lead(room_members, room_host_agent_id),
    [room_host_agent_id, room_members],
  );
  const [current_goal, set_current_goal] = useState<Goal | null>(null);
  const [selected_lead_agent_id, set_selected_lead_agent_id] =
    useState(default_lead_agent_id);

  useEffect(() => {
    set_selected_lead_agent_id((current) => {
      if (current && room_members.some((agent) => agent.agent_id === current)) {
        return current;
      }
      return default_lead_agent_id;
    });
  }, [default_lead_agent_id, room_members]);

  const handle_goal_change = useCallback(
    (goal: Goal | null) => {
      set_current_goal(goal);
      const lead_agent_id = resolve_goal_lead_agent_id(
        goal,
        room_members,
        default_lead_agent_id,
      );
      set_selected_lead_agent_id((current) =>
        current === lead_agent_id ? current : lead_agent_id,
      );
    },
    [default_lead_agent_id, room_members],
  );

  const effective_lead_agent_id = useMemo(
    () =>
      resolve_goal_lead_agent_id(
        current_goal,
        room_members,
        selected_lead_agent_id || default_lead_agent_id,
      ),
    [current_goal, default_lead_agent_id, room_members, selected_lead_agent_id],
  );
  const lead_agent = useMemo(
    () =>
      room_members.find((agent) => agent.agent_id === effective_lead_agent_id) ??
      null,
    [effective_lead_agent_id, room_members],
  );
  const continuation_hold = useMemo(
    () =>
      goal_continuation_hold_for_room_target(
        room_members,
        effective_lead_agent_id,
      ),
    [effective_lead_agent_id, room_members],
  );
  const draft_metadata = useMemo(
    () => ({
      [ROOM_GOAL_SCOPE_KEY]: "room",
      [ROOM_GOAL_LEAD_AGENT_ID_KEY]: selected_lead_agent_id,
      [ROOM_GOAL_LEAD_AGENT_NAME_KEY]:
        room_members.find((agent) => agent.agent_id === selected_lead_agent_id)
          ?.name ?? "",
      [ROOM_GOAL_COLLABORATION_REQUIRED_KEY]: room_members.length > 1,
    }),
    [room_members, selected_lead_agent_id],
  );
  const lead_select = (
    <label className="flex h-8 min-w-[150px] shrink-0 items-center gap-1.5 rounded-md border border-border/70 bg-background px-2 text-xs text-muted-foreground">
      <UserRound className="h-3.5 w-3.5 shrink-0" />
      <select
        className="min-w-0 flex-1 bg-transparent text-xs font-medium text-foreground outline-none disabled:cursor-not-allowed disabled:opacity-60"
        disabled={!can_control_session || is_loading}
        title="选择 Room Goal 负责人"
        value={selected_lead_agent_id}
        onChange={(event) => set_selected_lead_agent_id(event.target.value)}
      >
        <option value="">负责人</option>
        {room_members.map((agent) => (
          <option key={agent.agent_id} value={agent.agent_id}>
            {agent.name}
          </option>
        ))}
      </select>
    </label>
  );
  const status_extra = lead_agent ? (
    <span
      className={cn(
        "inline-flex h-6 max-w-[160px] shrink-0 items-center gap-1 rounded-md border border-border/60 bg-muted/30 px-2 text-[11px] text-muted-foreground",
      )}
      title={`Room Goal 负责人：${lead_agent.name}`}
    >
      <UserRound className="h-3.5 w-3.5 shrink-0" />
      <span className="truncate">负责人 {lead_agent.name}</span>
    </span>
  ) : null;

  return (
    <GoalPanel
      activity_key={activity_key}
      can_submit_draft={selected_lead_agent_id.trim() !== ""}
      compact={is_mobile_layout}
      continuation_hold={continuation_hold}
      disabled={!can_control_session}
      draft_extra_controls={lead_select}
      draft_metadata={draft_metadata}
      draft_submit_disabled_title="请选择 Room Goal 负责人"
      empty_state_variant="launcher"
      hide_budget_input
      is_generating={is_loading}
      session_key={session_key}
      scope_label={ROOM_GOAL_SCOPE_LABEL}
      status_extra={status_extra}
      on_goal_change={handle_goal_change}
    />
  );
}
