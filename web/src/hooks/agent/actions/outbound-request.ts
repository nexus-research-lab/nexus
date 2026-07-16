import { generateUuid } from "@/lib/uuid";

export interface OutboundRequestDescriptor {
  client_message_id: string;
  client_request_id: string;
}

export function createOutboundClientMessageId(): string {
  return `local_msg_${generateUuid()}`;
}

/**
 * 每次发送都使用新的 request ID；重试队列草稿时可复用 message ID，
 * 让后端把多次传输识别为同一条用户输入。
 */
export function createOutboundRequestDescriptor(
  clientMessageId?: string,
): OutboundRequestDescriptor {
  return {
    client_message_id: clientMessageId || createOutboundClientMessageId(),
    client_request_id: `req_${generateUuid()}`,
  };
}
