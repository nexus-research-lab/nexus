/**
 * INPUT: MemoryDocument 类型、标题、摘要与路径。
 * OUTPUT: 统一类型图标和面向用户的主显示标题。
 * POS: 记忆目录与文档头共用的纯展示模型。
 */
import {
  BookOpenText,
  FileText,
  FolderKanban,
  History,
  Link2,
  MessageSquareWarning,
  UserRound,
  type LucideIcon,
} from "lucide-react";

import type { MemoryDocument } from "@/types/memory/memory";

type MemoryPresentationKey = "index" | "daily_log" | "user" | "feedback" | "project" | "reference" | "topic";

const ICON_BY_KEY: Readonly<Record<MemoryPresentationKey, LucideIcon>> = {
  daily_log: History,
  feedback: MessageSquareWarning,
  index: BookOpenText,
  project: FolderKanban,
  reference: Link2,
  topic: FileText,
  user: UserRound,
};

export function getMemoryDocumentIcon(
  document: MemoryDocument,
): LucideIcon {
  const key = document.kind === "topic"
    ? document.type || "topic"
    : document.kind;
  return ICON_BY_KEY[key];
}

export function getMemoryDocumentDisplayTitle(
  document: MemoryDocument,
): string {
  const description = document.description?.trim();
  return description && description !== document.path
    ? description
    : document.title;
}
