import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import { en } from "./i18n";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  location.hash = "";
});

test("manual TV refresh replaces the displayed cached episodes", async () => {
  location.hash = "detail/tv/73";
  let refreshed = false;
  const metadata = {
    id: 73,
    name: "Cached show",
    seasons: [{ season_number: 1, name: "One", episode_count: 1 }],
  };
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) => {
      let data: unknown = [];
      if (path === "/api/me")
        data = { ID: "fixture", Name: "Fixture", Admin: false, Settings: { language: "en" } };
      else if (path.startsWith("/api/catalog/tv/73/seasons/"))
        data = {
          episodes: [
            {
              id: refreshed ? 732 : 731,
              name: refreshed ? "New episode" : "Cached episode",
              season_number: 1,
              episode_number: 1,
            },
          ],
        };
      else if (path.startsWith("/api/catalog/tv/73")) {
        if (path.includes("/refresh")) refreshed = true;
        data = { metadata, fetched_at: "2026-01-01T00:00:00Z", stale: false };
      }
      return Promise.resolve(new Response(JSON.stringify(data), { status: 200 }));
    }),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <App />
    </QueryClientProvider>,
  );
  expect(await screen.findByRole("link", { name: "Cached episode" })).toBeTruthy();
  await userEvent.click(screen.getByRole("button", { name: en.refresh }));
  expect(await screen.findByRole("link", { name: "New episode" })).toBeTruthy();
  expect(screen.queryByRole("link", { name: "Cached episode" })).toBeNull();
});

test("whole-show unwatch needs no uncached season during a provider outage", async () => {
  location.hash = "detail/tv/74";
  const paths: string[] = [];
  const actions: Record<string, unknown>[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      paths.push(path);
      let data: unknown = [];
      if (path === "/api/me")
        data = { ID: "fixture", Name: "Fixture", Admin: false, Settings: { language: "en" } };
      else if (path === "/api/library")
        data = [
          { id: "tmdb:tv:74", kind: "tv", base: "watching", title: "Cached show", metadata: {} },
        ];
      else if (path.includes("/seasons/0"))
        return Promise.resolve(new Response("Provider unavailable", { status: 503 }));
      else if (path.includes("/seasons/1"))
        data = {
          episodes: [{ id: 741, name: "Cached episode", season_number: 1, episode_number: 1 }],
        };
      else if (path.startsWith("/api/catalog/tv/74?"))
        data = {
          metadata: {
            id: 74,
            name: "Cached show",
            seasons: [
              { season_number: 0, name: "Uncached specials", episode_count: 1 },
              { season_number: 1, name: "Cached season", episode_count: 1 },
            ],
          },
          fetched_at: "2026-01-01T00:00:00Z",
          stale: true,
        };
      else if (path.endsWith("/track")) {
        const body = JSON.parse(typeof init?.body === "string" ? init.body : "{}") as Record<
          string,
          unknown
        >;
        actions.push(body);
        data = { earlier: [], offer_return_watching: false };
      } else if (path.endsWith("/alternate"))
        data = { structure: "native", incomplete: true, titles: [] };
      return Promise.resolve(new Response(JSON.stringify(data), { status: 200 }));
    }),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <App />
    </QueryClientProvider>,
  );
  await userEvent.click(await screen.findByRole("button", { name: en.unwatchShow }));
  await waitFor(() =>
    expect(
      actions.some(
        (action) => action.whole === true && action.watched === false && action.preview !== true,
      ),
    ).toBe(true),
  );
  expect(paths.some((path) => path.includes("/seasons/0"))).toBe(false);
});

test("manual refresh retains the provider warning and original timestamp until a successful retry", async () => {
  location.hash = "detail/movie/72";
  const cached = {
    metadata: { id: 72, title: "Cached movie" },
    fetched_at: "2026-01-01T00:00:00Z",
    stale: false,
  };
  let refreshes = 0;
  let reads = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) => {
      let data: unknown = [];
      if (path === "/api/me")
        data = { ID: "fixture", Name: "Fixture", Admin: false, Settings: { language: "en" } };
      else if (path.startsWith("/api/catalog/movie/72/refresh")) {
        refreshes++;
        data =
          refreshes === 1
            ? { ...cached, stale: true, warning: "Provider unavailable; cached metadata retained" }
            : {
                ...cached,
                metadata: { id: 72, title: "Updated movie" },
                fetched_at: "2026-01-02T00:00:00Z",
              };
      } else if (path.startsWith("/api/catalog/movie/72?")) {
        reads++;
        data = cached;
      }
      return Promise.resolve(new Response(JSON.stringify(data), { status: 200 }));
    }),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <App />
    </QueryClientProvider>,
  );
  expect(await screen.findByRole("heading", { name: "Cached movie", level: 1 })).toBeTruthy();
  await userEvent.click(screen.getByRole("button", { name: en.refresh }));
  expect(await screen.findByText("Provider unavailable; cached metadata retained")).toBeTruthy();
  expect(screen.getByText(new RegExp(en.stale))).toBeTruthy();
  expect(reads).toBe(1);
  await userEvent.click(screen.getByRole("button", { name: en.refresh }));
  expect(await screen.findByRole("heading", { name: "Updated movie", level: 1 })).toBeTruthy();
  expect(screen.queryByText("Provider unavailable; cached metadata retained")).toBeNull();
  expect(screen.queryByText(new RegExp(en.stale))).toBeNull();
  expect(reads).toBe(1);
});
