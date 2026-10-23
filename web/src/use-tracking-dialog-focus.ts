import { useEffect, useRef } from "react";
export function useTrackingDialogFocus(pending: boolean) {
  const trigger = useRef<HTMLElement | null>(null);
  const restore = useRef(false);
  useEffect(() => {
    if (!pending && restore.current) {
      restore.current = false;
      if (trigger.current?.isConnected) trigger.current.focus();
    }
  }, [pending]);
  return {
    capture: () => {
      trigger.current =
        document.activeElement instanceof HTMLElement ? document.activeElement : null;
      restore.current = false;
    },
    requestRestore: () => {
      restore.current = true;
    },
  };
}
