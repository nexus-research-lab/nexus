/**
 * INPUT: 共享 WebSocket 上的 session bind 消息与逻辑消费者租约。
 * OUTPUT: 按 session 引用计数发送 bind/unbind，并在重连后重放仍有效的绑定。
 * POS: 共享物理连接与会话级业务订阅之间的生命周期仲裁层。
 */

import type {
  WebSocketMessage,
  WebSocketSendResult,
} from "@/types/system/websocket";

type SessionBindingLease = object;

interface SessionBindMessage extends WebSocketMessage {
  session_key: string;
  type: "bind_session";
}

interface SessionBindingEntry {
  leases: Map<SessionBindingLease, SessionBindMessage>;
}

type SendMessage = (message: WebSocketMessage) => WebSocketSendResult;

export class SessionBindingLeaseRegistry {
  private readonly bindings = new Map<string, SessionBindingEntry>();

  constructor(
    private readonly send: SendMessage,
    private readonly isConnected: () => boolean,
  ) {}

  acquire(
    lease: SessionBindingLease,
    message: WebSocketMessage,
  ): () => void {
    return this.retain(lease, message, true);
  }

  /** 请求传输租约只延长已有 binding，不为同一消费者制造额外 replay。 */
  retain(
    lease: SessionBindingLease,
    message: WebSocketMessage,
    announce = false,
  ): () => void {
    const binding = parseSessionBindMessage(message);
    if (!binding) {
      return () => {};
    }

    const entry = this.bindings.get(binding.session_key) ?? {
      leases: new Map<SessionBindingLease, SessionBindMessage>(),
    };
    entry.leases.set(lease, binding);
    this.bindings.set(binding.session_key, entry);

    // 每个新消费者都主动 bind 一次，让服务端把当前 pending 请求重放给
    // 刚挂载的订阅者；后续 release 仍由引用计数保护，不会误解绑其他消费者。
    if (announce && this.isConnected()) {
      this.send(binding);
    }

    let released = false;
    return () => {
      if (released) {
        return;
      }
      released = true;
      this.release(binding.session_key, lease);
    };
  }

  replay(): void {
    if (!this.isConnected()) {
      return;
    }
    for (const entry of this.bindings.values()) {
      const binding = Array.from(entry.leases.values()).at(-1);
      if (binding) {
        this.send(binding);
      }
    }
  }

  private release(
    sessionKey: string,
    lease: SessionBindingLease,
  ): void {
    const entry = this.bindings.get(sessionKey);
    if (!entry || !entry.leases.delete(lease)) {
      return;
    }
    if (entry.leases.size > 0) {
      return;
    }

    this.bindings.delete(sessionKey);
    if (this.isConnected()) {
      this.send({
        type: "unbind_session",
        session_key: sessionKey,
      });
    }
  }
}

function parseSessionBindMessage(
  message: WebSocketMessage,
): SessionBindMessage | null {
  const sessionKey = typeof message.session_key === "string"
    ? message.session_key.trim()
    : "";
  if (message.type !== "bind_session" || !sessionKey) {
    return null;
  }
  return {
    ...message,
    session_key: sessionKey,
    type: "bind_session",
  };
}
