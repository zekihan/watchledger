import { afterEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { ChoiceDialog } from "./tracking-ui";
afterEach(cleanup);
test("tracking dialog is named, defaults to the safe option, and restores initiating focus", () => {
  HTMLDialogElement.prototype.showModal = function () {
    this.open = true;
  };
  const opener = document.createElement("button");
  document.body.append(opener);
  opener.focus();
  const close = vi.fn();
  const view = render(
    <ChoiceDialog
      title="Review linked removal"
      yes="Remove linked"
      no="Keep linked"
      submit="Continue"
      cancel="Cancel"
      onClose={close}
    />,
  );
  const dialog = screen.getByRole("dialog", { name: "Review linked removal" });
  expect(screen.getByRole<HTMLInputElement>("radio", { name: "Keep linked" }).checked).toBe(true);
  fireEvent(dialog, new Event("cancel", { cancelable: true }));
  expect(close).toHaveBeenCalledWith(null);
  view.unmount();
  expect(document.activeElement).toBe(opener);
  opener.remove();
});
