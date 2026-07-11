/**
 * 会话消息附件契约。
 */

type MessageAttachmentKind = "text" | "image" | "file";

export type MessageAttachmentScope = "agentWorkspace" | "roomConversation";

export interface MessageAttachment {
  file_name: string;
  workspace_path: string;
  workspace_agent_id?: string;
  room_id?: string;
  conversation_id?: string;
  scope?: MessageAttachmentScope;
  kind: MessageAttachmentKind;
  mime_type?: string | null;
  size?: number;
}
