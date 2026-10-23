import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ImportReview } from "./import-review";
import { en } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
const data = {
  source: {
    title: "Synthetic gaps",
    issues: [],
    episodes: [
      { key: "shows:1:s1:e1", season: 1, number: 1, date: { day: "2025-02-03" } },
      { key: "shows:1:s1:e3", season: 1, number: 3, date: {} },
    ],
  },
  target: "tmdb:tv:1",
  episodes: [
    { id: "t1", season: 1, number: 1, retired: false },
    { id: "t2", season: 1, number: 2, retired: false },
  ],
  links: [],
};
function mount(backed = true) {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
        })
      }
    >
      <ImportReview id="review" t={(key) => en[key]} backed={backed} close={() => undefined} />
    </QueryClientProvider>,
  );
}
test("episode mappings start empty and require distinct targets plus explicit confirmation", async () => {
  const fetch = vi.fn((path: string) =>
    Promise.resolve(
      new Response(
        JSON.stringify(
          path.endsWith("/resolve")
            ? { status: "resolved", history_added: 2, app_decisions_preserved: 0 }
            : data,
        ),
        { status: 200 },
      ),
    ),
  );
  vi.stubGlobal("fetch", fetch);
  mount();
  const targets = await screen.findAllByRole<HTMLSelectElement>("combobox");
  const [first, second] = targets;
  if (!first || !second) throw new Error("expected two episode selectors");
  expect(targets.map((target) => target.value)).toEqual(["", ""]);
  expect(screen.getByText(en.unknown)).toBeTruthy();
  const button = screen.getByRole<HTMLButtonElement>("button", { name: en.resolveImport });
  const checkbox = screen.getByRole<HTMLInputElement>("checkbox", {
    name: en.confirmImportMapping,
  });
  await userEvent.click(checkbox);
  expect(button.disabled).toBe(true);
  await userEvent.selectOptions(first, "t1");
  expect(checkbox.checked).toBe(false);
  await userEvent.selectOptions(second, "t1");
  await userEvent.click(checkbox);
  expect(button.disabled).toBe(true);
  await userEvent.selectOptions(second, "t2");
  expect(button.disabled).toBe(true);
  await userEvent.click(checkbox);
  expect(button.disabled).toBe(false);
  await userEvent.click(button);
  expect(await screen.findByText(en.importResolved)).toBeTruthy();
  expect(fetch.mock.calls.filter(([path]) => path.endsWith("/resolve"))).toHaveLength(1);
});
test("confirmed identities remain fixed and backup prerequisite blocks applying", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(() =>
      Promise.resolve(
        new Response(
          JSON.stringify({ ...data, links: [{ source_key: "shows:1:s1:e1", episode_id: "t1" }] }),
          { status: 200 },
        ),
      ),
    ),
  );
  mount(false);
  const targets = await screen.findAllByRole<HTMLSelectElement>("combobox");
  const [first, second] = targets;
  if (!first || !second) throw new Error("expected two episode selectors");
  expect(first.disabled).toBe(true);
  expect(first.value).toBe("t1");
  await userEvent.selectOptions(second, "t2");
  await userEvent.click(screen.getByRole("checkbox", { name: en.confirmImportMapping }));
  expect(screen.getByRole<HTMLButtonElement>("button", { name: en.resolveImport }).disabled).toBe(
    true,
  );
});
