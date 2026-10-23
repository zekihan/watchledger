import { useEffect, useId, useRef, useState } from "react";
export function ChoiceDialog({
  title,
  yes,
  no,
  submit,
  cancel,
  onClose,
}: {
  title: string;
  yes: string;
  no: string;
  submit: string;
  cancel: string;
  onClose: (value: boolean | null) => void;
}) {
  const titleID = useId();
  const ref = useRef<HTMLDialogElement>(null);
  const [value, setValue] = useState(false);
  useEffect(() => {
    const previous = document.activeElement;
    ref.current?.showModal();
    return () => {
      if (previous instanceof HTMLElement && previous.isConnected) previous.focus();
    };
  }, []);
  return (
    <dialog
      ref={ref}
      aria-labelledby={titleID}
      onCancel={(e) => {
        e.preventDefault();
        onClose(null);
      }}
      className="choice-dialog"
    >
      <form
        onSubmit={(e) => {
          e.preventDefault();
          onClose(value);
        }}
      >
        <h2 id={titleID}>{title}</h2>
        <label className="check-label">
          <input
            autoFocus
            type="radio"
            name="choice"
            checked={!value}
            onChange={() => {
              setValue(false);
            }}
          />
          {no}
        </label>
        <label className="check-label">
          <input
            type="radio"
            name="choice"
            checked={value}
            onChange={() => {
              setValue(true);
            }}
          />
          {yes}
        </label>
        <div className="toolbar">
          <button type="submit" className="primary">
            {submit}
          </button>
          <button
            type="button"
            onClick={() => {
              onClose(null);
            }}
          >
            {cancel}
          </button>
        </div>
      </form>
    </dialog>
  );
}
