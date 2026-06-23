import type { Options as SpreadsheetOptions } from "x-data-spreadsheet";

import {
  estimate_spreadsheet_sheet_content_width,
  type XSpreadsheetData,
} from "./spreadsheet-x-data-adapter";

const MIN_SHEET_ROWS = 1;
const MIN_SHEET_COLS = 1;

interface SpreadsheetRuntime {
  loadData: (data: XSpreadsheetData) => SpreadsheetRuntime;
  reRender?: () => SpreadsheetRuntime;
  sheet?: {
    reload?: () => unknown;
  };
}

type SpreadsheetEntrypoint =
  | ((container: HTMLElement, options?: SpreadsheetOptions) => SpreadsheetRuntime)
  | (new (container: HTMLElement, options?: SpreadsheetOptions) => SpreadsheetRuntime);

interface CapturedListener {
  listener: EventListenerOrEventListenerObject;
  options?: AddEventListenerOptions | boolean;
  target: EventTarget;
  type: string;
}

interface MountedSpreadsheet {
  cleanup: () => void;
  refresh_layout: () => void;
}

export function mount_spreadsheet(
  container: HTMLElement,
  spreadsheet_entrypoint: SpreadsheetEntrypoint,
  data: XSpreadsheetData,
): MountedSpreadsheet {
  const event_scope = capture_event_listeners([window, document, document.body]);
  let spreadsheet: SpreadsheetRuntime | null = null;
  let active_sheet_index = 0;
  const sheet_view_widths = data.map(estimate_spreadsheet_sheet_content_width);
  const get_view_width = () => get_spreadsheet_view_width(container, sheet_view_widths, active_sheet_index);
  const options: SpreadsheetOptions = {
    mode: "read",
    showContextmenu: false,
    showToolbar: false,
    view: {
      height: () => Math.max(container.clientHeight, 300),
      width: get_view_width,
    },
    row: {
      height: 24,
      len: MIN_SHEET_ROWS,
    },
    col: {
      indexWidth: 60,
      len: MIN_SHEET_COLS,
      minWidth: 60,
      width: 80,
    },
  };

  try {
    spreadsheet = create_spreadsheet_runtime(spreadsheet_entrypoint, container, options)
      .loadData(data);
  } finally {
    event_scope.restore();
  }

  const refresh_layout = () => {
    refresh_spreadsheet_layout(spreadsheet, container);
  };
  refresh_layout();

  const handle_sheet_tab_click = (event: MouseEvent) => {
    const tab = (event.target as HTMLElement | null)?.closest<HTMLLIElement>(".x-spreadsheet-menu > li");
    if (!tab) {
      return;
    }

    const menu = tab.parentElement;
    const sheet_tabs = Array.from(menu?.querySelectorAll<HTMLLIElement>(":scope > li") ?? []).slice(1);
    const next_index = sheet_tabs.indexOf(tab);
    if (next_index < 0 || next_index >= data.length) {
      return;
    }

    active_sheet_index = next_index;
    requestAnimationFrame(refresh_layout);
  };
  container.addEventListener("click", handle_sheet_tab_click);

  const resize_observer = new ResizeObserver(() => {
    refresh_layout();
  });
  resize_observer.observe(container);

  return {
    cleanup: () => {
      container.removeEventListener("click", handle_sheet_tab_click);
      resize_observer.disconnect();
      event_scope.cleanup();
      container.innerHTML = "";
      spreadsheet = null;
    },
    refresh_layout,
  };
}

function refresh_spreadsheet_layout(spreadsheet: SpreadsheetRuntime | null, container: HTMLElement) {
  spreadsheet?.sheet?.reload?.();
  spreadsheet?.reRender?.();
  sync_spreadsheet_root_width(container);
}

function get_spreadsheet_view_width(
  container: HTMLElement,
  sheet_view_widths: number[],
  active_sheet_index: number,
): number {
  const content_width = sheet_view_widths[Math.min(Math.max(active_sheet_index, 0), sheet_view_widths.length - 1)] ?? 320;
  return Math.max(Math.min(container.clientWidth, content_width), 320);
}

function sync_spreadsheet_root_width(container: HTMLElement) {
  const root = container.querySelector<HTMLElement>(".x-spreadsheet");
  const sheet = container.querySelector<HTMLElement>(".x-spreadsheet-sheet");
  if (!root || !sheet) {
    return;
  }

  const width = sheet.getBoundingClientRect().width || sheet.clientWidth;
  if (width <= 0) {
    return;
  }

  root.style.width = `${Math.round(width)}px`;
  root.style.maxWidth = "100%";
}

export function resolve_spreadsheet_entrypoint(module_value: unknown): SpreadsheetEntrypoint {
  const candidates = [
    read_record_property(module_value, "default"),
    read_record_property(read_record_property(module_value, "default"), "default"),
    module_value,
    typeof window !== "undefined" ? window.x_spreadsheet : undefined,
  ];

  for (const candidate of candidates) {
    if (typeof candidate === "function") {
      return candidate as SpreadsheetEntrypoint;
    }
  }

  throw new Error("x-data-spreadsheet 初始化入口不可用");
}

function create_spreadsheet_runtime(
  entrypoint: SpreadsheetEntrypoint,
  container: HTMLElement,
  options: SpreadsheetOptions,
): SpreadsheetRuntime {
  try {
    return new (entrypoint as new (
      container: HTMLElement,
      options?: SpreadsheetOptions,
    ) => SpreadsheetRuntime)(container, options);
  } catch (error) {
    const message = error instanceof Error ? error.message : "";
    if (!(error instanceof TypeError) || !message.toLowerCase().includes("constructor")) {
      throw error;
    }
    return (entrypoint as (
      container: HTMLElement,
      options?: SpreadsheetOptions,
    ) => SpreadsheetRuntime)(container, options);
  }
}

function read_record_property(value: unknown, key: string): unknown {
  if (!value || typeof value !== "object") {
    return undefined;
  }
  return (value as Record<string, unknown>)[key];
}

function capture_event_listeners(targets: EventTarget[]) {
  const captured: CapturedListener[] = [];
  const restores = targets.map((target) => {
    const original_add = target.addEventListener;
    target.addEventListener = function patched_add_event_listener(type, listener, options) {
      if (listener) {
        captured.push({
          listener,
          options,
          target,
          type: String(type),
        });
      }
      return original_add.call(this, type, listener, options);
    };
    return () => {
      target.addEventListener = original_add;
    };
  });

  return {
    cleanup: () => {
      for (const item of captured) {
        item.target.removeEventListener(item.type, item.listener, item.options);
      }
    },
    restore: () => {
      for (const restore of restores) {
        restore();
      }
    },
  };
}
