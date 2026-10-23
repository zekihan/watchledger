import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { CustomListImport } from "./custom-list-import";
import { en } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
const preview = {
  id: "synthetic",
  source: { id: "42", name: "Synthetic list", warnings: [] },
  members: [{ key: "movies:1", title: "A", catalog_id: "tmdb:movie:1" }],
  action: "create_manual_snapshot",
  unresolved: 0,
  existing_list_id: "",
};
function mount(backed: boolean) {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
        })
      }
    >
      <CustomListImport t={(key) => en[key]} backed={backed} />
    </QueryClientProvider>,
  );
}
function file(name: string, value: unknown) {
  const text = JSON.stringify(value);
  const f = new File([text], name, { type: "application/json" });
  Object.defineProperty(f, "text", { value: () => Promise.resolve(text) });
  return f;
}
test("custom-list pages are bundled without applying or sending credentials", async () => {
  const calls: { path: string; body: unknown }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      calls.push({ path, body: typeof init?.body === "string" ? JSON.parse(init.body) : null });
      return Promise.resolve(
        new Response(JSON.stringify(path.endsWith("/preview") ? preview : { status: "created" }), {
          status: 200,
        }),
      );
    }),
  );
  mount(true);
  await userEvent.upload(screen.getByLabelText(en.customListFiles), [
    file("page2.json", { id: 42, pagination: { page: 2 } }),
    file("page1.json", { id: 42, pagination: { page: 1 } }),
  ]);
  const apply = await screen.findByRole("button", { name: en.importManualSnapshot });
  expect(calls[0]?.body).toEqual([
    { id: 42, pagination: { page: 2 } },
    { id: 42, pagination: { page: 1 } },
  ]);
  expect(calls.filter((call) => call.path.endsWith("/apply"))).toHaveLength(0);
  await userEvent.click(apply);
  expect(await screen.findByText(en.customListCreated)).toBeTruthy();
  expect(calls.filter((call) => call.path.endsWith("/apply"))).toHaveLength(1);
});
test("backup prerequisite and premium-only failure cannot apply a stale list preview", async () => {
  let calls = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn(() => {
      calls++;
      return Promise.resolve(
        calls === 1
          ? new Response(JSON.stringify(preview), { status: 200 })
          : new Response("SIMKL returned premium-only, not list data", { status: 400 }),
      );
    }),
  );
  mount(false);
  await userEvent.upload(screen.getByLabelText(en.customListFiles), file("list.json", { id: 42 }));
  expect(
    (await screen.findByRole<HTMLButtonElement>("button", { name: en.importManualSnapshot }))
      .disabled,
  ).toBe(true);
  await userEvent.upload(
    screen.getByLabelText(en.customListFiles),
    file("premium.json", { error: "premium_only" }),
  );
  expect(await screen.findByRole("alert")).toBeTruthy();
  await waitFor(() =>
    expect(screen.queryByRole("button", { name: en.importManualSnapshot })).toBeNull(),
  );
});
test("existing app deletion is explicit and results are reported as retained rather than imported", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) =>
      Promise.resolve(
        new Response(
          JSON.stringify(
            path.endsWith("/preview")
              ? { ...preview, action: "preserve_existing_app_deletion" }
              : { status: "retained_for_review" },
          ),
          { status: 200 },
        ),
      ),
    ),
  );
  mount(true);
  await userEvent.upload(screen.getByLabelText(en.customListFiles), file("list.json", { id: 42 }));
  expect(await screen.findByText(en.keepDeletedList)).toBeTruthy();
  await userEvent.click(screen.getByRole("button", { name: en.archiveForReview }));
  expect(await screen.findByText(en.customListRetained)).toBeTruthy();
  expect(screen.queryByText(en.customListCreated)).toBeNull();
});
