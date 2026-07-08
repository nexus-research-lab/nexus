import {
  type PointerEvent as ReactPointerEvent,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";

const SIDEBAR_RESIZE_HOTZONE_WIDTH = 8;
const MODAL_ROOT_SELECTOR = "[data-modal-root='true']";

interface UseSidebarPanelResizeOptions {
  set_wide_panel_width: (width: number) => void;
  wide_panel_width: number;
}

export function useSidebarPanelResize({
  set_wide_panel_width: setWidePanelWidth,
  wide_panel_width: widePanelWidth,
}: UseSidebarPanelResizeOptions) {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const [isResizeHotzoneActive, setIsResizeHotzoneActive] = useState(false);
  const isDraggingRef = useRef(false);
  const startXRef = useRef(0);
  const startWidthRef = useRef(0);

  const handlePointerDown = useCallback(
    (event: ReactPointerEvent) => {
      if (event.target instanceof HTMLElement && event.target.closest(MODAL_ROOT_SELECTOR)) {
        return;
      }
      const rootElement = rootRef.current;
      if (!rootElement) {
        return;
      }

      const rect = rootElement.getBoundingClientRect();
      const distanceToRightEdge = rect.right - event.clientX;
      if (distanceToRightEdge > SIDEBAR_RESIZE_HOTZONE_WIDTH) {
        return;
      }

      event.preventDefault();
      isDraggingRef.current = true;
      startXRef.current = event.clientX;
      startWidthRef.current = widePanelWidth;
      setIsResizeHotzoneActive(true);
      event.currentTarget.setPointerCapture(event.pointerId);
    },
    [widePanelWidth],
  );

  const handlePointerMove = useCallback(
    (event: ReactPointerEvent) => {
      if (event.target instanceof HTMLElement && event.target.closest(MODAL_ROOT_SELECTOR)) {
        if (!isDraggingRef.current) {
          setIsResizeHotzoneActive(false);
        }
        return;
      }
      const rootElement = rootRef.current;
      if (!rootElement) {
        return;
      }

      if (!isDraggingRef.current) {
        const rect = rootElement.getBoundingClientRect();
        const distanceToRightEdge = rect.right - event.clientX;
        setIsResizeHotzoneActive(distanceToRightEdge <= SIDEBAR_RESIZE_HOTZONE_WIDTH);
        return;
      }

      const delta = event.clientX - startXRef.current;
      const nextWidth = startWidthRef.current + delta;
      setWidePanelWidth(nextWidth);
    },
    [setWidePanelWidth],
  );

  const handlePointerUp = useCallback(() => {
    isDraggingRef.current = false;
    setIsResizeHotzoneActive(false);
  }, []);

  const handlePointerLeave = useCallback(() => {
    if (isDraggingRef.current) {
      return;
    }
    setIsResizeHotzoneActive(false);
  }, []);

  useEffect(() => {
    const handleSelectStart = (event: Event) => {
      if (isDraggingRef.current) {
        event.preventDefault();
      }
    };
    document.addEventListener("selectstart", handleSelectStart);
    return () => document.removeEventListener("selectstart", handleSelectStart);
  }, []);

  return {
    handle_pointer_down: handlePointerDown,
    handle_pointer_leave: handlePointerLeave,
    handle_pointer_move: handlePointerMove,
    handle_pointer_up: handlePointerUp,
    is_resize_hotzone_active: isResizeHotzoneActive,
    root_ref: rootRef,
  };
}
