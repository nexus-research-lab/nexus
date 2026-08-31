/**
 * INPUT: 当前认证 owner scope 与一个非敏感 Agent 创建 request ID。
 * OUTPUT: 跨关闭/重载保留的 exact 创建状态与跨标签页互斥。
 * POS: Agent 创建副作用发出前的 owner-scoped 恢复边界；不保存表单、digest、秘密或 HTTP 请求号。
 */

const JOURNAL_VERSION = 1;
const STORAGE_PREFIX = "nexus:agent-creation-journal:v1:";

export type AgentCreationJournalStatus = "pending" | "unconfirmed";

export interface AgentCreationJournalEntry {
  requestId: string;
  status: AgentCreationJournalStatus;
}

interface StoredAgentCreationJournalEntry extends AgentCreationJournalEntry {
  version: typeof JOURNAL_VERSION;
}

export interface AgentCreationJournalSnapshot {
  available: boolean;
  entry: AgentCreationJournalEntry | null;
}

function journalKey(ownerScope: string): string {
  return `${STORAGE_PREFIX}${encodeURIComponent(ownerScope)}`;
}

function localStorageTarget(): Storage | null {
  try {
    return typeof window === "undefined" ? null : window.localStorage;
  } catch {
    return null;
  }
}

export function readAgentCreationJournal(
  ownerScope: string | null,
): AgentCreationJournalSnapshot {
  const target = localStorageTarget();
  if (!ownerScope || !target) {
    return { available: false, entry: null };
  }
  try {
    const raw = target.getItem(journalKey(ownerScope));
    if (raw === null) {
      return { available: true, entry: null };
    }
    const value = JSON.parse(raw) as Partial<StoredAgentCreationJournalEntry> | null;
    if (
      value?.version !== JOURNAL_VERSION
      || typeof value.requestId !== "string"
      || !value.requestId.trim()
      || (value.status !== "pending" && value.status !== "unconfirmed")
    ) {
      return { available: false, entry: null };
    }
    return {
      available: true,
      entry: { requestId: value.requestId.trim(), status: value.status },
    };
  } catch {
    return { available: false, entry: null };
  }
}

export function writeAgentCreationJournal(
  ownerScope: string | null,
  entry: AgentCreationJournalEntry,
): boolean {
  const target = localStorageTarget();
  const requestId = entry.requestId.trim();
  if (!ownerScope || !target || !requestId) {
    return false;
  }
  try {
    const key = journalKey(ownerScope);
    const serialized = JSON.stringify({
      requestId,
      status: entry.status,
      version: JOURNAL_VERSION,
    } satisfies StoredAgentCreationJournalEntry);
    target.setItem(key, serialized);
    return target.getItem(key) === serialized;
  } catch {
    return false;
  }
}

export function clearAgentCreationJournal(ownerScope: string | null): boolean {
  const target = localStorageTarget();
  if (!ownerScope || !target) {
    return false;
  }
  try {
    const key = journalKey(ownerScope);
    target.removeItem(key);
    return target.getItem(key) === null;
  } catch {
    return false;
  }
}

export function createAgentCreationRequestId(): string | null {
  try {
    if (typeof crypto === "undefined") {
      return null;
    }
    if (typeof crypto.randomUUID === "function") {
      return `web-create:${crypto.randomUUID()}`;
    }
    const bytes = new Uint8Array(16);
    crypto.getRandomValues(bytes);
    const token = Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("");
    return `web-create:${token}`;
  } catch {
    return null;
  }
}

export async function withAgentCreationJournalLock<Result>(
  ownerScope: string | null,
  execute: () => Promise<Result>,
): Promise<Result> {
  const locks = typeof navigator === "undefined" ? null : navigator.locks;
  if (!ownerScope || !locks) {
    throw new AgentCreationCoordinationUnavailableError();
  }
  let started = false;
  try {
    return await locks.request(
      `nexus:agent-create:${ownerScope}`,
      { mode: "exclusive" },
      async () => {
        started = true;
        return execute();
      },
    );
  } catch (error) {
    if (!started) {
      throw new AgentCreationCoordinationUnavailableError();
    }
    throw error;
  }
}

export class AgentCreationCoordinationUnavailableError extends Error {
  constructor() {
    super("当前环境无法安全保留 Agent 创建记录");
    this.name = "AgentCreationCoordinationUnavailableError";
  }
}
