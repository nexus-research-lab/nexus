"use client";

import {
  getDesktopPersistentState,
  isDesktopBridgeAvailable,
  removeDesktopPersistentState,
  setDesktopPersistentState,
} from "@/lib/desktop-bridge";

const TOUR_COMPLETION_STORAGE_KEY = "nexus:onboarding:tours";
const TOUR_DISMISS_STORAGE_KEY = "nexus:onboarding:dismissed-tours";
const TOUR_PENDING_REQUEST_STORAGE_KEY = "nexus:onboarding:pending-tour";
const SIDEBAR_HINT_DISMISSED_STORAGE_KEY = "nexus:sidebar-onboarding-dismissed";

const DESKTOP_COMPLETED_TOURS_KEY = "onboarding.completed_tours";
const DESKTOP_DISMISSED_TOURS_KEY = "onboarding.dismissed_tours";
const DESKTOP_SIDEBAR_HINT_KEY = "onboarding.sidebar_hint_dismissed";

export interface HydratedOnboardingState {
  completedTours: Record<string, boolean>;
}

function readBooleanMap(storageKey: string): Record<string, boolean> {
  if (typeof window === "undefined") {
    return {};
  }

  try {
    const raw = window.localStorage.getItem(storageKey);
    if (!raw) {
      return {};
    }
    const parsed = JSON.parse(raw) as Record<string, boolean>;
    return parsed && typeof parsed === "object" ? normalizeBooleanMap(parsed) : {};
  } catch (err) {
    console.debug("[tour-state] Failed to read storage:", storageKey, err);
    return {};
  }
}

function writeBooleanMap(
  storageKey: string,
  nextValue: Record<string, boolean>,
) {
  if (typeof window === "undefined") {
    return;
  }

  window.localStorage.setItem(storageKey, JSON.stringify(normalizeBooleanMap(nextValue)));
}

function normalizeBooleanMap(value: Record<string, boolean>): Record<string, boolean> {
  return Object.fromEntries(
    Object.entries(value).filter((entry): entry is [string, boolean] => (
      typeof entry[0] === "string" && entry[0].trim().length > 0 && entry[1] === true
    )),
  );
}

function persistDesktopValue(key: string, value: string) {
  if (!isDesktopBridgeAvailable()) {
    return;
  }
  void setDesktopPersistentState(key, value).catch(() => {});
}

function removeDesktopValue(key: string) {
  if (!isDesktopBridgeAvailable()) {
    return;
  }
  void removeDesktopPersistentState(key).catch(() => {});
}

async function readDesktopBooleanMap(key: string): Promise<Record<string, boolean> | null> {
  if (!isDesktopBridgeAvailable()) {
    return null;
  }

  const result = await getDesktopPersistentState(key);
  if (!result.value) {
    return null;
  }
  try {
    const parsed = JSON.parse(result.value) as Record<string, boolean>;
    return parsed && typeof parsed === "object" ? normalizeBooleanMap(parsed) : {};
  } catch {
    return {};
  }
}

async function readDesktopBoolean(key: string): Promise<boolean | null> {
  if (!isDesktopBridgeAvailable()) {
    return null;
  }

  const result = await getDesktopPersistentState(key);
  if (result.value === null || typeof result.value === "undefined") {
    return null;
  }
  return result.value === "true";
}

export async function hydrateOnboardingStateFromDesktop(): Promise<HydratedOnboardingState> {
  const localCompletedTours = readCompletedTours();
  if (!isDesktopBridgeAvailable()) {
    return { completedTours: localCompletedTours };
  }

  try {
    const [desktopCompletedTours, desktopDismissedTours, desktopSidebarHintDismissed] = await Promise.all([
      readDesktopBooleanMap(DESKTOP_COMPLETED_TOURS_KEY),
      readDesktopBooleanMap(DESKTOP_DISMISSED_TOURS_KEY),
      readDesktopBoolean(DESKTOP_SIDEBAR_HINT_KEY),
    ]);

    const completedTours = {
      ...localCompletedTours,
      ...(desktopCompletedTours ?? {}),
    };
    const dismissedTours = {
      ...readDismissedTours(),
      ...(desktopDismissedTours ?? {}),
    };

    writeBooleanMap(TOUR_COMPLETION_STORAGE_KEY, completedTours);
    writeBooleanMap(TOUR_DISMISS_STORAGE_KEY, dismissedTours);

    if (Object.keys(completedTours).length > 0) {
      persistDesktopValue(DESKTOP_COMPLETED_TOURS_KEY, JSON.stringify(completedTours));
    }
    if (Object.keys(dismissedTours).length > 0) {
      persistDesktopValue(DESKTOP_DISMISSED_TOURS_KEY, JSON.stringify(dismissedTours));
    }

    if (desktopSidebarHintDismissed === true) {
      window.localStorage.setItem(SIDEBAR_HINT_DISMISSED_STORAGE_KEY, "true");
    } else if (window.localStorage.getItem(SIDEBAR_HINT_DISMISSED_STORAGE_KEY) === "true") {
      persistDesktopValue(DESKTOP_SIDEBAR_HINT_KEY, "true");
    }

    return { completedTours: completedTours };
  } catch {
    return { completedTours: localCompletedTours };
  }
}

export function readCompletedTours(): Record<string, boolean> {
  return readBooleanMap(TOUR_COMPLETION_STORAGE_KEY);
}

export function writeCompletedTours(nextValue: Record<string, boolean>) {
  const normalized = normalizeBooleanMap(nextValue);
  writeBooleanMap(TOUR_COMPLETION_STORAGE_KEY, normalized);
  persistDesktopValue(DESKTOP_COMPLETED_TOURS_KEY, JSON.stringify(normalized));
}

function readDismissedTours(): Record<string, boolean> {
  return readBooleanMap(TOUR_DISMISS_STORAGE_KEY);
}

function writeDismissedTours(nextValue: Record<string, boolean>) {
  const normalized = normalizeBooleanMap(nextValue);
  writeBooleanMap(TOUR_DISMISS_STORAGE_KEY, normalized);
  persistDesktopValue(DESKTOP_DISMISSED_TOURS_KEY, JSON.stringify(normalized));
}

export function isTourDismissed(tourId: string): boolean {
  return Boolean(readDismissedTours()[tourId]);
}

export function setTourDismissed(tourId: string, dismissed: boolean) {
  const nextValue = readDismissedTours();
  if (dismissed) {
    nextValue[tourId] = true;
  } else {
    delete nextValue[tourId];
  }
  writeDismissedTours(nextValue);
}

export function readRequestedTourId(): string | null {
  if (typeof window === "undefined") {
    return null;
  }

  const raw = window.localStorage.getItem(TOUR_PENDING_REQUEST_STORAGE_KEY);
  return raw?.trim() || null;
}

export function setRequestedTourId(tourId: string) {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(TOUR_PENDING_REQUEST_STORAGE_KEY, tourId);
}

export function clearRequestedTourId(expectedTourId?: string) {
  if (typeof window === "undefined") {
    return;
  }

  if (!expectedTourId) {
    window.localStorage.removeItem(TOUR_PENDING_REQUEST_STORAGE_KEY);
    return;
  }

  const currentTourId = readRequestedTourId();
  if (currentTourId === expectedTourId) {
    window.localStorage.removeItem(TOUR_PENDING_REQUEST_STORAGE_KEY);
  }
}

export function resetAllTourState() {
  if (typeof window === "undefined") {
    return;
  }

  window.localStorage.removeItem(TOUR_COMPLETION_STORAGE_KEY);
  window.localStorage.removeItem(TOUR_DISMISS_STORAGE_KEY);
  window.localStorage.removeItem(TOUR_PENDING_REQUEST_STORAGE_KEY);
  window.localStorage.removeItem(SIDEBAR_HINT_DISMISSED_STORAGE_KEY);
  removeDesktopValue(DESKTOP_COMPLETED_TOURS_KEY);
  removeDesktopValue(DESKTOP_DISMISSED_TOURS_KEY);
  removeDesktopValue(DESKTOP_SIDEBAR_HINT_KEY);
}
