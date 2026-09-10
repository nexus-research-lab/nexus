import { useCallback, useMemo, useReducer } from "react";

import { createUiSearchMatcher } from "@/shared/ui/form/search-query";

import type {
  RoomDialogFormState,
  RoomDialogSubmission,
  RoomMemberAgentOption,
  RoomMemberUserOption,
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
  users: RoomMemberUserOption[];
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
  const selectedUserIdSet = useMemo(
    () => new Set(state.selectedUserIds),
    [state.selectedUserIds],
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
  const filteredUsers = useMemo(() => {
    const search = createUiSearchMatcher(state.memberQuery);
    return options.users.filter((user) => search.matches([
      user.display_name,
      user.username,
    ]));
  }, [options.users, state.memberQuery]);
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
  const toggleUser = useCallback((userId: string) => {
    dispatch((current) => ({
      ...current,
      selectedUserIds: toggleMemberId(current.selectedUserIds, userId),
    }));
  }, []);

  return {
    canSubmit:
      state.name.trim().length > 0 &&
      (state.location === "online" || state.selectedAgentIds.length > 0),
    filteredAgents,
    filteredUsers,
    pausedAgentIdSet,
    selectedAgentIdSet,
    selectedUserIdSet,
    selectedAgents,
    setAvatar: (avatar: string) => update("avatar", avatar),
    setHostAgentId,
    setHostAutoReplyEnabled: (enabled: boolean) =>
      update("hostAutoReplyEnabled", enabled),
    setMemberQuery: (query: string) => update("memberQuery", query),
    setLocation: (location: RoomDialogFormState["location"]) => update("location", location),
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
    toggleUser,
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
    location: "local",
    name: options.initialName,
    pausedAgentIds: [...options.initialPausedAgentIds],
    privateMessagesEnabled: options.initialPrivateMessagesEnabled,
    selectedAgentIds: [...options.initialSelectedAgentIds],
    selectedSkillNames: [...options.initialRoomSkillNames],
    selectedUserIds: [],
    skillQuery: "",
  });
}

function normalizeRoomForm(state: RoomDialogFormState): RoomDialogFormState {
  const selectedAgentIds = new Set(state.selectedAgentIds);
  const pausedAgentIds = state.pausedAgentIds.filter((agentId) => (
    selectedAgentIds.has(agentId)
  ));
  const next = {
    ...state,
    pausedAgentIds,
  };
  if (!state.hostAgentId || !selectedAgentIds.has(state.hostAgentId)) {
    next.hostAgentId = "";
    next.hostAutoReplyEnabled = false;
  }
  if (state.location === "online") {
    next.hostAutoReplyEnabled = false;
  }
  return next;
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
    location: state.location,
    userIds: state.location === "online" ? state.selectedUserIds : [],
  };
}
