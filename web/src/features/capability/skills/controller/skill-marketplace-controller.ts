import type { RefObject } from "react";

import type {
  ExternalSkillSearchItem,
  ExternalSkillSourceInfo,
  ExternalSkillSourceStatus,
  SkillInfo,
} from "@/types/capability/skill";

import type { SkillUpdateCheckNotice } from "./skill-update-check-model";

export type DiscoveryMode = "catalog" | "external";
export type SkillImportDialogMode = "local" | "git";
export type SkillMarketplaceFeedbackTone = "error" | "success" | "warning";

export interface SkillMarketplaceFeedback {
  action?: {
    label: string;
    onClick: () => void;
  };
  dismiss: () => void;
  impact?: string;
  message: string;
  nextStep?: string;
  pending: boolean;
  persistent?: boolean;
  title?: string;
  tone: SkillMarketplaceFeedbackTone;
}

export interface SkillMarketplaceFeedbackInput extends Omit<
  SkillMarketplaceFeedback,
  "dismiss"
> {}

export interface SkillMarketplaceFeedbackActions {
  clear: () => void;
  error: (message: string) => void;
  report: (feedback: SkillMarketplaceFeedbackInput) => void;
  start: (message: string) => void;
  success: (message: string) => void;
  warning: (message: string) => void;
}

export interface SkillCatalogController {
  activeCategory: string;
  categories: Array<{ key: string; label: string }>;
  groupedSkills: Array<[string, SkillInfo[]]>;
  importedExternalSources: Map<string, Set<string>>;
  loading: boolean;
  query: string;
  refresh: () => Promise<boolean>;
  setActiveCategory: (category: string) => void;
  setQuery: (query: string) => void;
  skills: SkillInfo[];
  updateAvailableSkills: SkillInfo[];
}

export interface ExternalSkillSearchController {
  closePreview: () => void;
  loading: boolean;
  preview: (item: ExternalSkillSearchItem) => Promise<void>;
  previewItem: ExternalSkillSearchItem | null;
  previewLoading: boolean;
  query: string;
  results: ExternalSkillSearchItem[];
  setQuery: (query: string) => void;
  setSourceId: (sourceId: string) => void;
  sourceId: string;
  sourceStatuses: ExternalSkillSourceStatus[];
  submit: () => void;
  submittedQuery: string;
}

export interface PrivateSkillSourceDraft {
  authType: "none" | "bearer";
  name: string;
  token: string;
  url: string;
}

export interface ExternalSkillSourcesController {
  closeManager: () => void;
  items: ExternalSkillSourceInfo[];
  loading: boolean;
  managerOpen: boolean;
  openManager: () => void;
  revision: number;
  remove: (source: ExternalSkillSourceInfo) => Promise<void>;
  save: (
    source: ExternalSkillSourceInfo | null,
    draft: PrivateSkillSourceDraft,
  ) => Promise<boolean>;
  toggle: (source: ExternalSkillSourceInfo, enabled: boolean) => Promise<void>;
}

export interface SkillOperationsController {
  busyExternalKeys: ReadonlySet<string>;
  busySkillNames: ReadonlySet<string>;
  checkUpdateNotice: SkillUpdateCheckNotice | null;
  checkUpdates: () => Promise<void>;
  checkingUpdates: boolean;
  deleteSkill: (skill: SkillInfo) => Promise<boolean>;
  fileInputRef: RefObject<HTMLInputElement | null>;
  importDialogMode: SkillImportDialogMode | null;
  importExternal: (item: ExternalSkillSearchItem) => Promise<void>;
  importGit: (url: string, branch?: string, path?: string) => Promise<void>;
  importLocal: (file: File) => Promise<void>;
  importing: boolean;
  lastUpdateCheckedAt: number | null;
  setImportDialogMode: (mode: SkillImportDialogMode | null) => void;
  updateSkill: (skillName: string) => Promise<boolean>;
}

export interface SkillMarketplaceController {
  catalog: SkillCatalogController;
  discoveryMode: DiscoveryMode;
  external: ExternalSkillSearchController;
  feedback: SkillMarketplaceFeedback | null;
  operations: SkillOperationsController;
  setDiscoveryMode: (mode: DiscoveryMode) => void;
  sources: ExternalSkillSourcesController;
}
