import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SimklPanel } from "./imports";
import { PortabilityPanel } from "./pages";
import { en } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
function mount(backed: boolean) {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
        })
      }
    >
      <SimklPanel t={(key) => en[key]} backed={backed} />
    </QueryClientProvider>,
  );
}
function file(name: string, contents: string) {
  const f = new File([contents], name, { type: "application/json" });
  Object.defineProperty(f, "text", { value: () => Promise.resolve(contents) });
  return f;
}
const preview = {
  id: "synthetic",
  additions: 1,
  conflicts: 0,
  unresolved: 2,
  warnings: ["custom_lists_absent_from_backup"],
  items: [],
};
test("SIMKL requires preview and explicit backed-up application", async () => {
  const fetch = vi.fn((input: string) =>
    Promise.resolve(
      new Response(
        JSON.stringify(
          input.endsWith("/preview")
            ? preview
            : input.endsWith("/apply")
              ? { status: "applied", reviews: 2 }
              : [],
        ),
        { status: 200 },
      ),
    ),
  );
  vi.stubGlobal("fetch", fetch);
  mount(true);
  expect(screen.queryByRole("button", { name: en.apply })).toBeNull();
  await userEvent.upload(screen.getByLabelText(en.simklFile), file("source.json", '{"movies":[]}'));
  const apply = await screen.findByRole("button", { name: en.apply });
  expect(fetch.mock.calls.some(([path]) => path.endsWith("/apply"))).toBe(false);
  await userEvent.click(apply);
  expect(await screen.findByText(en.simklApplied)).toBeTruthy();
  expect(fetch.mock.calls.filter(([path]) => path.endsWith("/apply"))).toHaveLength(1);
  expect(screen.queryByRole("button", { name: en.apply })).toBeNull();
});
test("backup is offered inline and invalid replacement cannot apply stale preview", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn((input: string) =>
      Promise.resolve(
        new Response(JSON.stringify(input.endsWith("/preview") ? preview : []), { status: 200 }),
      ),
    ),
  );
  mount(false);
  await userEvent.upload(screen.getByLabelText(en.simklFile), file("source.json", '{"movies":[]}'));
  expect(
    (await screen.findByRole<HTMLButtonElement>("button", { name: en.importBackupAndApply }))
      .disabled,
  ).toBe(false);
  await userEvent.upload(screen.getByLabelText(en.simklFile), file("invalid.json", "not json"));
  await waitFor(() => expect(screen.queryByRole("button", { name: en.apply })).toBeNull());
  expect(await screen.findByRole("alert")).toBeTruthy();
});
test("older preview response cannot replace newer uploaded backup", async () => {
  let resolveOld: ((value: Response) => void) | undefined;
  let calls = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn((input: string) => {
      if (input.endsWith("/preview")) {
        calls++;
        if (calls === 1)
          return new Promise<Response>((resolve) => {
            resolveOld = resolve;
          });
        return Promise.resolve(
          new Response(JSON.stringify({ ...preview, id: "new", additions: 9 }), { status: 200 }),
        );
      }
      return Promise.resolve(new Response("[]", { status: 200 }));
    }),
  );
  mount(true);
  await userEvent.upload(screen.getByLabelText(en.simklFile), file("old.json", '{"movies":[]}'));
  await userEvent.upload(screen.getByLabelText(en.simklFile), file("new.json", '{"shows":[]}'));
  expect(await screen.findByText(/9 ·/)).toBeTruthy();
  resolveOld?.(new Response(JSON.stringify(preview), { status: 200 }));
  await waitFor(() => expect(screen.getByText(/9 ·/)).toBeTruthy());
});

test("restore discards an earlier valid preview when a replacement file is invalid", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn((input: string) =>
      Promise.resolve(
        new Response(
          JSON.stringify(
            input === "/api/restore/preview"
              ? { id: "restore", additions: 1, conflicts: [], history_candidates: 1 }
              : [],
          ),
          { status: 200 },
        ),
      ),
    ),
  );
  render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
        })
      }
    >
      <PortabilityPanel t={(key) => en[key]} />
    </QueryClientProvider>,
  );
  await userEvent.click(screen.getByRole("checkbox", { name: en.backupSaved }));
  await userEvent.upload(screen.getByLabelText(en.restore), file("valid.json", '{"version":1}'));
  expect((await screen.findByRole<HTMLButtonElement>("button", { name: en.apply })).disabled).toBe(
    false,
  );
  await userEvent.upload(screen.getByLabelText(en.restore), file("bad.json", "not json"));
  await waitFor(() => expect(screen.queryByRole("button", { name: en.apply })).toBeNull());
  expect(await screen.findByRole("alert")).toBeTruthy();
});

test("large previews and unresolved queues are paged without truncating the import", async () => {
  const writes: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      if (init?.method === "POST") writes.push(path);
      const data = path.endsWith("/preview")
        ? {
            ...preview,
            items: Array.from({ length: 51 }, (_, i) => ({
              key: `item-${i}`,
              title: `Source ${i}`,
              action: "review",
              issues: [],
              warnings: [],
            })),
          }
        : path === "/api/import-reviews"
          ? Array.from({ length: 51 }, (_, i) => ({
              id: `review-${i}`,
              reason: "simkl_source_review",
              item: { source: { title: `Review ${i}` } },
            }))
          : [];
      return Promise.resolve(new Response(JSON.stringify(data), { status: 200 }));
    }),
  );
  mount(true);
  await screen.findByText("Review 0");
  expect(screen.queryByText("Review 50")).toBeNull();
  await userEvent.click(
    within(screen.getByRole("navigation", { name: en.unresolved })).getByRole("button", {
      name: en.nextPage,
    }),
  );
  expect(screen.getByText("Review 50")).toBeTruthy();
  await userEvent.upload(screen.getByLabelText(en.simklFile), file("large.json", '{"movies":[]}'));
  await screen.findByRole("button", { name: en.apply });
  await userEvent.click(screen.getByText(`${en.sourceItems} (51)`));
  expect(screen.getByText(/Source 0 ·/)).toBeTruthy();
  expect(screen.queryByText(/Source 50 ·/)).toBeNull();
  await userEvent.click(
    within(screen.getByRole("navigation", { name: en.sourceItems })).getByRole("button", {
      name: en.nextPage,
    }),
  );
  expect(screen.getByText(/Source 50 ·/)).toBeTruthy();
  expect(screen.getByText(en.importAllPages)).toBeTruthy();
  expect(writes).toEqual(["/api/simkl/preview"]);
  await userEvent.click(screen.getByRole("button", { name: en.apply }));
  await waitFor(() => expect(writes).toEqual(["/api/simkl/preview", "/api/simkl/synthetic/apply"]));
});

test("unsigned shared metadata needs explicit administrator approval and resets on replacement", async () => {
  const writes: Record<string, unknown>[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      if (path.endsWith("/apply"))
        writes.push(
          JSON.parse(typeof init?.body === "string" ? init.body : "{}") as Record<string, unknown>,
        );
      return Promise.resolve(
        new Response(
          JSON.stringify(
            path.endsWith("/preview")
              ? {
                  id: "unsigned",
                  additions: 1,
                  conflicts: [],
                  history_candidates: 0,
                  shared_metadata_requires_approval: true,
                  can_approve_shared_metadata: true,
                }
              : [],
          ),
          { status: 200 },
        ),
      );
    }),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <PortabilityPanel t={(key) => en[key]} />
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  await user.click(screen.getByRole("checkbox", { name: en.backupSaved }));
  await user.upload(screen.getByLabelText(en.restore), file("unsigned.json", '{"version":1}'));
  const apply = await screen.findByRole<HTMLButtonElement>("button", {
    name: en.apply,
  });
  expect(apply.disabled).toBe(true);
  await user.click(screen.getByRole("checkbox", { name: en.sharedRestoreApprove }));
  expect(apply.disabled).toBe(false);
  await user.upload(
    screen.getByLabelText(en.restore),
    file("replacement.json", '{"version":1,"created_at":"changed"}'),
  );
  await waitFor(() =>
    expect(screen.getByRole<HTMLButtonElement>("button", { name: en.apply }).disabled).toBe(true),
  );
  await user.click(screen.getByRole("checkbox", { name: en.sharedRestoreApprove }));
  await user.click(screen.getByRole("button", { name: en.apply }));
  await waitFor(() => expect(writes).toEqual([{ trust_shared_metadata: true }]));
});
test("ordinary accounts cannot approve unknown shared metadata", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) =>
      Promise.resolve(
        new Response(
          JSON.stringify(
            path.endsWith("/preview")
              ? {
                  id: "unsigned",
                  additions: 1,
                  conflicts: [],
                  history_candidates: 0,
                  shared_metadata_requires_approval: true,
                  can_approve_shared_metadata: false,
                }
              : [],
          ),
          { status: 200 },
        ),
      ),
    ),
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <PortabilityPanel t={(key) => en[key]} />
    </QueryClientProvider>,
  );
  await userEvent.click(screen.getByRole("checkbox", { name: en.backupSaved }));
  await userEvent.upload(screen.getByLabelText(en.restore), file("unsigned.json", '{"version":1}'));
  expect(await screen.findByText(en.sharedRestoreBlocked)).toBeTruthy();
  expect(screen.queryByRole("checkbox", { name: en.sharedRestoreApprove })).toBeNull();
  expect(screen.getByRole<HTMLButtonElement>("button", { name: en.apply }).disabled).toBe(true);
});

test("restore names incompatible custom statuses and blocks apply", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) =>
      Promise.resolve(
        new Response(
          JSON.stringify(
            path.endsWith("/preview")
              ? {
                  id: "collision",
                  additions: 0,
                  conflicts: [],
                  history_candidates: 0,
                  status_conflicts: ["My watching"],
                }
              : [],
          ),
          { status: 200 },
        ),
      ),
    ),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <PortabilityPanel t={(key) => en[key]} />
    </QueryClientProvider>,
  );
  await userEvent.click(screen.getByRole("checkbox", { name: en.backupSaved }));
  await userEvent.upload(
    screen.getByLabelText(en.restore),
    file("collision.json", '{"version":1}'),
  );
  expect((await screen.findByRole("alert")).textContent).toContain("My watching");
  expect(screen.getByRole<HTMLButtonElement>("button", { name: en.apply }).disabled).toBe(true);
});

test.each(["preview", "applied"])(
  "saved SIMKL %s archives prepare in batches and require explicit episode approval",
  async (state) => {
    const bodies: { path: string; body: unknown }[] = [];
    let prepared = 0;
    const initial = { ...preview, preparation_total: 8 };
    const ready = {
      ...initial,
      unresolved: 0,
      episode_matches: 2,
      episode_match_digest: "fixture-proposal-version",
      items: [
        {
          key: "show",
          title: "Synthetic series",
          action: "add",
          issues: [],
          warnings: [],
          matches: [
            {
              source_key: "source-1",
              episode_id: "tmdb:episode:1",
              season: 1,
              number: 1,
              title: "Pilot",
            },
            {
              source_key: "source-3",
              episode_id: "tmdb:episode:3",
              season: 1,
              number: 3,
              title: "Third episode",
            },
          ],
        },
      ],
    };
    vi.stubGlobal(
      "fetch",
      vi.fn((path: string, init?: RequestInit) => {
        if (init?.method === "POST")
          bodies.push({ path, body: JSON.parse(init.body as string) as unknown });
        let data: unknown = [];
        if (path === "/api/imports") data = [{ id: "synthetic", kind: "simkl", state }];
        if (path.endsWith("/preview")) data = initial;
        if (path.endsWith("/prepare"))
          data =
            ++prepared === 1
              ? { next: 6, total: 8, paused: false }
              : { next: 8, total: 8, paused: false, preview: ready };
        if (path.endsWith("/apply")) data = { status: "applied", reviews: 0 };
        return Promise.resolve(new Response(JSON.stringify(data), { status: 200 }));
      }),
    );
    mount(true);
    await userEvent.click(await screen.findByRole("button", { name: en.importContinue }));
    expect(await screen.findByText(en.importConfirmEpisodes)).toBeTruthy();
    expect(bodies.filter((row) => row.path.endsWith("/prepare")).map((row) => row.body)).toEqual([
      { offset: 0 },
      { offset: 6 },
    ]);
    expect(bodies.some((row) => row.path.endsWith("/apply"))).toBe(false);
    expect(
      screen.getByRole<HTMLInputElement>("checkbox", { name: en.importConfirmEpisodes }).checked,
    ).toBe(false);
    await userEvent.click(screen.getByRole("checkbox", { name: en.importConfirmEpisodes }));
    await userEvent.click(screen.getByRole("button", { name: en.apply }));
    await waitFor(() =>
      expect(bodies.find((row) => row.path.endsWith("/apply"))?.body).toEqual({
        confirm_episode_matches: true,
        episode_match_digest: "fixture-proposal-version",
      }),
    );
  },
);

test("SIMKL downloads a backup before applying and stops if backup creation fails", async () => {
  const requests: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) => {
      requests.push(path);
      return Promise.resolve(
        new Response(
          path === "/api/export"
            ? "backup unavailable"
            : JSON.stringify(path.endsWith("/preview") ? preview : []),
          { status: path === "/api/export" ? 503 : 200 },
        ),
      );
    }),
  );
  mount(false);
  await userEvent.upload(screen.getByLabelText(en.simklFile), file("source.json", '{"movies":[]}'));
  await userEvent.click(await screen.findByRole("button", { name: en.importBackupAndApply }));
  expect(await screen.findByRole("alert")).toHaveProperty("textContent", "backup unavailable");
  expect(requests).toContain("/api/export");
  expect(requests.some((path) => path.endsWith("/apply"))).toBe(false);
});

test("SIMKL backup download precedes import and unresolved reviews receive focus", async () => {
  const requests: string[] = [];
  const createURL = vi.fn(() => "blob:synthetic-backup"),
    revokeURL = vi.fn();
  vi.stubGlobal(
    "URL",
    Object.assign(class extends URL {}, { createObjectURL: createURL, revokeObjectURL: revokeURL }),
  );
  const download = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) => {
      requests.push(path);
      let data: unknown = [];
      if (path.endsWith("/preview")) data = preview;
      if (path.endsWith("/apply")) data = { status: "applied", reviews: 1 };
      if (path === "/api/import-reviews" && requests.some((item) => item.endsWith("/apply")))
        data = [
          {
            id: "review",
            import_id: "synthetic",
            reason: "simkl_source_review",
            item: { source: { title: "Exception" } },
          },
        ];
      return Promise.resolve(new Response(JSON.stringify(data), { status: 200 }));
    }),
  );
  mount(false);
  await userEvent.upload(screen.getByLabelText(en.simklFile), file("source.json", '{"movies":[]}'));
  await userEvent.click(await screen.findByRole("button", { name: en.importBackupAndApply }));
  await screen.findByText(en.simklApplied);
  expect(download).toHaveBeenCalledOnce();
  expect(requests.indexOf("/api/export")).toBeLessThan(
    requests.indexOf("/api/simkl/synthetic/apply"),
  );
  expect(document.activeElement?.textContent).toContain(en.unresolved);
  download.mockRestore();
});
