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
  catalog_cached: true,
  seasons: [],
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

test("review displays incoming versus existing values and explicit unwatch decisions", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(() =>
      Promise.resolve(
        new Response(
          JSON.stringify({
            ...data,
            source: { ...data.source, base: "completed", rating: 7 },
            current: { base: "watching", rating: 9, status_name: "My active queue" },
            episodes: [{ ...data.episodes[0], explicit_unwatch: true }, data.episodes[1]],
          }),
          { status: 200 },
        ),
      ),
    ),
  );
  mount();
  expect(await screen.findByText(en.incomingSource)).toBeTruthy();
  expect(screen.getByText(/My active queue/)).toBeTruthy();
  const targets = screen.getAllByRole<HTMLSelectElement>("combobox");
  const first = targets[0];
  if (!first) throw new Error("missing episode selector");
  await userEvent.selectOptions(first, "t1");
  expect(screen.getByText(en.explicitUnwatch)).toBeTruthy();
  expect(screen.getByRole("link", { name: en.openSourceTitle }).getAttribute("href")).toBe(
    "#detail/tv/1",
  );
});

test("metadata loading is separate from importing and target seasons have no inferred selection", async () => {
  let loaded = false;
  let seasonLoaded = false;
  const fetch = vi.fn((path: string, init?: RequestInit) => {
    if (path.endsWith("/catalog")) {
      loaded = true;
      if (init?.body === '{"season":2}') seasonLoaded = true;
      return Promise.resolve(new Response('{"stale":false}', { status: 200 }));
    }
    return Promise.resolve(
      new Response(
        JSON.stringify({
          ...data,
          catalog_cached: loaded,
          seasons: loaded ? [2] : [],
          episodes: seasonLoaded ? data.episodes : [],
        }),
        { status: 200 },
      ),
    );
  });
  vi.stubGlobal("fetch", fetch);
  mount();
  await userEvent.click(await screen.findByRole("button", { name: en.loadImportMetadata }));
  expect(await screen.findByText(en.catalogLoadedNoHistory)).toBeTruthy();
  const season = await screen.findByRole<HTMLSelectElement>("combobox", { name: en.targetSeason });
  expect(season.value).toBe("");
  const loadSeason = screen.getByRole<HTMLButtonElement>("button", { name: en.loadImportSeason });
  expect(loadSeason.disabled).toBe(true);
  await userEvent.selectOptions(season, "2");
  await userEvent.click(loadSeason);
  expect(fetch.mock.calls.filter(([path]) => path.endsWith("/resolve"))).toHaveLength(0);
  expect(
    fetch.mock.calls.some(
      ([path, init]) => path.endsWith("/catalog") && init?.body === '{"season":2}',
    ),
  ).toBe(true);
});

test("failed source review can be retried without resolving or applying it", async () => {
  let attempts = 0;
  const paths: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) => {
      paths.push(path);
      attempts++;
      return Promise.resolve(
        attempts === 1
          ? new Response("Synthetic source outage", { status: 503 })
          : new Response(JSON.stringify(data), { status: 200 }),
      );
    }),
  );
  mount();
  expect(await screen.findByRole("alert")).toBeTruthy();
  await userEvent.click(screen.getByRole("button", { name: en.retry }));
  expect(await screen.findByText("Synthetic gaps")).toBeTruthy();
  expect(paths).toEqual([
    "/api/import-reviews/review/candidates",
    "/api/import-reviews/review/candidates",
  ]);
});
