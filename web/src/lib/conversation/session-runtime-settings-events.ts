const SESSION_RUNTIME_SETTINGS_UPDATED_EVENT =
  "nexus:session-runtime-settings-updated";

export function notifySessionRuntimeSettingsUpdated(
  sessionKey: string,
): void {
  const normalizedSessionKey = sessionKey.trim();
  if (!normalizedSessionKey || typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent<string>(
    SESSION_RUNTIME_SETTINGS_UPDATED_EVENT,
    { detail: normalizedSessionKey },
  ));
}

export function subscribeSessionRuntimeSettingsUpdated(
  listener: (sessionKey: string) => void,
): () => void {
  if (typeof window === "undefined") {
    return () => undefined;
  }
  const handleUpdate = (event: Event) => {
    const sessionKey = (event as CustomEvent<string>).detail?.trim();
    if (sessionKey) {
      listener(sessionKey);
    }
  };
  window.addEventListener(
    SESSION_RUNTIME_SETTINGS_UPDATED_EVENT,
    handleUpdate,
  );
  return () => {
    window.removeEventListener(
      SESSION_RUNTIME_SETTINGS_UPDATED_EVENT,
      handleUpdate,
    );
  };
}
