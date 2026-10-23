import { useEffect, useId, useRef, useState } from "react";
import type { Status } from "./api";
import type { T } from "./App";
export function StatusReplacement({
  status,
  statuses,
  pending,
  error,
  t,
  cancel,
  submit,
}: {
  status: Status;
  statuses: Status[];
  pending: boolean;
  error: Error | null;
  t: T;
  cancel: () => void;
  submit: (replacement: string) => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const title = useId();
  const [replacement, setReplacement] = useState("");
  useEffect(() => {
    const previous = document.activeElement;
    dialog.current?.showModal();
    return () => {
      if (previous instanceof HTMLElement && previous.isConnected) previous.focus();
    };
  }, []);
  return (
    <dialog
      ref={dialog}
      aria-labelledby={title}
      className="choice-dialog"
      onCancel={(event) => {
        event.preventDefault();
        if (!pending) cancel();
      }}
    >
      <form
        onSubmit={(event) => {
          event.preventDefault();
          if (replacement && !pending) submit(replacement);
        }}
      >
        <h2 id={title}>{t("replaceStatusTitle")}</h2>
        <p>{status.name}</p>
        <p>{t("replaceStatusHint")}</p>
        <label>
          {t("replacement")}
          <select
            value={replacement}
            disabled={pending}
            onChange={(event) => {
              setReplacement(event.target.value);
            }}
          >
            <option value="">{t("select")}</option>
            {(["planned", "watching", "completed", "hold", "dropped"] as const).map((base) => (
              <option key={base} value={base}>
                {t(base)}
              </option>
            ))}
            {statuses
              .filter((item) => item.id !== status.id)
              .map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                </option>
              ))}
          </select>
        </label>
        {error && <p role="alert">{error.message}</p>}
        <div className="toolbar">
          <button type="button" autoFocus disabled={pending} onClick={cancel}>
            {t("cancel")}
          </button>
          <button type="submit" disabled={!replacement || pending}>
            {t("remove")}
          </button>
        </div>
      </form>
    </dialog>
  );
}
