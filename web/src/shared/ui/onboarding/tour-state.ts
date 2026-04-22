"use client";

const TOUR_COMPLETION_STORAGE_KEY = "nexus:onboarding:tours";
const TOUR_DISMISS_STORAGE_KEY = "nexus:onboarding:dismissed-tours";
const TOUR_PENDING_REQUEST_STORAGE_KEY = "nexus:onboarding:pending-tour";
const SIDEBAR_HINT_DISMISSED_STORAGE_KEY = "nexus:sidebar-onboarding-dismissed";

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
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
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

  window.localStorage.setItem(storage_key, JSON.stringify(next_value));
}

export function read_completed_tours(): Record<string, boolean> {
  return read_boolean_map(TOUR_COMPLETION_STORAGE_KEY);
}

export function write_completed_tours(next_value: Record<string, boolean>) {
  write_boolean_map(TOUR_COMPLETION_STORAGE_KEY, next_value);
}

export function read_dismissed_tours(): Record<string, boolean> {
  return read_boolean_map(TOUR_DISMISS_STORAGE_KEY);
}

export function write_dismissed_tours(next_value: Record<string, boolean>) {
  write_boolean_map(TOUR_DISMISS_STORAGE_KEY, next_value);
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
}
