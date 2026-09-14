export interface RoomMemberAgentOption {
  agent_id: string;
  avatar?: string | null;
  name: string;
  room_participation_paused?: boolean;
}

export interface RoomMemberUserOption {
  avatar?: string | null;
  display_name: string;
  user_id: string;
  username: string;
}

export type RoomLocation = "local" | "online";

export interface RoomDialogSubmission {
  agentIds: string[];
  avatar?: string;
  hostAgentId: string | null;
  hostAutoReplyEnabled: boolean;
  name: string;
  pausedAgentIds: string[];
  privateMessagesEnabled: boolean;
  skillNames: string[];
  location: RoomLocation;
  userIds: string[];
}

export type RoomDialogMode = "create" | "manage";

export interface CreateRoomDialogProps {
  agents: RoomMemberAgentOption[];
  initialAvatar?: string;
  initialHostAgentId?: string | null;
  initialHostAutoReplyEnabled?: boolean;
  initialName?: string;
  initialPausedAgentIds?: string[];
  initialPrivateMessagesEnabled?: boolean;
  initialRoomSkillNames?: string[];
  initialSelectedAgentIds?: string[];
  isCreating?: boolean;
  isOpen: boolean;
  mode?: RoomDialogMode;
  onlineAvailable?: boolean;
  users?: RoomMemberUserOption[];
  onCancel: () => void;
  onConfirm: (submission: RoomDialogSubmission) => void | Promise<void>;
}

export interface RoomDialogFormState {
  avatar: string;
  hostAgentId: string;
  hostAutoReplyEnabled: boolean;
  memberQuery: string;
  location: RoomLocation;
  name: string;
  pausedAgentIds: string[];
  privateMessagesEnabled: boolean;
  selectedAgentIds: string[];
  selectedSkillNames: string[];
  selectedUserIds: string[];
  skillQuery: string;
}
