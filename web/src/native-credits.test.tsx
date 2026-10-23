import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import { en } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  location.hash = "";
});
function mount(page: string) {
  location.hash = page;
  const paths: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) => {
      paths.push(path);
      const data =
        path === "/api/me"
          ? { ID: "fixture", Name: "Fixture", Admin: false, Settings: { language: "en" } }
          : path.startsWith("/api/catalog/anime/")
            ? {
                metadata: {
                  id: 42,
                  name: "Synthetic anime",
                  native_tags: [{ id: "anidb:tag:1", name: "Safe source tag" }],
                  credits: {
                    cast: [
                      {
                        id: 9,
                        name: "Native voice",
                        source: "anidb",
                        profile_path: "https://cdn-eu.anidb.net/images/main/10.jpeg",
                        character: "Synthetic role",
                      },
                    ],
                    crew: [{ id: 7, name: "Native director", source: "anidb", job: "Direction" }],
                  },
                  recommendations: {
                    results: [{ id: 43, name: "Synthetic sequel", media_type: "anime" }],
                  },
                  seasons: [],
                  episodes: [],
                },
                fetched_at: "2026-01-01T00:00:00Z",
              }
            : path.startsWith("/api/anidb/creators/")
              ? {
                  id: 9,
                  name: "Native voice",
                  cached_only: true,
                  combined_credits: {
                    cast: [{ id: 42, name: "Synthetic credited anime", media_type: "anime" }],
                    crew: [],
                  },
                }
              : [];
      return Promise.resolve(new Response(JSON.stringify(data)));
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
  return paths;
}
test("AniDB cast and related works navigate with their own identities, never matching TMDB numeric IDs", async () => {
  const paths = mount("detail/anime/42");
  const voice = await screen.findByRole("link", { name: /Native voice/ });
  expect(voice.getAttribute("href")).toBe("#creator/9");
  expect(screen.getByRole("link", { name: /Native director/ }).getAttribute("href")).toBe(
    "#creator/7",
  );
  expect(screen.getByRole("link", { name: /Synthetic sequel/ }).getAttribute("href")).toBe(
    "#detail/anime/43",
  );
  expect(screen.getByText(en.nativeTagsHint)).toBeTruthy();
  expect(
    paths.some((path) => path.includes("/catalog/person/") || path.includes("/catalog/tv/")),
  ).toBe(false);
});
test("native creator credits are explicitly cached-only with source-safe anime links", async () => {
  const paths = mount("creator/9");
  expect(await screen.findByRole("heading", { name: "Native voice" })).toBeTruthy();
  expect(screen.getByText(en.nativeCreatorScope)).toBeTruthy();
  expect(screen.getByRole("link", { name: /Synthetic credited anime/ }).getAttribute("href")).toBe(
    "#detail/anime/42",
  );
  expect(screen.getByRole("option", { name: en.anime })).toBeTruthy();
  expect(paths).toContain("/api/anidb/creators/9");
  expect(paths.some((path) => path.includes("/catalog/person/"))).toBe(false);
});
