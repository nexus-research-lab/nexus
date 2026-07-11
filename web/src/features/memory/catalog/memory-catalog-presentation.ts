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

import type { TranslationKey } from "@/shared/i18n/messages";
import type { MemoryDocument } from "@/types/memory/memory";

interface MemoryDocumentPresentation {
  icon: LucideIcon;
  labelKey: TranslationKey;
  tone: string;
}

type MemoryPresentationKey = "index" | "daily_log" | "user" | "feedback" | "project" | "reference" | "topic";

const PRESENTATION_BY_KEY: Readonly<Record<MemoryPresentationKey, MemoryDocumentPresentation>> = {
  index: {
    icon: BookOpenText,
    labelKey: "capability.memory_index",
    tone: "bg-sky-500/10 text-sky-600 dark:text-sky-400",
  },
  daily_log: {
    icon: History,
    labelKey: "capability.memory_type_daily_log",
    tone: "bg-zinc-500/10 text-zinc-600 dark:text-zinc-400",
  },
  user: {
    icon: UserRound,
    labelKey: "capability.memory_type_user",
    tone: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
  },
  feedback: {
    icon: MessageSquareWarning,
    labelKey: "capability.memory_type_feedback",
    tone: "bg-amber-500/10 text-amber-700 dark:text-amber-400",
  },
  project: {
    icon: FolderKanban,
    labelKey: "capability.memory_type_project",
    tone: "bg-blue-500/10 text-blue-600 dark:text-blue-400",
  },
  reference: {
    icon: Link2,
    labelKey: "capability.memory_type_reference",
    tone: "bg-rose-500/10 text-rose-600 dark:text-rose-400",
  },
  topic: {
    icon: FileText,
    labelKey: "capability.memory_type_topic",
    tone: "bg-zinc-500/10 text-zinc-600 dark:text-zinc-400",
  },
};

export function getMemoryDocumentPresentation(
  document: MemoryDocument,
): MemoryDocumentPresentation {
  const key = document.kind === "topic"
    ? document.type || "topic"
    : document.kind;
  return PRESENTATION_BY_KEY[key];
}
