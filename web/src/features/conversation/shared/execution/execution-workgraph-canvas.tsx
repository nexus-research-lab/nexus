/**
 * INPUT: 权威 Execution Graph、Agent 目录、当前 Graph 节点、节点展示密度与精确 Agent round Task run。
 * OUTPUT: 在焦点稳定、全边界可达且不叠加伪主图底框的工作板上显示图标或可读摘要卡片、可整体悬停聚焦的子图、跨子图边框端口、带语义分叉点的中性正交流程边、按需展开的精确端点短引线、降饱和控制回连、节点完整上下游路径聚焦，以及无可见标题栏并复用全部交互能力的大图弹窗。
 * POS: DM/Room 共用的只读 Execution Graph 主视图；一级运行树外框与内部方向边只按结构化父身份投影，不从自由文本反推关系。
 */
"use client";

import {
  Fragment,
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent as ReactKeyboardEvent,
  type MouseEvent as ReactMouseEvent,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
  type TouchEvent as ReactTouchEvent,
} from "react";
import { ChevronsDownUp, ChevronsUpDown, X } from "lucide-react";

import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { cn } from "@/shared/ui/class-name";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogCloseButton,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import type {
  ExecutionAttemptView,
  ExecutionGraphEdgeKind,
  ExecutionGraphEdgeView,
  ExecutionGraphNodeView,
  ExecutionView,
  ExecutionWorkItemStatus,
  ExecutionWorkItemView,
} from "@/types/conversation/execution";

import { ExecutionNodeAvatar } from "./execution-node-avatar";
import { ExecutionNodeRunHistory } from "./execution-node-run-history";
import { ExecutionNodeTaskList } from "./execution-node-task-list";
import { resolveExecutionNodeTaskRun } from "./execution-node-task-model";
import {
  compactExecutionNodeObjective,
  normalizeExecutionNodeDisplayText,
  resolveExecutionGraphNodeAgent,
  resolveExecutionGraphNodeStatus,
  type ExecutionAgentDirectory,
  WORK_ITEM_STATUS_LABEL_KEY,
} from "./execution-process-model";
import {
  buildExecutionGraphLayout,
  type ExecutionGraphNodePresentation,
} from "./execution-workgraph-layout";
import { ExecutionWorkGraphControls } from "./execution-workgraph-controls";
import {
  clampExecutionGraphZoom,
  EXECUTION_GRAPH_ZOOM_STEP,
  nextExecutionGraphSearchResult,
  projectExecutionGraphCollapse,
  resolveExecutionGraphAnchoredScroll,
  resolveExecutionGraphFitZoom,
  resolveExecutionGraphGroupTrace,
  resolveExecutionGraphInitialScroll,
  resolveExecutionGraphNodeAncestors,
  resolveExecutionGraphPanPadding,
  resolveExecutionGraphTrace,
  resolveExecutionGraphWheelZoom,
  searchExecutionGraphNodes,
} from "./execution-workgraph-interaction-model";

const ATTEMPT_STATUS_LABEL_KEY: Record<
  ExecutionAttemptView["status"],
  TranslationKey
> = {
  cancelled: "execution.attempt_cancelled",
  failed: "execution.attempt_failed",
  interrupted: "execution.attempt_interrupted",
  pending: "execution.attempt_pending",
  running: "execution.attempt_running",
  succeeded: "execution.attempt_succeeded",
  timed_out: "execution.attempt_timed_out",
};

const EDGE_KIND_LABEL_KEY: Record<ExecutionGraphEdgeKind, TranslationKey> = {
  coordination: "execution.edge_coordination",
  dependency: "execution.edge_dependency",
  dispatch: "execution.edge_dispatch",
  guard: "execution.edge_guard",
  invoke: "execution.edge_invoke",
  loop_back: "execution.edge_loop_back",
  retry: "execution.edge_retry",
  review: "execution.edge_review",
  spawn: "execution.edge_spawn",
};

const EDGE_KIND_DETAIL_KEY: Record<ExecutionGraphEdgeKind, TranslationKey> = {
  coordination: "execution.edge_coordination_detail",
  dependency: "execution.edge_dependency_detail",
  dispatch: "execution.edge_dispatch_detail",
  guard: "execution.edge_guard_detail",
  invoke: "execution.edge_invoke_detail",
  loop_back: "execution.edge_loop_back_detail",
  retry: "execution.edge_retry_detail",
  review: "execution.edge_review_detail",
  spawn: "execution.edge_spawn_detail",
};

const NODE_INSPECTOR_WIDTH = 304;
const NODE_INSPECTOR_GAP = 12;
const NODE_INSPECTOR_EDGE_PADDING = 8;
const EXECUTION_GRAPH_PAN_THRESHOLD = 4;
const EXECUTION_GRAPH_INTERACTIVE_TARGET_SELECTOR = [
  "button",
  "a",
  "input",
  "textarea",
  "select",
  "label",
  "[data-execution-edge-line-hit]",
  "[data-execution-selected-node-detail]",
  "[data-execution-selected-edge-detail]",
  "[data-execution-workgraph-controls]",
].join(",");

interface ExecutionGraphPanGesture {
  mode: "blank" | "middle" | "right" | "space";
  moved: boolean;
  pointerId: number;
  scrollLeft: number;
  scrollTop: number;
  startX: number;
  startY: number;
}

interface ExecutionGraphPinchGesture {
  contentX: number;
  contentY: number;
  initialDistance: number;
  initialZoom: number;
}

interface ExecutionGraphPendingZoom {
  contentX: number;
  contentY: number;
  viewportX: number;
  viewportY: number;
  zoom: number;
}

interface ExecutionGraphViewportSize {
  height: number;
  width: number;
}

interface ExecutionWorkGraphCanvasProps {
  currentId: string | null;
  directory: ExecutionAgentDirectory;
  execution: ExecutionView;
  expandedMode?: boolean;
  nodePresentation?: ExecutionGraphNodePresentation;
  onOpenWorkspaceFile?: (
    path: string,
    workspaceAgentId?: string | null,
  ) => void;
  taskRuns: readonly ConversationTaskRun[];
}

function isExecutionGraphInteractiveTarget(target: EventTarget | null): boolean {
  return target instanceof Element
    && target.closest(EXECUTION_GRAPH_INTERACTIVE_TARGET_SELECTOR) !== null;
}

function isExecutionGraphTypingTarget(target: EventTarget | null): boolean {
  return target instanceof HTMLElement
    && (
      target.isContentEditable
      || ["INPUT", "SELECT", "TEXTAREA"].includes(target.tagName)
    );
}

export function ExecutionWorkGraphCanvas({
  currentId,
  directory,
  execution,
  expandedMode = false,
  nodePresentation = "icon",
  onOpenWorkspaceFile,
  taskRuns,
}: ExecutionWorkGraphCanvasProps) {
  const { t } = useI18n();
  const markerId = `execution-arrow-${useId().replace(/:/g, "")}`;
  const loopMarkerId = `${markerId}-loop`;
  const retryMarkerId = `${markerId}-retry`;
  const viewportRef = useRef<HTMLDivElement | null>(null);
  const panGestureRef = useRef<ExecutionGraphPanGesture | null>(null);
  const panPaddingRef = useRef<{ x: number; y: number } | null>(null);
  const pinchGestureRef = useRef<ExecutionGraphPinchGesture | null>(null);
  const pendingZoomRef = useRef<ExecutionGraphPendingZoom | null>(null);
  const spacePanRef = useRef(false);
  const suppressCanvasClickRef = useRef(false);
  const zoomRef = useRef(1);
  const [viewportSize, setViewportSize] = useState<ExecutionGraphViewportSize>({
    height: 0,
    width: 0,
  });
  const [collapsedNodeIds, setCollapsedNodeIds] = useState<Set<string>>(
    () => new Set(),
  );
  const [hoveredEdgeId, setHoveredEdgeId] = useState<string | null>(null);
  const [hoveredGroupId, setHoveredGroupId] = useState<string | null>(null);
  const [hoveredNodeId, setHoveredNodeId] = useState<string | null>(null);
  const [expandedOpen, setExpandedOpen] = useState(false);
  const [pendingFocusId, setPendingFocusId] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [selectedEdgeId, setSelectedEdgeId] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [panning, setPanning] = useState(false);
  const [spacePanReady, setSpacePanReady] = useState(false);
  const [zoom, setZoom] = useState(1);
  const panPaddingX = resolveExecutionGraphPanPadding(viewportSize.width);
  const panPaddingY = resolveExecutionGraphPanPadding(viewportSize.height);
  const selectEdge = (edgeId: string) => {
    setSelectedId(null);
    setSelectedEdgeId(edgeId);
  };
  const collapse = useMemo(
    () => projectExecutionGraphCollapse(execution, collapsedNodeIds),
    [collapsedNodeIds, execution],
  );
  const layout = useMemo(
    () => buildExecutionGraphLayout(
      execution,
      viewportSize.width || undefined,
      collapse.hiddenNodeIds,
      nodePresentation,
    ),
    [collapse.hiddenNodeIds, execution, nodePresentation, viewportSize.width],
  );
  const searchResultIds = useMemo(
    () => searchExecutionGraphNodes(execution, query),
    [execution, query],
  );
  const currentSearchResultIndex = selectedId
    ? searchResultIds.indexOf(selectedId)
    : -1;
  const selectedLayoutNode = layout.nodes.find(
    (candidate) => candidate.node.id === selectedId,
  ) ?? null;
  const selectedLayoutEdge = layout.edges.find(
    (candidate) => candidate.id === selectedEdgeId,
  ) ?? null;
  const activeTraceNodeId = selectedId ?? (selectedEdgeId ? null : hoveredNodeId);
  const activeTraceEdgeId = selectedEdgeId ?? (selectedId ? null : hoveredEdgeId);
  const graphTrace = useMemo(
    () => resolveExecutionGraphTrace(layout.edges, activeTraceNodeId),
    [activeTraceNodeId, layout.edges],
  );
  const activeTraceEdge = activeTraceEdgeId
    ? layout.edges.find((edge) => edge.id === activeTraceEdgeId) ?? null
    : null;
  const activeTraceGroup = activeTraceNodeId === null && activeTraceEdge === null
    ? layout.groups.find((group) => group.id === hoveredGroupId) ?? null
    : null;
  const groupTrace = useMemo(
    () => resolveExecutionGraphGroupTrace(
      layout.edges,
      activeTraceGroup?.nodeIds ?? [],
    ),
    [activeTraceGroup?.nodeIds, layout.edges],
  );
  const traceActive = activeTraceNodeId !== null
    || activeTraceEdge !== null
    || activeTraceGroup !== null;
  const tracedNodeIds = activeTraceEdge
    ? new Set([activeTraceEdge.sourceId, activeTraceEdge.targetId])
    : activeTraceGroup
    ? groupTrace.nodeIds
    : graphTrace.nodeIds;
  const tracedEdgeIds = activeTraceEdge
    ? new Set([activeTraceEdge.id])
    : activeTraceGroup
    ? groupTrace.edgeIds
    : graphTrace.edgeIds;
  const selectedItem = selectedLayoutNode?.item ?? null;
  const selectedAttempt = selectedLayoutNode?.node.attempt_id
    ? selectedItem?.attempts?.find(
      (attempt) => attempt.id === selectedLayoutNode.node.attempt_id,
    ) ?? null
    : null;
  const selectedTaskRun = selectedItem && selectedLayoutNode?.node.kind === "agent"
    ? resolveExecutionNodeTaskRun(selectedItem, taskRuns)
    : null;
  const selectedInspectorStyle = selectedLayoutNode
    ? resolveNodeInspectorStyle(
        layout.width,
        selectedLayoutNode.x,
        selectedLayoutNode.y,
        selectedLayoutNode.width,
        zoom,
      )
    : undefined;
  const selectedEdgeInspectorStyle = selectedLayoutEdge
    ? resolveNodeInspectorStyle(
        layout.width,
        selectedLayoutEdge.x,
        selectedLayoutEdge.y,
        0,
        zoom,
      )
    : undefined;

  useEffect(() => {
    if (
      selectedId
      && !(execution.graph?.nodes ?? []).some((node) => node.id === selectedId)
    ) {
      setSelectedId(null);
    }
  }, [execution.graph?.nodes, selectedId]);

  useEffect(() => {
    if (selectedEdgeId && !layout.edges.some((edge) => edge.id === selectedEdgeId)) {
      setSelectedEdgeId(null);
    }
  }, [layout.edges, selectedEdgeId]);

  useEffect(() => {
    if (hoveredGroupId && !layout.groups.some((group) => group.id === hoveredGroupId)) {
      setHoveredGroupId(null);
    }
  }, [hoveredGroupId, layout.groups]);

  useEffect(() => {
    zoomRef.current = zoom;
  }, [zoom]);

  useEffect(() => {
    const releaseSpace = () => {
      spacePanRef.current = false;
      setSpacePanReady(false);
    };
    const handleWindowKeyUp = (event: KeyboardEvent) => {
      if (event.code === "Space") {
        releaseSpace();
      }
    };
    window.addEventListener("blur", releaseSpace);
    window.addEventListener("keyup", handleWindowKeyUp);
    return () => {
      window.removeEventListener("blur", releaseSpace);
      window.removeEventListener("keyup", handleWindowKeyUp);
    };
  }, []);

  useEffect(() => {
    if (!query || searchResultIds.length === 0 || currentSearchResultIndex >= 0) {
      return;
    }
    const nodeId = searchResultIds[0];
    const ancestors = new Set(resolveExecutionGraphNodeAncestors(execution, nodeId));
    setCollapsedNodeIds((current) => {
      const next = new Set([...current].filter((id) => !ancestors.has(id)));
      return next.size === current.size ? current : next;
    });
    setSelectedId(nodeId);
    setSelectedEdgeId(null);
    setPendingFocusId(nodeId);
  }, [currentSearchResultIndex, execution, query, searchResultIds]);

  useEffect(() => {
    if (!pendingFocusId || !layout.nodes.some((item) => item.node.id === pendingFocusId)) {
      return;
    }
    const viewport = viewportRef.current;
    const target = layout.nodes.find((item) => item.node.id === pendingFocusId);
    if (!viewport || !target) {
      return;
    }
    const frame = window.requestAnimationFrame(() => {
      viewport.scrollTo({
        behavior: "smooth",
        left: panPaddingX + target.x * zoomRef.current - viewport.clientWidth / 2,
        top: panPaddingY + target.y * zoomRef.current - viewport.clientHeight / 2,
      });
      setPendingFocusId(null);
    });
    return () => window.cancelAnimationFrame(frame);
  }, [layout.nodes, panPaddingX, panPaddingY, pendingFocusId]);

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) {
      return;
    }
    const updateSize = () => {
      const style = window.getComputedStyle(viewport);
      const horizontalPadding = (Number.parseFloat(style.paddingLeft) || 0)
        + (Number.parseFloat(style.paddingRight) || 0);
      const verticalPadding = (Number.parseFloat(style.paddingTop) || 0)
        + (Number.parseFloat(style.paddingBottom) || 0);
      const nextSize = {
        height: Math.floor(Math.max(0, viewport.clientHeight - verticalPadding)),
        width: Math.floor(Math.max(0, viewport.clientWidth - horizontalPadding)),
      };
      setViewportSize((current) => (
        current.height === nextSize.height && current.width === nextSize.width
          ? current
          : nextSize
      ));
    };
    updateSize();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(updateSize);
    observer.observe(viewport);
    return () => observer.disconnect();
  }, []);

  useLayoutEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport || viewportSize.height <= 0 || viewportSize.width <= 0) {
      return;
    }
    const pendingZoom = pendingZoomRef.current;
    const previousPadding = panPaddingRef.current;
    if (pendingZoom && pendingZoom.zoom === zoom) {
      viewport.scrollLeft = panPaddingX
        + pendingZoom.contentX * zoom
        - pendingZoom.viewportX;
      viewport.scrollTop = panPaddingY
        + pendingZoom.contentY * zoom
        - pendingZoom.viewportY;
      pendingZoomRef.current = null;
    } else if (previousPadding) {
      viewport.scrollLeft += panPaddingX - previousPadding.x;
      viewport.scrollTop += panPaddingY - previousPadding.y;
    } else {
      const initialScroll = resolveExecutionGraphInitialScroll({
        contentHeight: layout.height,
        contentWidth: layout.width,
        panPaddingX,
        panPaddingY,
        viewportHeight: viewportSize.height,
        viewportWidth: viewportSize.width,
        zoom,
      });
      viewport.scrollLeft = initialScroll.left;
      viewport.scrollTop = initialScroll.top;
    }
    panPaddingRef.current = { x: panPaddingX, y: panPaddingY };
  }, [
    layout.height,
    layout.width,
    panPaddingX,
    panPaddingY,
    viewportSize.height,
    viewportSize.width,
    zoom,
  ]);

  const revealNode = (nodeId: string | null) => {
    if (!nodeId) {
      return;
    }
    const ancestors = new Set(resolveExecutionGraphNodeAncestors(execution, nodeId));
    setCollapsedNodeIds((current) => {
      const next = new Set([...current].filter((id) => !ancestors.has(id)));
      return next.size === current.size ? current : next;
    });
    setSelectedId(nodeId);
    setSelectedEdgeId(null);
    setPendingFocusId(nodeId);
  };
  const navigateSearch = (direction: -1 | 1) => {
    revealNode(nextExecutionGraphSearchResult(searchResultIds, selectedId, direction));
  };
  const collapsibleNodeIds = [...collapse.descendantCountByNodeId.keys()];
  const collapsedCount = collapsibleNodeIds.filter((id) => collapsedNodeIds.has(id)).length;
  const requestZoomToContent = (
    nextZoomValue: number,
    contentX: number,
    contentY: number,
    viewportX: number,
    viewportY: number,
  ) => {
    const viewport = viewportRef.current;
    if (!viewport) {
      return;
    }
    const nextZoom = clampExecutionGraphZoom(nextZoomValue);
    if (nextZoom === zoomRef.current) {
      if (pendingZoomRef.current?.zoom === nextZoom) {
        pendingZoomRef.current = {
          contentX,
          contentY,
          viewportX,
          viewportY,
          zoom: nextZoom,
        };
      } else {
        viewport.scrollLeft = panPaddingX + contentX * nextZoom - viewportX;
        viewport.scrollTop = panPaddingY + contentY * nextZoom - viewportY;
      }
      return;
    }
    pendingZoomRef.current = {
      contentX,
      contentY,
      viewportX,
      viewportY,
      zoom: nextZoom,
    };
    zoomRef.current = nextZoom;
    setZoom(nextZoom);
  };
  const requestZoomAtViewportPoint = (
    nextZoomValue: number,
    viewportX: number,
    viewportY: number,
  ) => {
    const viewport = viewportRef.current;
    if (!viewport) {
      return;
    }
    const nextZoom = clampExecutionGraphZoom(nextZoomValue);
    if (nextZoom === zoomRef.current) {
      return;
    }
    const pendingZoom = pendingZoomRef.current;
    const virtualScrollLeft = pendingZoom?.zoom === zoomRef.current
      ? panPaddingX
        + pendingZoom.contentX * pendingZoom.zoom
        - pendingZoom.viewportX
      : viewport.scrollLeft;
    const virtualScrollTop = pendingZoom?.zoom === zoomRef.current
      ? panPaddingY
        + pendingZoom.contentY * pendingZoom.zoom
        - pendingZoom.viewportY
      : viewport.scrollTop;
    const anchor = resolveExecutionGraphAnchoredScroll({
      currentZoom: zoomRef.current,
      nextZoom,
      panPaddingX,
      panPaddingY,
      scrollLeft: virtualScrollLeft,
      scrollTop: virtualScrollTop,
      viewportX,
      viewportY,
    });
    requestZoomToContent(
      nextZoom,
      anchor.contentX,
      anchor.contentY,
      viewportX,
      viewportY,
    );
  };
  const requestZoomAtViewportCenter = (nextZoom: number) => {
    const viewport = viewportRef.current;
    if (!viewport) {
      return;
    }
    requestZoomAtViewportPoint(
      nextZoom,
      viewport.clientWidth / 2,
      viewport.clientHeight / 2,
    );
  };
  const fitGraph = () => {
    const viewport = viewportRef.current;
    if (!viewport) {
      return;
    }
    const nextZoom = resolveExecutionGraphFitZoom({
      contentHeight: layout.height,
      contentWidth: layout.width,
      viewportHeight: viewport.clientHeight,
      viewportWidth: viewport.clientWidth,
    });
    requestZoomToContent(
      nextZoom,
      layout.width / 2,
      layout.height / 2,
      viewport.clientWidth / 2,
      viewport.clientHeight / 2,
    );
  };
  const closeGraphDetails = () => {
    setSelectedId(null);
    setSelectedEdgeId(null);
  };
  const handleGraphClick = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (suppressCanvasClickRef.current) {
      suppressCanvasClickRef.current = false;
      return;
    }
    if (isExecutionGraphInteractiveTarget(event.target)) {
      return;
    }
    closeGraphDetails();
  };
  const suppressPanClick = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (!suppressCanvasClickRef.current) {
      return;
    }
    suppressCanvasClickRef.current = false;
    event.preventDefault();
    event.stopPropagation();
  };
  const handlePanStart = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.pointerType === "touch") {
      return;
    }
    const interactive = isExecutionGraphInteractiveTarget(event.target);
    const mode = event.button === 1
      ? "middle"
      : event.button === 2
      ? "right"
      : event.button === 0 && spacePanRef.current
      ? "space"
      : event.button === 0 && !interactive
      ? "blank"
      : null;
    if (!mode) {
      return;
    }
    if (mode !== "blank") {
      event.preventDefault();
    }
    event.currentTarget.focus({ preventScroll: true });
    panGestureRef.current = {
      mode,
      moved: false,
      pointerId: event.pointerId,
      scrollLeft: event.currentTarget.scrollLeft,
      scrollTop: event.currentTarget.scrollTop,
      startX: event.clientX,
      startY: event.clientY,
    };
    suppressCanvasClickRef.current = false;
    event.currentTarget.setPointerCapture(event.pointerId);
    setPanning(true);
  };
  const handlePanMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    const gesture = panGestureRef.current;
    if (!gesture || gesture.pointerId !== event.pointerId) {
      return;
    }
    const deltaX = event.clientX - gesture.startX;
    const deltaY = event.clientY - gesture.startY;
    if (!gesture.moved && Math.hypot(deltaX, deltaY) >= EXECUTION_GRAPH_PAN_THRESHOLD) {
      gesture.moved = true;
    }
    if (!gesture.moved) {
      return;
    }
    event.preventDefault();
    event.currentTarget.scrollLeft = gesture.scrollLeft - deltaX;
    event.currentTarget.scrollTop = gesture.scrollTop - deltaY;
  };
  const finishPan = (
    event: ReactPointerEvent<HTMLDivElement>,
    cancelled = false,
  ) => {
    const gesture = panGestureRef.current;
    if (!gesture || gesture.pointerId !== event.pointerId) {
      return;
    }
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    panGestureRef.current = null;
    setPanning(false);
    if (gesture.moved) {
      suppressCanvasClickRef.current = true;
      window.setTimeout(() => {
        suppressCanvasClickRef.current = false;
      }, 0);
    } else if (gesture.mode !== "blank") {
      suppressCanvasClickRef.current = true;
      window.setTimeout(() => {
        suppressCanvasClickRef.current = false;
      }, 0);
    } else if (!cancelled) {
      closeGraphDetails();
    }
  };
  const handleGraphKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (isExecutionGraphTypingTarget(event.target)) {
      return;
    }
    if (event.key === "Escape") {
      event.preventDefault();
      closeGraphDetails();
      return;
    }
    if (event.code === "Space") {
      if (event.target !== event.currentTarget) {
        return;
      }
      event.preventDefault();
      spacePanRef.current = true;
      setSpacePanReady(true);
      return;
    }
    const command = event.ctrlKey || event.metaKey;
    if ((event.key === "+" || event.key === "=") && (command || !event.altKey)) {
      event.preventDefault();
      requestZoomAtViewportCenter(
        zoomRef.current + EXECUTION_GRAPH_ZOOM_STEP,
      );
    } else if (event.key === "-" && (command || !event.altKey)) {
      event.preventDefault();
      requestZoomAtViewportCenter(
        zoomRef.current - EXECUTION_GRAPH_ZOOM_STEP,
      );
    } else if (event.key === "0" && command) {
      event.preventDefault();
      requestZoomAtViewportCenter(1);
    } else if (event.key === "1" && event.altKey) {
      event.preventDefault();
      fitGraph();
    } else if (
      ["ArrowDown", "ArrowLeft", "ArrowRight", "ArrowUp"].includes(event.key)
      && !command
      && !event.altKey
    ) {
      event.preventDefault();
      const viewport = event.currentTarget;
      const distance = event.shiftKey ? 120 : 48;
      viewport.scrollBy({
        left: event.key === "ArrowLeft"
          ? -distance
          : event.key === "ArrowRight"
          ? distance
          : 0,
        top: event.key === "ArrowUp"
          ? -distance
          : event.key === "ArrowDown"
          ? distance
          : 0,
      });
    }
  };
  const releaseSpacePan = () => {
    spacePanRef.current = false;
    setSpacePanReady(false);
  };
  const handleGraphKeyUp = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (event.code === "Space") {
      releaseSpacePan();
    }
  };
  const handlePinchStart = (event: ReactTouchEvent<HTMLDivElement>) => {
    if (event.touches.length !== 2) {
      return;
    }
    const viewport = event.currentTarget;
    const bounds = viewport.getBoundingClientRect();
    const [first, second] = [event.touches[0], event.touches[1]];
    const viewportX = (first.clientX + second.clientX) / 2 - bounds.left;
    const viewportY = (first.clientY + second.clientY) / 2 - bounds.top;
    const anchor = resolveExecutionGraphAnchoredScroll({
      currentZoom: zoomRef.current,
      nextZoom: zoomRef.current,
      panPaddingX,
      panPaddingY,
      scrollLeft: viewport.scrollLeft,
      scrollTop: viewport.scrollTop,
      viewportX,
      viewportY,
    });
    pinchGestureRef.current = {
      contentX: anchor.contentX,
      contentY: anchor.contentY,
      initialDistance: Math.hypot(
        first.clientX - second.clientX,
        first.clientY - second.clientY,
      ),
      initialZoom: zoomRef.current,
    };
    event.preventDefault();
  };
  const handlePinchMove = (event: ReactTouchEvent<HTMLDivElement>) => {
    const gesture = pinchGestureRef.current;
    if (!gesture || event.touches.length !== 2 || gesture.initialDistance <= 0) {
      return;
    }
    const bounds = event.currentTarget.getBoundingClientRect();
    const [first, second] = [event.touches[0], event.touches[1]];
    requestZoomToContent(
      gesture.initialZoom * (
        Math.hypot(
          first.clientX - second.clientX,
          first.clientY - second.clientY,
        ) / gesture.initialDistance
      ),
      gesture.contentX,
      gesture.contentY,
      (first.clientX + second.clientX) / 2 - bounds.left,
      (first.clientY + second.clientY) / 2 - bounds.top,
    );
    event.preventDefault();
  };
  const finishPinch = (event: ReactTouchEvent<HTMLDivElement>) => {
    if (event.touches.length < 2) {
      pinchGestureRef.current = null;
    }
  };

  return (
    <Fragment>
      <div
        className="relative flex min-h-0 flex-1 overflow-hidden"
        data-execution-node-map
      >
      <ExecutionWorkGraphControls
        collapsibleCount={collapsibleNodeIds.length}
        collapsedCount={collapsedCount}
        currentResultIndex={currentSearchResultIndex}
        onCollapseAll={() => setCollapsedNodeIds(new Set(collapsibleNodeIds))}
        onExpandAll={() => setCollapsedNodeIds(new Set())}
        onFit={fitGraph}
        onLocateCurrent={() => revealNode(currentId)}
        onNextResult={() => navigateSearch(1)}
        onOpenExpanded={expandedMode ? undefined : () => setExpandedOpen(true)}
        onPreviousResult={() => navigateSearch(-1)}
        onQueryChange={setQuery}
        onResetZoom={() => requestZoomAtViewportCenter(1)}
        onZoomIn={() => requestZoomAtViewportCenter(
          zoomRef.current + EXECUTION_GRAPH_ZOOM_STEP,
        )}
        onZoomOut={() => requestZoomAtViewportCenter(
          zoomRef.current - EXECUTION_GRAPH_ZOOM_STEP,
        )}
        query={query}
        resultCount={searchResultIds.length}
        zoom={zoom}
      />
      {/* eslint-disable jsx-a11y/no-noninteractive-element-interactions, jsx-a11y/no-noninteractive-tabindex -- The focusable ARIA application is the native interaction surface for this read-only whiteboard; no HTML element provides equivalent pan and zoom semantics. */}
      <div
        aria-keyshortcuts="Control+= Meta+= Control+- Meta+- Control+0 Meta+0 Alt+1 Escape ArrowUp ArrowDown ArrowLeft ArrowRight"
        aria-label={t("execution.label")}
        ref={viewportRef}
        className={cn(
          "soft-scrollbar min-h-0 min-w-0 flex-1 overflow-auto overscroll-contain p-2 outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-(--primary)",
          panning ? "cursor-grabbing select-none" : "cursor-grab",
        )}
        data-execution-board-grid
        data-execution-board-panning={panning ? "true" : "false"}
        data-execution-board-space-pan={spacePanReady ? "true" : "false"}
        onAuxClick={(event) => {
          if (event.button === 1) {
            event.preventDefault();
          }
        }}
        onBlurCapture={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
            releaseSpacePan();
          }
        }}
        onClick={handleGraphClick}
        onClickCapture={suppressPanClick}
        onContextMenu={(event) => event.preventDefault()}
        onDoubleClick={(event) => {
          if (isExecutionGraphInteractiveTarget(event.target)) {
            return;
          }
          event.preventDefault();
          const bounds = event.currentTarget.getBoundingClientRect();
          requestZoomAtViewportPoint(
            zoomRef.current + EXECUTION_GRAPH_ZOOM_STEP * 2,
            event.clientX - bounds.left,
            event.clientY - bounds.top,
          );
        }}
        onPointerCancel={(event) => finishPan(event, true)}
        onPointerDown={handlePanStart}
        onLostPointerCapture={() => {
          if (panGestureRef.current) {
            panGestureRef.current = null;
            setPanning(false);
          }
        }}
        onPointerMove={handlePanMove}
        onPointerUp={finishPan}
        onKeyDown={handleGraphKeyDown}
        onKeyUp={handleGraphKeyUp}
        onTouchCancel={finishPinch}
        onTouchEnd={finishPinch}
        onTouchMove={handlePinchMove}
        onTouchStart={handlePinchStart}
        onWheel={(event) => {
          if (event.ctrlKey || event.metaKey) {
            event.preventDefault();
            const bounds = event.currentTarget.getBoundingClientRect();
            const deltaY = event.deltaY * (event.deltaMode === 1
              ? 16
              : event.deltaMode === 2
              ? event.currentTarget.clientHeight
              : 1);
            requestZoomAtViewportPoint(
              resolveExecutionGraphWheelZoom(zoomRef.current, deltaY),
              event.clientX - bounds.left,
              event.clientY - bounds.top,
            );
            return;
          }
          if (event.shiftKey && Math.abs(event.deltaY) > Math.abs(event.deltaX)) {
            event.preventDefault();
            event.currentTarget.scrollLeft += event.deltaY;
          }
        }}
        role="application"
        style={{
          backgroundImage: "linear-gradient(to right, color-mix(in srgb, var(--divider-subtle-color) 58%, transparent) 1px, transparent 1px), linear-gradient(to bottom, color-mix(in srgb, var(--divider-subtle-color) 58%, transparent) 1px, transparent 1px)",
          backgroundPosition: "-1px -1px",
          backgroundSize: "24px 24px",
          touchAction: "pan-x pan-y",
        }}
        tabIndex={0}
      >
        <div
          className="relative shrink-0"
          data-execution-pan-padding-x={panPaddingX}
          data-execution-pan-padding-y={panPaddingY}
          data-execution-workgraph-scale={zoom}
          style={{
            height: layout.height * zoom + panPaddingY * 2,
            width: layout.width * zoom + panPaddingX * 2,
          }}
        >
        <div
          aria-label={t("execution.label")}
          className="absolute origin-top-left overflow-visible"
          data-execution-workgraph-canvas
          data-execution-node-detail-mode="popover"
          role="group"
          style={{
            height: layout.height,
            left: panPaddingX,
            top: panPaddingY,
            transform: `scale(${zoom})`,
            width: layout.width,
          }}
        >
          {layout.groups.map((group) => {
            const traced = group.nodeIds.some((nodeId) => tracedNodeIds.has(nodeId));
            const hovered = activeTraceGroup?.id === group.id;
            return (
              <div
                aria-hidden="true"
                className={cn(
                  "pointer-events-auto absolute z-0 rounded-[18px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_78%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-control-background)_48%,transparent)] transition-[border-color,background-color,box-shadow,opacity] duration-150",
                  hovered
                    && "border-[color:color-mix(in_srgb,var(--primary)_48%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_4%,var(--surface-control-background))] ring-2 ring-[color:color-mix(in_srgb,var(--primary)_8%,transparent)]",
                )}
                data-execution-subgraph-hovered={hovered ? "true" : undefined}
                data-execution-subgraph-root={group.id}
                data-execution-subgraph-traced={traceActive && traced ? "true" : undefined}
                key={group.id}
                onMouseEnter={() => setHoveredGroupId(group.id)}
                onMouseLeave={() => setHoveredGroupId((current) => (
                  current === group.id ? null : current
                ))}
                style={{
                  height: group.height,
                  left: group.x,
                  opacity: traceActive && !traced ? 0.22 : 1,
                  top: group.y,
                  width: group.width,
                }}
              />
            );
          })}
          <svg
            aria-label={t("execution.edge_layer")}
            className="pointer-events-none absolute inset-0 h-full w-full overflow-visible"
            data-execution-edge-layer
            role="group"
            viewBox={`0 0 ${layout.width} ${layout.height}`}
          >
            <defs>
              <marker
                id={markerId}
                markerHeight="5"
                markerUnits="strokeWidth"
                markerWidth="5"
                orient="auto"
                refX="4"
                refY="2.5"
                viewBox="0 0 5 5"
              >
                <path d="M 0 0 L 5 2.5 L 0 5 z" fill="var(--icon-muted)" />
              </marker>
              <marker
                id={loopMarkerId}
                markerHeight="5"
                markerUnits="strokeWidth"
                markerWidth="5"
                orient="auto"
                refX="4"
                refY="2.5"
                viewBox="0 0 5 5"
              >
                <path
                  d="M 0 0 L 5 2.5 L 0 5 z"
                  fill="color-mix(in srgb, var(--warning) 62%, var(--icon-muted))"
                />
              </marker>
              <marker
                id={retryMarkerId}
                markerHeight="5"
                markerUnits="strokeWidth"
                markerWidth="5"
                orient="auto"
                refX="4"
                refY="2.5"
                viewBox="0 0 5 5"
              >
                <path d="M 0 0 L 5 2.5 L 0 5 z" fill="var(--primary)" />
              </marker>
            </defs>
            {layout.ports.map((port) => {
              const traced = port.edgeIds.some((edgeId) => tracedEdgeIds.has(edgeId));
              return (
                <circle
                  aria-hidden="true"
                  cx={port.x}
                  cy={port.y}
                  data-execution-boundary-port={port.id}
                  data-execution-boundary-port-role={port.role}
                  data-execution-boundary-port-side={port.side}
                  fill={traceActive && traced
                    ? "var(--primary)"
                    : "var(--surface-panel-background)"}
                  key={port.id}
                  opacity={traceActive && !traced ? 0.16 : 0.9}
                  r={traceActive && traced ? 3 : 2.5}
                  stroke={traceActive && traced ? "var(--primary)" : "var(--icon-muted)"}
                  strokeWidth="1.2"
                  style={{ transition: "opacity 150ms ease, fill 150ms ease" }}
                />
              );
            })}
            {layout.edges.map((edge) => {
              const selected = edge.id === selectedEdgeId;
              const hovered = edge.id === hoveredEdgeId;
              const traced = tracedEdgeIds.has(edge.id);
              const dimmed = traceActive && !traced;
              const connected = edge.sourceId === selectedId
                || edge.targetId === selectedId
                || edge.targetId === currentId;
              const control = edge.kind === "loop_back" || edge.kind === "retry";
              const paired = edge.paired;
              return (
                <Fragment key={edge.id}>
                  {traceActive && traced && edge.sourceTailPath ? (
                    <path
                      aria-hidden="true"
                      d={edge.sourceTailPath}
                      data-execution-edge-endpoint-tail="source"
                      data-execution-edge-endpoint-tail-for={edge.id}
                      fill="none"
                      opacity="0.72"
                      stroke="var(--primary)"
                      strokeDasharray="2 3"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth="1.1"
                    />
                  ) : null}
                  {traceActive && traced && edge.targetTailPath ? (
                    <path
                      aria-hidden="true"
                      d={edge.targetTailPath}
                      data-execution-edge-endpoint-tail="target"
                      data-execution-edge-endpoint-tail-for={edge.id}
                      fill="none"
                      opacity="0.72"
                      stroke="var(--primary)"
                      strokeDasharray="2 3"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth="1.1"
                    />
                  ) : null}
                  <path
                    aria-hidden="true"
                    d={edge.path}
                    data-execution-edge-kind={edge.kind}
                    data-execution-edge-paired={paired ? "true" : undefined}
                    data-execution-edge-selected={edge.id === selectedEdgeId
                      ? "true"
                      : undefined}
                    data-execution-edge-source={edge.sourceId}
                    data-execution-edge-target={edge.targetId}
                    data-execution-edge-traced={traceActive && traced
                      ? "true"
                      : undefined}
                    fill="none"
                    markerEnd={`url(#${edge.kind === "loop_back"
                      ? loopMarkerId
                      : edge.kind === "retry"
                      ? retryMarkerId
                      : markerId})`}
                    opacity={dimmed
                      ? control ? 0.14 : 0.1
                      : selected || hovered
                      ? 0.98
                      : traceActive && traced
                      ? control ? 0.88 : 0.82
                      : control
                      ? paired
                        ? connected ? 0.76 : 0.66
                        : connected ? 0.72 : 0.58
                      : paired
                      ? connected ? 0.72 : 0.6
                      : connected ? 0.66 : 0.48}
                    stroke={edge.kind === "loop_back"
                      ? "color-mix(in srgb, var(--warning) 62%, var(--icon-muted))"
                      : edge.kind === "retry" || selected || hovered || (traceActive && traced)
                      ? "var(--primary)"
                      : "var(--icon-muted)"}
                    strokeDasharray={!paired
                      && (edge.kind === "spawn" || edge.kind === "invoke")
                      ? "3 5"
                      : undefined}
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={selected || hovered
                      ? 1.7
                      : traceActive && traced
                      ? 1.4
                      : control
                      ? 1.15
                      : 1.1}
                    style={{ transition: "opacity 150ms ease, stroke 150ms ease" }}
                  />
                  <path
                    aria-hidden="true"
                    className="pointer-events-auto cursor-pointer"
                    d={edge.path}
                    data-execution-edge-line-hit={edge.id}
                    fill="none"
                    onClick={(event) => {
                      event.stopPropagation();
                      selectEdge(edge.id);
                    }}
                    onMouseEnter={() => setHoveredEdgeId(edge.id)}
                    onMouseLeave={() => setHoveredEdgeId((current) => (
                      current === edge.id ? null : current
                    ))}
                    stroke="transparent"
                    strokeLinecap="round"
                    strokeWidth="12"
                    style={{ pointerEvents: "stroke" }}
                  />
                </Fragment>
              );
            })}
            {layout.junctions.map((junction) => {
              const traced = junction.edgeIds.some((edgeId) => tracedEdgeIds.has(edgeId));
              return (
                <circle
                  aria-hidden="true"
                  cx={junction.x}
                  cy={junction.y}
                  data-execution-edge-junction={junction.id}
                  data-execution-edge-junction-kind={junction.kind}
                  fill={traceActive && traced ? "var(--primary)" : "var(--icon-muted)"}
                  key={junction.id}
                  opacity={traceActive && !traced ? 0.12 : traceActive ? 0.9 : 0.68}
                  r={traceActive && traced ? 2.6 : 2.1}
                  stroke="var(--surface-panel-background)"
                  strokeWidth="1.2"
                  style={{ transition: "opacity 150ms ease, fill 150ms ease" }}
                />
              );
            })}
          </svg>

          {layout.edges.map((edge) => (
            <button
              aria-label={`${t("execution.edge_details")}: ${t(EDGE_KIND_LABEL_KEY[edge.kind])}`}
              className="absolute z-10 h-6 w-6 -translate-x-1/2 -translate-y-1/2 rounded-full bg-transparent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--primary) focus-visible:ring-offset-1"
              data-execution-edge-hit-kind={edge.kind}
              data-execution-edge-hit-target={edge.id}
              key={`edge-control:${edge.id}`}
              onClick={(event) => {
                event.stopPropagation();
                selectEdge(edge.id);
              }}
              onBlur={() => setHoveredEdgeId((current) => (
                current === edge.id ? null : current
              ))}
              onFocus={() => setHoveredEdgeId(edge.id)}
              onKeyDown={(event) => {
                if (event.key !== "Enter" && event.key !== " ") {
                  return;
                }
                event.preventDefault();
                event.stopPropagation();
                selectEdge(edge.id);
              }}
              onMouseEnter={() => setHoveredEdgeId(edge.id)}
              onMouseLeave={() => setHoveredEdgeId((current) => (
                current === edge.id ? null : current
              ))}
              style={{ left: edge.x, top: edge.y }}
              title={`${t("execution.edge_details")}: ${t(EDGE_KIND_LABEL_KEY[edge.kind])}`}
              type="button"
            />
          ))}

          {layout.nodes.map(({ height, item, node, size, width, x, y }) => {
            const owner = resolveExecutionGraphNodeAgent(directory, node, item);
            const status = resolveExecutionGraphNodeStatus(node, item);
            const selected = node.id === selectedId;
            const current = node.id === currentId;
            const traced = tracedNodeIds.has(node.id);
            const title = graphNodeTitle(node, item, owner?.name, t);
            const summaryHeading = item?.subject.trim()
              || graphNodeHeading(node, item, t);
            const summaryObjective = compactExecutionNodeObjective(
              item?.objective ?? node.description ?? "",
              owner?.name,
            );
            const descendantCount = collapse.descendantCountByNodeId.get(node.id) ?? 0;
            const collapsed = collapsedNodeIds.has(node.id);
            return (
              <Fragment key={node.id}>
              <button
                aria-label={`${t("execution.details")}: ${title}`}
                aria-pressed={selected}
                className={cn(
                  "absolute z-10 transition-[left,top,transform,border-color,box-shadow,opacity,filter] duration-300 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--primary)",
                  nodePresentation === "summary"
                    ? "grid grid-cols-[auto_minmax(0,1fr)] items-start gap-2 overflow-hidden rounded-[14px] border border-(--surface-control-border) bg-(--surface-panel-background) px-3 py-2 text-left shadow-(--surface-control-shadow) hover:border-[color:color-mix(in_srgb,var(--primary)_34%,var(--surface-control-border))]"
                    : "grid place-items-center rounded-[16px]",
                  selected
                    && nodePresentation === "summary"
                    && "border-(--primary) ring-2 ring-[color:color-mix(in_srgb,var(--primary)_16%,transparent)]",
                  traceActive && !traced && "opacity-25 saturate-[0.55]",
                )}
                data-execution-attempt-id={node.attempt_id || undefined}
                data-execution-current-node={current ? "true" : undefined}
                data-execution-graph-node-id={node.id}
                data-execution-node-presentation={nodePresentation}
                data-execution-node-selected={selected ? "true" : undefined}
                data-execution-node-traced={traceActive && traced ? "true" : undefined}
                data-execution-work-item-id={node.work_item_id || undefined}
                onClick={() => {
                  setSelectedEdgeId(null);
                  setSelectedId(node.id);
                }}
                onBlur={() => setHoveredNodeId((hovered) => (
                  hovered === node.id ? null : hovered
                ))}
                onFocus={() => setHoveredNodeId(node.id)}
                onMouseEnter={() => setHoveredNodeId(node.id)}
                onMouseLeave={() => setHoveredNodeId((hovered) => (
                  hovered === node.id ? null : hovered
                ))}
                style={{
                  height: nodePresentation === "summary" ? height : size + 8,
                  left: x - (nodePresentation === "summary" ? width : size + 8) / 2,
                  top: y - (nodePresentation === "summary" ? height : size + 8) / 2,
                  width: nodePresentation === "summary" ? width : size + 8,
                }}
                title={title}
                type="button"
              >
                {nodePresentation === "summary" ? (
                  <>
                    <ExecutionNodeAvatar
                      agent={owner}
                      current={current}
                      kind={node.kind}
                      selected={selected}
                      size="nested"
                      status={status}
                      title={title}
                      toolName={node.name}
                    />
                    <span className="min-h-0 min-w-0 overflow-hidden">
                      <span className="flex min-w-0 items-center gap-1">
                        <span className="min-w-0 flex-1 truncate text-xs font-semibold leading-4 text-(--text-strong)">
                          {summaryHeading}
                        </span>
                        {item?.terminal ? (
                          <span className="shrink-0 rounded-full bg-[color:color-mix(in_srgb,var(--primary)_10%,var(--surface-muted-background))] px-1.5 py-0.5 text-[9px] font-medium leading-3 text-(--primary)">
                            {t("execution.workflow_terminal_short")}
                          </span>
                        ) : item?.required ? (
                          <span className="shrink-0 rounded-full bg-(--surface-muted-background) px-1.5 py-0.5 text-[9px] font-medium leading-3 text-(--text-soft)">
                            {t("execution.required")}
                          </span>
                        ) : null}
                      </span>
                      {summaryObjective ? (
                        <span className="mt-0.5 line-clamp-2 max-h-[30px] overflow-hidden text-[10px] leading-[15px] text-(--text-muted)">
                          {summaryObjective}
                        </span>
                      ) : null}
                    </span>
                  </>
                ) : (
                  <ExecutionNodeAvatar
                    agent={owner}
                    current={current}
                    kind={node.kind}
                    selected={selected}
                    size={node.kind === "subagent" || node.kind === "tool"
                      ? "nested"
                      : "graph"}
                    status={status}
                    title={title}
                    toolName={node.name}
                  />
                )}
              </button>
              {descendantCount > 0 ? (
                <button
                  aria-label={collapsed
                    ? t("execution.expand_node")
                    : t("execution.collapse_node")}
                  aria-pressed={collapsed}
                  className={cn(
                    "absolute z-20 flex h-5 min-w-5 items-center justify-center gap-0.5 rounded-full border border-(--surface-control-border) bg-(--surface-panel-background) px-1 text-[9px] font-semibold tabular-nums text-(--text-soft) shadow-sm transition hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--primary)",
                    traceActive && !traced && "opacity-25",
                  )}
                  data-execution-collapse-node={node.id}
                  data-execution-hidden-node-count={collapsed ? descendantCount : 0}
                  onClick={() => setCollapsedNodeIds((currentValue) => {
                    const next = new Set(currentValue);
                    if (next.has(node.id)) {
                      next.delete(node.id);
                    } else {
                      next.add(node.id);
                    }
                    return next;
                  })}
                  style={{
                    left: x + width / 2 - 5,
                    top: y + height / 2 - 5,
                  }}
                  title={collapsed
                    ? t("execution.expand_node")
                    : t("execution.collapse_node")}
                  type="button"
                >
                  {collapsed ? (
                    <ChevronsUpDown className="h-2.5 w-2.5" />
                  ) : (
                    <ChevronsDownUp className="h-2.5 w-2.5" />
                  )}
                  <span>{descendantCount}</span>
                </button>
              ) : null}
              </Fragment>
            );
          })}
          {selectedLayoutNode && selectedInspectorStyle ? (
            <ExecutionNodeInspector
              attempt={selectedAttempt}
              directory={directory}
              execution={execution}
              item={selectedItem}
              node={selectedLayoutNode.node}
              onClose={() => setSelectedId(null)}
              onOpenWorkspaceFile={onOpenWorkspaceFile}
              style={selectedInspectorStyle}
              taskRun={selectedTaskRun}
            />
          ) : null}
          {selectedLayoutEdge && selectedEdgeInspectorStyle ? (
            <ExecutionEdgeInspector
              edge={selectedLayoutEdge.edge}
              execution={execution}
              onClose={() => setSelectedEdgeId(null)}
              style={selectedEdgeInspectorStyle}
            />
          ) : null}
        </div>
      </div>
      {/* eslint-enable jsx-a11y/no-noninteractive-element-interactions, jsx-a11y/no-noninteractive-tabindex */}
        </div>
      </div>
      {!expandedMode && expandedOpen ? (
        <ExecutionWorkGraphExpandedDialog
          currentId={currentId}
          directory={directory}
          execution={execution}
          nodePresentation={nodePresentation}
          onClose={() => setExpandedOpen(false)}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          taskRuns={taskRuns}
        />
      ) : null}
    </Fragment>
  );
}

function ExecutionWorkGraphExpandedDialog({
  currentId,
  directory,
  execution,
  nodePresentation,
  onClose,
  onOpenWorkspaceFile,
  taskRuns,
}: Omit<ExecutionWorkGraphCanvasProps, "expandedMode"> & {
  onClose: () => void;
}) {
  const { t } = useI18n();
  const titleId = `execution-workgraph-expanded-${useId().replace(/:/g, "")}`;
  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="p-4"
        data-execution-workgraph-expanded-dialog
        labelledBy={titleId}
        layer="dialogNested"
        onClose={onClose}
      >
        <UiDialogShell
          className="relative h-[calc(100dvh-32px)]"
          size="wide"
          style={{ maxWidth: "calc(100vw - 32px)" }}
        >
          <h2 className="sr-only" id={titleId}>
            {t("execution.label")}
          </h2>
          <UiDialogCloseButton
            className="absolute right-3 top-3 z-50 border border-(--surface-control-border) bg-(--surface-panel-background) shadow-(--surface-control-shadow)"
            onClose={onClose}
          />
          <UiDialogBody className="flex min-h-0 flex-1 overflow-hidden bg-(--surface-canvas-background) p-0">
            <ExecutionWorkGraphCanvas
              currentId={currentId}
              directory={directory}
              execution={execution}
              expandedMode
              nodePresentation={nodePresentation}
              onOpenWorkspaceFile={onOpenWorkspaceFile}
              taskRuns={taskRuns}
            />
          </UiDialogBody>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

function ExecutionNodeInspector({
  attempt,
  directory,
  execution,
  item,
  node,
  onClose,
  onOpenWorkspaceFile,
  style,
  taskRun,
}: {
  attempt: ExecutionAttemptView | null;
  directory: ExecutionAgentDirectory;
  execution: ExecutionView;
  item: ExecutionWorkItemView | null;
  node: ExecutionGraphNodeView;
  onClose: () => void;
  onOpenWorkspaceFile?: (
    path: string,
    workspaceAgentId?: string | null,
  ) => void;
  style: CSSProperties;
  taskRun: ConversationTaskRun | null;
}) {
  const { t } = useI18n();
  const parentNode = node.parent_node_id
    ? execution.graph?.nodes?.find((candidate) => candidate.id === node.parent_node_id)
      ?? null
    : null;
  const parentItem = parentNode
    ? execution.work_items?.find((candidate) => (
      candidate.id === parentNode.work_item_id
    )) ?? null
    : null;
  const owner = resolveExecutionGraphNodeAgent(directory, node, item)
    ?? (parentNode
      ? resolveExecutionGraphNodeAgent(directory, parentNode, parentItem)
      : null);
  const objectiveSource = item?.objective
    ?? node.description
    ?? node.name
    ?? "";
  const objective = compactExecutionNodeObjective(objectiveSource, owner?.name);
  const deliverable = item?.deliverable.trim() ?? "";
  const showDeliverable = deliverable
    && deliverable.toLocaleLowerCase() !== objective.toLocaleLowerCase();
  const status = resolveExecutionGraphNodeStatus(node, item);
  const statusLabel = attempt && node.kind === "subagent"
    ? t(ATTEMPT_STATUS_LABEL_KEY[attempt.status])
    : t(WORK_ITEM_STATUS_LABEL_KEY[status]);
  const heading = graphNodeHeading(node, item, t);
  const relatedSubject = node.kind === "agent" ? "" : item?.subject.trim() ?? "";
  const currentSubmissionMatchesNode = Boolean(item?.submission && (
    node.attempt_id
      ? item.submission.attempt_id === node.attempt_id
      : node.kind === "gate" && node.subject_id === item.submission.id
  ));
  const submission = currentSubmissionMatchesNode
    ? item?.submission?.result_summary.trim() ?? ""
    : "";
  const review = currentSubmissionMatchesNode
    ? item?.acceptance?.feedback?.trim() ?? ""
    : "";
  const resultSummary = node.result_summary?.trim() ?? "";
  const errorSummary = normalizeExecutionNodeDisplayText(
    node.error_summary || attempt?.failure_reason || "",
  );
  const visibleErrorSummary = errorSummary
    || (status === "failed" ? t("execution.error_summary_unavailable") : "");
  const retryEdges = (execution.graph?.edges ?? []).filter((edge) => (
    edge.kind === "retry"
    && (edge.source_node_id === node.id || edge.target_node_id === node.id)
  ));
  const controlReturnObserved = (execution.graph?.edges ?? []).some((edge) => (
    edge.kind === "loop_back" && edge.source_node_id === node.id
  ));
  const childNodes = (execution.graph?.nodes ?? [])
    .filter((candidate) => candidate.parent_node_id === node.id)
    .sort((left, right) => (
      left.position - right.position || left.id.localeCompare(right.id)
    ));
  return (
    <aside
      className="soft-scrollbar absolute z-30 max-h-[min(70vh,28rem)] w-[19rem] max-w-[calc(100%-1rem)] cursor-auto overflow-auto rounded-[14px] border border-(--surface-popover-border) bg-(--surface-popover-background) shadow-(--surface-popover-shadow)"
      aria-label={`${t("execution.details")}: ${heading}`}
      data-execution-selected-node-detail={node.id}
      data-execution-selected-node-detail-mode="popover"
      style={style}
    >
      <div className="sticky top-0 z-10 flex min-w-0 items-center gap-2 border-b dialog-divider bg-(--surface-popover-background) px-3 py-3">
        <ExecutionNodeAvatar
          agent={owner}
          current={status === "running"}
          kind={node.kind}
          size="graph"
          status={status}
          title={heading}
          toolName={node.name}
        />
        <div className="min-w-0 flex-1">
          <h3 className="truncate text-compact font-semibold text-(--text-strong)">
            {heading}
          </h3>
          <p className="mt-0.5 flex min-w-0 items-center gap-1 text-[10px] text-(--text-soft)">
            {owner ? <span className="truncate">{owner.name}</span> : null}
            {owner ? <span aria-hidden="true">·</span> : null}
            <span className={cn("shrink-0 font-medium", selectedStatusTone(status))}>
              {statusLabel}
            </span>
          </p>
        </div>
        <button
          aria-label={t("execution.close_node_details")}
          className="grid h-7 w-7 shrink-0 place-items-center rounded-[8px] text-(--icon-muted) transition-[background,color] hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--primary)"
          onClick={onClose}
          title={t("execution.close_node_details")}
          type="button"
        >
          <X aria-hidden="true" className="h-3.5 w-3.5" />
        </button>
      </div>
      <div className="space-y-3 px-3 py-3">
        {relatedSubject ? (
          <p className="text-[11px] font-medium leading-4 text-(--text-default)">
            {relatedSubject}
          </p>
        ) : null}
        {objective ? (
          <NodeDetailSection label={t("execution.objective")}>
            <p>{objective}</p>
          </NodeDetailSection>
        ) : null}
        {showDeliverable ? (
          <NodeDetailSection label={t("execution.deliverable")}>
            <p>{deliverable}</p>
          </NodeDetailSection>
        ) : null}
        {(item?.acceptance_criteria?.length ?? 0) > 0 ? (
          <NodeDetailSection label={t("execution.acceptance")}>
            <ul className="space-y-1">
              {item?.acceptance_criteria?.slice(0, 4).map((criterion) => (
                <li className="flex gap-2" key={criterion}>
                  <span aria-hidden="true" className="mt-[7px] h-1 w-1 shrink-0 rounded-full bg-(--icon-muted)" />
                  <span>{criterion}</span>
                </li>
              ))}
            </ul>
          </NodeDetailSection>
        ) : null}
        {item?.block_reason?.trim() ? (
          <NodeDetailSection label={t("execution.blocker")}>
            <p>{item.block_reason.trim()}</p>
          </NodeDetailSection>
        ) : null}
        {item?.needed_input?.trim() ? (
          <NodeDetailSection label={t("execution.needed_input")}>
            <p>{item.needed_input.trim()}</p>
          </NodeDetailSection>
        ) : null}
        {visibleErrorSummary ? (
          <NodeDetailSection label={t("execution.error_summary")}>
            <p>{visibleErrorSummary}</p>
            {node.error_code?.trim() ? (
              <p className="mt-1 font-mono text-[10px] text-(--text-soft)">
                {node.error_code.trim()}
              </p>
            ) : null}
          </NodeDetailSection>
        ) : null}
        {resultSummary ? (
          <NodeDetailSection label={t("execution.result_summary")}>
            <p>{resultSummary}</p>
            {node.summary_truncated ? (
              <p className="mt-1 text-[10px] text-(--text-soft)">
                {t("execution.summary_truncated")}
              </p>
            ) : null}
          </NodeDetailSection>
        ) : null}
        {(node.duration_ms ?? 0) > 0 ? (
          <NodeDetailSection label={t("execution.duration")}>
            <p>{formatNodeDuration(node.duration_ms ?? 0)}</p>
          </NodeDetailSection>
        ) : null}
        {controlReturnObserved ? (
          <NodeDetailSection label={t("execution.control_return")}>
            <p>{t("execution.control_return_observed")}</p>
          </NodeDetailSection>
        ) : null}
        {retryEdges.length > 0 ? (
          <NodeDetailSection label={t("execution.retry_relation")}>
            <p>{t("execution.retry_relation_count", { count: retryEdges.length })}</p>
          </NodeDetailSection>
        ) : null}
        {submission ? (
          <NodeDetailSection label={t("execution.submission")}>
            <p>{submission}</p>
          </NodeDetailSection>
        ) : null}
        {review ? (
          <NodeDetailSection label={t("execution.review")}>
            <p>{review}</p>
          </NodeDetailSection>
        ) : null}
        {taskRun ? <ExecutionNodeTaskList run={taskRun} /> : null}
        <ExecutionNodeRunHistory
          item={item}
          node={node}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          workspaceAgentId={owner?.id ?? node.agent_id}
        />
        {childNodes.length > 0 ? (
          <ExecutionNodeRunList
            directory={directory}
            execution={execution}
            nodes={childNodes}
          />
        ) : null}
      </div>
    </aside>
  );
}

function ExecutionEdgeInspector({
  edge,
  execution,
  onClose,
  style,
}: {
  edge: ExecutionGraphEdgeView;
  execution: ExecutionView;
  onClose: () => void;
  style: CSSProperties;
}) {
  const { t } = useI18n();
  const nodes = execution.graph?.nodes ?? [];
  const sourceNode = nodes.find((node) => node.id === edge.source_node_id) ?? null;
  const targetNode = nodes.find((node) => node.id === edge.target_node_id) ?? null;
  const sourceItem = sourceNode
    ? execution.work_items?.find((item) => item.id === sourceNode.work_item_id) ?? null
    : null;
  const targetItem = targetNode
    ? execution.work_items?.find((item) => item.id === targetNode.work_item_id) ?? null
    : null;
  const sourceHeading = sourceNode
    ? graphNodeHeading(sourceNode, sourceItem, t)
    : edge.source_node_id;
  const targetHeading = targetNode
    ? graphNodeHeading(targetNode, targetItem, t)
    : edge.target_node_id;
  const sourceSummary = normalizeExecutionNodeDisplayText(
    sourceNode?.error_summary || sourceNode?.result_summary || "",
  );
  const targetSummary = normalizeExecutionNodeDisplayText(
    targetNode?.error_summary || targetNode?.result_summary || "",
  );
  return (
    <aside
      aria-label={`${t("execution.edge_details")}: ${t(EDGE_KIND_LABEL_KEY[edge.kind])}`}
      className="soft-scrollbar absolute z-30 max-h-[min(70vh,28rem)] w-[19rem] max-w-[calc(100%-1rem)] cursor-auto overflow-auto rounded-[14px] border border-(--surface-popover-border) bg-(--surface-popover-background) shadow-(--surface-popover-shadow)"
      data-execution-selected-edge-detail={edge.id}
      style={style}
    >
      <div className="sticky top-0 z-10 flex min-w-0 items-center gap-2 border-b dialog-divider bg-(--surface-popover-background) px-3 py-3">
        <span
          aria-hidden="true"
          className={cn(
            "h-8 w-1 shrink-0 rounded-full",
            edge.kind === "loop_back"
              ? "bg-(--warning)"
              : edge.kind === "retry"
              ? "bg-(--primary)"
              : "bg-(--icon-muted)",
          )}
        />
        <div className="min-w-0 flex-1">
          <h3 className="truncate text-compact font-semibold text-(--text-strong)">
            {t("execution.edge_details")}
          </h3>
          <p className="mt-0.5 truncate text-[10px] font-medium text-(--text-soft)">
            {t(EDGE_KIND_LABEL_KEY[edge.kind])}
          </p>
        </div>
        <button
          aria-label={t("execution.close_edge_details")}
          className="grid h-7 w-7 shrink-0 place-items-center rounded-[8px] text-(--icon-muted) transition-[background,color] hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--primary)"
          onClick={onClose}
          title={t("execution.close_edge_details")}
          type="button"
        >
          <X aria-hidden="true" className="h-3.5 w-3.5" />
        </button>
      </div>
      <div className="space-y-3 px-3 py-3">
        <NodeDetailSection label={t("execution.edge_relation")}>
          <p>{t(EDGE_KIND_DETAIL_KEY[edge.kind])}</p>
        </NodeDetailSection>
        <NodeDetailSection label={t("execution.edge_source")}>
          <p className="font-medium text-(--text-default)">{sourceHeading}</p>
          {sourceSummary ? (
            <p className="mt-1 text-(--text-soft)">{sourceSummary}</p>
          ) : null}
        </NodeDetailSection>
        <NodeDetailSection label={t("execution.edge_target")}>
          <p className="font-medium text-(--text-default)">{targetHeading}</p>
          {targetSummary ? (
            <p className="mt-1 text-(--text-soft)">{targetSummary}</p>
          ) : null}
        </NodeDetailSection>
        {edge.created_at ? (
          <NodeDetailSection label={t("execution.edge_observed_at")}>
            <p className="font-mono text-[10px]">
              {formatEdgeObservedAt(edge.created_at)}
            </p>
          </NodeDetailSection>
        ) : null}
        {edge.source_node_run_id || edge.target_node_run_id ? (
          <NodeDetailSection label={t("execution.edge_run_identity")}>
            <p className="break-all font-mono text-[10px] text-(--text-soft)">
              {edge.source_node_run_id || edge.source_node_id}
              <span aria-hidden="true"> → </span>
              {edge.target_node_run_id || edge.target_node_id}
            </p>
          </NodeDetailSection>
        ) : null}
      </div>
    </aside>
  );
}

function NodeDetailSection({
  children,
  label,
}: {
  children: ReactNode;
  label: string;
}) {
  return (
    <section>
      <h4 className="mb-1 text-[10px] font-medium text-(--text-soft)">
        {label}
      </h4>
      <div className="text-[11px] leading-[1.55] text-(--text-default)">
        {children}
      </div>
    </section>
  );
}

function ExecutionNodeRunList({
  directory,
  execution,
  nodes,
}: {
  directory: ExecutionAgentDirectory;
  execution: ExecutionView;
  nodes: readonly ExecutionGraphNodeView[];
}) {
  const { t } = useI18n();
  return (
    <NodeDetailSection label={t("execution.runtime_activity")}>
      <ul className="space-y-1.5" data-execution-runtime-activity>
        {nodes.slice(0, 8).map((node) => {
          const item = execution.work_items?.find(
            (candidate) => candidate.id === node.work_item_id,
          ) ?? null;
          const owner = resolveExecutionGraphNodeAgent(directory, node, item);
          const status = resolveExecutionGraphNodeStatus(node, item);
          const summary = node.error_summary?.trim()
            || node.result_summary?.trim()
            || node.description?.trim()
            || "";
          return (
            <li
              className="flex min-w-0 gap-2 rounded-[9px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-control-background)_68%,transparent)] px-2 py-1.5"
              data-execution-runtime-node={node.id}
              key={node.id}
            >
              <ExecutionNodeAvatar
                agent={owner}
                current={status === "running"}
                kind={node.kind}
                size="nested"
                status={status}
                title={graphNodeHeading(node, item, t)}
                toolName={node.name}
              />
              <div className="min-w-0 flex-1">
                <div className="flex min-w-0 items-center gap-1.5">
                  <span className="truncate font-medium text-(--text-default)">
                    {graphNodeHeading(node, item, t)}
                  </span>
                  <span
                    aria-hidden="true"
                    className={cn(
                      "h-1.5 w-1.5 shrink-0 rounded-full bg-current",
                      selectedStatusTone(status),
                    )}
                  />
                </div>
                {summary ? (
                  <p className="mt-0.5 line-clamp-2 text-[10px] leading-4 text-(--text-soft)">
                    {summary}
                  </p>
                ) : null}
              </div>
            </li>
          );
        })}
      </ul>
      {nodes.length > 8 ? (
        <p className="mt-1 text-[10px] text-(--text-soft)">
          {t("execution.runtime_activity_more", { count: nodes.length - 8 })}
        </p>
      ) : null}
    </NodeDetailSection>
  );
}

function resolveNodeInspectorStyle(
  canvasWidth: number,
  x: number,
  y: number,
  nodeSize: number,
  zoom: number,
): CSSProperties {
  const safeZoom = clampExecutionGraphZoom(zoom);
  const visualWidth = Math.min(
    NODE_INSPECTOR_WIDTH,
    Math.max(
      240,
      canvasWidth * safeZoom - NODE_INSPECTOR_EDGE_PADDING * 2,
    ),
  );
  const localWidth = visualWidth / safeZoom;
  const localGap = NODE_INSPECTOR_GAP / safeZoom;
  const localEdgePadding = NODE_INSPECTOR_EDGE_PADDING / safeZoom;
  const right = x + nodeSize / 2 + localGap;
  const fitsRight = right + localWidth
    <= canvasWidth - localEdgePadding;
  const left = fitsRight
    ? right
    : Math.max(
        localEdgePadding,
        x - nodeSize / 2 - localGap - localWidth,
      );
  return {
    left,
    top: Math.max(localEdgePadding, y - 32 / safeZoom),
    transform: `scale(${1 / safeZoom})`,
    transformOrigin: "top left",
    width: visualWidth,
  };
}

function formatNodeDuration(durationMS: number): string {
  if (durationMS < 1_000) {
    return `${Math.round(durationMS)}ms`;
  }
  const seconds = durationMS / 1_000;
  if (seconds < 60) {
    return `${seconds < 10 ? seconds.toFixed(1) : Math.round(seconds)}s`;
  }
  const minutes = Math.floor(seconds / 60);
  return `${minutes}m ${Math.round(seconds % 60)}s`;
}

function formatEdgeObservedAt(value: string): string {
  const timestamp = new Date(value);
  return Number.isNaN(timestamp.getTime()) ? value : timestamp.toISOString();
}

function graphNodeTitle(
  node: ExecutionGraphNodeView,
  item: ExecutionWorkItemView | null,
  ownerName: string | undefined,
  t: (key: TranslationKey) => string,
): string {
  if (node.kind === "tool") {
    return node.name?.trim() || t("execution.node_tool");
  }
  if (node.kind === "gate") {
    const subject = item?.subject ?? node.description?.trim();
    const gate = node.name === "objective_alignment"
      ? t("execution.node_alignment_gate")
      : t("execution.node_gate");
    return ownerName
      ? `${subject ? `${subject} · ` : ""}${gate} · ${ownerName}`
      : `${subject ? `${subject} · ` : ""}${gate}`;
  }
  if (node.kind === "subagent") {
    const identity = ownerName || t("execution.attempt_subagent");
    return item?.subject ? `${identity} · ${item.subject}` : identity;
  }
  const subject = item?.subject
    ?? node.description?.trim()
    ?? ownerName
    ?? t("execution.owner_unassigned");
  return ownerName ? `${subject} · ${ownerName}` : subject;
}

function graphNodeHeading(
  node: ExecutionGraphNodeView,
  item: ExecutionWorkItemView | null,
  t: (key: TranslationKey) => string,
): string {
  if (node.kind === "tool") {
    return node.name?.trim() || t("execution.node_tool");
  }
  if (node.kind === "gate") {
    return node.name === "objective_alignment"
      ? t("execution.node_alignment_gate")
      : t("execution.node_gate");
  }
  if (node.kind === "subagent") {
    return node.name?.trim() || t("execution.attempt_subagent");
  }
  return item?.subject
    ?? node.description?.trim()
    ?? node.name?.trim()
    ?? t("execution.owner_unassigned");
}

function selectedStatusTone(status: ExecutionWorkItemStatus): string {
  if (status === "accepted") {
    return "text-(--success)";
  }
  if (
    status === "blocked"
    || status === "changes_requested"
    || status === "failed"
  ) {
    return "text-(--warning)";
  }
  if (
    status === "running"
    || status === "submitted"
    || status === "ready"
    || status === "assigned"
  ) {
    return "text-(--primary)";
  }
  return "text-(--text-soft)";
}
