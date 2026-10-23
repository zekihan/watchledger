import { afterEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { StatusReplacement } from "./status-replacement";
import { en } from "./i18n";
afterEach(cleanup);
test("status removal starts without a destination and supports an explicit different custom status", () => {
  HTMLDialogElement.prototype.showModal = function () {
    this.open = true;
  };
  const submit = vi.fn();
  const cancel = vi.fn();
  render(
    <StatusReplacement
      status={{ id: "old", name: "Old", base: "watching", used: true }}
      statuses={[
        { id: "old", name: "Old", base: "watching", used: true },
        { id: "next", name: "Next custom", base: "hold", used: false },
      ]}
      pending={false}
      error={null}
      t={(key) => en[key]}
      submit={submit}
      cancel={cancel}
    />,
  );
  expect(screen.getByRole("dialog", { name: en.replaceStatusTitle })).toBeTruthy();
  expect(screen.getByRole<HTMLButtonElement>("button", { name: en.remove }).disabled).toBe(true);
  expect(screen.queryByRole("option", { name: "Old" })).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: en.cancel }));
  expect(cancel).toHaveBeenCalledTimes(1);
  expect(submit).not.toHaveBeenCalled();
  fireEvent.change(screen.getByRole("combobox"), { target: { value: "next" } });
  fireEvent.click(screen.getByRole("button", { name: en.remove }));
  expect(submit).toHaveBeenCalledWith("next");
});
