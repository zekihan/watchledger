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
test("native AniDB details and tracking keep source identity without TMDB season requests", async () => {
  location.hash = "detail/anime/42";
  const paths: string[] = [];
  const writes: Record<string, unknown>[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      paths.push(path);
      let data: unknown = [];
      if (path === "/api/me")
        data = {
          ID: "synthetic",
          Name: "Fixture",
          Admin: false,
          Settings: { language: "en", timezone: "UTC", anime_title: "romaji" },
        };
      else if (path === "/api/library")
        data = [
          {
            id: "anidb:anime:42",
            title: "Synthetic native",
            kind: "anime",
            base: "watching",
            position: 0,
            metadata: { en: { name: "Synthetic native" } },
          },
        ];
      else if (path.startsWith("/api/catalog/anime/42"))
        data = {
          id: "anidb:anime:42",
          fetched_at: "2026-01-01T00:00:00Z",
          stale: false,
          metadata: {
            id: 42,
            name: "Synthetic native",
            seasons: [{ season_number: 1, name: "Regular", episode_count: 1 }],
            episodes: [
              {
                id: 501,
                name: "Native first",
                season_number: 1,
                episode_number: 1,
                air_date: "2026-01-01",
              },
            ],
          },
        };
      else if (path.endsWith("/alternate"))
        data = { structure: "native", incomplete: true, titles: [] };
      else if (path.endsWith("/track")) {
        const body = JSON.parse(typeof init?.body === "string" ? init.body : "{}") as Record<
          string,
          unknown
        >;
        writes.push(body);
        data = { earlier: [], offer_return_watching: false };
      }
      return Promise.resolve(new Response(JSON.stringify(data), { status: 200 }));
    }),
  );
  render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
        })
      }
    >
      <App />
    </QueryClientProvider>,
  );
  expect(await screen.findByRole("heading", { name: "Native first" })).toBeTruthy();
  await userEvent.click(screen.getByRole("button", { name: new RegExp(en.watched) }));
  await waitFor(() => expect(writes.filter((x) => !x.preview)).toHaveLength(1));
  expect(writes[0]?.episode_id).toBe("anidb:episode:501");
  expect(paths.some((x) => x.includes("/catalog/tv/") || x.includes("/library/tmdb:"))).toBe(false);
  expect(screen.getByText(en.anidbNotice, { exact: false })).toBeTruthy();
});
