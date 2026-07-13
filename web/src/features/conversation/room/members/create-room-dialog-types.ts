export interface RoomMemberAgentOption {
  agent_id: string;
  avatar?: string | null;
  name: string;
}

export interface RoomDialogSubmission {
  agentIds: string[];
  avatar?: string;
  hostAgentId: string | null;
  hostAutoReplyEnabled: boolean;
  name: string;
  privateMessagesEnabled: boolean;
  skillNames: string[];
}

export type RoomDialogMode = "create" | "manage";

export interface CreateRoomDialogProps {
  agents: RoomMemberAgentOption[];
  initialAvatar?: string;
  initialHostAgentId?: string | null;
  initialHostAutoReplyEnabled?: boolean;
  initialName?: string;
  initialPrivateMessagesEnabled?: boolean;
  initialRoomSkillNames?: string[];
  initialSelectedAgentIds?: string[];
  isCreating?: boolean;
  isOpen: boolean;
  mode?: RoomDialogMode;
  onCancel: () => void;
  onConfirm: (submission: RoomDialogSubmission) => void;
}

export interface RoomDialogFormState {
  avatar: string;
  hostAgentId: string;
  hostAutoReplyEnabled: boolean;
  memberQuery: string;
  name: string;
  privateMessagesEnabled: boolean;
  selectedAgentIds: string[];
  selectedSkillNames: string[];
  skillQuery: string;
}
