import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderConfigRecord,
  ProviderPreset,
} from "@/types/capability/provider";

import {
  getProviderDraftError,
  providerDraftHasChanges,
} from "../../model/provider-config-model";
import type { ProviderDraft } from "../../model/provider-settings-types";

export interface ProviderPersistResult {
  changed: boolean;
  record: ProviderConfigRecord;
}

export interface ProviderPersistenceSnapshot {
  currentPreset: ProviderPreset | null;
  draft: ProviderDraft;
  isCreating: boolean;
  isEditing: boolean;
  isEmptyMode: boolean;
  selectedRecord: ProviderConfigRecord | null;
}

export type ProviderPersistenceStop =
  | { kind: "invalid"; message: string }
  | { kind: "return"; result: ProviderPersistResult | null };

type PersistenceStopRule = () => ProviderPersistenceStop | null;

export function resolveProviderPersistenceStop(
  snapshot: ProviderPersistenceSnapshot,
  translate: I18nContextValue["t"],
): ProviderPersistenceStop | null {
  const rules: PersistenceStopRule[] = [
    () => stopWhenEmpty(snapshot),
    () => stopWhenReadOnly(snapshot),
    () => stopWhenInvalid(snapshot, translate),
    () => stopWhenUnchanged(snapshot),
  ];
  for (const rule of rules) {
    const stop = rule();
    if (stop) {
      return stop;
    }
  }
  return null;
}

function stopWhenEmpty(
  snapshot: ProviderPersistenceSnapshot,
): ProviderPersistenceStop | null {
  return snapshot.isEmptyMode ? { kind: "return", result: null } : null;
}

function stopWhenReadOnly(
  snapshot: ProviderPersistenceSnapshot,
): ProviderPersistenceStop | null {
  if (!snapshot.isEditing || snapshot.selectedRecord?.can_manage !== false) {
    return null;
  }
  return {
    kind: "return",
    result: { changed: false, record: snapshot.selectedRecord },
  };
}

function stopWhenInvalid(
  snapshot: ProviderPersistenceSnapshot,
  translate: I18nContextValue["t"],
): ProviderPersistenceStop | null {
  const message = getProviderDraftError(
    snapshot.draft,
    snapshot.currentPreset,
    snapshot.isCreating,
    translate,
  );
  return message ? { kind: "invalid", message } : null;
}

function stopWhenUnchanged(
  snapshot: ProviderPersistenceSnapshot,
): ProviderPersistenceStop | null {
  if (
    !snapshot.isEditing
    || providerDraftHasChanges(
      snapshot.draft,
      snapshot.selectedRecord,
      snapshot.currentPreset,
    )
  ) {
    return null;
  }
  return {
    kind: "return",
    result: snapshot.selectedRecord
      ? { changed: false, record: snapshot.selectedRecord }
      : null,
  };
}
