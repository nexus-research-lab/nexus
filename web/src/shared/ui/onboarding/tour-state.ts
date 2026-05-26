"use client";

import {
  get_desktop_persistent_state,
  is_desktop_bridge_available,
  remove_desktop_persistent_state,
  set_desktop_persistent_state,
} from "@/lib/desktop-bridge";

const TOUR_COMPLETION_STORAGE_KEY = "nexus:onboarding:tours";
const TOUR_DISMISS_STORAGE_KEY = "nexus:onboarding:dismissed-tours";
const TOUR_PENDING_REQUEST_STORAGE_KEY = "nexus:onboarding:pending-tour";
const SIDEBAR_HINT_DISMISSED_STORAGE_KEY = "nexus:sidebar-onboarding-dismissed";

const DESKTOP_COMPLETED_TOURS_KEY = "onboarding.completed_tours";
const DESKTOP_DISMISSED_TOURS_KEY = "onboarding.dismissed_tours";
const DESKTOP_SIDEBAR_HINT_KEY = "onboarding.sidebar_hint_dismissed";

export interface HydratedOnboardingState {
  completed_tours: Record<string, boolean>;
}

function read_boolean_map(storage_key: string): Record<string, boolean> {
  if (typeof window === "undefined") {
    return {};
  }

  try {
    const raw = window.localStorage.getItem(storage_key);
    if (!raw) {
      return {};
    }
    const parsed = JSON.parse(raw) as Record<string, boolean>;
    return parsed && typeof parsed === "object" ? normalize_boolean_map(parsed) : {};
  } catch (err) {
    console.debug("[tour-state] Failed to read storage:", storage_key, err);
    return {};
  }
}

function write_boolean_map(
  storage_key: string,
  next_value: Record<string, boolean>,
) {
  if (typeof window === "undefined") {
    return;
  }

  window.localStorage.setItem(storage_key, JSON.stringify(normalize_boolean_map(next_value)));
}

function normalize_boolean_map(value: Record<string, boolean>): Record<string, boolean> {
  return Object.fromEntries(
    Object.entries(value).filter((entry): entry is [string, boolean] => (
      typeof entry[0] === "string" && entry[0].trim().length > 0 && entry[1] === true
    )),
  );
}

function persist_desktop_value(key: string, value: string) {
  if (!is_desktop_bridge_available()) {
    return;
  }
  void set_desktop_persistent_state(key, value).catch(() => {});
}

function remove_desktop_value(key: string) {
  if (!is_desktop_bridge_available()) {
    return;
  }
  void remove_desktop_persistent_state(key).catch(() => {});
}

async function read_desktop_boolean_map(key: string): Promise<Record<string, boolean> | null> {
  if (!is_desktop_bridge_available()) {
    return null;
  }

  const result = await get_desktop_persistent_state(key);
  if (!result.value) {
    return null;
  }
  try {
    const parsed = JSON.parse(result.value) as Record<string, boolean>;
    return parsed && typeof parsed === "object" ? normalize_boolean_map(parsed) : {};
  } catch {
    return {};
  }
}

async function read_desktop_boolean(key: string): Promise<boolean | null> {
  if (!is_desktop_bridge_available()) {
    return null;
  }

  const result = await get_desktop_persistent_state(key);
  if (result.value === null || typeof result.value === "undefined") {
    return null;
  }
  return result.value === "true";
}

export async function hydrate_onboarding_state_from_desktop(): Promise<HydratedOnboardingState> {
  const local_completed_tours = read_completed_tours();
  if (!is_desktop_bridge_available()) {
    return { completed_tours: local_completed_tours };
  }

  try {
    const [desktop_completed_tours, desktop_dismissed_tours, desktop_sidebar_hint_dismissed] = await Promise.all([
      read_desktop_boolean_map(DESKTOP_COMPLETED_TOURS_KEY),
      read_desktop_boolean_map(DESKTOP_DISMISSED_TOURS_KEY),
      read_desktop_boolean(DESKTOP_SIDEBAR_HINT_KEY),
    ]);

    const completed_tours = {
      ...local_completed_tours,
      ...(desktop_completed_tours ?? {}),
    };
    const dismissed_tours = {
      ...read_dismissed_tours(),
      ...(desktop_dismissed_tours ?? {}),
    };

    write_boolean_map(TOUR_COMPLETION_STORAGE_KEY, completed_tours);
    write_boolean_map(TOUR_DISMISS_STORAGE_KEY, dismissed_tours);

    if (Object.keys(completed_tours).length > 0) {
      persist_desktop_value(DESKTOP_COMPLETED_TOURS_KEY, JSON.stringify(completed_tours));
    }
    if (Object.keys(dismissed_tours).length > 0) {
      persist_desktop_value(DESKTOP_DISMISSED_TOURS_KEY, JSON.stringify(dismissed_tours));
    }

    if (desktop_sidebar_hint_dismissed === true) {
      window.localStorage.setItem(SIDEBAR_HINT_DISMISSED_STORAGE_KEY, "true");
    } else if (window.localStorage.getItem(SIDEBAR_HINT_DISMISSED_STORAGE_KEY) === "true") {
      persist_desktop_value(DESKTOP_SIDEBAR_HINT_KEY, "true");
    }

    return { completed_tours };
  } catch {
    return { completed_tours: local_completed_tours };
  }
}

export function read_completed_tours(): Record<string, boolean> {
  return read_boolean_map(TOUR_COMPLETION_STORAGE_KEY);
}

export function write_completed_tours(next_value: Record<string, boolean>) {
  const normalized = normalize_boolean_map(next_value);
  write_boolean_map(TOUR_COMPLETION_STORAGE_KEY, normalized);
  persist_desktop_value(DESKTOP_COMPLETED_TOURS_KEY, JSON.stringify(normalized));
}

export function read_dismissed_tours(): Record<string, boolean> {
  return read_boolean_map(TOUR_DISMISS_STORAGE_KEY);
}

export function write_dismissed_tours(next_value: Record<string, boolean>) {
  const normalized = normalize_boolean_map(next_value);
  write_boolean_map(TOUR_DISMISS_STORAGE_KEY, normalized);
  persist_desktop_value(DESKTOP_DISMISSED_TOURS_KEY, JSON.stringify(normalized));
}

export function is_tour_dismissed(tour_id: string): boolean {
  return Boolean(read_dismissed_tours()[tour_id]);
}

export function set_tour_dismissed(tour_id: string, dismissed: boolean) {
  const next_value = read_dismissed_tours();
  if (dismissed) {
    next_value[tour_id] = true;
  } else {
    delete next_value[tour_id];
  }
  write_dismissed_tours(next_value);
}

export function is_sidebar_onboarding_hint_dismissed(): boolean {
  if (typeof window === "undefined") {
    return true;
  }
  return window.localStorage.getItem(SIDEBAR_HINT_DISMISSED_STORAGE_KEY) === "true";
}

export function set_sidebar_onboarding_hint_dismissed() {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(SIDEBAR_HINT_DISMISSED_STORAGE_KEY, "true");
  persist_desktop_value(DESKTOP_SIDEBAR_HINT_KEY, "true");
}

export function read_requested_tour_id(): string | null {
  if (typeof window === "undefined") {
    return null;
  }

  const raw = window.localStorage.getItem(TOUR_PENDING_REQUEST_STORAGE_KEY);
  return raw?.trim() || null;
}

export function set_requested_tour_id(tour_id: string) {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(TOUR_PENDING_REQUEST_STORAGE_KEY, tour_id);
}

export function clear_requested_tour_id(expected_tour_id?: string) {
  if (typeof window === "undefined") {
    return;
  }

  if (!expected_tour_id) {
    window.localStorage.removeItem(TOUR_PENDING_REQUEST_STORAGE_KEY);
    return;
  }

  const current_tour_id = read_requested_tour_id();
  if (current_tour_id === expected_tour_id) {
    window.localStorage.removeItem(TOUR_PENDING_REQUEST_STORAGE_KEY);
  }
}

export function reset_all_tour_state() {
  if (typeof window === "undefined") {
    return;
  }

  window.localStorage.removeItem(TOUR_COMPLETION_STORAGE_KEY);
  window.localStorage.removeItem(TOUR_DISMISS_STORAGE_KEY);
  window.localStorage.removeItem(TOUR_PENDING_REQUEST_STORAGE_KEY);
  window.localStorage.removeItem(SIDEBAR_HINT_DISMISSED_STORAGE_KEY);
  remove_desktop_value(DESKTOP_COMPLETED_TOURS_KEY);
  remove_desktop_value(DESKTOP_DISMISSED_TOURS_KEY);
  remove_desktop_value(DESKTOP_SIDEBAR_HINT_KEY);
}
