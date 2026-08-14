/**
 * INPUT: 已发送请求的 client_request_id、原 Session binding 与共享 WebSocket 原始事件。
 * OUTPUT: 跨 React 路由生命周期持有的传输租约，以及 exact ACK/error 终态收口。
 * POS: 共享 Socket 的请求级生命周期层；不投影会话 UI，也不解释业务结果正文。
 */

import { parseEventMessage } from "./protocol/event-message";
import type { WebSocketMessage } from "@/types/system/websocket";

const REQUEST_ACK_EVENT_TYPES = new Set([
  "chat_ack",
  "input_queue_ack",
  "interrupt_ack",
]);

export interface RequestTransportLeaseOptions {
  clientRequestId: string;
  onAccepted: () => void;
  onRejected: (reason: string) => void;
  onTimeout?: () => void;
  sessionBinding: WebSocketMessage;
  timeoutMs?: number;
}

export type RequestTransportSettlement =
  | {
      clientRequestId: string;
      kind: "accepted";
    }
  | {
      clientRequestId: string;
      kind: "rejected";
      reason: string;
    };

interface ActiveRequestTransportLease {
  clientRequestId: string;
  onAccepted: () => void;
  onRejected: (reason: string) => void;
  releaseSessionBinding: () => void;
  released: boolean;
  timeoutId: ReturnType<typeof globalThis.setTimeout> | null;
}

type RetainSessionBinding = (
  lease: object,
  binding: WebSocketMessage,
) => () => void;

/**
 * 请求租约由共享通道持有，而不是由页面组件持有。页面卸载后原 Session
 * 仍保持 bind，ACK/error 也能按 exact request identity 收口原 Promise。
 */
export class RequestTransportLeaseRegistry {
  private readonly leases = new Map<string, ActiveRequestTransportLease>();

  constructor(
    private readonly retainSessionBinding: RetainSessionBinding,
    private readonly onIdle: () => void,
  ) {}

  acquire(options: RequestTransportLeaseOptions): () => void {
    const clientRequestId = options.clientRequestId.trim();
    if (!clientRequestId) {
      return () => {};
    }

    const existing = this.leases.get(clientRequestId);
    if (existing) {
      // client_request_id 是单 owner identity；重复 acquire 既不替换回调，
      // 也不能把第二个调用者的 cleanup 变成原 owner 的 release 权限。
      return () => {};
    }

    const sessionLease = {};
    const active: ActiveRequestTransportLease = {
      clientRequestId,
      onAccepted: options.onAccepted,
      onRejected: options.onRejected,
      releaseSessionBinding: this.retainSessionBinding(
        sessionLease,
        options.sessionBinding,
      ),
      released: false,
      timeoutId: null,
    };
    this.leases.set(clientRequestId, active);
    if (options.timeoutMs && options.timeoutMs > 0) {
      active.timeoutId = globalThis.setTimeout(() => {
        this.release(active);
        options.onTimeout?.();
      }, options.timeoutMs);
    }
    return () => this.release(active);
  }

  handleMessage(message: unknown): boolean {
    const settlement = parseRequestTransportSettlement(message);
    if (!settlement) {
      return false;
    }
    const active = this.leases.get(settlement.clientRequestId);
    if (!active) {
      return false;
    }

    // 先撤销所有权，回调重入或普通 subscriber 随后再次消费同一 ACK
    // 都只能得到幂等 no-op。
    this.release(active);
    if (settlement.kind === "accepted") {
      active.onAccepted();
    } else {
      active.onRejected(settlement.reason);
    }
    return true;
  }

  hasLeases(): boolean {
    return this.leases.size > 0;
  }

  private release(active: ActiveRequestTransportLease): void {
    if (active.released) {
      return;
    }
    active.released = true;
    if (active.timeoutId !== null) {
      globalThis.clearTimeout(active.timeoutId);
      active.timeoutId = null;
    }
    if (this.leases.get(active.clientRequestId) === active) {
      this.leases.delete(active.clientRequestId);
    }
    active.releaseSessionBinding();
    if (this.leases.size === 0) {
      this.onIdle();
    }
  }
}

export function parseRequestTransportSettlement(
  message: unknown,
): RequestTransportSettlement | null {
  const event = parseEventMessage(message);
  const data = event?.data;
  const eventType = event?.event_type;
  const clientRequestId = readTrimmedString(data?.client_request_id);
  if (!eventType || !clientRequestId) {
    return null;
  }
  if (
    eventType === "chat_ack"
    || (
      REQUEST_ACK_EVENT_TYPES.has(eventType)
      && data?.accepted === true
    )
  ) {
    return {
      clientRequestId,
      kind: "accepted",
    };
  }
  if (
    REQUEST_ACK_EVENT_TYPES.has(eventType)
    && data?.accepted === false
  ) {
    return {
      clientRequestId,
      kind: "rejected",
      reason: readTrimmedString(data?.message) || "请求未被后端受理",
    };
  }
  if (eventType !== "error") {
    return null;
  }
  return {
    clientRequestId,
    kind: "rejected",
    reason: readTrimmedString(data?.message) || "请求被后端拒绝",
  };
}

function readTrimmedString(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}
