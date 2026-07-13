import { useCallback, useEffect, useRef } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { fetchOfficePreviewBuffer } from "../office-preview-resource";
import {
  calculateDocumentPreviewScale,
  normalizeDocumentMedia,
} from "./document-preview-dom";
import {
  DOCX_RENDER_OPTIONS,
  type DocumentPreviewStatus,
} from "./document-preview-model";

interface UseDocumentPreviewOptions {
  agentId: string;
  path: string;
}

interface RenderedDocument {
  content: HTMLDivElement;
  styles: HTMLDivElement;
}

function clearPreviewHosts(
  container: HTMLElement | null,
  styleContainer: HTMLElement | null,
): void {
  container?.replaceChildren();
  styleContainer?.replaceChildren();
}

async function renderDocumentBuffer(
  buffer: ArrayBuffer,
): Promise<RenderedDocument> {
  const content = document.createElement("div");
  const styles = document.createElement("div");
  const { renderAsync } = await import("docx-preview");
  await renderAsync(buffer, content, styles, DOCX_RENDER_OPTIONS);
  return { content, styles };
}

function commitRenderedDocument(
  rendered: RenderedDocument,
  container: HTMLElement,
  styleContainer: HTMLElement,
): void {
  container.replaceChildren(...rendered.content.childNodes);
  styleContainer.replaceChildren(...rendered.styles.childNodes);
}

export function useDocumentPreview({
  agentId,
  path,
}: UseDocumentPreviewOptions) {
  const viewportRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const styleContainerRef = useRef<HTMLDivElement>(null);
  const previewKey = `${agentId}\x1f${path}`;
  const [previewScale, setPreviewScale] = useResettableState(1, previewKey);
  const [status, setStatus] = useResettableState<DocumentPreviewStatus>({
    state: "loading",
    message: "加载文档预览中",
  }, previewKey);

  const updatePreviewScale = useCallback(() => {
    const viewport = viewportRef.current;
    const container = containerRef.current;
    if (!viewport || !container) {
      return;
    }

    const nextScale = calculateDocumentPreviewScale(viewport, container);
    setPreviewScale((current) => (
      Math.abs(current - nextScale) > 0.005 ? nextScale : current
    ));
  }, [setPreviewScale]);

  useEffect(() => {
    const container = containerRef.current;
    const styleContainer = styleContainerRef.current;
    const abortController = new AbortController();
    let animationFrameId: number | null = null;
    let active = true;

    clearPreviewHosts(container, styleContainer);

    const loadPreview = async (): Promise<void> => {
      if (!container || !styleContainer) {
        return;
      }

      try {
        const buffer = await fetchOfficePreviewBuffer({
          agentId,
          fileLabel: "docx",
          path,
          signal: abortController.signal,
        });
        if (!active) {
          return;
        }

        setStatus({ state: "loading", message: "解析 docx 文件中" });
        const rendered = await renderDocumentBuffer(buffer);
        if (!active) {
          return;
        }

        commitRenderedDocument(rendered, container, styleContainer);
        normalizeDocumentMedia(container);
        updatePreviewScale();
        animationFrameId = requestAnimationFrame(() => {
          if (!active) {
            return;
          }
          normalizeDocumentMedia(container);
          updatePreviewScale();
        });
        setStatus({ state: "loaded" });
      } catch (error) {
        if (!active || abortController.signal.aborted) {
          return;
        }
        clearPreviewHosts(container, styleContainer);
        setPreviewScale(1);
        setStatus({
          state: "error",
          message: error instanceof Error ? error.message : "docx 预览失败",
        });
      }
    };

    void loadPreview();

    return () => {
      active = false;
      abortController.abort();
      if (animationFrameId !== null) {
        cancelAnimationFrame(animationFrameId);
      }
      clearPreviewHosts(container, styleContainer);
      setPreviewScale(1);
    };
  }, [agentId, path, setPreviewScale, setStatus, updatePreviewScale]);

  useEffect(() => {
    if (status.state !== "loaded") {
      return;
    }

    const viewport = viewportRef.current;
    const container = containerRef.current;
    if (!viewport || !container) {
      return;
    }

    const observer = new ResizeObserver(updatePreviewScale);
    observer.observe(viewport);
    observer.observe(container);
    window.addEventListener("resize", updatePreviewScale);
    updatePreviewScale();

    return () => {
      observer.disconnect();
      window.removeEventListener("resize", updatePreviewScale);
    };
  }, [status.state, updatePreviewScale]);

  return {
    containerRef,
    previewScale,
    status,
    styleContainerRef,
    viewportRef,
  };
}
