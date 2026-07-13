import type {
  CreateRoomDialogProps,
  RoomDialogMode,
  RoomDialogSubmission,
  RoomMemberAgentOption,
} from "./create-room-dialog-types";

const ROOM_DIALOG_LABEL_KEYS = {
  create: {
    confirm: "room.create_action",
    subtitle: "room.create_dialog_subtitle",
    title: "room.create_dialog_title",
  },
  manage: {
    confirm: "common.save",
    subtitle: "room.manage_dialog_subtitle",
    title: "room.manage_dialog_title",
  },
} as const;

type RoomDialogLabelKey = typeof ROOM_DIALOG_LABEL_KEYS[RoomDialogMode][
  keyof typeof ROOM_DIALOG_LABEL_KEYS[RoomDialogMode]
];

export interface RoomDialogContentProps {
  agents: RoomMemberAgentOption[];
  initialAvatar: string;
  initialHostAgentId: string | null;
  initialHostAutoReplyEnabled: boolean;
  initialName: string;
  initialPrivateMessagesEnabled: boolean;
  initialRoomSkillNames: string[];
  initialSelectedAgentIds: string[];
  isCreating: boolean;
  mode: RoomDialogMode;
  onCancel: () => void;
  onConfirm: (submission: RoomDialogSubmission) => void;
}

interface RoomDialogLabels {
  confirm: string;
  subtitle: string;
  title: string;
}

function valueOrDefault<Value>(
  value: Value | undefined,
  fallback: Value,
): Value {
  return value === undefined ? fallback : value;
}

export function resolveRoomDialogContentProps(
  props: CreateRoomDialogProps,
): RoomDialogContentProps {
  return {
    agents: props.agents,
    initialAvatar: valueOrDefault(props.initialAvatar, ""),
    initialHostAgentId: valueOrDefault(props.initialHostAgentId, null),
    initialHostAutoReplyEnabled: valueOrDefault(
      props.initialHostAutoReplyEnabled,
      false,
    ),
    initialName: valueOrDefault(props.initialName, ""),
    initialPrivateMessagesEnabled: valueOrDefault(
      props.initialPrivateMessagesEnabled,
      false,
    ),
    initialRoomSkillNames: valueOrDefault(props.initialRoomSkillNames, []),
    initialSelectedAgentIds: valueOrDefault(
      props.initialSelectedAgentIds,
      [],
    ),
    isCreating: valueOrDefault(props.isCreating, false),
    mode: valueOrDefault(props.mode, "create"),
    onCancel: props.onCancel,
    onConfirm: props.onConfirm,
  };
}

export function buildRoomDialogInstanceKey(
  props: RoomDialogContentProps,
): string {
  return JSON.stringify({
    avatar: props.initialAvatar,
    hostAgentId: props.initialHostAgentId?.trim() ?? "",
    hostAutoReplyEnabled: props.initialHostAutoReplyEnabled,
    name: props.initialName,
    privateMessagesEnabled: props.initialPrivateMessagesEnabled,
    selectedAgentIds: props.initialSelectedAgentIds,
    selectedSkillNames: props.initialRoomSkillNames,
  });
}

export function resolveRoomDialogLabels(
  mode: RoomDialogMode,
  translate: (key: RoomDialogLabelKey) => string,
): RoomDialogLabels {
  const keys = ROOM_DIALOG_LABEL_KEYS[mode];
  return {
    confirm: translate(keys.confirm),
    subtitle: translate(keys.subtitle),
    title: translate(keys.title),
  };
}
