// INPUT: Room 创建/管理 Props、模式与窄翻译函数。
// OUTPUT: 完整内容参数、稳定重建 key 与标题/主动作标签。
// POS: Room 弹窗可选参数和可见标签的唯一纯模型，不生成说明性副标题。
import type {
  CreateRoomDialogProps,
  RoomDialogMode,
  RoomDialogSubmission,
  RoomMemberAgentOption,
} from "./create-room-dialog-types";

const ROOM_DIALOG_LABEL_KEYS = {
  create: {
    confirm: "room.create_action",
    title: "room.create_dialog_title",
  },
  manage: {
    confirm: "common.save",
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
  initialPausedAgentIds: string[];
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
    initialPausedAgentIds: valueOrDefault(props.initialPausedAgentIds, []),
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
    pausedAgentIds: props.initialPausedAgentIds,
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
    title: translate(keys.title),
  };
}
