import type { NexusOperationEvent } from "../operation-types";

const FILE_CONTENT_KEYS = [
  "content",
  "new_str",
  "new_string",
  "newString",
  "new_source",
  "newSource",
  "replacement",
  "text",
  "body",
  "source",
] as const;

const OLD_CONTENT_KEYS = [
  "old_str",
  "old_string",
  "oldString",
] as const;

const NEW_CONTENT_KEYS = [
  "new_str",
  "new_string",
  "newString",
  "replacement",
] as const;

export function resolveFilePreviewValue(
  event: NexusOperationEvent,
  payload_preview: unknown,
): unknown {
  const payload_content = extract_file_preview(payload_preview);
  if (payload_content) {
    return payload_content;
  }

  const input_content = extract_file_preview(event.input_preview);
  if (input_content) {
    return input_content;
  }

  const result_content = extract_file_preview(event.result_preview);
  if (result_content) {
    return result_content;
  }

  if (is_workspace_writer_event(event) && payload_preview && typeof payload_preview === "object") {
    return event.summary ?? event.target ?? "";
  }

  return payload_preview ?? event.result_preview ?? event.summary;
}

function extract_file_preview(value: unknown): string | null {
  if (typeof value === "string" && value.trim()) {
    return value;
  }
  if (!is_record(value)) {
    return null;
  }

  const edit_preview = extract_edit_preview(value);
  if (edit_preview) {
    return edit_preview;
  }

  for (const key of FILE_CONTENT_KEYS) {
    const field_value = value[key];
    if (typeof field_value === "string" && field_value.trim()) {
      return field_value;
    }
    if (Array.isArray(field_value) && field_value.every((item) => typeof item === "string")) {
      return field_value.join("\n");
    }
  }

  const patch = value.patch ?? value.diff;
  if (typeof patch === "string" && patch.trim()) {
    return patch;
  }

  return null;
}

function extract_edit_preview(input: Record<string, unknown>): string | null {
  const edits = input.edits;
  if (Array.isArray(edits)) {
    const edit_blocks = edits.flatMap((item) => {
      if (!is_record(item)) {
        return [];
      }
      const preview = format_edit_hunk(
        first_string_value(item, OLD_CONTENT_KEYS),
        first_string_value(item, NEW_CONTENT_KEYS),
      );
      return preview ? [preview] : [];
    });
    if (edit_blocks.length) {
      return edit_blocks.join("\n\n");
    }
  }

  return format_edit_hunk(
    first_string_value(input, OLD_CONTENT_KEYS),
    first_string_value(input, NEW_CONTENT_KEYS),
  );
}

function format_edit_hunk(old_text: string | null, new_text: string | null): string | null {
  if (!old_text && !new_text) {
    return null;
  }
  const old_lines = old_text ? old_text.split(/\r?\n/).map((line) => `- ${line}`) : [];
  const new_lines = new_text ? new_text.split(/\r?\n/).map((line) => `+ ${line}`) : [];
  return [...old_lines, ...new_lines].join("\n");
}

function first_string_value(
  input: Record<string, unknown>,
  keys: readonly string[],
): string | null {
  for (const key of keys) {
    const value = input[key];
    if (typeof value === "string" && value.trim()) {
      return value;
    }
  }
  return null;
}

function is_workspace_writer_event(event: NexusOperationEvent): boolean {
  return (
    event.kind === "workspace_edit" ||
    [
      "Write",
      "Edit",
      "FileWrite",
      "FileEdit",
      "MultiEdit",
      "NotebookEdit",
      "filesystem.write",
      "patch.apply",
      "notebook.edit",
    ].includes(event.tool_name ?? "")
  );
}

function is_record(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === "object" && !Array.isArray(value));
}
