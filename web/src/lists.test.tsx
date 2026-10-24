import { afterEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ListsPage } from "./pages";
import { en } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
const manual = {
  id: "list-a",
  name: "Favorites",
  kind: "manual",
  rule: null,
  sort: "title",
  members: ["tmdb:movie:1", "tmdb:movie:2"],
  dependents: [],
};
const library = [
  {
    id: "tmdb:movie:1",
    title: "Alpha",
    metadata: { selected: { genres: [{ id: 35, name: "Comedy" }] } },
  },
  {
    id: "tmdb:movie:2",
    title: "Beta",
    metadata: { selected: { genres: [{ id: 18, name: "Drama" }] } },
  },
];
function mount() {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
        })
      }
    >
      <ListsPage
        t={(key) => en[key]}
        user={{ ID: "synthetic", Name: "Synthetic", Admin: false, Settings: { country: "TR" } }}
      />
    </QueryClientProvider>,
  );
}
function fixtures(path: string) {
  if (path === "/api/lists") return [manual];
  if (path === "/api/library") return library;
  if (path === "/api/statuses")
    return [{ id: "status-weekend", name: "Weekend watching", base: "watching", used: false }];
  if (path.endsWith("/results"))
    return {
      count: 2,
      results: [
        { id: "tmdb:movie:2", title: "Beta" },
        { id: "tmdb:movie:1", title: "Alpha" },
      ],
    };
  if (path === "/api/lists/preview")
    return { count: 1, results: [{ id: "tmdb:movie:1", title: "Alpha" }] };
  return [];
}
test("saved list contents are browsable separately from editing", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) =>
      Promise.resolve(new Response(JSON.stringify(fixtures(path)), { status: 200 })),
    ),
  );
  mount();
  await userEvent.click(await screen.findByRole("button", { name: en.viewList }));
  const beta = await screen.findByRole("link", { name: "Beta" });
  const alpha = screen.getByRole("link", { name: "Alpha" });
  expect(beta.getAttribute("href")).toBe("#detail/movie/2");
  expect(beta.compareDocumentPosition(alpha) & Node.DOCUMENT_POSITION_FOLLOWING).not.toBe(0);
  expect(screen.queryByRole("textbox", { name: en.name })).toBeNull();
});
test("smart rule selectors show names while preserving stable status and list IDs", async () => {
  const calls: { path: string; body: unknown }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      const body: unknown = typeof init?.body === "string" ? JSON.parse(init.body) : null;
      calls.push({ path, body });
      return Promise.resolve(
        new Response(
          JSON.stringify(
            path === "/api/lists" && init?.method === "POST" ? { id: "created" } : fixtures(path),
          ),
          { status: 200 },
        ),
      );
    }),
  );
  mount();
  await screen.findByText("Favorites");
  await userEvent.click(screen.getByRole("button", { name: `＋ ${en.newList}` }));
  await userEvent.type(screen.getByRole("textbox", { name: en.name }), "Smart weekend");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.kind }), "smart");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.field }), "status");
  const value = screen.getByRole("combobox", { name: en.value });
  expect(
    within(value).getByRole("option", { name: "Weekend watching" }).getAttribute("value"),
  ).toBe("status-weekend");
  await userEvent.selectOptions(value, "status-weekend");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.field }), "list");
  expect(
    within(screen.getByRole("combobox", { name: en.value }))
      .getByRole("option", { name: "Favorites" })
      .getAttribute("value"),
  ).toBe("list-a");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.value }), "list-a");
  await waitFor(() =>
    expect(screen.getByRole<HTMLButtonElement>("button", { name: en.save }).disabled).toBe(false),
  );
  await userEvent.click(screen.getByRole("button", { name: en.save }));
  await waitFor(() =>
    expect(
      calls.some(
        (call) =>
          call.path === "/api/lists" && JSON.stringify(call.body).includes('"value":"list-a"'),
      ),
    ).toBe(true),
  );
});
test("manual-list move-down control saves the chosen member order", async () => {
  const writes: unknown[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      if (path === "/api/lists" && init?.method === "POST" && typeof init.body === "string")
        writes.push(JSON.parse(init.body));
      return Promise.resolve(new Response(JSON.stringify(fixtures(path)), { status: 200 }));
    }),
  );
  mount();
  await userEvent.click(await screen.findByRole("button", { name: en.editList }));
  await userEvent.click(screen.getByRole("button", { name: `${en.moveDown}: Alpha` }));
  await userEvent.click(screen.getByRole("button", { name: en.save }));
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0]).toMatchObject({ members: ["tmdb:movie:2", "tmdb:movie:1"] });
});

test("manual drag order is announced and a cancelled drag cannot reorder a later drop", async () => {
  const writes: unknown[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      if (path === "/api/lists" && init?.method === "POST" && typeof init.body === "string")
        writes.push(JSON.parse(init.body));
      return Promise.resolve(new Response(JSON.stringify(fixtures(path)), { status: 200 }));
    }),
  );
  mount();
  await userEvent.click(await screen.findByRole("button", { name: en.editList }));
  const alpha = screen.getByText("⠿ Alpha").closest("li");
  const beta = screen.getByText("⠿ Beta").closest("li");
  if (!alpha || !beta) throw new Error("Missing ordered members");
  const dataTransfer = { setData: vi.fn(), effectAllowed: "" };
  fireEvent.dragStart(alpha, { dataTransfer });
  fireEvent.drop(beta, { dataTransfer });
  expect(screen.getByRole("status").textContent).toBe("Alpha: Position 2 / 2");
  fireEvent.dragStart(alpha, { dataTransfer });
  fireEvent.dragEnd(alpha, { dataTransfer });
  fireEvent.drop(beta, { dataTransfer });
  await userEvent.click(screen.getByRole("button", { name: en.save }));
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0]).toMatchObject({ members: ["tmdb:movie:2", "tmdb:movie:1"] });
});

test("failed list loading can be retried and an empty collection is explicit", async () => {
  let attempts = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) => {
      if (path === "/api/lists") {
        attempts++;
        return Promise.resolve(
          attempts === 1
            ? new Response("Synthetic temporary failure", { status: 503 })
            : new Response("[]", { status: 200 }),
        );
      }
      return Promise.resolve(new Response(JSON.stringify(fixtures(path)), { status: 200 }));
    }),
  );
  mount();
  const alert = await screen.findByRole("alert");
  expect(alert.textContent).toContain("Synthetic temporary failure");
  await userEvent.click(within(alert).getByRole("button", { name: en.retry }));
  expect(await screen.findByText(en.empty)).toBeTruthy();
  expect(screen.queryByRole("alert")).toBeNull();
});

test("smart preview failures have explicit retry and cannot be saved until recovered", async () => {
  let fail = true;
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) =>
      Promise.resolve(
        path === "/api/lists/preview" && fail
          ? new Response("Synthetic preview failure", { status: 503 })
          : new Response(JSON.stringify(fixtures(path)), { status: 200 }),
      ),
    ),
  );
  mount();
  await screen.findByText("Favorites");
  await userEvent.click(screen.getByRole("button", { name: `＋ ${en.newList}` }));
  await userEvent.type(screen.getByRole("textbox", { name: en.name }), "Recovered preview");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.kind }), "smart");
  const alert = await screen.findByRole("alert");
  expect(screen.getByRole<HTMLButtonElement>("button", { name: en.save }).disabled).toBe(true);
  fail = false;
  await userEvent.click(within(alert).getByRole("button", { name: en.retry }));
  await waitFor(() =>
    expect(screen.getByRole<HTMLButtonElement>("button", { name: en.save }).disabled).toBe(false),
  );
  expect(screen.getByRole("link", { name: "Alpha" })).toBeTruthy();
});

test("visual nested rules preserve OR, NOT and explicit unknown semantics when saved", async () => {
  const writes: unknown[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      if (path === "/api/lists" && init?.method === "POST" && typeof init.body === "string")
        writes.push(JSON.parse(init.body));
      return Promise.resolve(new Response(JSON.stringify(fixtures(path)), { status: 200 }));
    }),
  );
  mount();
  await screen.findByText("Favorites");
  await userEvent.click(screen.getByRole("button", { name: `＋ ${en.newList}` }));
  await userEvent.type(screen.getByRole("textbox", { name: en.name }), "Unknown or not watching");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.kind }), "smart");
  const root = screen.getByRole("button", { name: `＋ ${en.addGroup}` }).closest("fieldset");
  if (!root) throw new Error("Missing rule group");
  await userEvent.selectOptions(
    within(root).getAllByRole("combobox", { name: en.condition })[0]!,
    "or",
  );
  await userEvent.selectOptions(within(root).getByRole("combobox", { name: en.field }), "rating");
  await userEvent.selectOptions(
    within(root).getAllByRole("combobox", { name: en.condition })[1]!,
    "unknown",
  );
  expect(within(root).queryByRole("spinbutton", { name: en.value })).toBeNull();
  await userEvent.click(within(root).getByRole("button", { name: `＋ ${en.addGroup}` }));
  const groups = within(root).getAllByRole("group");
  const nested = groups[0];
  if (!nested) throw new Error("Missing nested group");
  await userEvent.selectOptions(
    within(nested).getAllByRole("combobox", { name: en.condition })[0]!,
    "not",
  );
  await waitFor(() =>
    expect(screen.getByRole<HTMLButtonElement>("button", { name: en.save }).disabled).toBe(false),
  );
  await userEvent.click(screen.getByRole("button", { name: en.save }));
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0]).toMatchObject({
    rule: {
      op: "or",
      children: [
        { op: "unknown", field: "rating" },
        { op: "not", children: [{ op: "eq", field: "base", value: "watching" }] },
      ],
    },
  });
});

test("dependent list names block deletion and smart membership retains a stable identifier", async () => {
  const writes: unknown[] = [];
  const dependent = {
    id: "smart-dependent",
    name: "High ratings",
    kind: "smart",
    rule: { op: "eq", field: "list", value: "list-a" },
    sort: "rating",
    members: [],
    dependents: [],
  };
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      if (path === "/api/lists" && init?.method === "POST" && typeof init.body === "string")
        writes.push(JSON.parse(init.body));
      return Promise.resolve(
        new Response(
          JSON.stringify(
            path === "/api/lists"
              ? [{ ...manual, dependents: [dependent.id] }, dependent]
              : fixtures(path),
          ),
          { status: 200 },
        ),
      );
    }),
  );
  mount();
  const card = (await screen.findByRole("heading", { name: "Favorites" })).closest("article");
  if (!card) throw new Error("Missing manual list");
  expect(within(card).getByRole<HTMLButtonElement>("button", { name: en.remove }).disabled).toBe(
    true,
  );
  expect(within(card).getByText(/High ratings/)).toBeTruthy();
  await userEvent.click(screen.getByRole("button", { name: `＋ ${en.newList}` }));
  await userEvent.type(screen.getByRole("textbox", { name: en.name }), "Smart reference");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.kind }), "smart");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.field }), "list");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.value }), dependent.id);
  await waitFor(() =>
    expect(screen.getByRole<HTMLButtonElement>("button", { name: en.save }).disabled).toBe(false),
  );
  await userEvent.click(screen.getByRole("button", { name: en.save }));
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0]).toMatchObject({
    rule: { op: "and", children: [{ op: "eq", field: "list", value: "smart-dependent" }] },
  });
});
