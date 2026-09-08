import { useCallback, useMemo, useReducer } from "react";

import { createUiSearchMatcher } from "@/shared/ui/form/search-query";

import type {
  RoomDialogFormState,
  RoomDialogSubmission,
  RoomMemberAgentOption,
} from "./create-room-dialog-types";

type RoomFormTransition = (
  current: RoomDialogFormState,
) => RoomDialogFormState;

interface UseCreateRoomFormOptions {
  agents: RoomMemberAgentOption[];
  initialAvatar: string;
  initialHostAgentId: string | null;
  initialHostAutoReplyEnabled: boolean;
  initialName: string;
  initialPausedAgentIds: string[];
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
  const pausedAgentIdSet = useMemo(
    () => new Set(state.pausedAgentIds),
    [state.pausedAgentIds],
  );
  const selectedAgents = useMemo(
    () =>
      options.agents.filter((agent) =>
        selectedAgentIdSet.has(agent.agent_id),
      ),
    [options.agents, selectedAgentIdSet],
  );
  const filteredAgents = useMemo(() => {
    const search = createUiSearchMatcher(state.memberQuery);
    return options.agents.filter((agent) => search.matches([agent.name]));
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
  const toggleParticipation = useCallback((agentId: string) => {
    dispatch((current) => {
      if (!current.selectedAgentIds.includes(agentId)) {
        return current;
      }
      return {
        ...current,
        pausedAgentIds: toggleMemberId(current.pausedAgentIds, agentId),
      };
    });
  }, []);

  return {
    canSubmit:
      state.selectedAgentIds.length > 0 && state.name.trim().length > 0,
    filteredAgents,
    pausedAgentIdSet,
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
    toggleParticipation,
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
    pausedAgentIds: [...options.initialPausedAgentIds],
    privateMessagesEnabled: options.initialPrivateMessagesEnabled,
    selectedAgentIds: [...options.initialSelectedAgentIds],
    selectedSkillNames: [...options.initialRoomSkillNames],
    skillQuery: "",
  });
}

function normalizeRoomForm(state: RoomDialogFormState): RoomDialogFormState {
  const selectedAgentIds = new Set(state.selectedAgentIds);
  const pausedAgentIds = state.pausedAgentIds.filter((agentId) => (
    selectedAgentIds.has(agentId)
  ));
  if (state.hostAgentId && selectedAgentIds.has(state.hostAgentId)) {
    return { ...state, pausedAgentIds };
  }
  return {
    ...state,
    hostAgentId: "",
    hostAutoReplyEnabled: false,
    pausedAgentIds,
  };
}

function toggleMemberId(memberIds: string[], agentId: string): string[] {
  const nextIds = new Set(memberIds);
  if (nextIds.has(agentId)) {
    nextIds.delete(agentId);
  } else {
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
    pausedAgentIds: state.pausedAgentIds,
    privateMessagesEnabled: state.privateMessagesEnabled,
    skillNames: state.selectedSkillNames,
  };
}
