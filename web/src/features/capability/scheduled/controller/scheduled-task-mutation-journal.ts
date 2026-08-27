// INPUT: 当前 owner scope、Scheduled exact command identity 与非敏感对账期望。
// OUTPUT: localStorage 中按 owner/intent 独立保存、跨标签页通知且不会按时间淘汰的 pending/unconfirmed journal。
// POS: Scheduled 页面、标签页与桌面 App 重启恢复边界；不保存任务正文、请求头或 HTTP trace ID。

import {
  SCHEDULED_TASK_COMMAND_KINDS,
  scheduledTaskCommandTargetsJob,
  type ScheduledTaskCommandKind,
  type ScheduledTaskReconcileExpectation,
} from "./scheduled-task-directory-model";

const JOURNAL_VERSION = 3;
const STORAGE_PREFIX = "nexus:scheduled-mutation-journal:v3:";

export interface ScheduledTaskMutationJournalEntry {
  command: ScheduledTaskCommandKind;
  expectation?: ScheduledTaskReconcileExpectation;
  phase: "pending" | "unconfirmed";
  targetId: string;
  updatedAt: number;
}

interface StoredMutationEntry extends ScheduledTaskMutationJournalEntry {
  version: typeof JOURNAL_VERSION;
}

interface StoredCreateIntent {
  requestId: string;
  updatedAt: number;
  version: typeof JOURNAL_VERSION;
}

function storage(): Storage | null {
  try {
    return typeof window === "undefined" ? null : window.localStorage;
  } catch {
    return null;
  }
}

function scopePrefix(scopeKey: string): string {
  return `${STORAGE_PREFIX}${encodeURIComponent(scopeKey)}:`;
}

function entryPrefix(scopeKey: string): string {
  return `${scopePrefix(scopeKey)}entry:`;
}

function createPrefix(scopeKey: string): string {
  return `${scopePrefix(scopeKey)}create:`;
}

function entryKey(
  scopeKey: string,
  command: ScheduledTaskCommandKind,
  targetId: string,
): string {
  return `${entryPrefix(scopeKey)}${encodeURIComponent(command)}:${encodeURIComponent(targetId)}`;
}

function createKey(scopeKey: string, requestId: string): string {
  return `${createPrefix(scopeKey)}${encodeURIComponent(requestId)}`;
}

function storageKeys(target: Storage, prefix: string): string[] {
  const keys: string[] = [];
  for (let index = 0; index < target.length; index += 1) {
    const key = target.key(index);
    if (key?.startsWith(prefix)) {
      keys.push(key);
    }
  }
  return keys;
}

function parseJSON(target: Storage, key: string): unknown {
  try {
    return JSON.parse(target.getItem(key) ?? "null");
  } catch {
    return null;
  }
}

function isJournalEntry(value: unknown): value is StoredMutationEntry {
  if (!value || typeof value !== "object") {
    return false;
  }
  const entry = value as Partial<StoredMutationEntry>;
  return entry.version === JOURNAL_VERSION
    && typeof entry.targetId === "string"
    && (entry.phase === "pending" || entry.phase === "unconfirmed")
    && typeof entry.updatedAt === "number"
    && SCHEDULED_TASK_COMMAND_KINDS.includes(
      entry.command as ScheduledTaskCommandKind,
    );
}

function isCreateIntent(value: unknown): value is StoredCreateIntent {
  if (!value || typeof value !== "object") {
    return false;
  }
  const intent = value as Partial<StoredCreateIntent>;
  return intent.version === JOURNAL_VERSION
    && typeof intent.requestId === "string"
    && intent.requestId.trim() !== ""
    && typeof intent.updatedAt === "number";
}

export function loadScheduledTaskMutationJournal(
  scopeKey: string | null,
): ScheduledTaskMutationJournalEntry[] {
  const target = storage();
  if (!scopeKey || !target) {
    return [];
  }
  return storageKeys(target, entryPrefix(scopeKey))
    .flatMap((key) => {
      const entry = parseJSON(target, key);
      return isJournalEntry(entry) ? [entry] : [];
    })
    .sort((left, right) => (
      left.updatedAt - right.updatedAt
      || left.command.localeCompare(right.command)
      || left.targetId.localeCompare(right.targetId)
    ));
}

export function upsertScheduledTaskMutationJournalEntry(
  scopeKey: string | null,
  entry: ScheduledTaskMutationJournalEntry,
): boolean {
  const target = storage();
  if (!scopeKey || !target) {
    return false;
  }
  try {
    target.setItem(entryKey(scopeKey, entry.command, entry.targetId), JSON.stringify({
      ...entry,
      version: JOURNAL_VERSION,
    } satisfies StoredMutationEntry));
    return true;
  } catch {
    // 调用方必须在副作用发出前看到失败并停止；不能静默失去重载保护。
    return false;
  }
}

export function removeScheduledTaskMutationJournalEntry(
  scopeKey: string | null,
  command: ScheduledTaskCommandKind,
  targetId: string,
): void {
  const target = storage();
  if (!scopeKey || !target) {
    return;
  }
  try {
    target.removeItem(entryKey(scopeKey, command, targetId));
  } catch {
    // 存储不可用时保留当前内存保护；后续重载不会错误制造“已确认”。
  }
}

export function clearScheduledTaskMutationJournal(scopeKey: string | null): void {
  const target = storage();
  if (!scopeKey || !target) {
    return;
  }
  try {
    storageKeys(target, scopePrefix(scopeKey)).forEach((key) => target.removeItem(key));
  } catch {
    // 与写入一致：存储不可用时不尝试推断或改写结果。
  }
}

export function loadScheduledTaskCreateRequestIds(
  scopeKey: string | null,
): string[] {
  const target = storage();
  if (!scopeKey || !target) {
    return [];
  }
  return storageKeys(target, createPrefix(scopeKey))
    .flatMap((key) => {
      const intent = parseJSON(target, key);
      return isCreateIntent(intent) ? [intent] : [];
    })
    .sort((left, right) => (
      left.updatedAt - right.updatedAt || left.requestId.localeCompare(right.requestId)
    ))
    .map((intent) => intent.requestId);
}

export function loadScheduledTaskCreateRequestId(
  scopeKey: string | null,
): string | null {
  return loadScheduledTaskCreateRequestIds(scopeKey)[0] ?? null;
}

export function saveScheduledTaskCreateRequestId(
  scopeKey: string | null,
  requestId: string,
): boolean {
  const target = storage();
  requestId = requestId.trim();
  if (!scopeKey || !target || !requestId) {
    return false;
  }
  try {
    target.setItem(createKey(scopeKey, requestId), JSON.stringify({
      requestId,
      updatedAt: Date.now(),
      version: JOURNAL_VERSION,
    } satisfies StoredCreateIntent));
    return true;
  } catch {
    return false;
  }
}

export function clearScheduledTaskCreateRequestId(
  scopeKey: string | null,
  requestId: string,
): void {
  const target = storage();
  requestId = requestId.trim();
  if (!scopeKey || !target || !requestId) {
    return;
  }
  try {
    target.removeItem(createKey(scopeKey, requestId));
  } catch {
    // 保留持久记录比误清一个结果未知的创建意图更安全。
  }
}

export function subscribeScheduledTaskMutationJournal(
  scopeKey: string | null,
  listener: () => void,
): () => void {
  if (!scopeKey || typeof window === "undefined" || !window.addEventListener) {
    return () => undefined;
  }
  const prefix = scopePrefix(scopeKey);
  const handleStorage = (event: StorageEvent): void => {
    if (event.storageArea === window.localStorage && (
      event.key === null || event.key.startsWith(prefix)
    )) {
      listener();
    }
  };
  window.addEventListener("storage", handleStorage);
  return () => window.removeEventListener("storage", handleStorage);
}

export class ScheduledTaskMutationLockUnavailableError extends Error {
  constructor() {
    super("另一个页面正在处理这个任务，请等它完成后再刷新");
    this.name = "ScheduledTaskMutationLockUnavailableError";
  }
}

export class ScheduledTaskMutationCoordinationUnavailableError extends Error {
  constructor() {
    super("当前运行环境无法安全协调多个窗口中的任务操作");
    this.name = "ScheduledTaskMutationCoordinationUnavailableError";
  }
}

export async function withScheduledTaskMutationLock<Result>(
  scopeKey: string | null,
  jobId: string,
  execute: () => Promise<Result>,
): Promise<Result> {
  const lockManager = typeof navigator === "undefined" ? null : navigator.locks;
  if (!scopeKey || !jobId.trim() || !lockManager) {
    // 没有 Web Locks 时不能用 localStorage lease 假装互斥：冻结页面或
    // lease 超时恢复会产生 split-brain。受支持宿主都提供 Web Locks；
    // 其他环境宁可不发送，也不能让两个 request identity 同时生效。
    throw new ScheduledTaskMutationCoordinationUnavailableError();
  }
  const lockName = `nexus:scheduled-task:${encodeURIComponent(scopeKey)}:${encodeURIComponent(jobId)}`;
  return lockManager.request(
    lockName,
    { ifAvailable: true, mode: "exclusive" },
    async (lock) => {
      if (!lock) {
        throw new ScheduledTaskMutationLockUnavailableError();
      }
      return execute();
    },
  );
}

export function withScheduledTaskMutationGate<Result>(
  scopeKey: string | null,
  jobId: string,
  execute: () => Promise<Result>,
  ignoreUnconfirmedCommands?: ReadonlySet<ScheduledTaskCommandKind>,
): Promise<Result> {
  return withScheduledTaskMutationLock(scopeKey, jobId, async () => {
    const persistedConflict = loadScheduledTaskMutationJournal(scopeKey)
      .some((entry) => (
        scheduledTaskCommandTargetsJob(entry.targetId, jobId)
        && !ignoreUnconfirmedCommands?.has(entry.command)
      ));
    if (persistedConflict) {
      throw new ScheduledTaskMutationLockUnavailableError();
    }
    return execute();
  });
}
