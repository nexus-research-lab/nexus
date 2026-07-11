import {
  markDesktopPerformance,
  notifyDesktopWebFatal,
  recoverDesktopSessionTokenError,
} from "@/config/desktop-runtime";
import { getAgentApiBaseUrl } from "@/config/options";

import { shouldReloadOnce } from "./reload-guard";

const APP_CHUNK_ERROR_RELOAD_KEY_PREFIX = "nexus:app-chunk-error-reload:";
const CHUNK_ERROR_PATTERNS = [
  /ChunkLoadError/i,
  /Loading chunk [\w-]+ failed/i,
  /Failed to fetch dynamically imported module/i,
  /Importing a module script failed/i,
  /error loading dynamically imported module/i,
  /Unable to preload CSS/i,
  /not a valid JavaScript MIME type/i,
  /Expected a JavaScript module script but the server responded with a MIME type/i,
];

let didInstallGlobalErrorHandlers = false;

function diagnosticText(error: unknown): string {
  if (error instanceof Error) {
    return `${error.name}\n${error.message}\n${error.stack ?? ""}`;
  }
  if (typeof error === "string") {
    return error;
  }
  try {
    return JSON.stringify(error);
  } catch {
    return String(error);
  }
}

function isChunkLoadError(error: unknown): boolean {
  const diagnostic = diagnosticText(error);
  return CHUNK_ERROR_PATTERNS.some((pattern) => pattern.test(diagnostic));
}

export function recoverFromChunkLoadError(source: string, error: unknown): boolean {
  if (!isChunkLoadError(error)) {
    return false;
  }

  notifyDesktopWebFatal(`${source}.chunkLoad`, error);
  if (!shouldReloadOnce(APP_CHUNK_ERROR_RELOAD_KEY_PREFIX, source)) {
    return false;
  }

  markDesktopPerformance(`web.chunkErrorReload.${source}`);
  window.setTimeout(() => window.location.reload(), 0);
  return true;
}

export function installGlobalErrorHandlers(): void {
  if (didInstallGlobalErrorHandlers) {
    return;
  }
  didInstallGlobalErrorHandlers = true;

  window.addEventListener("error", (event) => {
    const error = event.error ?? event.message;
    notifyDesktopWebFatal("window.error", error);
    recoverFromChunkLoadError("window.error", error);
  });
  window.addEventListener("unhandledrejection", (event) => {
    notifyDesktopWebFatal("window.unhandledrejection", event.reason);
    recoverFromChunkLoadError("window.unhandledrejection", event.reason);
  });
}

export function shouldRecoverAfterDesktopRuntimeAuthError(error: unknown): boolean {
  return error instanceof Error
    && recoverDesktopSessionTokenError(
      error.message,
      `${getAgentApiBaseUrl()}/runtime/options`,
    );
}
