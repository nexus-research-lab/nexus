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
  setWidth: (width: number) => void;
  width: number;
}

export function useSidebarPanelResize({
  setWidth,
  width,
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
      startWidthRef.current = width;
      setIsResizeHotzoneActive(true);
      event.currentTarget.setPointerCapture(event.pointerId);
    },
    [width],
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
      setWidth(nextWidth);
    },
    [setWidth],
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
    handlePointerDown,
    handlePointerLeave,
    handlePointerMove,
    handlePointerUp,
    isResizeHotzoneActive,
    rootRef,
  };
}
