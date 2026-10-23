import { afterEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import { en } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  location.hash = "";
});
function mount(failedLibrary = false) {
  location.hash = "discover";
  const paths: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) => {
      paths.push(path);
      if (path === "/api/library" && failedLibrary)
        return Promise.resolve(new Response("Library unavailable", { status: 503 }));
      const data =
        path === "/api/me"
          ? {
              ID: "fixture",
              Name: "Fixture",
              Admin: false,
              Settings: { language: "en", services: [] },
            }
          : path === "/api/library"
            ? [{ id: "tmdb:movie:1", base: "completed" }]
            : path.startsWith("/api/discover/")
              ? {
                  results: [{ id: 1, title: "Already watched", media_type: "movie" }],
                  total_pages: 3,
                  total_results: 500,
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
test("discovery counts only visible page matches and retains pagination when exclusions empty the page", async () => {
  mount();
  expect(await screen.findByText(`${en.pageMatches}: 1`)).toBeTruthy();
  await userEvent.click(screen.getByRole("checkbox", { name: en.excludeWatched }));
  expect(await screen.findByText(`${en.pageMatches}: 0`)).toBeTruthy();
  expect(screen.queryByRole("link", { name: /Already watched/ })).toBeNull();
  expect(screen.getByText(en.noPageMatches)).toBeTruthy();
  expect(screen.getByRole<HTMLButtonElement>("button", { name: en.nextPage }).disabled).toBe(false);
  expect(screen.getByRole<HTMLButtonElement>("button", { name: en.myServices }).disabled).toBe(
    true,
  );
});
test("watched exclusion cannot silently treat an unavailable library as unknown unwatched titles", async () => {
  mount(true);
  await screen.findByText(`${en.pageMatches}: 1`);
  await userEvent.click(screen.getByRole("checkbox", { name: en.excludeWatched }));
  expect(await screen.findByRole("alert")).toBeTruthy();
  expect(screen.queryByText(`${en.pageMatches}: 1`)).toBeNull();
  expect(screen.queryByRole("link", { name: /Already watched/ })).toBeNull();
});
test("movie cast and crew filters stay distinct and switching content type clears incompatible filters", async () => {
  const paths = mount();
  await screen.findByText(`${en.pageMatches}: 1`);
  fireEvent.change(screen.getByLabelText(en.castIDs), { target: { value: "10|11" } });
  fireEvent.change(screen.getByLabelText(en.crewIDs), { target: { value: "20" } });
  await waitFor(() =>
    expect(
      paths.some((path) => {
        const url = new URL(path, "http://localhost");
        return (
          url.searchParams.get("with_cast") === "10|11" &&
          url.searchParams.get("with_crew") === "20"
        );
      }),
    ).toBe(true),
  );
  fireEvent.change(screen.getByRole("combobox", { name: en.kind }), { target: { value: "tv" } });
  await waitFor(() => expect(paths.some((path) => path.startsWith("/api/discover/tv"))).toBe(true));
  const last = paths.filter((path) => path.startsWith("/api/discover/tv")).at(-1) ?? "";
  expect(last).not.toContain("with_cast");
  expect(last).not.toContain("with_crew");
});

test("native discovery uses the labelled source feed without silently forwarding TMDB filters", async () => {
  location.hash = "discover";
  const paths: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) => {
      paths.push(path);
      const data =
        path === "/api/me"
          ? { ID: "fixture", Name: "Fixture", Admin: false, Settings: { language: "en" } }
          : path.startsWith("/api/discover/anime")
            ? {
                results: [{ id: 41, name: "Native hot title", media_type: "anime" }],
                total_pages: 1,
                exhaustive: false,
                search_scope: "native_hot",
                fetched_at: "2026-01-01T00:00:00Z",
                stale: true,
              }
            : path.startsWith("/api/discover/")
              ? { results: [], total_pages: 1 }
              : [];
      return Promise.resolve(new Response(JSON.stringify(data)));
    }),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <App />
    </QueryClientProvider>,
  );
  fireEvent.change(await screen.findByRole("combobox", { name: en.kind }), {
    target: { value: "anime" },
  });
  expect(await screen.findByText(en.nativeHotScope, { exact: false })).toBeTruthy();
  expect(screen.getByRole("link", { name: /Native hot title/ }).getAttribute("href")).toBe(
    "#detail/anime/41",
  );
  expect(screen.queryByLabelText(en.genre)).toBeNull();
  expect(screen.queryByLabelText(en.minVotes)).toBeNull();
  const path = paths.find((path) => path.startsWith("/api/discover/anime")) ?? "";
  expect(path).not.toContain("vote_count");
  expect(path).not.toContain("sort_by");
  expect(screen.getByText(en.stale, { exact: false })).toBeTruthy();
});
