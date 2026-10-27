import type { ReactNode } from "react";

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
  /** 在线不可用时的说明插槽，占据位置切换器的位置；由调用方决定文案与恢复动作。 */
  onlineNotice?: ReactNode;
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
