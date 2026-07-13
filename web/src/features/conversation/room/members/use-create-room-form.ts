import { useCallback, useMemo, useReducer } from "react";

import type {
  RoomDialogFormState,
  RoomDialogSubmission,
  RoomMemberAgentOption,
} from "./create-room-dialog-types";

const MAX_ROOM_MEMBERS = 10;

type RoomFormTransition = (
  current: RoomDialogFormState,
) => RoomDialogFormState;

interface UseCreateRoomFormOptions {
  agents: RoomMemberAgentOption[];
  initialAvatar: string;
  initialHostAgentId: string | null;
  initialHostAutoReplyEnabled: boolean;
  initialName: string;
  initialPrivateMessagesEnabled: boolean;
  initialRoomSkillNames: string[];
  initialSelectedAgentIds: string[];
}

export function useCreateRoomForm(options: UseCreateRoomFormOptions) {
  const [state, dispatch] = useReducer(
    applyRoomFormTransition,
    options,
    createInitialRoomFormState,
  );
  const selectedAgentIdSet = useMemo(
    () => new Set(state.selectedAgentIds),
    [state.selectedAgentIds],
  );
  const selectedAgents = useMemo(
    () =>
      options.agents.filter((agent) =>
        selectedAgentIdSet.has(agent.agent_id),
      ),
    [options.agents, selectedAgentIdSet],
  );
  const filteredAgents = useMemo(() => {
    const query = state.memberQuery.trim().toLowerCase();
    return query
      ? options.agents.filter((agent) =>
          agent.name.toLowerCase().includes(query),
        )
      : options.agents;
  }, [options.agents, state.memberQuery]);
  const update = useCallback(
    <Field extends keyof RoomDialogFormState>(
      field: Field,
      value: RoomDialogFormState[Field],
    ) => {
      dispatch((current) => ({ ...current, [field]: value }));
    },
    [],
  );
  const toggleAgent = useCallback((agentId: string) => {
    dispatch((current) => ({
      ...current,
      selectedAgentIds: toggleMemberId(current.selectedAgentIds, agentId),
    }));
  }, []);
  const setHostAgentId = useCallback((agentId: string) => {
    dispatch((current) => ({ ...current, hostAgentId: agentId }));
  }, []);

  return {
    canSubmit:
      state.selectedAgentIds.length > 0 && state.name.trim().length > 0,
    filteredAgents,
    selectedAgentIdSet,
    selectedAgents,
    setAvatar: (avatar: string) => update("avatar", avatar),
    setHostAgentId,
    setHostAutoReplyEnabled: (enabled: boolean) =>
      update("hostAutoReplyEnabled", enabled),
    setMemberQuery: (query: string) => update("memberQuery", query),
    setName: (name: string) => update("name", name),
    setPrivateMessagesEnabled: (enabled: boolean) =>
      update("privateMessagesEnabled", enabled),
    setSelectedSkillNames: (names: string[]) =>
      update("selectedSkillNames", names),
    setSkillQuery: (query: string) => update("skillQuery", query),
    state,
    submission: buildRoomDialogSubmission(state),
    toggleAgent,
  };
}

function applyRoomFormTransition(
  current: RoomDialogFormState,
  transition: RoomFormTransition,
): RoomDialogFormState {
  return normalizeRoomForm(transition(current));
}

function createInitialRoomFormState(
  options: UseCreateRoomFormOptions,
): RoomDialogFormState {
  return normalizeRoomForm({
    avatar: options.initialAvatar,
    hostAgentId: options.initialHostAgentId?.trim() ?? "",
    hostAutoReplyEnabled: options.initialHostAutoReplyEnabled,
    memberQuery: "",
    name: options.initialName,
    privateMessagesEnabled: options.initialPrivateMessagesEnabled,
    selectedAgentIds: [...options.initialSelectedAgentIds],
    selectedSkillNames: [...options.initialRoomSkillNames],
    skillQuery: "",
  });
}

function normalizeRoomForm(state: RoomDialogFormState): RoomDialogFormState {
  if (state.hostAgentId && state.selectedAgentIds.includes(state.hostAgentId)) {
    return state;
  }
  return {
    ...state,
    hostAgentId: "",
    hostAutoReplyEnabled: false,
  };
}

function toggleMemberId(memberIds: string[], agentId: string): string[] {
  const nextIds = new Set(memberIds);
  if (nextIds.has(agentId)) {
    nextIds.delete(agentId);
  } else if (nextIds.size < MAX_ROOM_MEMBERS) {
    nextIds.add(agentId);
  }
  return [...nextIds];
}

function buildRoomDialogSubmission(
  state: RoomDialogFormState,
): RoomDialogSubmission {
  return {
    agentIds: state.selectedAgentIds,
    avatar: state.avatar || undefined,
    hostAgentId: state.hostAgentId || null,
    hostAutoReplyEnabled:
      state.hostAutoReplyEnabled && state.hostAgentId !== "",
    name: state.name.trim(),
    privateMessagesEnabled: state.privateMessagesEnabled,
    skillNames: state.selectedSkillNames,
  };
}
