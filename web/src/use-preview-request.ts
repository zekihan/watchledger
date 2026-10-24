import { useEffect, useRef } from "react";

// Preview requests may persist a source archive, but must never apply it. Stop
// superseded work before sending it, and ignore late responses after navigation.
export function usePreviewRequest() {
  const active = useRef<AbortController | null>(null);
  useEffect(
    () => () => {
      active.current?.abort();
      active.current = null;
    },
    [],
  );
  return () => {
    active.current?.abort();
    const controller = new AbortController();
    active.current = controller;
    return {
      signal: controller.signal,
      current: () => active.current === controller && !controller.signal.aborted,
    };
  };
}
