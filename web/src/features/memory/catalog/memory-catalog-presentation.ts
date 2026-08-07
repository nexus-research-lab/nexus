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
